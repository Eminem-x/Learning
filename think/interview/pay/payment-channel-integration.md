# 各支付渠道接入说明

> 状态：草案
> 日期：2026-07-30
> 适用范围：hertz_pay 已接入的五个渠道
> 配套文档：[三方支付对接后端链路规范](./third-party-payment-integration.md)（讲通用规范和范式，本文讲各渠道的具体差异）

---

## 0. 怎么读这份文档

上面那份规范讲的是"应该怎么做"，这份讲的是"每个渠道实际是怎么接的"。每个渠道一节，统一按下面的顺序展开：

1. **这是什么渠道** — 定位、市场、结算币种
2. **接入前提** — 需要哪些凭据、怎么初始化
3. **平台侧处理流程** — 用户在渠道那边经历了什么，交易状态怎么流转
4. **本项目怎么调** — 暴露的接口、调的 API、参数要点
5. **结果回流** — 回调还是轮询、验签方式、状态映射
6. **特有能力与缺口**

先看第 1 节的共享层——五个渠道共用同一套入参和订单模型，差异从渠道适配层才开始。

---

## 1. 共享层：所有渠道之上的统一部分

### 1.1 统一入参 CheckoutReq

五个渠道的建单接口**共用同一个请求结构**（`idl/payment.thrift:100-119`），这是"聚合"体现得最实在的地方：

| 字段 | 说明 |
|---|---|
| `user_id` | 从请求头 `x-biyi-user-id` 取，不在 body 里 |
| `order_id` | **必填**。上游的业务订单号，本项目直接拿它当渠道侧的商户单号 |
| `currency` | **必填**。上游报价币种（下单币种） |
| `line_item[]` | **必填**。商品明细，`price` 为最小货币单位整数 |
| `success_url` / `cancel_url` | 支付完成/取消后的跳转地址 |
| `payment_method` | 可选，取值如 `wechatpay`、`alipaycn`。**只有 Airwallex 读这个字段**，用于路由到本地支付方式 |
| `email` | 可选，用于预填 Stripe Checkout |
| `line_item[].price_id` | 可选，预创建的 Stripe Price ID，设了就忽略 `price`/`name` |

出参 `CheckoutResp`（`:127-136`）也是共享的，但**不同渠道填的字段不同**——这是需要注意的地方：

- `link` — 支付宝/微信填二维码内容，Stripe/UseePay 填托管收银台 URL
- `payment_secret` — Airwallex 和 Stripe PaymentIntent 填 client_secret，前端要用它挂载支付组件
- `total_amount` — 换汇后的实付金额（目标币种）
- `rate` — 汇率快照

所以上游拿到 `CheckoutResp` 后**必须按渠道分别处理**：拿 `link` 去渲染二维码、还是跳转、还是用 `payment_secret` 初始化 SDK，取决于用的哪个渠道。共享 DTO 只统一了传输结构，没统一消费方式。

### 1.2 统一订单模型

不管哪个渠道，建单最终都落到同一张 `order` 表，关键字段（`idl/payment.thrift:44-60`）：

- `order_id` — 上游业务单号
- `out_trade_no` — 渠道侧商户单号
- `sessions_id` — 渠道侧交易号
- `from_amount` / `from_currency` — 下单币种和金额
- `to_amount` / `to_currency` — 实际支付币种和金额
- `rate` — 汇率字符串快照
- `snapshot` — 下单时的完整请求快照

### 1.3 商户单号设计：微信是唯一的例外

`out_trade_no` 的取值方式，五个渠道里有四个一致（`biz/dal/order_dao.go`）：

| 渠道 | `out_trade_no` | `sessions_id` |
|---|---|---|
| 支付宝 | `order_id`（:302） | 支付宝 `trade_no`，回流时回填 |
| Stripe | `order_id`（:334） | Checkout Session ID，建单时即存（:320） |
| Airwallex | `order_id`（:366） | PaymentIntent ID，建单时即存（:352） |
| UseePay | `order_id`（:271） | — |
| **微信** | **`order_id` + "_" + 当前 Unix 时间戳**（:240） | — |

⚠️ **微信这个设计破坏了渠道侧幂等。** `biz/adapter/wechatpay/wechatpay.go:24` 里 `OutTradeNo: core.String(fmt.Sprintf("%s_%d", orderID, time.Now().Unix()))` ——每次建单生成的商户单号都不同。后果是：同一个 `order_id` 重复建单时，微信侧会**积累多笔独立的待支付交易**，而不是像其他渠道那样因商户单号重复被拒。

结合本项目"先调三方后落库"的顺序（见规范 §5.2），这个风险被放大：建单超时重试几次，微信侧就多出几笔活的待支付交易，用户可能付了本地没记录的那一笔。

### 1.4 汇率：只有支付宝和微信需要

`biz/domain/calculate/calculate.go:35-66` 按渠道分两条路：

- **Stripe / Airwallex / UseePay** — 1:1，不换汇，`to_currency = from_currency`
- **支付宝 / 微信** — 强制换成 CNY，走 `biz/adapter/exchangerate`

换汇这条路有几个需要知道的事实：

**汇率源是欧洲央行的每日参考汇率**（`exchangerate.go:14`，`https://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml`）。这带来三个后果：

1. **一天只更新一次**（欧洲中部时间下午 4 点左右发布），周末和欧洲节假日不发布，拿到的是上一个工作日的汇率
2. **是参考汇率，不含买卖差价**。收单机构按自己的汇率结算，所以你按 ECB 算出的 CNY 金额和支付宝实际结算金额会有差异——这是汇损来源，也是对账时"金额不一致"的常见原因
3. **以 EUR 为基准**，USD→CNY 要经过 EUR 中转（`exchangerate.go:77` 的 `value / fromRate * toRate`）

关于精度，实测结论比预想的好，但有一处潜在脆弱：ECB 发布的汇率只有 4~5 位小数，所以 `fmt.Sprintf("%f:%f")` 的 6 位小数格式**当前是无损的**——落库金额（用全精度汇率算）和上游按 `rate` 字段反算的金额在常见金额区间内完全一致，同币种往返（CNY→CNY）也无偏差。但这依赖于汇率源的精度恰好 ≤6 位；一旦换成返回更高精度的商业汇率 API，`%f` 就会截断，届时"应用的汇率"和"上报的汇率"会出现分叉。

真正需要修的是 `RateStr2Float`（`biz/domain/calculate/calculate.go:93-102`）的健壮性：`strings.Split(str, ":")` 不判长度（下标越界 panic）、`1/fromRateFloat` 不判零（可得 +Inf/NaN）、`ParseFloat` 的 error 被丢弃。

⚠️ **没有同币种短路。** `ProductPrice` 对支付宝/微信一律走换汇分支，所以即使调用方传的 `currency` 已经是 `CNY`，仍然会执行一次 `ConvertRate("CNY", "CNY")`——数学上是恒等变换，但照样要发一次跨境 HTTP 请求。

⚠️ **完全没有缓存。** `ConvertRate` 每次都调 `ExchangeRates()`，每次都发一个跨境 HTTP 请求（5 秒超时）。而数据一天只变一次。这意味着：

- 每笔支付宝/微信建单都要同步等一次对 ECB 的请求，ECB 慢或挂了建单就失败
- `/api/v1/payment/exchangerate/query` 和 `/convert` 这两个**免认证**的公开接口也直接打 ECB（`biz/handler/exchangerate/service/exchangerate_service.go:12,36`），任何人都能无认证地放大这个出站流量

这里应该加一层 Redis 缓存，TTL 设几小时即可。

### 1.5 环境隔离：四个渠道里三个做对了

| 渠道 | local / pre | prod | 评价 |
|---|---|---|---|
| Stripe | `sk_test_…` | `sk_live_…` | ✅ 正确隔离 |
| Airwallex | `api-demo.airwallex.com` | 生产域名 | ✅ 正确隔离 |
| UseePay | `pay-gateway1.uat.useepay.com` | **同样是 uat** | ❌ 生产也指向测试网关 |
| 支付宝 | `is_production: true` + 生产 app_id | 同一套 | ❌ 沙箱配置被注释掉（`config/local.yaml:57-60`） |
| 微信 | 三环境完全相同的商户号与证书 | 同一套 | ⚠️ 微信 APIv3 已下线沙箱，客观上无法隔离 |

两条需要处理：

**支付宝本地和预发直连生产。** `config/local.yaml:57-60` 里沙箱的 app_id 和测试密钥路径都被注释了，实际生效的是生产 app_id + `is_production: true`。也就是说在本地跑测试会在真实支付宝上建单。沙箱配置还在文件里，取消注释即可恢复。

**UseePay 生产环境指向 UAT 网关。** 三个环境的 `domain` 都是 `pay-gateway1.uat.useepay.com`。结合 UseePay 的 webhook 是空实现、且没有轮询分支（见 §6），这个渠道基本可以判定**没有在生产真实使用**。

**微信的生产 `notify_url` 指向预发域名。** `config/prod.yaml:36` 是 `https://pay-pre.be.service.aws.datanebula.work/api/v1/payment/wechatpay/webhook`，三个环境的值完全一样。这个值会在下单时传给微信（`biz/adapter/wechatpay/wechatpay.go:26`），所以**生产订单的微信回调会被推送到预发环境**，生产自己收不到。影响被另一个 bug 掩盖了——微信的 webhook 处理只落库不更新订单，实际靠轮询推进状态，所以回调推错地方暂时没造成可见故障。

---

## 2. 支付宝

### 2.1 这是什么渠道

国内收单，只结算 **CNY**。SDK 用的是 `github.com/smartwalle/alipay/v3 v3.2.26`——**社区实现，不是支付宝官方 SDK**。

### 2.2 接入前提

需要提前准备两个文件（`biz/adapter/alipay/init.go:20-34`）：

- **商户私钥** pem（PKCS8 格式）——给发出的请求签名
- **支付宝公钥** pem——验证支付宝的响应

配置项（`config/prod.yaml:56-62`）：

```yaml
alipay:
  app_id: 2021005113624907
  alipay_private_key_pem_path: "config/alipay_private_key.pem"
  alipay_public_key_pem_path: "config/alipay_public_key.pem"
  is_production: true
  time_expire: 15m                        # 双重用途，见下
  product_code: FAST_INSTANT_TRADE_PAY
```

初始化是 `alipay.New(appID, privateKey, isProduction)` + `LoadAliPayPublicKey(aliPublicKey)`，任一步失败直接 `panic`。

⚠️ `time_expire` 被**同时用于两处**：传给支付宝当 `TimeoutExpress`，以及用 `time.ParseDuration` 算本地过期时间（`biz/domain/payment/payment.go:101`）。`15m` 两边都能解析所以现在没事，但支付宝的 `timeout_express` 还支持 `1c`（当天）、`1d` 这类格式，而 Go 的 `ParseDuration` 解析不了——真填了本地建单会直接报 `ErrSystemErr`。

### 2.3 平台侧处理流程

本项目用的是**当面付预下单**（扫码支付）：

```
1. 商户后端调 alipay.trade.precreate
2. 支付宝创建交易，状态 = WAIT_BUYER_PAY，返回 qr_code
   （形如 https://qr.alipay.com/xxx 的字符串，不是图片）
3. 商户把 qr_code 渲染成二维码展示给用户
4. 用户用支付宝 App 扫码 → App 内确认付款 → 支付宝扣款
5. 交易状态 WAIT_BUYER_PAY → TRADE_SUCCESS
6. 支付宝向 notify_url 发异步通知 ← 本项目未启用
```

支付宝的交易状态只有四个：

| 状态 | 含义 | 映射到本地 |
|---|---|---|
| `WAIT_BUYER_PAY` | 交易创建，等待付款 | WaitingPayment (2) |
| `TRADE_SUCCESS` | 支付成功 | Success (4) |
| `TRADE_FINISHED` | 交易结束，不可退款 | Success (4) |
| `TRADE_CLOSED` | 未付款超时关闭，或支付后全额退款 | Closed (7) |
| `TRADE_CANCELLATION` | 已取消（本项目自定义传入） | Cancelled (5) |

映射函数在 `biz/consts/order_consts.go:117-131`。

⚠️ **`default` 分支返回 `WaitingPayment`。** 支付宝将来返回一个未识别的状态，已经成功的订单会被改回待付款。应该报错并告警，而不是给一个具体的中间状态。

### 2.4 本项目怎么调

暴露四个接口（`router/alipay/alipay.go`）：

| 接口 | 调的支付宝 API | 说明 |
|---|---|---|
| `POST /payment/alipay/checkoutlink` | `alipay.trade.precreate` | 扫码支付，返回二维码内容 |
| `POST /payment/alipay/h5` | `alipay.trade.wap.pay` | 手机网站支付，返回跳转 URL |
| `GET /payment/alipay/trade_query/:order_id` | `alipay.trade.query` | 主动查单 |
| `POST /payment/alipay/trade_close` | `alipay.trade.close` | 关单 |

建单链路（`biz/handler/alipay/service/alipay_service.go:23-50`）：

```
PreProcessFlow            查已有未过期订单 → 命中直接返回原二维码（幂等）
                          定 toCurrency = CNY，expireAt = now + 15m
                          调 ECB 换汇
alipay.CreatePaymentQR    TotalAmount = fmt.Sprintf("%.2f", float64(分)/100)   ← 元
                          TimeoutExpress = "15m"
                          aliClient.TradePreCreate() → resp.QRCode
EndProcessFlow            CreateOrderFromAliPay 落库，status = WaitingPayment
go func()                 发 WaitingPayment 事件
```

三个实现细节值得注意：

**`checkoutlink` 返回的不是链接，是二维码内容。** 用 `TradePreCreate` 拿到的 `resp.QRCode` 塞进了 `resp.Link` 字段。同时传进去的 `ReturnURL` 对 `trade.precreate` 是无效参数——预下单接口不支持同步跳转。

**H5 那条路有类型截断。** `alipay_service.go:65` 把 `flow.PayAmount`（int64 分）强转成 `int32` 再传。21474836 分（约 21.4 万元）以上会溢出。三个建单函数的金额参数类型不统一：`CreatePaymentQR` 收 int64，`CreatePayment` 和 `CreateH5Payment` 收 int32。

**`CreatePayment`（PC 网页支付 `trade.page.pay`）是死代码**，没有任何调用方。而 `H5Link` 的注释写着 "TradePagePay"，实际调的是 `TradeWapPay`。

### 2.5 结果回流：只有轮询

**`notify_url` 在两处都被注释掉了**（`biz/adapter/alipay/alipay.go:21` 和 `:49`）：

```go
//param.NotifyURL = "https://your-domain.com/notify" // 异步通知地址
```

所以支付宝根本不会回调本系统，也没有 webhook 路由。支付结果完全依赖轮询（`biz/cronjob/order_task/alipay_task.go`，每 5 秒一轮）：

```
alipay.QueryOrder(order_id)
├─ 报 40004 且订单已过期 → status = Closed，发 Closed 事件
├─ 报 40004 且未过期     → 什么都不做，下一轮再来
└─ 查到了 → 状态映射 → 改 status / pay_at / sessions_id → 发事件 → UpdateOrder
```

⚠️ **40004 的判断方式不安全。** `alipay_task.go:25` 和 `alipay_service.go:96,176` 都是 `strings.Contains(err.Error(), "40004")`。但 40004 是支付宝的**通用业务失败码**（`resp.Error.Code`），"交易不存在"只是它的一个子码 `ACQ.TRADE_NOT_EXIST`；系统繁忙、参数非法、商户权限不足同样返回 40004。

adapter 里其实已经区分了 SubCode 用于打日志（`biz/adapter/alipay/alipay.go:110-115`），但 `return` 时只把顶层 `Code` 带出去，SubCode 丢了。结果是：**支付宝侧任何一次业务异常，只要订单已过期，轮询就会把订单误判为"用户没付款"并关闭。**

修复方式是把 `SubCode` 透传出来，用 `ACQ.TRADE_NOT_EXIST` 精确判断。

### 2.6 关单是全系统唯一有终态守卫的地方

`alipay_service.go:167-172`：

```go
if order.Status == Success || Cancelled || Refunded || Closed {
    return bizerror.ErrStatusCannotBeCompleted
}
```

流程是"先查单确认存在 → 调 `trade.close` → 本地置 Cancelled + 记 trade_no"，顺序正确（先关渠道后改本地）。

超时关单的策略也是五个渠道里最保守、最安全的一个：**只在查单返回 40004 且已过期时才关**；如果支付宝那边还能查到活跃交易，就一直等支付宝自己把状态推进到 `TRADE_CLOSED`。这样本地状态永远是渠道状态的镜像，不会脱钩。

### 2.7 能力与缺口

**有**：扫码支付、H5 支付、查单、关单、终态守卫。

**没有**：异步回调、退款、订阅。`consts.Refunded` 在支付宝路径上永远不会被设置。

---

## 3. 微信支付

### 3.1 这是什么渠道

国内收单，只结算 **CNY**。用的是**官方 SDK** `github.com/wechatpay-apiv3/wechatpay-go v0.2.21`，APIv3 协议。

### 3.2 接入前提

与支付宝的机制**不同**——只需要一个文件加两个字符串（`biz/adapter/wechatpay/init.go:18-40`）：

| 项 | 说明 |
|---|---|
| 商户私钥 pem | 给请求签名，从本地文件加载 |
| 商户号 `mch_id` | |
| 商户证书序列号 | |
| APIv3 密钥 | 用于回调解密和证书下载 |
| `app_id` | 本项目是 `wx16e714fedebe9f81`。Native 支付要求这是与该商户号绑定的**公众号/服务号或 APP** 的 appid，小程序 appid 不能用于 Native 下单 |

**平台证书不需要你保存。** `option.WithWechatPayAutoAuthCipher(mchID, certSerialNo, privateKey, apiV3Key)` 会让 client **自动定期下载并轮换**微信支付平台证书。微信这样设计正是因为平台证书约五年一换、有重叠期，手工维护容易出事故。

这是支付宝和微信在密钥管理上最大的差异，别用同一套心智去理解。

### 3.3 平台侧处理流程

用的是 **Native 支付**（扫码）：

```
1. 商户后端调 POST /v3/pay/transactions/native
2. 微信返回 code_url（形如 weixin://wxpay/bizpayurl?pr=xxx）
3. 商户渲染成二维码
4. 用户用微信扫码 → 确认付款 → 微信扣款
5. 微信向请求里带的 notify_url 推送加密的支付结果通知
```

注意微信的 `notify_url` 是**每次下单请求里传的**（`native.PrepayRequest.NotifyUrl`），不是在商户平台后台配置的。

微信的交易状态映射（`biz/consts/order_consts.go:53-74`）——这是五个渠道里映射得最完整的一张表，也是唯一能产出 `Refunded` 的：

| 微信 `trade_state` | 映射到本地 |
|---|---|
| `SUCCESS` | Success (4) |
| `REFUND` | **Refunded (6)** |
| `NOTPAY` | WaitingPayment (2) |
| `CLOSED` | Closed (7) |
| `REVOKED` | Cancelled (5) |
| `USERPAYING` | PaymentIng (3) |
| `PAYERROR` | Cancelled (5) |
| `ACCEPT` | PaymentIng (3) |
| 其他 / 空 | WaitingPayment (2) |

### 3.4 本项目怎么调

暴露三个接口（`router/wechatpay/wechatpay.go`）：

| 接口 | 调的微信 API |
|---|---|
| `POST /payment/wechatpay/qrlink` | `/v3/pay/transactions/native`（Prepay） |
| `POST /payment/wechatpay/webhook` | 接收回调 |
| `GET /payment/wechatpay/tansactions/:order_id` | 查单（注意路径拼写是 `tansactions`） |

建单请求的关键字段（`biz/adapter/wechatpay/wechatpay.go:20-32`）：

```go
Appid:         config.Configs.WechatPay.AppID
Mchid:         config.Configs.WechatPay.MchID
Description:   desc
OutTradeNo:    fmt.Sprintf("%s_%d", orderID, time.Now().Unix())   // ← 带时间戳，见 §1.3
TimeExpire:    time.Unix(expireAt, 0)
NotifyUrl:     config.Configs.WechatPay.WechatpayNotifyUrl        // ← prod 指向预发域名
SupportFapiao: true                                              // 开启发票支持
Amount:        { Currency: currency, Total: amount }              // 分
```

查单和关单都按 `out_trade_no`（`QueryOrderByOutTradeNo`、`CloseOrder`），不用 `transaction_id`。函数名 `GetOrderBySessionID` 有点误导——参数实际传的是 `out_trade_no`。

### 3.5 结果回流：验签做了，但订单不更新

微信是五个渠道里最矛盾的一个——**回调链路的安全部分做对了，业务部分却是空的**。

验签用官方的 `notify.NewRSANotifyHandler`（SHA256-RSA），这部分没问题。但验签成功之后，`biz/handler/wechatpay/service/wechatpay_service.go:58-98` **只调 `CreateWebhook` 把 payload 落库，不更新任何订单状态**。

所以微信的实际状态推进 **100% 靠轮询**（`biz/cronjob/order_task/wechatpay_task.go`），webhook 在业务上没有任何作用。`webhook` 表全仓库也没有任何读取方，纯留痕。

三个问题：

⚠️ **回调可能在验签之前就被 400 拒掉。** `wechatpay_handler.go:53` 先做了 `c.BindAndValidate(&wechatpay.WebhookReq)`，而这个 IDL 结构体（`idl/wechatpay.thrift:6-16`）的字段全是 `required` 的自定义业务字段（`order_id`、`payment_status`、`meta`…），**跟微信真实回调报文毫无关系**。微信的报文不可能满足它。

⚠️ **`*app.RequestContext` 被跨 goroutine 传递。** `wechatpay_handler.go:57` 是 `go service.Webhook(ctx, c)` 然后立刻返回成功。Hertz 会回收复用 `RequestContext` 及其 body 缓冲区，goroutine 里再读 `c.Request.Body()` 属于对已释放对象的并发读——可能读到空 body 或别的请求的数据，导致验签随机失败。

⚠️ **轮询里任何查单失败都会误关订单。** 这是本渠道最严重的问题。`wechatpay_task.go:20-36` 的结构是：

```go
tradeResp, err := wechatPay.GetOrderBySessionID(ctx, order.OutTradeNo)
if err != nil {
    if tradeResp == nil {                          // ← 恒为真
        order.Status = int64(consts.Cancelled)     // 落库 Cancelled(5)
        dal.UpdateOrder(...)
    }
    go func() {
        pro.FirstBatchPublish(ctx, consts.Closed)  // 发 Closed(7) ← 在 if 外面
    }()
    return
}
```

两层缺陷叠加：

1. `GetOrderBySessionID` 在出错时**永远返回 `nil, err`**（`biz/adapter/wechatpay/wechatpay.go:52-54`），所以 `tradeResp == nil` 这个条件**恒成立**。注释写的"订单不存在"判断形同虚设。
2. 那个 `go func()` 在 `if` 块**外面**。

合起来的结果是：**任何一次查单失败——网络超时、微信 5xx、平台证书下载失败——都会把订单直接改成 `Cancelled` 落库，并向业务方推一条"订单已关闭"。** 而且落库的是 `Cancelled(5)`、推出去的事件是 `Closed(7)`，两边状态还不一致。

### 3.6 超时关单的顺序是对的

微信是唯一把关单顺序做对的渠道：先调 `wechatPay.CloseOrderBySessionID()` 通知微信，成功后才改本地状态。

不过实现上有个隐患：微信关单成功返回的是 **204 No Content**，而 adapter 把所有非 200 一律转成 error（`biz/adapter/wechatpay/wechatpay.go:64-79`）。所以"成功"路径实际是返回一个 error，调用方靠**字符串比较** `e.Error() == "204 No Content"` 来识别（`wechatpay_task.go:45`）。微信哪天改了状态文案，这里就失效了。

### 3.7 能力与缺口

**有**：Native 扫码、回调验签、查单、关单（顺序正确）、发票支持标记。

**没有**：退款、订阅、回调驱动的状态更新、事件去重。

**配置问题**：生产 `notify_url` 指向预发域名（见 §1.5）；商户单号带时间戳破坏渠道侧幂等（见 §1.3）。

---

## 4. Stripe

### 4.1 这是什么渠道

国际卡收单，多币种结算不换汇。SDK 是官方的 `github.com/stripe/stripe-go/v82`，用的是新版 service 风格 API（`c.cli.V1CheckoutSessions.Create`）。

**这是五个渠道里唯一功能完整的一个**——订阅、发票、优惠券、Billing Portal、退款接收都只有它有。`biz/handler/stripe/service/stripe_service.go` 有 1777 行，是第二大文件的两倍多。

### 4.2 接入前提

只需要 API key，没有证书和私钥文件（`biz/adapter/stripe/init.go:15-17`）：

```yaml
stripe:
  trenz:
    private_key: sk_live_…      # 服务端调用用
    public_key:  pk_live_…      # 前端用
    webhook_secret: whsec_…     # 回调验签用
  brigo:
    …同上三项
  time_expire: 1800             # 秒，用于 Session expires_at 和本地订单过期
```

⚠️ **两套 key 只有一套在工作。** `NewClientByPlatform(ctx)`（`init.go:23-27`）**忽略了 ctx、没有 switch、直接返回全局 Trenz client**，函数名与实现不符。`Init()` 也只加载了 `Trenz.PrivateKey`，**Brigo 的 private_key 从未被加载**。

历史上是对的——`git show b450f29`（"feat: brigo 接入 stripe"）里有完整的 `switch platform` 分支，在 Stripe SDK 升级到 v82 的过程中丢掉了。

后果是一个不对称状态：**Brigo 能收回调，但不能用 Brigo 账号建单**。因为 webhook 验签那里保留了双 secret 回退（见 §4.6），而所有出向 API 都走 Trenz key。

### 4.3 平台侧处理流程

用的是 **Checkout Session**（Stripe 托管收银台）：

```
1. 商户后端调 POST /v1/checkout/sessions（mode=payment）
2. Stripe 创建 Session，返回 hosted page URL（或 client_secret）
3. 用户跳转到 Stripe 托管页面 → 填卡号 → 3DS 验证
4. Stripe 创建 PaymentIntent → 授权 → 扣款 → 生成 Charge
5. 跳回 success_url，同时推 webhook
```

Session 有两个维度的状态，这是理解 Stripe 状态映射的关键：

| 维度 | 取值 |
|---|---|
| `status`（会话生命周期） | `open` / `complete` / `expired` |
| `payment_status`（支付结果） | `unpaid` / `paid` / `no_payment_required` |

映射逻辑是两级的（`biz/consts/order_consts.go:139-163`）：

```
status = expired            → Closed(7)
status = open 或 complete   → 看 payment_status：
                               paid / no_payment_required → Success(4)
                               unpaid                     → WaitingPayment(2)
default                     → WaitingPayment(2)
```

⚠️ 两个注意点：`open + paid` 也会被判成 `Success`（两个分支走的是同一个 `stripePaymentStatus` 函数）；`Refunded` / `PaymentIng` / `Cancelled` 在 Stripe 映射里**永远不会出现**。

### 4.4 收单：两个入口，同一个 API

| 接口 | ui_mode | 返回给上游 |
|---|---|---|
| `POST /stripe/checkout` | 硬编码 `hosted` | `link`（Stripe 托管页 URL） |
| `POST /stripe/payment-intents` | 默认 `embedded`，可传 `hosted` | `payment_secret`（client_secret） |

⚠️ **`/payment-intents` 名不副实。** 它从不调用 Stripe 的 PaymentIntents API，底层同样是 Checkout Session。真正调 `V1PaymentIntents.Create` 的 `biz/adapter/stripe/payment_intent.go:21-50` **是死代码，全仓零调用**。写对接文档时别按名字描述这个接口。

`CreateSession` 的关键参数（`biz/adapter/stripe/session.go:39-129`）：

- **`ClientReferenceID = req.OrderID`** ← 一次性支付靠这个字段把 order_id 带进 Stripe。订阅走的是 `metadata`（见 §4.5）
- `Mode = payment`，`ExpiresAt = expireAt`
- **ui_mode 分支**（`:59-66`）：`embedded` 设 `UIMode="embedded"` + `RedirectOnCompletion="never"`，**不设 success_url**，前端用返回的 client_secret 挂载 Embedded Checkout；`hosted` 只设 `SuccessURL`
- **line_items**：`item.price_id` 非空走预创建 Price，否则 inline `PriceData`
- **`metadata` 一次性支付从不设置**
- ⚠️ **`cancel_url` 从未传给 Stripe**（thrift 里定义了，adapter 忽略）

**一个容易忽略的能力**（`session.go:114-127`）：当 `currency` 是 `CNY` 或 `HKD` 时，会把支付方式限定为 `[card, alipay, wechat_pay]` 并设 `WeChatPay.Client = "web"`。

也就是说 **Stripe 也能收微信和支付宝**——见 §7 的三条路径对比。

⚠️ **embedded 模式下 client_secret 被存进了 `order.pay_link` 列**（`stripe_service.go:1204`）。语义混用，而且 client_secret 属敏感信息，会随订单快照一起进数据库和 Kafka 消息体。

### 4.5 订阅：全渠道唯一

订阅底层也是 Checkout Session，只是 `mode=subscription`（`biz/adapter/stripe/subscription.go:121-129`）。

| 接口 | 调的 Stripe API | 说明 |
|---|---|---|
| `POST /stripe/subscribe` | `V1CheckoutSessions.Create`（mode=subscription） | 需要 user_info，会建/复用 Customer |
| `POST /stripe/landing_subscribe` | 同上 | 落地页版，不建 Customer，由 Stripe 收集 email，事后靠 webhook 回填 |
| `POST /stripe/unsubscribe` | `V1Subscriptions.Update{CancelAtPeriodEnd: true}` | **不是立即取消**，是到期不续 |
| `POST /stripe/subscription/resume` | `V1Subscriptions.Update{CancelAtPeriodEnd: false}` | 撤销到期取消 |
| `POST /stripe/subscription/update` | `V1Subscriptions.Update` + `ProrationBehavior=always_invoice` | 换套餐，**立即出票立即扣款** |
| `POST /stripe/subscription/info` | `V1Subscriptions.List` + `V1Invoices.CreatePreview` | **完全不读本地库，全查 Stripe** |
| `POST /stripe/billing/portal` | `V1BillingPortalSessions.Create` | Stripe 自助管理页 |
| `POST /stripe/invoice/hostedurl` | `V1Invoices.Retrieve` | 单张发票的托管页 |
| `POST /stripe/invoices/list` | `V1Invoices.List` | ⚠️ **无分页**，全量拉取 |
| `GET /stripe/coupon/info` | `V1Coupons.Retrieve` + `V1Prices.List` | 优惠券详情，Redis 缓存 5 分钟 |

**`order_id` 通过 `SubscriptionData.Metadata` 传入**（`subscription.go:90-97,113-115`），这是 webhook 里读 order_id 的唯一来源。落库时的雪花 ID 也会写进 metadata 的 `id` 字段，作为回调时的回查主键。

**`payment_subscribe` 表是"每个计费周期一条记录"，不是每个订阅一条。** 续费时不更新原记录，而是把旧记录置 `Stop` 再 **insert 一条新记录**（`stripe_service.go:260-297`）。表里存了 `subscription_id`、`session_id`、`period_start/end`、`invoices`（整个 Invoice JSON）、`refunds`（退款记录 JSON 数组）等。

⚠️ **year 周期的时长算错了**（`stripe_service.go:762-764`）：`365 * 24 * 30 * intervalCount` 小时约等于 30 年。month 按 30 天近似也不精确。

### 4.6 Webhook：双 secret 回退

验签在 `biz/handler/stripe/stripe_handler.go:105-119`：先用 `Trenz.WebhookSecret` 调 `webhook.ConstructEvent`，失败再用 `Brigo.WebhookSecret` 试一次，两个都失败才拒绝。**这是全仓唯一还在使用 Brigo 配置的代码路径。**

处理了 10 类事件（`stripe_service.go:351-605`）：

| 事件 | 做什么 | event_id 去重 |
|---|---|---|
| `customer.subscription.created` | **空 case**，占位 | — |
| `invoice.payment_succeeded` · 首付 | 回填 landing 用户信息、写 subscription_id、置 Success | ❌ |
| `invoice.payment_succeeded` · 续费 | 旧记录置 Stop、insert 新周期记录、推两条事件 | ✅ Redis 24h |
| `invoice.payment_succeeded` · 升级 | 写 invoice、置 Continue | ❌ |
| `customer.subscription.deleted` | 置 Stop + Cancelled，推 Cancelled | 靠业务字段 |
| `customer.subscription.updated` | 只处理 `cancel_at_period_end` 变化，推 CancelScheduled(8) / Resumed(9) | ✅ Redis 24h |
| `checkout.session.expired` | 查 `payment_subscribe` → 置 Closed | ❌ |
| `checkout.session.completed` | 查 `order` 表 → 置 Success | ❌ |
| `payment_intent.succeeded` | 查 order → 置 Success | ❌ |
| `payment_intent.payment_failed` | 查 order → 置 Closed | ❌ |
| `refund.created/updated/failed` | 见 §4.8 | ❌ |

**去重只做了两处**，其余全靠业务字段或完全没有。

⚠️ **三个分支存在结构性错配：**

1. **`checkout.session.completed` 对订阅 session 必然报错**（`:540-554`）——它只查 `order` 表，而订阅的 `cs_xxx` 存在 `payment_subscribe.session_id` 里。每次订阅支付完成都会报一次错（错误文案还复制错了，写的是 "checkout.session.expired"）。
2. **`checkout.session.expired` 对一次性支付 session 必然报错**（`:509-524`）——反向对称的问题，它只查 `payment_subscribe`。一次性支付的过期实际靠轮询兜底。
3. **`payment_intent.*` 对本系统订单永远查不到**（`:1240,1293`）——它用 `GetOrderBySessionID(paymentIntent.ID)` 查，但落库时 `sessions_id` 存的是 `cs_xxx` 而不是 `pi_xxx`。这两个 handler 是死路径，真正生效的是 `checkout.session.completed`。

⚠️ **处理失败不返回错误码。** `stripe_handler.go:120-123` 在 `service.Webhook` 返回错误时只打日志、不写响应体，Hertz 默认返回 200 → **Stripe 认为投递成功、不会重试**。数据库抖动会造成永久丢事件。

### 4.7 轮询

`biz/cronjob/order_task/strip_task.go:15-55`，查的是 **Checkout Session**（`V1CheckoutSessions.Retrieve`），如果 Session 上有 PaymentIntent 再额外 Retrieve 一次，只为在成功时取 `intent.Created` 当 `pay_at`。

过期处理是**只改本地不调 Stripe**（`:23-36`）：`now >= expire_at` 且两侧都不是成功 → 本地置 `Closed`。Session 本身有过期时间所以后果有限，但严格说本地与渠道状态脱钩了。

⚠️ **订阅侧完全没有轮询兜底**——`SyncStripeSubscribe` 整段被注释（`:69-138`），订阅状态 100% 依赖 webhook。

### 4.8 退款：只能被动接收

**没有主动发起退款的入口。** 全仓无 `V1Refunds.Create` 调用，也没有退款路由。退款只能在 Stripe Dashboard 或其他系统里发起，本服务只被动接收 `refund.*` 事件。

`processRefund`（`stripe_service.go:1331-1483`）的处理链路相当曲折：从 refund 拿 payment_intent → `V1InvoicePayments.List` 拿 invoice_id → 从 Invoice 的原始 JSON 里挖 `parent.subscription_details.metadata.id` → 查 `payment_subscribe` → 把退款记录 append 进 `refunds` JSON 列。

⚠️ **一次性支付的退款完全不处理。** `:1355-1358` 遇到 `refund.PaymentIntent == nil` 直接 return，`:1410-1413` 找不到订阅记录也直接跳过。`order` 表既不置 `Refunded` 也不通知上游。所以 `OrderStatus.Refunded=6` 在 Stripe 路径上**永远不会被写入**。

### 4.9 关单是空实现

`POST /stripe/trade_close` → `stripe_service.go:658-662`：**两行日志然后 `return nil`**，直接返回成功。既不调 Stripe（Session 是可以 `V1CheckoutSessions.Expire` 的），也不改本地订单状态。上游若依赖它关单会静默失败。

### 4.10 能力与缺口

**有**：Checkout（hosted + embedded）、订阅全生命周期、发票、优惠券、Billing Portal、双 secret 验签、轮询兜底（一次性支付）、退款接收。

**没有**：主动退款、一次性支付退款处理、订阅轮询兜底、Brigo 出向调用、真正的关单。

---

## 5. Airwallex

### 5.1 这是什么渠道

国际收单（空中云汇），多币种。**没有官方 Go SDK，adapter 是手写的 REST 封装**（`biz/adapter/airwallex/`，6 个三方调用）。

它在本项目里的定位有个特别之处：通过 `payment_method` 参数可以路由到**本地支付方式**（`wechatpay`、`alipaycn`），所以它同时是"国际卡通道"和"微信/支付宝的第三条接入路径"。

### 5.2 接入前提与认证

与其他渠道都不同——**Airwallex 用的是 API key 换 access token 的模式**：

```yaml
airwallex:
  client_id: …
  api_key: …
  api_url: https://api.airwallex.com        # demo 环境是 api-demo.airwallex.com
  time_expire: 1800                          # 秒，本地订单过期
  webhook_secret: whsec_…
```

认证流程（`biz/adapter/airwallex/client.go:42-90`）：

```
POST {api_url}/api/v1/authentication/login
  Header: x-client-id, x-api-key             ← 注意是请求头，不是 body
  → { token, expires_at }
存 Redis: airwallex_access_token:{clientID}
  TTL = time.Until(expireAt) - 30 分钟       ← 提前半小时过期
后续请求: Authorization: Bearer {token}
```

三个需要知道的实现细节：

⚠️ **没有主动刷新，只靠自然过期后重新 login。** 也没有 401 失效重取。
⚠️ **取 token 无并发锁。** 缓存未命中时并发请求会同时打 login 接口（thundering herd）。仓库里其实已有 `redis.AcquireNXLockWithRetry` 这个工具（`biz/adapter/redis/helper.go:81`），没用上。
⚠️ **HTTP client 裸用 `resty.New()`**（`client.go:27`），没设超时也没设重试。而 `RetrivePaymentIntent` 是在 Kafka 消费链路里同步调用的，Airwallex 慢响应会拖住消费者。

### 5.3 平台侧处理流程

用的是 **PaymentIntent + 前端组件**模式，这与 Stripe Checkout 的"跳转托管页"完全不同：

```
1. 商户后端调 POST /api/v1/pa/payment_intents/create
2. Airwallex 返回 { id, client_secret, status: REQUIRES_PAYMENT_METHOD }
3. 商户把 intent_id + client_secret 交给前端
4. 前端用 Airwallex.js（Drop-in 或 Embedded Elements）挂载支付组件
5. 用户填卡 → 前端用 client_secret 确认 → 3DS
6. intent status 流转 → Airwallex 推 webhook
```

PaymentIntent 的状态（`biz/adapter/airwallex/consts.go:59-75`）：

| Airwallex status | 映射到本地 |
|---|---|
| `REQUIRES_PAYMENT_METHOD` | WaitingPayment (2) |
| `REQUIRES_CUSTOMER_ACTION` | PaymentIng (3) |
| `REQUIRES_CAPTURE` | PaymentIng (3) |
| `PENDING` | PaymentIng (3) |
| `SUCCEEDED` | Success (4) |
| `CANCELLED` | Cancelled (5) |
| default（未知/空） | **Created (1)** |

⚠️ `REQUIRES_CAPTURE` 只映射成 `PaymentIng`，但**没有实现 capture API**——隐含依赖商户在 Airwallex 侧配置了自动 capture。

⚠️ adapter 里还定义了 10 个 `PaymentAttemptStatus` 常量（`consts.go:5-51`），但**没有任何映射函数、没有任何业务代码消费**，纯文档性常量。attempt 级的 `FAILED` / `EXPIRED` 因此无法映射到本地状态。

### 5.4 收单：返回的不是 URL

| 接口 | 调的 Airwallex API |
|---|---|
| `POST /payment/airwallex/intent/create` | `/api/v1/pa/payment_intents/create` |
| `POST /payment/airwallex/intent/confirm` | `/api/v1/pa/payment_intents/{id}/confirm` |
| `POST /payment/airwallex/webhook` | 接收回调 |

建单请求的关键字段（`biz/handler/airwallex/service/airwallex_service.go:82-100`）：

- `MerchantOrderID = order_id`
- `Amount = float64(flow.PayAmount) / 100` ← 元，float64
- `ReturnURL = success_url`
- `Metadata` 带 `user_id` / `client_id` / `order_id` / **`payment_method`**
- `Order.products` 从 line_item 转换，`Type` 硬编码 `"physical_good"`
- `RequestID = utils.GetTraceIDFromCtx(ctx)`

**返回给上游的是 `intent_id` + `client_secret` + `metadata`，没有任何 URL。** 这是与其他四个渠道最大的接入差异——**前端必须自己集成 Airwallex.js**，拿 client_secret 挂载支付组件。

⚠️ 仓库内**没有任何前端代码或 Airwallex.js 集成说明**，这个前端契约只体现在返回字段上。接入方拿到 `client_secret` 如果不知道要配 Airwallex.js，是无从下手的。这一点需要在对外文档里显式补充。

⚠️ **`request_id` 用 trace id，起不到幂等作用。** Airwallex 的 `request_id` 本来是幂等键，但 trace id 每次请求都变，所以同一订单重复调用会创建多个 PaymentIntent。

**`/intent/confirm` 本质是一个透传代理**（`airwallex_handler.go:80-82`）：把客户端的原始 JSON 整体反序列化成 map 原样转发给 Airwallex，**不校验 `intent_id` 是否属于当前 client_id 或当前订单**。任何持有合法 Client-ID 的调用方可以确认任意 intent，也可以注入任意 confirm 参数。

### 5.5 PlatformOrderSnapshot：为什么要存整个响应

Airwallex 是唯一用了 `order.platform_order_snapshot` 列的渠道，存的是**整个 `CreatePaymentIntentResponse` 的 JSON**（含 client_secret）。三个用途：

1. **重复下单免调三方。** `client_secret` 只在 create 响应里返回一次，存下来才能在幂等命中时把同一个 intent 原样再吐给前端。
2. **携带 `metadata.payment_method`**，作为"要不要换支付方式重开 intent"的判据（`airwallex_service.go:75`）。
3. **回调时被覆盖为最新快照**（`:178`），作为对账留痕。

⚠️ **复用分支返回的是过期快照。** `:71-77` 直接返回落库时的 snapshot，其 `status` 是 create 时刻的值（通常 `REQUIRES_PAYMENT_METHOD`）。如果这个 intent 实际已经 SUCCEEDED 或 CANCELLED，上游拿到的仍是过期状态和一个不可用的 client_secret，没有回源校验。

⚠️ **换支付方式的路径会重复插单。** 幂等命中但 `payment_method` 不一致时会继续往下创建新 intent，而 `CreateOrderFromAirwallex` 是无条件 `Create`（INSERT）不是 upsert，结果是同一 `order_id` 出现第二条订单记录，同时旧 intent 也没被取消。

### 5.6 Webhook：验签最规范，消费最脆弱

**验签是五个渠道里做得最规范的一个**（`biz/handler/airwallex/airwallex_handler.go:106-195`）：

```
待签串 = x-timestamp + body
签名   = HMAC-SHA256(webhook_secret, 待签串) → hex 小写
比较   = hmac.Equal()                        ← 常量时间
时间戳 = ±300 秒容差，超长值按毫秒处理
```

验签通过后**先立即返回 200 ACK**，再起 goroutine 投 Kafka 异步任务。这样做是为了避免 Airwallex 因超时重推（handler 里的注释明确写了这个意图）。

消费端（`biz/adapter/async_task/handler/airwallex_handler.go`）有一个**值得肯定的实践**：它不信 webhook body，而是拿 intent id **回源调 `RetrivePaymentIntent` 核对**（`airwallex_service.go:163`），用查回来的结果更新订单。这是五个渠道里唯一这么做的。

但消费链路有三个严重问题：

⚠️ **幂等 key 用 `defer` 无条件写入，把重试机制废掉了。** `airwallex_handler.go:42` 是 `defer redis.Set("airwallex:webhook:{ID}", "1", 30天)`——**handler 返回 error 时这个 defer 也会执行**。于是业务处理失败后，任务被重试任务重投时会在 `:38` 的 `Exists` 检查命中而直接跳过。**失败即永久丢单，且 30 天内无法补偿。**

⚠️ **Kafka 分区键用的是 `event.ID` 而不是 `order_id`**（`airwallex_service.go:139`）。每个事件的 ID 都不同，所以**同一订单的多个事件会散到不同分区，完全没有顺序保证**。配合状态映射的 default 返回 `Created(1)`、以及只判 `!=` 不判方向的更新逻辑，一个迟到的 `payment_intent.created` 就能把已成功的订单改回去，并向上游再推一次错误状态。

⚠️ **非 `payment_intent.*` 的事件被静默确认丢弃**（`:50-52`）：打个 error 日志就 return nil，配合上面那个 defer 直接标记为已处理。`refund.*`、`dispute.*` 全部丢弃且不可追回。

另外 `airwallex_service.go:170` 的 `orderIntent.LatestPaymentAttempt.CreatedAt` 没有 nil 判断，而这个字段是 `omitempty` 指针——空指针会被 recover 兜住，但结合上面的 defer 幂等问题，直接导致永久丢单。

### 5.7 没有轮询兜底

`biz/cronjob/order_task/task.go:26-27`：

```go
case consts.AirwallexPayChannel.String():
    // ProcessAirwallexOrder(ctx, order, dal)
```

分支在，调用被注释掉了，而且 `ProcessAirwallexOrder` 这个函数**在全仓根本不存在**。

所以 **Airwallex 是纯 webhook 单通道**。webhook 丢失、Kafka 不可用、或消费失败 3 次之后，订单永久停在 `WaitingPayment`。这是它与 Stripe 最大的能力差距。

### 5.8 能力与缺口

**有**：PaymentIntent 建单、confirm、规范的 HMAC 验签、Kafka 异步消费、回源核对、事件去重（虽然实现有缺陷）、本地支付方式路由。

**没有**：轮询兜底、退款、订阅、capture、取消。`Refunded(6)` 和 `Closed(7)` 对 Airwallex 永远不可达。

**死代码**：`CreatePaymentLink`（实现了但零调用）、`ListPaymentIntent`（GET 请求带 body，查询条件永远不生效）、`AuthenticationRequest`、全部 10 个 `PaymentAttemptStatus` 常量。

---

## 6. UseePay

### 6.1 这是什么渠道

海外收单，外币结算不换汇。**它是五个渠道里唯一同时用了两套完全不同协议的**：

| 用途 | 域名 | 协议 |
|---|---|---|
| 收银台下单 / 查单 | `…/cashier`、`…/api` | 表单 POST + **MD5 签名**（传统网关风格） |
| 订阅 / 客户 / 发票 | `openapi1.*.useepay.com/api/v1/…` | JSON + **Header 鉴权**（REST 风格） |

收银台侧的卡组和 3DS 味道很明显——`payerInfo.authorizationMethod = "cvv"`，状态码里有 `gather` / `challenge` / `pending_review` 这些 3DS 流程态。订阅侧的模型则几乎逐字照搬 Stripe（`biz/adapter/useepay/model.go` 有 1426 行，注释里甚至还留着 "Stripe will attempt to pay…"）。

⚠️ **`biz/adapter/useepay/` 下没有 `init.go`**，没有客户端初始化，`main.go` 里也没有 UseePay 的 Init。每次请求都是现场 `http.NewRequest` + `https.GetClient()`——而那个共享 client 设了 `InsecureSkipVerify: true`（`biz/adapter/https/https.go:22`），并且已经被标记为 `// Deprecated: 之前都是useepay用，目前没人再用`。也就是说 UseePay 唯一的出网方式是一个已废弃且不校验 TLS 证书的客户端。

### 6.2 接入前提

配置项比其他渠道多得多（`config/config.go:71-86`），因为每个 OpenAPI 端点都单独配一个 URL：

```yaml
useepay:
  domain:            https://pay-gateway1.uat.useepay.com/cashier    # 收银台
  get_order_domain:  https://pay-gateway1.uat.useepay.com/api        # 查单
  create_customers:  …    create_subscription: …    unsubscription: …
  create_invoices:   …    pay_invoices: …           get_invoices:   …
  app_id:      trenz.ai
  app_kay:     …          # 字段名拼写错误，应为 app_key
  merchant_no: 500000000011125
  md5_key:     pkey=…     # 值自带 pkey= 前缀，需确认是否应参与签名
  time_expire: 900
  subscription_time_expire: 86340
```

⚠️ 三个环境的这一段**完全相同**，生产也指向 `uat` 域名（见 §1.5）。

**签名算法**（`biz/adapter/useepay/useepay.go:229-246`）：把所有 string 类型参数按 key 升序拼成 `k=v&`，末尾直接追加 `md5_key`，`md5.Sum` 后取十六进制小写。`signType` 固定 `"MD5"`。

⚠️ **只有签名，没有验签。** `grep verify|checkSign` 在整个 UseePay 相关代码下零命中。`CashierReturnBody.Sign` 和 `CashierInfo.Sign` 这两个字段接收了但从不校验。

⚠️ `compressedStr`（`useepay.go:248-250`）本意是去掉换行再签名，但它把正则字符串 `"\r\n|\r|\n"` 直接喂给了 `strings.ReplaceAll`（当字面量处理），**实际不会去掉任何换行**。含换行的字段会导致签名不一致。

### 6.3 平台侧处理流程

**一次性支付**走收银台跳转：

```
1. 商户后端 POST {domain}/cashier，transactionType=pay
2. UseePay 返回 redirectUrl
3. 用户跳转到 UseePay 收银台 → 填卡 → 可能进入 3DS 流程
   （对应状态 gather → challenge → pending_review）
4. 完成后跳回 redirectUrl
```

**订阅**是四个 OpenAPI 串起来的（`biz/handler/useepay/service/useepay_service.go:79-224`）：

```
CreateCustomers    → 建客户（复用本地 payment_customer 记录）
CreateSubscriptions → 建订阅，拿 subscription_id
CreateInvoices      → 建首期发票
PaymentInvoices     → 支付该发票，拿 hosted_invoice_url
```

返回给前端的 `link` 是最后那个 `HostedInvoiceURL`。

⚠️ 订阅创建里有两个硬编码值可疑（`biz/adapter/useepay/subscription.go:21-23`）：`CancelAtPeriodEnd: true`（**周期末自动取消**）和 `Description: "Subscription test"`（生产用测试文案）。

状态映射（`biz/consts/order_consts.go:86-110`）：

| UseePay `resultCode` | 映射到本地 | 说明 |
|---|---|---|
| `succeed` | Success (4) | |
| `received` | WaitingPayment (2) | |
| `failed` | **Closed (7)** | ⚠️ 语义混同：支付失败 ≠ 订单关闭 |
| `cancelled` | Cancelled (5) | |
| `gather` | WaitingPayment (2) | 3DS 收集信息 |
| `challenge` | WaitingPayment (2) | 3DS 验证中 |
| `pending` | PaymentIng (3) | |
| `pending_review` | PaymentIng (3) | 风控人工审核 |
| `closed` | Closed (7) | |
| 其他 / 空 | WaitingPayment (2) | |

### 6.4 本项目怎么调

| 接口 | 说明 |
|---|---|
| `POST /payment/useepay/checkoutlink` | 收银台下单，返回 `redirectUrl` |
| `POST /payment/useepay/subscribe` | 四步订阅串联 |
| `POST /payment/useepay/unsubscribe` | 取消订阅 |
| `POST /payment/useepay/webhook` | **空实现** |
| `GET /payment/useepay/tansactions/:order_id` | 实时查单（注意拼写少了 `r`） |

下单的关键字段（`biz/adapter/useepay/useepay.go:113-139`）：`transactionId = order_id`（**不拼时间戳**）、`amount` 是分的十进制字符串、`currency` 大写、`redirectUrl = success_url`、`transactionType = "pay"`、`terminalType = "WEB"`、`autoRedirect = "false"`。

返回的 `CashierReturnBody` 有 13 个字段，但**只有 `redirectUrl` 被使用**。

⚠️ 三个实现细节值得注意：

**错误判定靠字符串比较。** 下单成功与否不看 `resultCode`，而是判断 `errorMsg != "" && errorMsg != "Successful payment"`（`useepay.go:79-81`）。另外还有一处靠"反序列化错误信息里是否包含 502"来判断网关升级（`useepay.go:74-76`）——用错误文本猜 HTTP 状态码，非常脆弱。整条链路**从不检查 `res.StatusCode`**。

**userID 被当成邮箱用。** `payerInfo.billingAddress.email` 和 `userInfo.userId` 填的是同一个 `auth.UserID(ctx, "")`（`useepay.go:126,136`）。

**金额单位两条链路不一致。** 收银台传的是**分**（`useepay.go:114`），而订阅和发票传的是 `UnitAmount / 100` 的**主单位**（`useepay_service.go:93,163`）。至少有一条需要对照官方文档核对。

### 6.5 Webhook 是空实现

`biz/handler/useepay/useepay_handler.go:56-66` 的全部内容是：`BindAndValidate` → `hlog.CtxInfof(ctx, req.ID)` → 返回成功。

**没有验签、不落库、不更新订单、不发事件。** 而且验签代码是**根本不存在**，不是"写了没调用"。

三个衍生问题：

- 这条路由在鉴权白名单里（`middleware/auth_middleware.go:35`），所以它是一个**公网可无鉴权调用的空接口**
- `hlog.CtxInfof(ctx, req.ID)` 把外部输入当作 format string，存在格式化字符串注入 / 日志污染
- `WebhookReq` 只有一个 `id` 字段（`idl/useepay.thrift:6-8`），显然不是 UseePay 真实的回调报文结构

### 6.6 没有任何自动状态推进

这是 UseePay 与其他四个渠道最本质的差别：

- `biz/cronjob/useepay/task.go` **整个文件 228 行全是注释**，里面包含被废弃的订单轮询和一个标了 `// Deprecated: 实现太差，后续需要重写` 的订阅轮询
- `biz/cronjob/order_task/task.go:19-30` 的 switch **没有 useepay 分支**
- `biz/cronjob/cronjob.go` 也没有注册任何 UseePay job
- `useepay.CloseOrder` 是空实现，直接 `return nil`（`useepay.go:225-227`）

所以 UseePay 订单会进入 `OrderTask` 的 `default` 分支，**每 5 秒刷一条 `[Cronjob-Order] Unknown payment channel: useepay` 错误日志**，然后什么都不做。

**结论：UseePay 订单会永久停在 `WaitingPayment`。** 既不会在支付成功后变 Success，也不会超时关闭，下游也收不到任何事件（除订阅创建/取消那两次手动发的）。唯一能拿到真实状态的路径是外部主动调查单接口，而那个接口**只回查不落库**。

结合 §1.5 里"生产指向 UAT 网关"这一点，可以判定这个渠道**没有在生产真实使用**。

### 6.7 能力与缺口

**有**：收银台下单、订阅四步串联、取消订阅、实时查单、MD5 签名。

**没有**：验签、webhook 处理、轮询、关单、退款、任何自动状态推进。

**已知 bug**：`Unsubscribe` 的分支条件写反了（`useepay_service.go:243` 写的是 `} else if req.SubID == ""`，本意应为 `!= ""`），配合上面"两者都传即冲突"的校验，**只传 `sub_id` 的请求会静默返回成功但什么都不做**。另外 `subscription.go:85-87` 里 `Unsubscribe` 的错误码检查被注释掉了，UseePay 返回业务错误也一律当成功。

⚠️ **查单接口存在越权。** `GetTransactions`（`useepay_service.go:54-77`）直接拿路径参数去打第三方，**不做本地订单归属校验**。对比微信版（`wechatpay_service.go:104-110`）会先 `dal.GetOrder(uid, orderID)` 确认订单属于当前用户。所以任何持有合法 Client-ID 的调用方可以查询任意 `order_id`。

---

## 7. 微信和支付宝其实有三条接入路径

这是读完五个渠道之后最容易忽略的一件事：**同一个支付方式，本项目存在三条并存的接入路径。**

| 路径 | 怎么走 | 结算币种 | 需要的资质 | 代码位置 |
|---|---|---|---|---|
| **直连** | 自己对接支付宝 / 微信商户号 | CNY | 国内主体 + 商户号 + 证书 | `biz/adapter/alipay`、`biz/adapter/wechatpay` |
| **经 Stripe** | `currency` 为 CNY/HKD 时自动把 `payment_method_types` 设为 `[card, alipay, wechat_pay]` | 原币 | Stripe 账户即可 | `biz/adapter/stripe/session.go:114-127` |
| **经 Airwallex** | 请求带 `payment_method = wechatpay` / `alipaycn`，写进 intent metadata | 原币 | Airwallex 账户即可 | `biz/handler/airwallex/service/airwallex_service.go:89-94` |

三条路径的差异不只是"谁收单"：

**费率与结算周期不同。** 直连是国内商户费率、T+1 结算到国内账户；经 Stripe/Airwallex 是国际收单费率 + 可能的跨境手续费，结算到境外账户。

**换汇位置不同。** 直连需要本项目自己把金额换成 CNY（走 ECB 汇率，见 §1.4，有精度损失和汇损风险）；经 Stripe/Airwallex 则是原币下单，换汇由渠道完成——本地不留汇率痕迹（`rate` 恒为 `1:1`），对账时无法还原实际汇率。

**用户体验不同。** 直连返回二维码内容需要自己渲染；经 Stripe 是跳转托管页；经 Airwallex 需要前端接 Airwallex.js。

**可靠性差异很大。** 直连的支付宝/微信有轮询兜底；经 Airwallex 是纯 webhook 单通道，没有兜底。

选路径时要明确的是**这不是一个纯技术选择**——它同时决定了资金流向哪个账户、按什么费率结算、以及出问题时找谁对账。代码层面目前没有任何地方记录"这笔单为什么走了这条路径"。

---

## 8. 全渠道对照总表

### 8.1 接入形态

| | 支付宝 | 微信支付 | Stripe | Airwallex | UseePay |
|---|---|---|---|---|---|
| SDK | 社区 `smartwalle/alipay/v3` | **官方** `wechatpay-go` | **官方** `stripe-go/v82` | 手写 REST | 手写 REST |
| 认证方式 | 商户私钥 + 平台公钥（静态） | 商户私钥 + APIv3 密钥（**证书自动轮换**） | API key | client_id + api_key **换 token** | MD5 签名 |
| 客户端初始化 | `Init()` | `Init()` | `Init()` | `Init()` | **无** |
| 收单 API | `trade.precreate` / `trade.wap.pay` | `/v3/pay/transactions/native` | Checkout Session | PaymentIntent | 收银台 form |
| 返回给上游 | 二维码内容 | 二维码内容 | 托管页 URL / client_secret | **intent_id + client_secret** | 收银台 URL |
| 前端要做什么 | 渲染二维码 | 渲染二维码 | 跳转 / 嵌入 | **必须接 Airwallex.js** | 跳转 |
| 结算币种 | CNY（需换汇） | CNY（需换汇） | 原币 | 原币 | 原币 |

### 8.2 可靠性

| | 支付宝 | 微信支付 | Stripe | Airwallex | UseePay |
|---|---|---|---|---|---|
| Webhook 路由 | ❌ 无 | ✅ 有 | ✅ 有 | ✅ 有 | ⚠️ 空实现 |
| 验签 | — | ✅ 官方 SDK | ✅ 双 secret 回退 | ✅ HMAC + 时间戳 | ❌ 无 |
| 回调更新订单 | — | ❌ 只落库 | ✅ | ✅ | ❌ |
| 事件去重 | — | ❌ | ⚠️ 仅 2 处 | ⚠️ 有但实现有缺陷 | ❌ |
| 轮询兜底 | ✅ | ✅ | ✅ 一次性支付 | ❌ | ❌ |
| 关单 | ✅ 顺序正确 | ✅ 顺序正确 | ❌ 空实现 | ❌ | ❌ 空实现 |
| 终态守卫 | ✅ **全项目唯一** | ❌ | ❌ | ❌ | ❌ |
| 环境隔离 | ❌ 直连生产 | ⚠️ 微信无沙箱 | ✅ | ✅ | ❌ 生产指向 UAT |
| 实际可用性 | 可用 | 可用（回调无效） | **完整可用** | 可用（无兜底） | **基本不可用** |

### 8.3 能力覆盖

| | 支付宝 | 微信支付 | Stripe | Airwallex | UseePay |
|---|---|---|---|---|---|
| 一次性支付 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 订阅 | ❌ | ❌ | ✅ 完整 | ❌ | ⚠️ 有但可疑 |
| 发票 | ❌ | 标记支持 | ✅ | ❌ | ✅ |
| 优惠券 | ❌ | ❌ | ✅ | ❌ | ❌ |
| 退款 | ❌ | ❌ | ⚠️ 仅被动接收订阅退款 | ❌ | ❌ |
| 可达的本地终态 | Success / Closed / Cancelled | Success / Closed / Cancelled / **Refunded** | Success / Closed | Success / Cancelled | Success / Closed / Cancelled |

值得注意的是最后一行：**`Refunded(6)` 只有微信的状态映射能产出**（`REFUND → Refunded`），而微信恰恰没有退款能力——这个状态只能在用户从微信侧发起退款、然后被轮询查到时才会出现。Stripe 有退款处理但只写 `payment_subscribe.refunds` 列，不改 `order.status`。所以 `order` 表里几乎不会出现 `Refunded`。

---

## 9. 如果要接第六个渠道

按本项目现在的结构，新增一个渠道需要改动 12 到 19 处，横跨 9 个顶层目录。完整清单和其中四处"忘了改就静默出错"的陷阱，见配套规范文档；这里只列最容易漏的三个：

1. **`biz/domain/calculate/calculate.go:35-66` 的 switch** — 漏了会走 `default` 直接返回 `ErrPaymentChannelErr`，建单必失败（这个会立刻暴露，反而安全）
2. **`biz/cronjob/order_task/task.go:19-30` 的 switch** — 漏了订单永远不会被轮询、永远不会超时关闭，**且没有任何报错**。Airwallex 和 UseePay 现在都是这个状态
3. **`biz/domain/payment/payment.go` 的两处 switch**（`:98-125` 和 `:163-181`）— 以及 `EndProcessFlowReq`（`:148-158`）这个"各渠道返回类型并集"的结构体，每加一个渠道它就更难拆

第 2 条是最危险的，因为它的失败是完全静默的。

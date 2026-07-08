# 从零手搓 WireGuard：在外网访问家庭内网完整实操清单

> 来源：个人思考 + 技术整理
> 收录时间：2026-05-27 17:20 CST

---

## 0. 目标与前置概念

**目标场景**：人在外面（手机 4G / 公司 / 咖啡馆），通过 WireGuard 隧道连回家，像坐在家里一样访问家庭内网的所有设备（NAS、软路由后台、内网网站等）。

**先记住一句话**：VPN 能不能通，99% 不取决于 WireGuard 本身，而取决于三件事——**可达性（能不能连到家）、IP 转发（包能不能在服务端转出去）、路由（包知不知道往哪走、怎么回来）**。本文每一步都围绕这三点。

### 拓扑

```
[外部客户端]                    公网                 [家庭网络]
 手机/笔记本  ──── 加密 UDP 隧道 ────►  家庭公网IP:51820
 wg: 10.10.10.2                         │  端口转发
                                        ▼
                              [WireGuard 服务端 Linux]
                               LAN IP: 10.0.0.50
                               wg:    10.10.10.1
                                        │ IP转发 + NAT
                                        ▼
                              家庭 LAN  10.0.0.0/24
                              （网关/OPNsense 10.0.0.1，NAS 10.0.0.x …）
```

### IP 规划（动手前先想好，避免冲突）

| 名称 | 网段 | 说明 |
|------|------|------|
| 家庭 LAN | `10.0.0.0/24` | 你现在的真实内网，网关是 OPNsense `10.0.0.1` |
| WireGuard 隧道网 | `10.10.10.0/24` | **必须和家庭 LAN 不重叠**，专给 VPN 客户端用 |
| 服务端隧道地址 | `10.10.10.1` | WireGuard 服务端的虚拟网卡地址 |
| 客户端隧道地址 | `10.10.10.2` | 第一个客户端，第二个用 `.3`，以此类推 |
| 监听端口 | `UDP 51820` | WireGuard 默认端口，可自定义 |

> ⚠️ 隧道网千万别选 `10.0.0.0/24` 或它的子集，否则路由打架（这正是之前讨论的「内网开 VPN 路由环路」的同类问题）。

---

## 1. 决定服务端装在哪

两种选择，本文走 **方案 A（手搓，学得透）**，文末附 **方案 B（OPNsense GUI）** 对照。

- **方案 A**：家里一台常开的 Linux（Ubuntu/Debian、NAS 上的 Docker、树莓派、虚拟机都行）。
- **方案 B**：直接在 OPNsense 的 `VPN → WireGuard` 图形界面配（你已有这台设备）。

---

## 2. 安装 WireGuard（服务端，Ubuntu/Debian）

```bash
sudo apt update
sudo apt install -y wireguard wireguard-tools qrencode
# qrencode 用于给手机生成二维码配置，可选
```

验证：

```bash
wg --version          # 看到版本号即安装成功
ip link               # 暂时还没有 wg0，正常
```

---

## 3. 生成密钥对（理解 WireGuard 的身份模型）

WireGuard 用**公私钥对**做身份认证，逻辑和 SSH 一样：私钥自己留着，公钥给对方。**服务端和每个客户端各一对**。

```bash
# 进入配置目录，设置严格权限（私钥绝不能泄露）
cd /etc/wireguard
umask 077

# 服务端密钥对
wg genkey | sudo tee server_private.key | wg pubkey | sudo tee server_public.key

# 客户端密钥对（每多一个客户端就再生成一对）
wg genkey | sudo tee client_private.key | wg pubkey | sudo tee client_public.key
```

查看（待会儿填配置要用）：

```bash
sudo cat server_private.key server_public.key client_private.key client_public.key
```

**对应关系（最容易填错的地方）**：
- 服务端配置里的 `PrivateKey` = 服务端**私钥**；`[Peer]` 里的 `PublicKey` = 客户端**公钥**。
- 客户端配置里的 `PrivateKey` = 客户端**私钥**；`[Peer]` 里的 `PublicKey` = 服务端**公钥**。
- 记忆法：**自己填自己的私钥，对端填对端的公钥**。

---

## 4. 开启 IP 转发（服务端，关键一步）

默认 Linux 不会把「从一个网卡进来的包转发到另一个网卡」。VPN 客户端要访问 LAN，必须打开转发，否则**握手成功也 ping 不通任何 LAN 设备**。

```bash
echo 'net.ipv4.ip_forward = 1' | sudo tee /etc/sysctl.d/99-wireguard.conf
# 若需 IPv6 再加： echo 'net.ipv6.conf.all.forwarding = 1' | sudo tee -a /etc/sysctl.d/99-wireguard.conf
sudo sysctl --system
```

验证（必须返回 1）：

```bash
sysctl net.ipv4.ip_forward
# net.ipv4.ip_forward = 1
```

---

## 5. 写服务端配置 `/etc/wireguard/wg0.conf`

先确认服务端连接 LAN 的物理网卡名：

```bash
ip route get 10.0.0.1     # 看 "dev xxx"，常见 eth0 / ens18 / enp1s0
```

假设网卡是 `eth0`，写入配置（把 `<...>` 换成第 3 步的实际密钥）：

```ini
[Interface]
Address = 10.10.10.1/24
ListenPort = 51820
PrivateKey = <服务端私钥 server_private.key 的内容>

# 客户端发来的包，转发并做 NAT，伪装成服务端自己的 LAN IP 出去
# 好处：LAN 设备无需任何额外路由就能回包（最省心的做法）
PostUp   = iptables -A FORWARD -i %i -j ACCEPT; iptables -A FORWARD -o %i -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i %i -j ACCEPT; iptables -D FORWARD -o %i -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE

[Peer]
# 这是「客户端」的信息
PublicKey  = <客户端公钥 client_public.key 的内容>
AllowedIPs = 10.10.10.2/32
```

**逐项解释**：
- `Address`：服务端在隧道里的地址 + 隧道网掩码。
- `ListenPort`：监听的 UDP 端口，和后面端口转发要一致。
- `PostUp/PostDown`：接口启停时执行的防火墙规则。这里做了 **MASQUERADE（源地址伪装）**，让 LAN 设备以为流量来自服务端本机 → 不用给每台 LAN 设备配回程路由。
- `[Peer].AllowedIPs`（**服务端侧含义**）：允许这个客户端**使用哪些源 IP**，也就是「这些 IP 的包才认是它的」。客户端只占隧道里一个地址，所以是 `10.10.10.2/32`。

> 注意：`AllowedIPs` 这个字段在服务端和客户端含义不同——服务端用它**识别来源**，客户端用它**决定哪些流量进隧道（路由）**。这是 WireGuard 最绕的一个点。

---

## 6. 写客户端配置 `client.conf`

```ini
[Interface]
PrivateKey = <客户端私钥 client_private.key 的内容>
Address    = 10.10.10.2/24
DNS        = 10.0.0.1          # 用家里的 DNS，才能解析内网域名；不需要可删

[Peer]
PublicKey           = <服务端公钥 server_public.key 的内容>
Endpoint            = home.example.com:51820   # 你家公网IP或DDNS域名 + 端口
AllowedIPs          = 10.0.0.0/24, 10.10.10.0/24   # 分流：只把"去家里"的流量送进隧道
PersistentKeepalive = 25
```

**逐项解释（重点是 `AllowedIPs`，它就是路由开关）**：
- `Endpoint`：家的公网入口。动态 IP 用 DDNS 域名（见第 7 步）。
- `AllowedIPs`（**客户端侧含义 = 路由表**）：**只有匹配这里网段的流量才进隧道**。
  - **分流模式（推荐日常用）**：`10.0.0.0/24, 10.10.10.0/24` → 只有访问家庭内网才走 VPN，刷网页/看视频仍走本地，速度快。
  - **全局模式**：改成 `AllowedIPs = 0.0.0.0/0` → 所有流量都从家里出口走（相当于"人在家上网"，可隐藏真实 IP、在公共 WiFi 上加密全部流量），但慢、且受家庭上行带宽限制。
- `PersistentKeepalive = 25`：每 25 秒发个心跳，防止客户端在 NAT 后面"隧道被运营商掐断"。客户端在 NAT 后必填。

---

## 7. 打通「可达性」——让外面能连到家（最容易卡住的一步）

### 7.1 先确认你有没有公网 IP（国内重灾区）

```bash
# 在家里执行，对比两个结果
curl -s https://ip.sb            # A：你的出口公网IP（运营商看到的）
# 然后看路由器 WAN 口 IP（OPNsense 大厅/接口页）   # B：路由器实际拿到的IP
```

- **A == B** → 有公网 IP，继续 7.2。
- **A != B**（B 是 `100.64.x`/`10.x`/`172.x` 等私网段）→ **你在运营商大内网（CGNAT），外面连不进来**。解决：
  1. 打运营商电话要公网 IPv4（家宽通常免费给）；
  2. 用 **IPv6**（Endpoint 填 IPv6 地址，绕开 CGNAT）；
  3. 实在不行用 **Tailscale/ZeroTier**（自动 NAT 穿透，下一节方案 C）。

### 7.2 配置端口转发（在家庭网关 / OPNsense 上）

把外网的 UDP 51820 转发到 WireGuard 服务端的 LAN IP：

- OPNsense：`防火墙 → NAT → 端口转发`，新增一条：
  - 接口 WAN，协议 **UDP**，目标端口 `51820`，重定向目标 IP `10.0.0.50`（服务端 LAN IP），重定向端口 `51820`。
  - 保存后记得放行对应防火墙规则（OPNsense 会提示自动生成）。
- 普通家用路由器：找「端口映射/虚拟服务器」，同理映射 UDP 51820 → 服务端 IP。

> 如果 WireGuard **直接装在 OPNsense 上**（方案 B），就不用端口转发，只需在 WAN 口放行 UDP 51820 的入站规则。

### 7.3 动态公网 IP → 配 DDNS

家宽公网 IP 通常会变。在 OPNsense `服务 → Dynamic DNS` 配一个（如 Cloudflare、阿里云 DDNS、花生壳等），拿到一个固定域名 `home.example.com`，填进客户端的 `Endpoint`。

---

## 8. 启动并验证

### 服务端

```bash
sudo wg-quick up wg0              # 启动隧道
sudo systemctl enable wg-quick@wg0  # 设为开机自启
sudo wg show                      # 查看状态（此时还没有客户端握手）
```

### 客户端

- **手机**：装 WireGuard App。服务端上用二维码导入最方便：
  ```bash
  qrencode -t ansiutf8 < client.conf   # 终端直接显示二维码，App 扫码导入
  ```
- **电脑（Linux）**：把 `client.conf` 放到 `/etc/wireguard/`，执行 `sudo wg-quick up client`。
- **Mac/Windows**：装官方 WireGuard 客户端，导入 `client.conf`。

### 验证三连（务必从外网环境测，比如手机关 WiFi 用 4G）

```bash
# 1. 看握手：服务端执行，出现 "latest handshake: xx seconds ago" 才算通
sudo wg show

# 2. 通隧道：客户端 ping 服务端隧道地址
ping 10.10.10.1

# 3. 通内网：客户端 ping 家里其它设备（如 OPNsense 后台）
ping 10.0.0.1
# 然后浏览器访问内网网站，成功 = 大功告成
```

---

## 9. 排障速查表（按现象定位）

| 现象 | 最可能原因 | 排查/解决 |
|------|-----------|-----------|
| `wg show` **完全没有 handshake** | 连不到服务端 | ①CGNAT（7.1）②端口转发没配/配成 TCP（要 UDP）③DDNS 域名解析过期 ④客户端 Endpoint 写错 |
| 有 handshake，但 **ping 不通 10.10.10.1** | 隧道地址/密钥配错 | 检查公私钥对应关系（第 3 步记忆法）、`Address` 写对 |
| 通 10.10.10.1，**ping 不通 10.0.0.x** | 服务端没转发/没 NAT | ①`sysctl net.ipv4.ip_forward` 是否为 1（第 4 步）②`PostUp` 的 MASQUERADE 网卡名是否写对（第 5 步 `eth0`）|
| 能 ping IP，**但内网域名打不开** | DNS 问题 | 客户端配置加 `DNS = 10.0.0.1` |
| 时通时断 | NAT 超时 | 客户端加 `PersistentKeepalive = 25` |
| 全局模式下能上家里网但**上不了外网** | NAT/转发链不全 | 检查 `FORWARD` 两条规则都在、MASQUERADE 出口网卡正确 |

调试利器：

```bash
sudo wg show                       # 看握手、收发字节数（收发都涨说明双向通）
sudo journalctl -u wg-quick@wg0    # 服务端日志
sudo iptables -t nat -L POSTROUTING -n -v   # 确认 MASQUERADE 规则生效
sudo tcpdump -i wg0 -n             # 抓隧道里的包，看流量到底走没走进来
```

---

## 10. 进阶：不做 NAT 的"更干净"路由方案

第 5 步用 MASQUERADE 是为了省事（LAN 设备无需配回程路由）。但它会**把所有 VPN 客户端的 IP 都伪装成服务端 IP**，LAN 设备日志里看不到真实来源。更规范的做法：

1. 服务端配置**去掉 MASQUERADE**，只留 FORWARD 放行。
2. 在**家庭网关（OPNsense）上加一条静态路由**：目标 `10.10.10.0/24`，网关指向 WireGuard 服务端的 LAN IP `10.0.0.50`。
3. 这样 LAN 设备回包时知道"`10.10.10.x` 要发回给服务端" → 保留真实客户端 IP，干净且可审计。

家用图省事用 NAT，要做访问控制/审计就用静态路由方案。

---

## 11. 方案 B：直接在 OPNsense GUI 配（对照参考）

你已经有 OPNsense，图形化更快：
1. `VPN → WireGuard → Instances`：新建实例，填隧道地址 `10.10.10.1/24`、监听端口 `51820`，生成密钥。
2. `VPN → WireGuard → Peers`：为每个客户端加 peer，填客户端公钥、`AllowedIPs = 10.10.10.2/32`。
3. `防火墙 → 规则 → WAN`：放行入站 `UDP 51820`。
4. `防火墙 → 规则 → WireGuard 组`：放行隧道流量到 LAN。
5. OPNsense 自动处理转发/NAT，不用手敲 iptables——但**原理和上面手搓完全一致**，建议先手搓理解，再用 GUI 就一通百通。

---

## 12. 方案 C：连公网 IP 都没有时的兜底——Tailscale

若卡在 CGNAT 又拿不到公网 IP：直接上 [Tailscale](https://tailscale.com)（基于 WireGuard），它用中继服务器自动完成 NAT 穿透和密钥分发，两端各装一个、登录同一账号即可互通。**5 分钟见效**，适合先体验"VPN 通了是什么感觉"，再回头手搓理解它替你解决了哪些难题（穿透、密钥交换、路由下发）。

---

## 附录：自我思考

**1. 为什么坚持先"手搓"再用 GUI？**
OPNsense / Tailscale 把 IP 转发、NAT、路由、密钥分发全部自动化了，好用但**屏蔽了原理**。一旦出问题（连不上、通隧道不通内网），不懂底层就只能瞎试。手搓一遍 WireGuard，本质是把"可达性 / IP 转发 / 路由"这三件事亲手过一遍——这三件事恰恰是**所有 VPN 问题的共同根因**，学会了能迁移到 OpenVPN、IPsec、甚至排查公司 VPN 故障。

**2. AllowedIPs 是理解 WireGuard 的钥匙。**
它在服务端是"身份白名单"，在客户端是"路由表"，同一个字段两种语义。想通这一点，分流 vs 全局、单客户端 vs 多客户端、能不能访问整个内网，全都顺了。WireGuard 的极简正体现在此：没有独立的路由协议，路由就藏在 AllowedIPs 里。

**3. 概念分层比记命令更重要。**
这次最大的收获不是命令，而是把 **VPN（接入网络，L3）** 和 **代理（转发流量，L7）** 彻底分开。一旦分层清晰，就不会再问"用 Clash 能不能访问家里 NAS"这种方向性错误的问题。技术选型的第一步永远是先问"我要解决的是哪一类问题"。

**4. 国内的特殊约束要前置。**
CGNAT / 没有公网 IP 是国内自建服务的头号拦路虎，且和技术无关——再完美的配置，包进不来也白搭。所以把"可达性检查"放在动手早期，而不是配完才发现连不上，是符合工程直觉的顺序。

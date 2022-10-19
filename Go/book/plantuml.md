plantuml：https://plantuml.com/zh/sequence-diagram

常用代码：https://juejin.cn/post/6844903839351455752

流程介绍：https://www.cnblogs.com/GuoYuying/p/14789182.html#3

Typora 绘图：https://zhuanlan.zhihu.com/p/172635547

IDEA 绘图：https://cloud.tencent.com/developer/article/1927441

```mermaid
sequenceDiagram
    actor u as 用户
    actor w as 运维
		participant c as 资产柜客户端
		participant cs as 资产柜服务端
		participant s as ITAM

    u->>c: 扫描二维码
    activate u
    activate c
    
    c->>cs: 请求数据
    activate cs
    
    cs->>s: 获取用户资产信息
    activate s
    s-->>cs: 回调信息
    deactivate s
    deactivate cs
    
    cs-->>c: 回调信息
    c-->>u: 展示数据
    deactivate u
    deactivate c
```

```mermaid
gantt
    title 甘特图
    dateFormat  YYYY-MM-DD
    section Section
    A task           :a1, 2020-01-01, 30d
    Another task     :after a1  , 20d
    section Another
    Task in sec      :2020-01-12  , 12d
    another task      : 24d
```


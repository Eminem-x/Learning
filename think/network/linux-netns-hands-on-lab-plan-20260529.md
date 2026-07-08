# 阶段 2 动手实验手册:在 lab VM 上手搓 namespace / veth / NAT

> 来源:个人学习规划(网络地基 · 阶段二「Linux 网络内功」落地)
> 收录时间:2026-05-29 16:20 CST
>
> 配套:`networking-infra-learning-roadmap-20260527.md`(总路线 · 阶段 2)、
> `nic-arp-lan-vs-router-nat-flow-20260528.md`(本手册要亲手验证的那张图)、
> `networking-routing-tunnel-nat-from-real-table-20260528.md`(路由/隧道/NAT 理论)。
>
> **实验环境**:multipass `lab` VM(Ubuntu 24.04,`192.168.252.2`)。
> **核心宗旨**:阶段 1 的理论你已经啃透,这一阶段不再读图,而是**把那张图亲手在 Linux 上跑出来、用 tcpdump 抓出来**。每个 Lab 都对应你笔记里的一个结论。

---

## 0. 为什么是这份计划(承上启下)

你阶段 1 的三篇笔记已经把这套心智模型立起来了:

> IP=终点,MAC=下一跳;出门两步「路由表算下一跳 IP → ARP 翻成 MAC」;同网段直连不经路由器;跨网段走网关;路由器=多网卡机器,NAT 发生在两块网卡交界。

**但这些目前全是「读懂的」,不是「造出来的」。** 阶段 2 的唯一目标,就是用 Linux network namespace 把上面每一句话变成你亲手敲出来、亲眼抓到包的东西。做完这 7 个 Lab,你对「软路由 / 容器网络 / VPN 转发」的底层会一次性看穿——因为它们用的就是同一套原语。

**毕业标准(路线图阶段 2)**:不查文档,手搓两个 network namespace,用 veth 连通,并让其中一个通过 NAT 访问外网,全程能用 tcpdump 解释每一个包。

---

## 心法:每个 Lab 三步走

1. **造**:敲命令把拓扑搭出来。
2. **抓**:用 `tcpdump` 抓包,对照笔记里那张图,确认「我以为会发生的」真的发生了。
3. **讲**:合上终端,白板/口头把刚才包的走向讲一遍。讲不出来 = 没学会,回去重抓。

> 进 VM:`multipass shell lab`。下面命令默认在 VM 里、用 `sudo` 跑。
> 建议每个 Lab 结束后 `sudo ip netns del <name>` 清场,保持环境干净、可随时重来。

---

## Lab 0:先在真机上确认阶段 1 学的东西(30 分钟)

**目的**:把 macOS 上用过的 `ifconfig`/`netstat -rn`/`arp -a`,换成 Linux 的现代命令(`ip`/`ss`),建立肌肉记忆。这是阶段 2 清单第一项「现代命令」。

```bash
ip addr            # 对应 ifconfig:看网卡和 IP
ip -br addr        # 简洁版,一行一个接口
ip route           # 对应 netstat -rn:看路由表,找到 default 网关
ip neigh           # 对应 arp -a:看 ARP 邻居缓存(L3→L2 翻译表)
ss -tunlp          # 对应 netstat:看本机监听的 TCP/UDP 端口
```

**对照练习**:在 VM 里 `ping 192.168.252.1`(网关)一下,再 `ip neigh`——你应该看到网关的 IP↔MAC 这条新缓存,亲眼复现你 `nic-arp` 笔记第 3 节的 ARP 缓存。

**毕业**:能说出 `ip route` 里哪条是 default、VM 出网走哪个网卡。

---

## Lab 1:tcpdump 抓真实流量,对上 ARP/DNS/TCP 三连(45 分钟)

**目的**:阶段 1 你是「读」抓包,这次自己抓。把 `nic-arp` 笔记里「出门两步」亲眼抓出来。

开两个终端(都 `multipass shell lab`):

```bash
# 终端 A:抓包(先清 ARP 缓存,才能抓到广播询问)
sudo ip neigh flush all
sudo tcpdump -ni any 'arp or port 53 or tcp[tcpflags] & (tcp-syn|tcp-fin) != 0'

# 终端 B:制造流量
curl -v https://example.com
```

**你要在终端 A 亲眼看到、并逐行对上笔记的**:
- `ARP, Request who-has 192.168.252.1 tell 192.168.252.2` → 笔记「广播问 MAC」
- `ARP, Reply 192.168.252.1 is-at ...` → 「单播答」
- 一条 `A? example.com` 的 DNS 查询 → 应用层
- `Flags [S]` → `[S.]` → `[.]` 的三次握手 → 笔记的 SYN/SYN-ACK/ACK

**毕业**:能指着抓包输出说出「这个包是第几步、源/目的为什么是这个」。

---

## Lab 2:第一个 network namespace——理解「独立协议栈」(30 分钟)

**目的**:搞懂 namespace 到底是什么。**一个 network namespace = 一套独立的网卡 / 路由表 / ARP 表 / 防火墙。** 这就是容器「网络隔离」的本质原语。

```bash
sudo ip netns add red                    # 造一个叫 red 的命名空间
sudo ip netns list                        # 看一眼
sudo ip netns exec red ip addr            # 进 red 看它的网卡:只有一个 lo,而且 DOWN
sudo ip netns exec red ip route           # 路由表:空的!它是个全新的、与世隔绝的网络世界
```

**关键体会**:`red` 里啥都没有——没网卡(除了 lo)、没路由、ping 不通任何东西。它是一台「刚出厂、网卡都没插」的虚拟机的网络部分。下一步就是给它「插网线」。

**毕业**:能解释「为什么 red 里 `ping 192.168.252.2` 不通」(它和主 namespace 是两个隔离的网络世界,没有任何连接)。

---

## Lab 3:两个 namespace + veth = 同网段直连(对应笔记「情况 A」)(60 分钟)★核心★

**目的**:亲手复现 `nic-arp` 笔记**情况 A:同网段直连,ARP 找对方,路由器不参与**。veth pair = 一根虚拟网线,两端各插一个 namespace。

```bash
sudo ip netns add red
sudo ip netns add blue

# 造一根虚拟网线 veth,两端 veth-r / veth-b
sudo ip link add veth-r type veth peer name veth-b
# 把两端分别塞进两个 namespace(像把网线两头插进两台机器)
sudo ip link set veth-r netns red
sudo ip link set veth-b netns blue

# 给两端配同网段 IP 并拉起来
sudo ip netns exec red  ip addr add 10.10.0.1/24 dev veth-r
sudo ip netns exec red  ip link set veth-r up
sudo ip netns exec red  ip link set lo up
sudo ip netns exec blue ip addr add 10.10.0.2/24 dev veth-b
sudo ip netns exec blue ip link set veth-b up
sudo ip netns exec blue ip link set lo up

# 见证奇迹
sudo ip netns exec red ping -c 3 10.10.0.2
```

**抓包验证(关键)**:开第三个终端先清缓存再抓,再 ping:
```bash
sudo ip netns exec red ip neigh flush all
sudo ip netns exec red tcpdump -ni veth-r   # 然后另一个终端发 ping
```
你会看到先 `ARP who-has 10.10.0.2`、`is-at ...`,再 `ICMP echo`。`ip netns exec red ip neigh` 能看到 blue 的 MAC 缓存。

**这就是笔记「情况 A」的实体**:同网段 → 直接 ARP 对方 → 直连,中间没有任何「路由器」做三层转发。
**毕业**:不看手册,能从零搭出这套并 ping 通,并讲清 ARP 在这里干了啥。

---

## Lab 4:三个 namespace,中间一个当「路由器」(对应笔记「情况 B」)(90 分钟)★重头★

**目的**:复现笔记**情况 B:跨网段,下一跳=网关,dstMAC 写网关、dstIP 写终点**。造一个「路由器」namespace(双网卡),两边各接一个网段——这就是你笔记里画的「路由器=多网卡的机器」。

拓扑:
```
  red(10.0.1.2/24) ── 10.0.1.1 [router] 10.0.2.1 ── (10.0.2.2)blue
```

```bash
sudo ip netns add red
sudo ip netns add blue
sudo ip netns add router

# 两根 veth:red↔router、router↔blue
sudo ip link add veth-r type veth peer name r-red
sudo ip link add veth-b type veth peer name r-blue
sudo ip link set veth-r netns red
sudo ip link set r-red  netns router
sudo ip link set veth-b netns blue
sudo ip link set r-blue netns router

# red 侧网段 10.0.1.0/24
sudo ip netns exec red    ip addr add 10.0.1.2/24 dev veth-r
sudo ip netns exec router ip addr add 10.0.1.1/24 dev r-red
# blue 侧网段 10.0.2.0/24
sudo ip netns exec blue   ip addr add 10.0.2.2/24 dev veth-b
sudo ip netns exec router ip addr add 10.0.2.1/24 dev r-blue
# 全部 up(含各自 lo)
for ns in red blue router; do sudo ip netns exec $ns ip link set lo up; done
sudo ip netns exec red    ip link set veth-r up
sudo ip netns exec blue   ip link set veth-b up
sudo ip netns exec router ip link set r-red up
sudo ip netns exec router ip link set r-blue up

# 关键 1:给 red/blue 配「默认网关」= 路由器对应那侧的 IP
sudo ip netns exec red  ip route add default via 10.0.1.1
sudo ip netns exec blue ip route add default via 10.0.2.1

# 关键 2:打开路由器的「转发」开关(否则它收到不是自己的包会丢弃)
sudo ip netns exec router sysctl -w net.ipv4.ip_forward=1

# 见证:红能 ping 通蓝(跨网段,经路由器三层转发)
sudo ip netns exec red ping -c 3 10.0.2.2
```

**抓包验证(最能打通任督二脉的一步)**:在 router 两侧分别抓:
```bash
sudo ip netns exec router tcpdump -ni r-red    # 看 red 侧
sudo ip netns exec router tcpdump -ni r-blue   # 看 blue 侧(再开一个终端)
```
对照笔记「IP 一路不变,MAC 每跳都换」:同一个 ICMP 包,**两侧抓到的 IP(10.0.1.2→10.0.2.2)完全相同,但以太网 MAC 不同**——进来时 dstMAC 是路由器 r-red,出去时 srcMAC 变成 r-blue。这就是「每过一跳 MAC 都换、IP 不变」的实锤。

**毕业**:① 能讲清「为什么必须给 red 配 default via 10.0.1.1」「为什么必须开 ip_forward」;② 能从抓包指出 MAC 变了、IP 没变。

---

## Lab 5:给「内网」做 SNAT 出公网(对应笔记第 5 节 NAT,毕业标准)(90 分钟)★毕业★

**目的**:复现笔记**NAT 出门完整流程**。让 `red`(私网 10.0.1.0/24)通过 router 做 SNAT/masquerade,经 VM 的真网卡访问外网——这就是你家路由器对你 Mac 做的事,现在你亲手造一遍。

延续 Lab 4 的拓扑,把 router 的「外网口」接到 VM 主网络。最简做法:router 直接用 VM 的真网卡出网(把 router 的默认路由指向 VM,VM 再 NAT;或在 router 里直接 masquerade)。这里给**在 router 里直接 masquerade 到 VM 网络**的最小版:

```bash
# 在 Lab 4 基础上,给 router 再接一条到「外面」的路(用一对 veth 连回主 namespace)
sudo ip link add veth-ext type veth peer name r-ext
sudo ip link set r-ext netns router
sudo ip addr add 192.168.99.1/24 dev veth-ext           # 主 namespace 侧
sudo ip link set veth-ext up
sudo ip netns exec router ip addr add 192.168.99.2/24 dev r-ext
sudo ip netns exec router ip link set r-ext up
sudo ip netns exec router ip route add default via 192.168.99.1   # router 默认走出去

# 在 router 上对来自 10.0.1.0/24 的包做 SNAT(masquerade)——核心一条
sudo ip netns exec router nft add table ip nat
sudo ip netns exec router nft 'add chain ip nat postrouting { type nat hook postrouting priority 100 ; }'
sudo ip netns exec router nft add rule ip nat postrouting ip saddr 10.0.1.0/24 oif r-ext masquerade

# 主 namespace(VM)要愿意转发 red 的包并 NAT 到真外网
sudo sysctl -w net.ipv4.ip_forward=1
sudo iptables -t nat -A POSTROUTING -s 192.168.99.0/24 -j MASQUERADE   # 或对应 nft
sudo ip route add 10.0.1.0/24 via 192.168.99.2                          # 回程路由

# 见证:red 直接 ping 外网
sudo ip netns exec red ping -c 3 1.1.1.1
```

**抓包验证(看见 NAT 改写)**:
```bash
# 在 router 内网侧看「改之前」的源 IP
sudo ip netns exec router tcpdump -ni r-red icmp
# 在 router 外网侧看「改之后」的源 IP
sudo ip netns exec router tcpdump -ni r-ext icmp
```
对照笔记第 5 节:内网侧源是 `10.0.1.2`,外网侧源被改成了 router 的地址——**亲眼看到 SNAT 把私网源地址换掉**。`nft list ruleset` / `conntrack -L`(装 `conntrack-tools`)能看到那张「NAT 账本」。

**毕业(=阶段 2 毕业标准)**:不查文档,从零搭两个 namespace+veth,让其中一个 NAT 出外网,并用抓包讲清 SNAT 在哪一步、改了什么。

---

## Lab 6:把 namespace 换成 bridge——逼近真实容器网络(选做,60 分钟)

**目的**:Docker 默认网络(docker0)就是「一个 Linux bridge + 每个容器一条 veth」。用 `ip link add type bridge` 把多个 namespace 挂到同一座网桥上,你就看穿了 `docker network` 的魔法。

要点:`ip link add br0 type bridge` → 把每个 namespace 的 veth 一端 `ip link set <veth> master br0` → bridge 当二层交换机。做完对比一句:**Lab 3 的 veth 直连 = 两台机器对插网线;bridge = 给它们接了台交换机。**

**毕业**:能用一句话说清「docker0 和我手搓的 br0 是不是一回事」。

---

## Lab 7:用 netns 模拟 VPN 隧道——回收阶段 1 最后一个悬念(选做,进阶)

**目的**:你 `nic-arp` 笔记结尾写的「下一步:把 VPN 隧道叠回这套流程」。在两个 namespace 间起一条 WireGuard,亲眼看「信封套信封」:

- 装 `wireguard-tools`,在 red/blue 各起一个 wg 接口,配公私钥(呼应路线图 1.5 公私钥验明身份)。
- `tcpdump` 抓**底层 UDP**(看到的是加密后的密文)+ 抓 **wg 接口内侧**(看到明文)——这就是隧道「外层信封是 UDP、内层是真实 IP 包」的实体。
- 顺手读一遍 `wg-quick` 这个 shell 脚本(路线图阶段 2 指定),看它到底帮你 `ip link add`/`ip addr`/`ip route` 了哪些——你会发现它干的就是 Lab 3~5 那套手活。

**毕业**:能讲清「VPN 不过是在出门流程前,先让虚拟网卡把包交给程序加密再封一层 UDP」——讲完这条,阶段 1 的最后一块拼图也合上了。

---

## 节奏与打卡

| Lab | 主题 | 对应笔记 | 预估 | 状态 |
|:---:|------|---------|:---:|:---:|
| 0 | 现代命令 `ip`/`ss` | nic-arp Lab0 | 30m | ☐ |
| 1 | tcpdump 抓 ARP/DNS/TCP | nic-arp §3、路线图 1.3 | 45m | ☐ |
| 2 | 单个 namespace | —— | 30m | ☐ |
| 3 | veth 同网段直连 ★ | nic-arp 情况 A | 60m | ☐ |
| 4 | 三 ns + 路由器 ★ | nic-arp 情况 B | 90m | ☐ |
| 5 | SNAT 出公网 ★毕业 | nic-arp §5 NAT | 90m | ☐ |
| 6 | bridge / 容器网络 | (选做) | 60m | ☐ |
| 7 | netns 里的 WireGuard | 路线图 1.5 / 阶段2 | 进阶 | ☐ |

**建议**:Lab 0~2 一晚拿下;Lab 3~5 是命根子,**每个都做到「合上手册能从零重搭+抓包讲清」再走**,这就是路线图说的「不能快进、要练到肌肉记忆」的部分。Lab 6/7 看兴趣。

**全部做完 = 阶段 2 毕业**,你对「软路由/容器/VPN 转发的底层」会一次性通透,然后才进阶段 3(Proxmox/Docker/自托管)就全是水到渠成的工程组合。

---

## 附录:自我思考

- **为什么是 namespace 而不是开三台 VP/VM**:network namespace 是「最便宜的整套协议栈」,一条命令一个、随手删,正好让你把「网卡/路由/ARP/NAT」这几样原语反复搭、反复抓,练到形成直觉。而且它本就是容器网络的底层——练它=同时看穿 Docker/K8s 网络,一举两得。
- **这份计划刻意「抓包驱动」**:阶段 1 你最大的收获是那张「出门两步 + IP不变MAC换」的图。但图是别人(或过去的你)总结的,容易停在「我以为我懂了」。每个 Lab 都要求亲手 tcpdump 把图里的每个包抓出来对上——这是把「读懂」夯成「确信」的唯一办法,也呼应心法的三角循环。
- **Lab 4 的「MAC 变、IP 不变」是整条路线的题眼**:一旦你在 router 两侧抓到同一个包的 IP 一致、MAC 不同,「IP=终点、MAC=下一跳、每跳换 MAC」就从背诵变成了亲眼所见。之后看任何路由/转发/VPN,你都能在脑子里自动展开成这套抓包画面。
- **毕业标准依旧是「能做+能讲」**:每个 Lab 末尾的毕业条件都不是「敲完命令」,而是「合上手册能重搭、能讲清为什么」。能把 `ip_forward`、`default via`、`masquerade` 这三处「为什么必须」讲明白,才算这阶段真的过关。

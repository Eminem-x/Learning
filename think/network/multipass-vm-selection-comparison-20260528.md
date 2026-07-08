# 网络实验环境选型：为什么选 Multipass，VMware / 其它怎么样

> 来源：个人学习（网络地基 · 进入阶段二「动手」前的选型）
> 收录时间：2026-05-28 17:06 CST
>
> 本机真实配置（选型依据）：**Apple M1 Pro / arm64 / macOS 15.5 / 10 核 / 16GB 内存**，
> 已装 Homebrew 与 Docker，未装 Multipass。
> 目标：搭一个能「随便搞坏」的环境，亲手验证已学的网卡 / ARP / 路由 / NAT，并往 Linux 网络 / 运维方向打地基。

---

## 0. 先回答：为什么非得开一台 Linux 虚拟机？Mac 上不能学吗？

不能（指真正的「网络栈」那部分）。原因很实在：

| | macOS（你的本机） | Linux（要学的目标） |
|---|---|---|
| 内核血统 | BSD | Linux |
| 命令工具 | `ifconfig` / `netstat -rn` / `pfctl` / `route` | `ip addr` / `ip route` / `ip neigh` / `iptables`·`nft` |
| **network namespace** | **没有** | **有**（核心！） |
| 防火墙 / NAT 框架 | pf | netfilter（iptables / nftables） |

关键在 **network namespace**：它能让你在**一台 Linux 机器里**，凭空造出好几台「独立的计算机」（各有自己的网卡、路由表、ARP 表），再用「虚拟网线」把它们连起来。**这正是练今天所有概念的最佳沙盒** —— 而 macOS 根本没有这个机制（BSD 不支持）。

> 所以结论倒过来看很清楚：**真正的学习发生在「一台 Linux 机器内部」**（用 namespace 搭迷你网络）。虚拟机本身只是「一个能随便搞坏的 Linux 盒子」。
> 那么选型问题就变成一句话：**用最低的摩擦，拿到一个能 root、能随便重装的 Linux 盒子。**

而且，Linux 网络栈正是 99% 的服务器、云、容器、路由器（包括你的 OPNsense 底层）跑的东西。学它 = 学整个基础设施行业的通用语言。

---

## 1. 主流方案横向对比（针对 Apple Silicon / M1 Pro）

| 方案 | 类型 | 价格 | M1(arm64) 支持 | 上手摩擦 | 适合 | 不适合 |
|------|------|------|---------------|---------|------|--------|
| **Multipass** ⭐ | 轻量 Linux VM（CLI） | 免费 | 原生（Apple 虚拟化框架） | **极低**：一条命令开机 | 快速拿一台可破坏的 Ubuntu，namespace 实验 | 想要 GUI 桌面 / 跑非 Ubuntu / 多 VM 拉二层拓扑 |
| **Lima** | 轻量 Linux VM（CLI） | 免费 | 原生 | 低（YAML 配置） | 同 Multipass，但更可定制、多发行版 | 想要图形界面 |
| **UTM** | 通用 VM（QEMU 前端，GUI） | 免费 | 原生 arm64；可**模拟** x86（慢） | 中 | 要 GUI、要跑各种 OS、偶尔要 x86 | 追求极致性能 |
| **VMware Fusion** | 全功能 Hypervisor（GUI） | **现已免费** | 仅 arm64 客户机 | 中高 | 要多台完整 VM + **自定义虚拟交换机/网络拓扑** + GUI | 只想快速搞台 Linux 练命令 |
| **Parallels Desktop** | 全功能 Hypervisor（GUI） | 付费（订阅/买断） | 最佳性能 | 中 | 愿意花钱买顺滑体验、跑 Windows | 预算敏感、纯 CLI 学习 |
| **VirtualBox** | 全功能 Hypervisor | 免费 | ⚠️ **仅开发者预览，不稳定** | 中 | Intel Mac 上很稳 | **你的 M1 Pro：基本排除** |
| **Docker**（已装） | 容器（共享内核） | 免费 | 原生 | 低 | 跑服务、后面玩 Containerlab | **手搓 namespace 学习**：被 Docker 封装了，学不到底层 |
| **GNS3 / EVE-NG / Containerlab** | 网络仿真平台 | 免费 | 视镜像而定 | **高** | 跑**真实路由器/交换机镜像**、画大型拓扑（CCNA 风格） | 刚入门、只想理解原理 |

> 说明几个容易踩的点（不含糊）：
> - **VirtualBox 在 Apple Silicon 上是开发者预览版**，毛病多，你这台 M1 Pro 别选。
> - **VMware Fusion 现在免费**了，但它的强项是「GUI + 多台完整 VM + 自定义虚拟网络」，对「快速拿台 Linux 练命令」属于杀鸡用牛刀。
> - **Docker 你已经装了**，但容器是「被管理好的 namespace」——你想学的恰恰是**手动 `ip netns` 那一层**，Docker 帮你做完了，反而学不到。Docker 留着以后玩 Containerlab。

---

## 2. 为什么这个目标下，Multipass 是最优解

1. **摩擦最低**：从零到「我在一台 Linux 里」总共三条命令、不到一分钟（见下文动手部分）。学习的精力应该花在 namespace 上，不是花在装 VM、配网络、点 GUI 上。
2. **原生快**：在 M1 上走 Apple 虚拟化框架，arm64 Ubuntu 跑得飞快，不是模拟。
3. **能随便搞坏、秒级重置**：`multipass delete lab && multipass purge` 一键清空，重开一台干净的，符合「要能随便搞坏重装」的要求。
4. **免费、无 GUI 负担、可脚本化**：纯命令行，适合你这种有编程基础的，后面能写脚本批量起环境。
5. **你的目标不需要别的**：手搓 namespace 是**在一台 Linux 内部**完成的，不需要多台 VM、不需要图形界面、不需要非 Ubuntu 系统。那些是 VMware/Parallels 的强项，但你这阶段用不上。

**Multipass 的诚实短板**（将来可能撞到）：
- 官方主打 Ubuntu，跑别的发行版不方便。
- **多台 Multipass VM 之间拉二层拓扑**比较弱（默认各自 NAT），要做「多台真机互联」不如 VMware 的虚拟交换机顺手——但我们用 namespace 在单机内造拓扑，绕开了这个短板。
- 没有 GUI。对你不是问题。

---

## 3. 装好后具体能做什么：在一台 VM 里造一个「迷你互联网」

核心玩法：用 **network namespace** 当「计算机」、**veth pair** 当「网线」、**Linux bridge** 当「交换机」、**开了转发的 namespace** 当「路由器」、**iptables** 当「NAT」，最后用 **tcpdump** 看包。

一个典型实验拓扑（全部在一台 Ubuntu 里）：

```
  [ host-A ]          [ host-B ]
 10.1.0.10            10.1.0.11
     │                    │
     └─────┬──────────────┘
       [ br0 ]  ← Linux bridge（虚拟交换机，L2）
           │ 10.1.0.1
     ┌─────┴─────┐
     │  router    │  ← 开 ip_forward 的 namespace（L3）
     │            │     做 MASQUERADE（NAT）
     └─────┬─────┘
           │ 10.2.0.1
       [ host-C ]  10.2.0.10   ← 另一个网段
```

你能亲手看到、并用 `tcpdump` 抓包验证：
- host-A ping host-B（**同网段**）：走 bridge，ARP 广播「谁是 10.1.0.11」，**不经过 router**。
- host-A ping host-C（**跨网段**）：先 ARP 找网关 router，再由 router 转发；包的 **dst MAC 每跳换、dst IP 不变**。
- 从内网访问「外面」：在 router 上开 `MASQUERADE`，看**源地址被 NAT 改写**、回包按 NAT 表改回——今天那套流程，逐字节复现。

---

## 4. 每个已学概念，在沙盒里学到什么（对照表）

| 已学概念 | 沙盒里对应的东西 | 关键命令（Linux） |
|---------|----------------|------------------|
| 网卡 NIC / 虚拟网卡 | veth pair（成对的虚拟网卡） | `ip link add veth0 type veth peer name veth1` |
| 一台机器=多块网卡 | router namespace 配两个接口 | `ip link set veth1 netns router` |
| 网段 / CIDR | 给接口配 `/24` 地址 | `ip addr add 10.1.0.10/24 dev veth0` |
| 路由表 / 下一跳 / 最长前缀 | 每个 namespace 各自的路由表 | `ip route`，`ip route add default via ...` |
| ARP（IP→MAC） | Linux 邻居表 | `ip neigh`，`tcpdump -i veth0 arp` |
| 交换机（L2，认 MAC） | Linux bridge | `ip link add br0 type bridge` |
| 路由器（L3，认 IP） | 开转发的 namespace | `sysctl -w net.ipv4.ip_forward=1` |
| NAT（SNAT / 出流量） | iptables / nftables | `iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE` |
| DHCP（可选进阶） | 在一端跑 dnsmasq | `dnsmasq --interface=...` |
| 抓包验证一切 | tcpdump / Wireshark | `tcpdump -i br0 -n -e`（`-e` 看 MAC） |

> namespace 的爽点：所有这些**不用开多台 VM**，全在一台机器内；`ip netns del host-A` 秒级清理，搞砸了重来零成本。这正是 macOS 给不了的（无 namespace），也是非选 Linux 不可的根本原因。

---

## 5. 更高阶（以后再说，先知道存在）

等你把 namespace 玩透、想进「跑真实网络设备 / 大型拓扑」：

- **Containerlab**（推荐，且你已装 Docker）：用 YAML 定义拓扑，拉起容器化的网络操作系统（FRR、Arista cEOS、SONiC 等），适合现代「云原生网络」方向。
- **GNS3 / EVE-NG**：经典网络实验台，能跑 Cisco/Juniper 真实镜像，CCNA/CCNP 备考常用；arm64 上跑 x86 镜像靠模拟，偏慢。
- 这些是「下一座山」，不是现在。**先用 Multipass + namespace 把原理摸透，再上它们会非常轻松。**

---

## 附录：自我思考

- **选型的本质不是「哪个 VM 强」，而是「学习目标需要什么」**。我的目标是理解 Linux 网络栈原理，真正的课堂在「一台 Linux 内部的 namespace」，所以 VM 只需满足「低摩擦、能 root、能随便毁」——这把看似复杂的选型直接坍缩成 Multipass。遇到选型纠结时，先把「我到底要学什么」写死，工具自然浮出来。
- **为什么不用已装的 Docker**：容器是「别人替我管好的 namespace」。要学底层，就得自己 `ip netns` 一步步搭，看见每一根 veth、每一条路由、每一次 ARP。工具越「贴心」，离原理越远——学习期要主动选「不贴心」的那条路。
- **macOS 学不了的根因**（值得记牢）：BSD 没有 network namespace，命令体系也和 Linux 两套。这不是「Mac 不行」，而是「要学的东西本就长在 Linux 上」。本机用来当控制台、抓真实 WiFi 包（验证 ARP/DHCP/路由那部分已经做过），Linux 栈的练习交给 VM。
- **硬件够用**：M1 Pro / 16GB 跑一两台轻量 Ubuntu + 一堆 namespace 绰绰有余，namespace 几乎不吃额外内存（共享一个内核）。
- **下一步**：装 Multipass → 开一台 Ubuntu → 进去先把 `ip a` / `ip route` / `ip neigh` 和今天 Mac 上的 `ifconfig` / `netstat -rn` / `arp -a` 一一对上（同样的概念、不同的命令），再开始搭第 3 节那张拓扑图。

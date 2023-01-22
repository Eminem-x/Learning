# Docker

> 安装可参考：https://yeasy.gitbook.io/docker_practice/install

## Debian

### 卸载旧版本

旧版本的 Docker 称为 `docker` 或者 `docker-engine`，使用一下命令卸载旧版本：

```shell
sudo apt-get remove docker \
               docker-engine \
               docker.io
```

### 使用 APT 安装

由于 apt 源使用 HTTPS 以确保软件下载过程中不被篡改，所以需要添加使用 HTTPS 传输的软件包以及 CA 证书：

```shell
sudo apt-get update

sudo apt-get install \
     apt-transport-https \
     ca-certificates \
     curl \
     gnupg \
     lsb-releas
```

为了确认所下载软件包的合法性，需要添加软件源的 GPG 密钥，

鉴于国内网络问题，强烈建议使用国内源：

```shell
curl -fsSL https://mirrors.aliyun.com/docker-ce/linux/debian/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
```

官方源如下：

````shell
curl -fsSL https://download.docker.com/linux/debian/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
````

> 以上命令会添加稳定版本的 Docker APT 源，如果需要测试版本的 Docker 请将 stable 改为 test。

### 安装 Docker

更新 apt 软件包缓存，并安装 `docker-ce`：

````shell
sudo apt-get update
sudo apt-get install docker-ce docker-ce-cli containerd.io
````

### 使用脚本自动安装

在测试或开发环境中 Docker 官方为了简化安装流程，提供了一套便捷的安装脚本，

Debian 系统上可以使用这套脚本安装，另外可以通过 `--mirror` 选项使用国内源进行安装：

```shell
curl -fsSL get.docker.com -o get-docker.sh
sudo sh get-docker.sh --mirror Aliyun
```

执行这个命令后，脚本就会自动的将一切准备工作做好，并且把 Docker 的稳定(stable)版本安装在系统中。

### 启动 Docker

```shell
sudo systemctl enable docker
sudo systemctl start docker
```

### 建立 Docker 用户组

默认情况下，`docker` 命令会使用 [Unix socket](https://en.wikipedia.org/wiki/Unix_domain_socket) 与 Docker 引擎通讯，

而只有 `root` 用户和 `docker` 组的用户才可以访问 Docker 引擎的 Unix socket，

出于安全考虑，一般 Linux 系统上不会直接使用 `root` 用户，因此，更好地做法是将需要使用 `docker` 的用户加入 `docker` 用户组，

建立 `docker` 组：`sudo groupadd docker`，

将当前用户加入 `docker` 组：`sudo usermod -aG docker $USER`，

退出当前终端并重新登录，进行测试。

### 测试 Docker 是否安装正确

```sh
docker run --rm hello-world

Unable to find image 'hello-world:latest' locally
latest: Pulling from library/hello-world
b8dfde127a29: Pull complete
Digest: sha256:308866a43596e83578c7dfa15e27a73011bdd402185a84c5cd7f32a88b501a24
Status: Downloaded newer image for hello-world:latest

Hello from Docker!
This message shows that your installation appears to be working correctly.

To generate this message, Docker took the following steps:
 1. The Docker client contacted the Docker daemon.
 2. The Docker daemon pulled the "hello-world" image from the Docker Hub.
    (amd64)
 3. The Docker daemon created a new container from that image which runs the
    executable that produces the output you are currently reading.
 4. The Docker daemon streamed that output to the Docker client, which sent it
    to your terminal.

To try something more ambitious, you can run an Ubuntu container with:
 $ docker run -it ubuntu bash

Share images, automate workflows, and more with a free Docker ID:
 https://hub.docker.com/

For more examples and ideas, visit:
 https://docs.docker.com/get-started/
```

若能正常输出以上信息，则说明安装成功。

# MacOS

直接通过 Homebrew 安装：`brew install --cask docker`，

然后在应用中找到 Docker 图标并点击运行，之后可以在终端通过命令检查安装后的 Docker 版本：`Docker --version`。
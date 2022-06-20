# Mac 安装 wget 的两种方法

## 一、传统的安装包

1. 从 ftp://ftp.gnu.org/gnu/wget/ 下载到最新的wget安装包到本地
2. 然后通过终端 `tar -zxvf` 命令解压到我们某个目录
3. 然后依次执行`./configure` 和 `make` 以及 `make install `命令

## 二、Homebrew

执行命令：`brew install wget`
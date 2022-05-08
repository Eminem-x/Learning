### Docker教程

#### 将当前用户添加到docker用户组

为了避免每次使用 `docker` 命令都需要加上 `sudo` 权限，可以将当前用户加入安装中自动创建的 `docker` 用户组(可以参考官方文档)：

<strong>出现Got permission denied while trying to connect to the Docker daemon socket at...问题的解决办法：</strong>

https://blog.csdn.net/weixin_43972154/article/details/123873175

`sudo usermod -aG docker $USER`
执行完此操作后，需要退出服务器，再重新登录回来，才可以省去 `sudo` 权限。

-----

#### 镜像（images）

```
docker pull ubuntu:20.04：拉取一个镜像
docker images：列出本地所有镜像
docker image rm ubuntu:20.04 或 docker rmi ubuntu:20.04：删除镜像ubuntu:20.04
docker [container] commit CONTAINER IMAGE_NAME:TAG：创建某个container的镜像
docker save -o ubuntu_20_04.tar ubuntu:20.04：将镜像ubuntu:20.04导出到本地文件ubuntu_20_04.tar中
docker load -i ubuntu_20_04.tar：将镜像ubuntu:20.04从本地文件ubuntu_20_04.tar中加载出来
```

-----

#### 容器(container)

````
docker [container] create -it ubuntu:20.04：利用镜像ubuntu:20.04创建一个容器。
docker ps -a：查看本地的所有容器
docker [container] start CONTAINER：启动容器
docker [container] stop CONTAINER：停止容器
docker [container] restart CONTAINER：重启容器
docker [contaienr] run -itd ubuntu:20.04：创建并启动一个容器
docker [container] attach CONTAINER：进入容器
先按Ctrl-p，再按Ctrl-q可以挂起容器
docker [container] exec CONTAINER COMMAND：在容器中执行命令
docker [container] rm CONTAINER：删除容器
docker container prune：删除所有已停止的容器
docker export -o xxx.tar CONTAINER：将容器CONTAINER导出到本地文件xxx.tar中
docker import xxx.tar image_name:tag：将本地文件xxx.tar导入成镜像，并将镜像命名为image_name:tag
docker export/import与docker save/load的区别：
export/import会丢弃历史记录和元数据信息，仅保存容器当时的快照状态
save/load会保存完整记录，体积更大
docker top CONTAINER：查看某个容器内的所有进程
docker stats：查看所有容器的统计信息，包括CPU、内存、存储、网络等信息
docker cp xxx CONTAINER:xxx 或 docker cp CONTAINER:xxx xxx：在本地和容器间复制文件
docker rename CONTAINER1 CONTAINER2：重命名容器
docker update CONTAINER --memory 500MB：修改容器限制
````

-----

#### 实战

进入 `Terminal`，然后：

````
# 将镜像上传到自己租的云端服务器
scp /var/lib/acwing/docker/images/docker_lesson_1_0.tar server_name:  

ssh server_name  # 登录自己的云端服务器

docker load -i docker_lesson_1_0.tar  # 将镜像加载到本地
docker run -p 20000:22 --name my_docker_server -itd docker_lesson:1.0  # 创建并运行docker_lesson:1.0镜像

docker attach my_docker_server  # 进入创建的docker容器
passwd  # 设置root密码
````

去云平台控制台中修改安全组配置，放行端口 `20000`。

返回 `Terminal`，即可通过 `ssh` 登录自己的 `docker` 容器：

`ssh root@xxx.xxx.xxx.xxx -p 20000  # 将xxx.xxx.xxx.xxx替换成自己租的服务器的IP地址`

<strong>如果 `-p 20000` 出现问题，参考：https://www.wbolt.com/how-to-fix-ssh-connection-refused.html</strong>

然后，可以仿照上节课内容，创建工作账户 `ycx`。

最后，可以参考 `4. ssh——ssh登录配置docker容器的别名和免密登录。`

------

#### 小Tips

如果 `apt-get` 下载软件速度较慢，可以参考清华大学开源软件镜像站中的内容，修改软件源。
# EMQX安装部署

1. 通过 docker 部署安装，跟随<a href="https://www.emqx.io/zh/downloads">官网</a>指引：`docker pull emqx/emqx:5.0.4`

2. 如果需要查看 docker 拉取后的镜像以及存储位置，参考博客：https://blog.csdn.net/sinat_28442665/article/details/114013807

   `docker info` 因为是通过服务器，并且一般为 Linux 系统，所以存储位置为 `/var/lib/docker`

3. docker 其他操作：

   ```
   docker pull ubuntu:20.04：拉取一个镜像
   docker images：列出本地所有镜像
   docker image rm ubuntu:20.04 或 docker rmi ubuntu:20.04：删除镜像ubuntu:20.04
   docker [container] commit CONTAINER IMAGE_NAME:TAG：创建某个container的镜像
   docker save -o ubuntu_20_04.tar ubuntu:20.04：将镜像ubuntu:20.04导出到本地文件ubuntu_20_04.tar中
   docker load -i ubuntu_20_04.tar：将镜像ubuntu:20.04从本地文件ubuntu_20_04.tar中加载出来
   ```

4. `docker run -d --name emqx -p 1883:1883 -p 8083:8083 -p 8084:8084 -p 8883:8883 -p 18083:18083 emqx/emqx:5.0.4`

5. 然后去所租服务器平台，开启对应的防火墙端口：1883、8083、8084、8883 即可

6. <a href="https://www.emqx.io/docs/zh/v5.0/getting-started/getting-started.html#%E7%89%88%E6%9C%AC%E9%80%89%E6%8B%A9">emqx5.0</a> 官网、<a href="https://www.emqx.io/docs/zh/v4.3/getting-started/dashboard.html#%E7%AE%80%E4%BB%8B">emqx4.x</a>官网，二者在配置文件上可以互补结合一起看

7. EMQX Dashboard 是一个 Web 应用程序，你可以直接通过浏览器来访问它，无需安装任何其他软件。

   当 EMQX 成功运行在你的本地计算机上且 EMQX Dashboard 被默认启用时，

   你可以访问 http://ip:18083 来查看你的 Dashboard，默认用户名是 `admin`，密码是 `public`。

8. 另外推荐一篇关于 MQTT 和 MQ 区别以及应用的文章：https://help.aliyun.com/document_detail/94521.html
# nginx

## Docker 安装

>镜像地址：https://hub.docker.com/_/nginx

1. 拉取 docker 镜像：`docker pull nginx`
2. 尝试运行一个 nginx 服务器：`docker run -d -p 80:80 --name webserver nginx`
3. 服务运行后，可以访问 http://locahost，如果访问成功 nginx，就说明运行成功
4. 停止 nginx 服务器：`docker stop webserver`
5. 重启 nginx 服务器：`docker restart webserver`
6. 删除 nginx 服务器：`docker rm webserver`

## Brew 安装

> 参考文章：https://blog.51cto.com/u_15127533/4318254

1. 安装 nginx：`brew install nginx`
2. 查看 nginx 版本：`nginx -v`
3. 运行 nginx：`nginx`
4. 查看进程：`ps aux | grep nginx`
5. 打开网页验证：`http:localhost:8080`
6. 关闭 nginx：`nginx -s stop`
7. 重启 nginx：`nginx -s reload`

-----

> nginx blog：https://dunwu.github.io/nginx-tutorial/#/nginx-quickstart
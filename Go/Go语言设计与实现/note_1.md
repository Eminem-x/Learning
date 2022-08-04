# 第一章 调试源代码

1. 获取 Go 源码：`git clone https://github.com/golang/go.git`
2. `cloc`命令获取：`brew install cloc`（相关介绍：https://juejin.cn/post/6844904015449309191）
3. 修改源代码后，执行 `./src/make.bash` 报错 `make.bash must be run from $GOROOT/src` ，
   只需要添加临时环境变量 `export GOROO=源码下载后pwd路径即可`，
   而后按照绝对路径运行程序即可（此时 `go run main.go` 为包管理器的二进制文件）
4. 编译中间代码：`go build -gcflags -S main.go` 获取 Go 语言编译后的汇编代码
5. `GOSSAFUNC=main go build main.go` 生成 `ssa.html` 文件，获得具体的编译优化过程
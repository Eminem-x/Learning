// @Title log
// @Description go package log
// @Author Yuanhao
// @Update 2022-08-25
package main

import (
	"bytes"
	"fmt"
	"log"
)

type User struct {
	Name string
	Age  int
}

var user = User{
	Name: "ycx",
	Age:  18,
}

// Blog: https://darjun.github.io/2020/02/07/godailylib/log/
func main() {
	stdLog()
	setLog()
	customLog()
}

func stdLog() {

	// If the log does not end with a newline, then the log will automatically add a newline.
	log.Printf("%s login, age: %d", user.Name, user.Age)
	// 输出日志后，以拼装好的字符串为参数调用panic
	log.Panicf("Oh, system error when %s login", user.Name)
	// 输出日志后，调用os.Exit(1)退出程序
	log.Fatalf("Danger! hacker %s login", user.Name)
}

func setLog() {
	log.SetPrefix("Login: ")
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds)
	log.Printf("%s login, age: %d", user.Name, user.Age)
}

func customLog() {
	buf := &bytes.Buffer{}
	// io.Writer：日志都会写到这个Writer中, 流氏日志也是采用这种方式传输到网络文件中
	// the core of package log is OutPut function
	logger := log.New(buf, "", log.Lshortfile|log.LstdFlags)

	logger.Printf("%s login, age:%d", user.Name, user.Age)

	fmt.Print(buf.String())
}

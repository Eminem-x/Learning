// @Title email
// @Description go package for email
// @Author Yuanhao
// @Update 2022-08-29
package main

import (
	"fmt"
	"log"
	"net/smtp"

	"github.com/jordan-wright/email"
)

// Official: https://github.com/jordan-wright/email
// Blog: https://darjun.github.io/2020/02/16/godailylib/email/
func main() {
	// 来到 qq 邮箱 设置 账户 打开 IMAP/SMTP 服务
	// qq 邮箱 SMTP 服务器地址 smtp.qq.com:25
	e := email.NewEmail()
	e.From = "xxx@qq.com"
	e.To = []string{"xxx@qq.com"}
	e.Subject = "Awesome Web"
	e.Text = []byte("Text Body is, of course, supported!")
	err := e.Send("smtp.qq.com:25", smtp.PlainAuth("", "xxx@qq.com", "yyy", "smtp.qq.com"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Send Successfully")
}

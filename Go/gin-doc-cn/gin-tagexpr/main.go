package main

import (
	"fmt"
	"github.com/bytedance/go-tagexpr/v2/validator"
)

type Request struct {
	ID       string `json:"id" tagexpr:"len($) == 1 && regexp('[0-1]')"` // 限制长度为 1,并且为 0｜1
	Username string `json:"username" tagexpr:"mblen($) <= 10"`           // mblen 长度不超过 10,包括中文字符以及特殊字符
	Password string `json:"password" tagexpr:"mblen($) <= 10"`
}

func main() {
	var tagexpr = validator.New("tagexpr")

	// 正确格式
	request := &Request{
		ID:       "1",
		Username: "ycx",
		Password: "ycx",
	}

	if tagexpr.Validate(request) != nil {
		fmt.Println("error")
	}

	fmt.Println(request)

	// ID、Username 不规范
	request = &Request{
		ID:       "2",
		Username: "123456789012345678901",
		Password: "ycx",
	}

	if tagexpr.Validate(&request) != nil {
		fmt.Println("error")
	}

	fmt.Println(request)
}

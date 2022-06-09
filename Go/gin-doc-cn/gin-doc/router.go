package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

func main() {
	router := gin.Default()

	// 测试 Get Post
	router.GET("/string/:name", func(c *gin.Context) {
		name := c.Param("name")
		fmt.Println(name)
	})

	// http://localhost:8080/welcome
	// http://localhost:8080/welcome?name=ycx
	// http://localhost:8080/welcome?name=ycx&lastname=yh
	router.GET("/welcome", func(c *gin.Context) {
		name := c.DefaultQuery("name", "Guest")
		lastname := c.Query("lastname")
		fmt.Printf("Hello %s %s!", name, lastname)
		fmt.Println()
	})

	// postman
	router.POST("/form", func(c *gin.Context) {
		types := c.DefaultPostForm("types", "alert")
		msg := c.PostForm("msg")
		title := c.PostForm("title")
		fmt.Printf("types are %s, msg is %s, title is %s", types, msg, title)
	})

	// 测试数据绑定
	type Login struct {
		User     string `form:"user" json:"user" binding:"required"`
		Password string `form:"password" json:"password" binding:"required"`
	}

	router.POST("/loginJSON", func(c *gin.Context) {
		var json Login
		if c.BindJSON(&json) == nil {
			if json.User == "manu" && json.Password == "123" {
				c.JSON(http.StatusOK, gin.H{"status": "you are logged in"})
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"status": "unauthorized"})
			}
		}
	})

	router.POST("/loginForm", func(c *gin.Context) {
		var form Login
		if c.Bind(&form) == nil {
			if form.User == "manu" && form.Password == "123" {
				c.JSON(http.StatusOK, gin.H{"status": "you are logged in"})
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"status": "unauthorized"})
			}
		}
	})

	// 简略的请求参数测试
	router.GET("/getRequest", func(c *gin.Context) {
		path := c.FullPath()
		url := c.Request.URL
		method := c.Request.Method
		c.SetCookie("cookie", "123", 100, "cookie", "cookie", false, false)
		fmt.Println(path + " " + url.Path + " " + method + " ")
	})

	// 测试响应
	router.GET("/moreJSON", func(c *gin.Context) {
		var msg struct {
			Name    string `json:"user" xml:"user"`
			Message string
			Number  int
		}
		msg.Name = "ycx"
		msg.Message = "hello"
		msg.Number = 123
		c.JSON(http.StatusOK, msg)
	})

	// 视图响应
	router.LoadHTMLGlob("template/*")
	router.GET("/index", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "Main Website",
		})
	})

	// 外部重定向
	router.GET("/redirect", func(c *gin.Context) {
		// http.StatusMovedPermanently为状态码301 永久移动 请求的页面已永久跳转到新的url
		// 如果请求错误 尝试清空浏览器缓存 因为状态码是永久移动
		// 返回301状态码进行跳转被Google认为是将网站地址由HTTP迁移到HTTPS的最佳方法
		c.Redirect(http.StatusMovedPermanently, "https://www.baidu.com")
	})

	// 内部重定向
	router.GET("/inner", func(c *gin.Context) {
		c.Request.URL.Path = "/index"
		router.HandleContext(c)
	})

	// 中间件 middleware 在实习中写的比较多

	router.Run()
}

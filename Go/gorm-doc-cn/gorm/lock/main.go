package main

import (
	"github.com/gin-gonic/gin"
	"gorm-lock/api"
	"gorm-lock/config"
)

func main() {
	config.InitDB()

	r := gin.Default()

	// test
	r.GET("/user", api.GetUserInfo)

	// 模拟多实例
	r.GET("/noLock1", api.UpdateUserWithoutLock)
	r.GET("/noLock2", api.UpdateUserWithoutLock)

	r.GET("/lock1", api.UpdateUserWithLock)
	r.GET("/lock2", api.UpdateUserWithLock)

	r.Run(":8080")
}

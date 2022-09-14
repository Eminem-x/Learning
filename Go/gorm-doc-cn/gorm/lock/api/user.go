package api

import (
	"github.com/gin-gonic/gin"
	"gorm-lock/service"
	"net/http"
)

func GetUserInfo(c *gin.Context) {
	nameLists := service.GetUserInfo()
	c.String(http.StatusOK, nameLists)
}

func UpdateUserWithoutLock(c *gin.Context) {
	c.String(http.StatusOK, service.UpdateUserInfoWithoutLock())
}

func UpdateUserWithLock(c *gin.Context) {
	c.String(http.StatusOK, service.UpdateUserInfoWithLock())
}

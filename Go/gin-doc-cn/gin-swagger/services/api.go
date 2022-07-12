package services

import (
	"gin-swagger/pkg"
	"gin-swagger/serializers"
	"github.com/gin-gonic/gin"
	"net/http"
)

// Login
// @Summary Login
// @Tags Admin
// @Produce json
// @Success 200 {object} pkg.SuccessResponse{data=serializers.LoginResponse{login=serializers.Login}}
// @Failure 404,500 {object} pkg.ErrorResponse
// @Router /login [get]
func Login(c *gin.Context) {
	_, _ = c.Writer.WriteString("Login!")

	// 通常需要封装好 response 和 serializer
	c.JSON(http.StatusOK, pkg.SuccessResponse{
		Status: "200 OK",
		Data:   serializers.SerializeLogin(),
	})
}

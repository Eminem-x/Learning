package main

import (
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm/config"
	"gorm/curd/query/paginate/paginator"
	"net/http"
	"time"
)

type User struct {
	gorm.Model
	Name         string
	Email        *string
	Age          uint8
	Birthday     time.Time
	MemberNumber sql.NullString
	ActivateAt   sql.NullTime
}

var db *gorm.DB

type SuccessResponses struct {
	Total  int64       `json:"total"`
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

func main() {
	db = config.InitDB()

	r := gin.Default()

	r.GET("/paginator", func(c *gin.Context) {
		// 分页一般是条件查询：先获取范围, 将结果存储到 scope
		scope := db.Model(&User{}).Where("name like ?", "ycx")

		// 将查询数量存储到 total
		var total int64
		scope.Count(&total)

		var users []User
		result := scope.Scopes(paginator.Paginate(c)).Find(&users)
		fmt.Println(result.RowsAffected)

		// 返回封装 具体的序列化
		c.JSON(http.StatusOK, SuccessResponses{
			Total:  total,
			Status: "200",
			//Data:   serializers.SerializeProductsResponse(products),
		})
	})

	r.Run()
}

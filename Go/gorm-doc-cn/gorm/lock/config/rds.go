package config

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	dsn := "root:password@tcp(127.0.0.1:3306)/gorm-doc?charset=utf8mb4&parseTime=True&loc=Local"
	// https://cloud.tencent.com/developer/article/1830807 gorm 输出执行的 sql
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	DB = db.Debug()
}

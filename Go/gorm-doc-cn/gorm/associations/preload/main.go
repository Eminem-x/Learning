package main

import (
    "fmt"
    "gorm.io/gorm"
    "gorm/config"
    "time"
)

var db *gorm.DB

type Product struct {
    gorm.Model // 自定义主键 ID
    Code       string
    Price      uint
}

// Get timestamp
func main() {
    db = config.InitDB()
    var product Product
    product.ID = 251
    db.Create(&product)
    fmt.Println(product.CreatedAt)
    time.Sleep(100000000)
    fmt.Println(uint(time.Since(product.CreatedAt).Seconds()))
}

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

    // 获取 duration
    fmt.Println(uint(time.Since(product.CreatedAt).Seconds()))

    // 获取 format 时间
    timestamp := product.UpdatedAt.Unix()
    tm := time.Unix(timestamp, 0)
    fmt.Println(tm.Format("2006-01-02 15:04:05")

    // 时间转为时间戳
    timestamp = strconv.FormatInt(product.UpdatedAt.Unix(), 10)
    fmt.Println(timestamp)
}

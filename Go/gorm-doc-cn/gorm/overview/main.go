package main

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Product struct {
	gorm.Model // 自定义主键 ID
	Code       string
	Price      uint
}

// 此为 GORM 官方文档的 overview 部分的详细实现
func main() {
	// 获得 GORM 框架：go get -u gorm.io/gorm
	// 获得 MySQL 数据库驱动：go get -u gorm.io/driver/mysql
	// 运行时替换配置中的 username,password,dbname
	dsn := "username:password@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
	// https://cloud.tencent.com/developer/article/1830807 gorm 输出执行的 sql
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// 迁移 schema：直接在数据库中创建表 products
	if err := db.AutoMigrate(&Product{}); err != nil {
		panic("failed to auto migrate")
	}

	db = db.Debug()

	var product Product

	// 以下都默认 主键 id = 1
	// Create
	db.Create(&Product{Code: "D42", Price: 100}) // 默认 id 自增

	// Read
	// select * from product where id = 1
	db.First(&product, 1) // 根据整型主键查询
	fmt.Printf("%s %d\n", product.Code, product.Price)

	// select * from product where code = D42 AND id = 1
	db.First(&product, "code = ?", "D42") // 防止 sql 注入
	fmt.Printf("%s %d\n", product.Code, product.Price)

	// Update
	db.Model(&product).Update("Price", 200)
	// 更新多个字段
	db.Model(&product).Updates(Product{Code: "F42", Price: 200}) // 注意这里更新非零值字段
	db.Model(&product).Updates(map[string]interface{}{"Price": 200, "Code": "F42"})

	// Delete 如果想重复运行 需要将此数据恢复
	db.Delete(&product, 1)
}

package main

import (
	"gorm.io/gorm"
	"gorm/config"
)

type Customer struct {
	gorm.Model
	Name       string
	Age        int64  `gorm:"default:21"` // Default Values
	Num        *int64 `gorm:"default:0"`
	CreditCard CreditCard
}

type CreditCard struct {
	gorm.Model
	Number     string
	CustomerId uint
}

var db *gorm.DB

func main() {
	db = config.InitDB()
	// Create users table in the db
	if err := db.AutoMigrate(&Customer{}); err != nil {
		panic("failed to auto migrate")
	}
	// Create users table in the db
	if err := db.AutoMigrate(&CreditCard{}); err != nil {
		panic("failed to auto migrate")
	}

	db.Create(&Customer{
		Name:       "jinZhu1",
		CreditCard: CreditCard{Number: "411111111111"},
	})

	// skip CreditCard
	db.Omit("CreditCard").Create(&Customer{
		Name:       "jinZhu2",
		CreditCard: CreditCard{Number: "422222222222"},
	})

	// Any zero value like '0', '', false won't be saved into the db, because the default value.
	// You might want to use pointer type or Scanner/Valuer to avoid this.
	var num int64
	num = 64
	db.Create(&Customer{
		Name:       "jinZhu3",
		Age:        0,    // the value will be 21 finally
		Num:        &num, // 64
		CreditCard: CreditCard{Number: "43333333333"},
	})

}

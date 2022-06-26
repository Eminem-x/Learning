package main

import (
	"fmt"
	"gorm.io/gorm"
	"gorm/config"
)

type User struct {
	gorm.Model
	Name       string
	CreditCard CreditCard
}

type CreditCard struct {
	gorm.Model
	Number string
	UserID uint // this field is necessary
}

var db *gorm.DB

func main() {
	db = config.InitDB()

	// CREATE TABLE `credit_cards` (`id` bigint unsigned AUTO_INCREMENT,`created_at` datetime(3) NULL,`updated_at` datetime(3) NULL,`deleted_at` datetime(3) NULL,
	// `number` longtext,`user_id` bigint unsigned,PRIMARY KEY (`id`),INDEX idx_credit_cards_deleted_at (`deleted_at`))
	if err := db.AutoMigrate(&CreditCard{}); err != nil {
		panic("failed to auto migrate")
	}
	// CREATE TABLE `users` (`id` bigint unsigned AUTO_INCREMENT,`created_at` datetime(3) NULL,`updated_at` datetime(3) NULL,`deleted_at` datetime(3) NULL,
	// `name` longtext,PRIMARY KEY (`id`),INDEX idx_users_deleted_at (`deleted_at`))
	if err := db.AutoMigrate(&User{}); err != nil {
		panic("failed to auto migrate")
	}

	retrieve()
	polymorphismAssociation()
	// Self-Referential Has One
	// FOREIGN KEY Constraints
}

func retrieve() {
	// Retrieve user list with edger loading credit card
	var users []User
	db.Model(&User{}).Preload("CreditCard").Find(&users)
	for _, v := range users {
		fmt.Print(v.Name + " ")
		fmt.Print(v.CreditCard)
		fmt.Println()
	}
}

type Cat struct {
	ID   int
	Name string
	Toy  Toy `gorm:"polymorphic:Owner;polymorphicValue:master"`
}

type Dog struct {
	ID   int
	Name string
	Toy  Toy `gorm:"polymorphic:Owner;"`
}

type Toy struct {
	ID        int
	Name      string
	OwnerID   int
	OwnerType string
}

func polymorphismAssociation() {
	// CREATE TABLE `toys` (`id` bigint AUTO_INCREMENT,`name` longtext,`owner_id` bigint,`owner_type` longtext,PRIMARY KEY (`id`))
	if err := db.AutoMigrate(&Toy{}); err != nil {
		panic("failed to auto migrate")
	}
	// CREATE TABLE `cats` (`id` bigint AUTO_INCREMENT,`name` longtext,PRIMARY KEY (`id`))
	if err := db.AutoMigrate(&Cat{}); err != nil {
		panic("failed to auto migrate")
	}
	//  CREATE TABLE `dogs` (`id` bigint AUTO_INCREMENT,`name` longtext,PRIMARY KEY (`id`))
	if err := db.AutoMigrate(&Dog{}); err != nil {
		panic("failed to auto migrate")
	}

	// INSERT INTO `toys` (`name`,`owner_id`,`owner_type`) VALUES ('toy1',1,'dogs') ON DUPLICATE KEY UPDATE `owner_type`=VALUES(`owner_type`),`owner_id`=VALUES(`owner_id`
	// INSERT INTO `dogs` (`name`) VALUES ('dog1')
	db.Create(&Dog{Name: "dog1", Toy: Toy{Name: "toy1"}})

	// You can change the polymorphic type value with tag polymorphicValue
	db.Create(&Cat{Name: "cat1", Toy: Toy{Name: "toy1"}})
}

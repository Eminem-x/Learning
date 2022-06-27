package main

import (
	"fmt"
	"gorm.io/gorm"
	"gorm/config"
	"time"
)

// Person has and belongs to many languages, `user_languages` is the join table
type Person struct {
	gorm.Model
	Languages []Language `gorm:"many2many:user_languages;"`
}

type Language struct {
	gorm.Model
	Name string
	// Back-Reference: Language contains persons
	Persons []Person `gorm:"many2many:user_languages;"`
}

var db *gorm.DB

func main() {
	db = config.InitDB()

	if err := db.AutoMigrate(&Language{}); err != nil {
		panic("failed to auto migrate")
	}
	if err := db.AutoMigrate(&Person{}); err != nil {
		panic("failed to auto migrate")
	}

	retrieve()
	selfReferential()
	customizeJoinTable()
}

func retrieve() {
	var people []Person
	// CREATE TABLE `employee_friends` (`employee_id` bigint unsigned,`friend_id` bigint unsigned,
	// PRIMARY KEY (`employee_id`,`friend_id`),CONSTRAINT `fk_employee_friends_employee` FOREIGN KEY (`employee_id`)
	// REFERENCES `employees`(`id`),CONSTRAINT `fk_employee_friends_friends` FOREIGN KEY (`friend_id`) REFERENCES `employees`(`id`))
	db.Model(&Person{}).Preload("Languages").Find(&people)
	for _, v := range people {
		fmt.Println(v.Languages)
	}

	var languages []Language
	db.Model(&Language{}).Preload("Persons").Find(&languages)
	for _, v := range languages {
		fmt.Println(v.Persons)
	}
}

type Employee struct {
	gorm.Model
	Friends []*Employee `gorm:"many2many:employee_friends"`
}

func selfReferential() {
	// self-referencing many2Many relationship
	if err := db.AutoMigrate(&Employee{}); err != nil {
		panic("failed to auto migrate")
	}
}

type Dog struct {
	ID        int
	Name      string
	Addresses []Address `gorm:"many2many:dog_addresses;"`
}

type Address struct {
	ID   uint
	Name string
}

type DogAddress struct {
	DogID     int `gorm:"primaryKey"`
	AddressID int `gorm:"primaryKey"`
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt
}

func customizeJoinTable() {
	if err := db.AutoMigrate(&Address{}); err != nil {
		panic("failed to auto migrate")
	}
	if err := db.AutoMigrate(&Dog{}); err != nil {
		panic("failed to auto migrate")
	}
	if err := db.AutoMigrate(&DogAddress{}); err != nil {
		panic("failed to auto migrate")
	}
	db.SetupJoinTable(&Dog{}, "Addresses", &DogAddress{})
}

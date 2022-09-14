package model

import (
	"gorm-lock/config"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type User struct {
	gorm.Model
	Name string
	Age  uint
}

func GetUsers() []User {
	var users []User
	config.DB.Find(&users)
	return users
}

func UpdateUserWithoutLock(user User) string {
	user.Name += "suffix"
	if err := config.DB.Updates(&user).Error; err != nil {
		return "fail"
	}
	return user.Name
}

func UpdateUserWithLock() string {
	var users []User
	time.Sleep(10 * time.Second)
	config.DB.Transaction(
		func(tx *gorm.DB) error {
			tx.Clauses(clause.Locking{Strength: "UPDATE"}).Find(&users)
			users[0].Name += "suffix"
			tx.Updates(&users[0])
			return nil
		})
	return users[0].Name
}

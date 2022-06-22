package main

import (
	"database/sql"
	"gorm.io/gorm"
	"gorm/config"
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

func main() {
	db = config.InitDB()

	deleteARecord()
	deleteWithPrimaryKey()
	batchDelete()
	sortAndPermanentlyDelete()
}

func getLastUser() (user *User) {
	db.Last(&user)
	return
}

func deleteARecord() {
	// 保证数据可测性 删除表中最后一个元素
	user := getLastUser()

	db.Delete(&user)
}

func deleteWithPrimaryKey() {
	// UPDATE `users` SET `deleted_at`='2022-06-22 10:13:12.723' WHERE `users`.`id` IN (10,11,12) AND `users`.`deleted_at` IS NULL
	db.Delete(&User{}, []int{10, 11, 12}) // &User{} or &[]User{} all work.
}

func batchDelete() {
	// UPDATE `users` SET `deleted_at`='2022-06-22 10:15:03.135' WHERE age = 21 AND `users`.`deleted_at` IS NULL
	db.Where("age = ?", 21).Delete(&User{})
}

func sortAndPermanentlyDelete() {
	// above examples are sort delete
	// you can find soft deleted records with Unscoped
	var users []User
	db.Unscoped().Where("age = 21").Find(&users)

	var user User
	user.ID = 22
	// DELETE FROM `users` WHERE `users`.`id` = 22
	db.Unscoped().Delete(&user)
}

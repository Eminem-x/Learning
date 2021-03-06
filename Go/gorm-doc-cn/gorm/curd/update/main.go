package main

import (
	"database/sql"
	"fmt"
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

	savaAllFields()
	updateSingleColumn()     // update single column
	updatesMultipleColumns() //updates multiple columns
	updateSelectedFile()     // Select or Omit
	batchUpdates()
	blockGlobalUpdates()
}

func savaAllFields() {
	var user User
	db.First(&user)
	fmt.Println(user.ID)

	user.Name = "jinZhu 2"
	user.Age = 100
	// Save will save all fields when performing the Updating SQL
	db.Save(&user)
}

func updateSingleColumn() {
	db.Model(&User{}).Where("age = ?", 100).Update("name", "YuanHao")

	// when using the Model method and its value has a primary value (ID),
	// the primary key will be used to build the condition
	var user User
	user.ID = 1
	db.Model(&user).Update("name", "jinZhu")
	// UPDATE users SET name='jinZhu', updated_at='2022-06-21 17:21:10' WHERE id=1;

	// If add Where, the condition will be combined with the primary key
	db.Model(&user).Where("age = ?", 100).Update("name", "YuanHao")
}

func updatesMultipleColumns() {
	var user User
	user.ID = 1

	// Declare the val's type to pointer which also can avoid this problem

	// UPDATE `users` SET `updated_at`='2022-06-21 18:08:51.48',`name`='ycx' WHERE `users`.`deleted_at` IS NULL AND `id` = 1
	db.Model(&user).Updates(User{Name: "ycx", Age: 0})

	// UPDATE `users` SET `age`=0,`name`='ycx',`updated_at`='2022-06-21 18:09:47.425' WHERE `users`.`deleted_at` IS NULL AND `id` = 1
	db.Model(&user).Updates(map[string]interface{}{"name": "ycx", "age": 0})
}

func updateSelectedFile() {
	var user User
	user.ID = 1

	// UPDATE `users` SET `name`='hello',`updated_at`='2022-06-21 18:22:45.211' WHERE `users`.`deleted_at` IS NULL AND `id` = 1
	db.Model(&user).Select("name").Updates(map[string]interface{}{"name": "ycx", "age": 18})

	// UPDATE `users` SET `age`=0,`name`='hello',`updated_at`='2022-06-21 18:23:16.88' WHERE `users`.`deleted_at` IS NULL AND `id` = 1
	db.Model(&user).Select("*").Updates(map[string]interface{}{"name": "ycx", "age": 0}) // it will affect non-zero fields

	// Select all fields but omit birthday and created_at (select all fields include zero value fields)
	// UPDATE `users` SET `id`=0,`updated_at`='2022-06-21 18:25:09.463',`deleted_at`=NULL,`name`='ycx',`email`=NULL,`age`=0,`member_number`=NULL,`activate_at`=NULL
	// WHERE `users`.`deleted_at` IS NULL AND `id` = 1
	db.Model(&user).Select("*").Omit("birthday", "created_at").Updates(User{Name: "ycx", Birthday: time.Time{}, Age: 0})
}

func batchUpdates() {
	var users []User
	users = append(users, User{Name: "ycx1"})
	users = append(users, User{Name: "ycx2"})
	users[0].ID = 1
	users[1].ID = 2

	//  UPDATE `users` SET `updated_at`='2022-06-21 19:25:17.648',`name`='ycx2' WHERE `users`.`deleted_at` IS NULL AND `id` = 2
	db.Model(&users[1]).Updates(&users[1])

	// UPDATE `users` SET `updated_at`='2022-06-21 19:26:27.037',`name`='ycx',`age`=18 WHERE age = '18' AND `users`.`deleted_at` IS NULL
	db.Model(User{}).Where("age = ?", "18").Updates(User{Name: "ycx", Age: 18})
}

func blockGlobalUpdates() {
	result := db.Model(&User{}).Update("name", "jinzhu")
	fmt.Println(result.Error)
}

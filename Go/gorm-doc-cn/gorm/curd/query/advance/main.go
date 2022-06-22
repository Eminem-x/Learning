package main

import (
	"database/sql"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

	smartSelectFields()
	Locking()
	inWithMultipleColumns()
	namedArgument()
	firstOrInit()
	firstOrCreate()
	scopes()
	count()
}

func smartSelectFields() {
	// GORM allows select specific fields with Select, if you often use this in your application,
	// maybe you want to define a smaller struct for API usage which can select specific fields automatically
	type APIUser struct {
		ID   uint
		Name string
	}

	var users []APIUser
	// SELECT `users`.`id`,`users`.`name` FROM `users` WHERE `users`.`deleted_at` IS NULL LIMIT 10
	db.Model(&User{}).Limit(10).Find(&users)
	fmt.Println(users)
}

func Locking() {
	// GORM supports different types of locks.
	var users []User
	// SELECT * FROM `users` WHERE `users`.`deleted_at` IS NULL FOR UPDATE
	db.Clauses(clause.Locking{Strength: "UPDATE"}).Find(&users)
	fmt.Println(users)
}

func inWithMultipleColumns() {
	var users []User
	// SELECT * FROM `users` WHERE (name, age, email) IN (('jinzhu',18,'admin'),('jinzhu2',19,'user')) AND `users`.`deleted_at` IS NULL
	db.Where("(name, age, email) IN ?", [][]interface{}{{"jinzhu", 18, "admin"}, {"jinzhu2", 19, "user"}}).Find(&users)

}

func namedArgument() {
	var user User
	//  SELECT * FROM `users` WHERE (name = 'jinzhu' OR email = 'jinzhu') AND `users`.`deleted_at` IS NULL ORDER BY `users`.`id` LIMIT 1
	db.Where("name = @name OR email = @name", map[string]interface{}{"name": "jinzhu"}).First(&user)

}

func firstOrInit() {
	var user User
	user.ID = 65535

	// Get first matched record or initialize a new instance with given conditions
	db.FirstOrInit(&user, User{Name: "non_existing"})
	fmt.Println(user)

	// initialize struct with more attributes if record not found
	// those Attrs won’t be used to build SQL query
	db.Where(User{Name: "ycx"}).Attrs(User{Age: 21}).FirstOrInit(&user)
	fmt.Println(user)

	// Assign attributes to struct regardless it is found or not
	// those attributes won’t be used to build SQL query and the final data won’t be saved into database
	db.Where(User{Name: "non_existing"}).Assign(User{Age: 20}).FirstOrInit(&user)
	fmt.Println(user)
}

func firstOrCreate() {
	// It's similar with firstOrInit except create.
}

func scopes() {
	// Scopes allows you to specify commonly-used queries which can be referenced as method calls
	var users []User
	// SELECT * FROM `users` WHERE age > 20 AND `users`.`deleted_at` IS NULL
	db.Scopes(AgeGreaterThan20).Find(&users)
	fmt.Println(users)
}

func AgeGreaterThan20(db *gorm.DB) *gorm.DB {
	return db.Where("age > ?", 20)
}

func count() {
	var count int64
	// SELECT count(*) FROM `users` WHERE (name = 'jinZhu' OR name = 'ycx') AND `users`.`deleted_at` IS NULL
	db.Model(&User{}).Where("name = ?", "jinZhu").Or("name = ?", "ycx").Count(&count)
	fmt.Println(count)
}

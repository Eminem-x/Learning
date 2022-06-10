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
	// Create users table in the db
	if err := db.AutoMigrate(&User{}); err != nil {
		panic("failed to auto migrate")
	}

	createRecord() // create 3 records into the users table
	batchInsert()  // create 6 records into the users table
}

func createRecord() {
	// Create Record
	user := User{Name: "JinZhu", Age: 18, Birthday: time.Now()}
	result := db.Create(&user) // pass pointer of data to Create
	if result == nil {
		return
	}
	fmt.Printf("user.ID: %d", user.ID)
	fmt.Printf("result.RowsAffected: %d", result.RowsAffected)

	// Create Record With Selected Fields

	// pay attention to the user's ID is defined.
	// create a record and assign a value to the fields specified which will ignore the ID,
	// so this record won't make error: Duplicate entry '*' for key 'users.PRIMARY'.
	db.Select("Name", "Age", "CreatedAt").Create(&user)

	// create a record and ignore the values for fields passed to omit,
	// however, it doesn't ignore the ID, so it will make the duplicate error.
	// db.Omit("Name", "Age", "CreatedAt").Create(&user)
	db.Omit("ID", "Name", "Age", "CreatedAt").Create(&user) // add ID filed into Omit
}

func batchInsert() {
	// pass a slice to the Create method
	var users = []User{
		{Name: "jinZhu1", Birthday: time.Now()},
		{Name: "jinZhu2", Birthday: time.Now()},
		{Name: "jinZhu3", Birthday: time.Now()}}
	db.Create(&users)

	for _, user := range users {
		fmt.Printf("%d ", user.ID)
	}
	fmt.Println()

	// specify batch size when creating with CreateInBatches
	users = []User{
		{Name: "jinZhu4", Birthday: time.Now()},
		{Name: "jinZhu5", Birthday: time.Now()},
		{Name: "jinZhu6", Birthday: time.Now()}}
	db.CreateInBatches(&users, 1)
}

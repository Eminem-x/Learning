package main

import (
	"database/sql"
	"errors"
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

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	// when create through a slice, it will prevent the current create if false
	if u.Name == "jinZhu8" {
		return errors.New("invalid name")
	}
	return
}

var db *gorm.DB

func main() {
	db = config.InitDB()
	// Create users table in the db
	if err := db.AutoMigrate(&User{}); err != nil {
		panic("failed to auto migrate")
	}

	createRecord()      // create 3 records into the users table
	batchInsert()       // create 6 records into the users table
	createHooks()       // create 2 records into the users table
	createFromMap()     // create 3 records into the users table
	createFromSqlExpr() // learn this when need
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

func createHooks() {
	// failed
	var users1 = []User{
		{Name: "jinZhu7", Birthday: time.Now()},
		{Name: "jinZhu8", Birthday: time.Now()}}
	db.Create(&users1)

	// success
	var users2 = []User{
		{Name: "jinZhu9", Birthday: time.Now()},
		{Name: "jinZhu10", Birthday: time.Now()}}
	db.Create(&users2)

	db.Session(&gorm.Session{SkipHooks: true}).Create(&users2)
}

func createFromMap() {
	// when creating from map, hooks won't be invoked,
	// associations won't be saved and primary key values won't be back filled.
	db.Model(&User{}).Create(map[string]interface{}{
		"Name": "jinZhu11", "Age": 18,
	})

	// batch insert from `[]map[string]interface{}{}`
	db.Model(&User{}).Create([]map[string]interface{}{
		{"Name": "jinZhu8", "Age": 18}, // hooks won't be invoked
		{"Name": "jinZhu12", "Age": 20},
	})

	// what's the meaning of key values won't be, db has three records.
	// https://stackoverflow.com/questions/72597438/gorm-create-from-map
}

// learn this when need
func createFromSqlExpr() {
	// GORM allows insert data with SQL expression
	// there are two ways to achieve this goal, create from map[string]interface{} or Customized Data Types,

	// create from map[string]interface{}
	db.Model(User{}).Create(map[string]interface{}{
		"Name":     "jinZhu13",
		"Location": clause.Expr{SQL: "ST_PointFromText(?)", Vars: []interface{}{"POINT(100 100)"}},
	})

	// customized data types
}

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

	stringConditions()
	structOrMapConditions()
	inlineCondition()
	notCondition()
	orCondition()
	selectingSpecificFields()
	orderAndDistinct()
	limitAndOffset()
}

func stringConditions() {
	// this demo is easy but used mostly
	var users []User

	// LIKE
	db.Where("name LIKE ?", "%jin%").Find(&users)
	// SELECT * FROM users WHERE name LIKE '%jin%';

	// AND
	db.Where("name = ? AND age >= ?", "jinzhu", "22").Find(&users)
	// SELECT * FROM users WHERE name = 'jinzhu' AND age >= 22;

	// Time
	lastWeek := time.Time{}
	db.Where("updated_at > ?", lastWeek).Find(&users)
	// SELECT * FROM users WHERE updated_at > '2000-01-01 00:00:00';

	// BETWEEN
	today := time.Time{}
	db.Where("created_at BETWEEN ? AND ?", lastWeek, today).Find(&users)
}

func structOrMapConditions() {
	var user User
	// Struct
	db.Where(&User{Name: "jinzhu", Age: 20}).First(&user)
	// SELECT * FROM users WHERE name = "jinzhu" AND age = 20 ORDER BY id LIMIT 1;

	var users []User
	// Map
	db.Where(map[string]interface{}{"name": "jinzhu", "age": 20}).Find(&users)
	// SELECT * FROM users WHERE name = "jinzhu" AND age = 20;

	// Slice of primary keys, this maybe useful which simply code
	db.Where([]int64{20, 21, 22}).Find(&users)
	// SELECT * FROM users WHERE id IN (20, 21, 22);

	// When querying with struct, GORM will only query with non-zero fields
	db.Where(&User{Name: "jinzhu", Age: 0}).Find(&users)
	// SELECT * FROM users WHERE name = "jinzhu";

	// To include zero values in the query conditions, you can use a map,
	// which will include all key-values as query conditions, for example:
	db.Where(map[string]interface{}{"Name": "jinzhu", "Age": 0}).Find(&users)
	// SELECT * FROM users WHERE name = "jinzhu" AND age = 0;
}

func inlineCondition() {
	var user User
	// Get by primary key if it were a non-integer type
	db.First(&user, "id = ?", "string_primary_key")
	// SELECT * FROM users WHERE id = 'string_primary_key';

	// Plain SQL
	db.Find(&user, "name = ?", "jinzhu")
	// SELECT * FROM users WHERE name = "jinzhu";

	var users []User
	db.Find(&users, "name <> ? AND age > ?", "jinzhu", 20)
	// SELECT * FROM users WHERE name <> "jinzhu" AND age > 20;

	// Struct
	db.Find(&users, User{Age: 20})
	// SELECT * FROM users WHERE age = 20;

	// Map
	db.Find(&users, map[string]interface{}{"age": 20})
	// SELECT * FROM users WHERE age = 20;
}

func notCondition() {
	// Build NOT conditions, works similar to Where

	var user User
	db.Not("name = ?", "jinzhu").First(&user)
	// SELECT * FROM users WHERE NOT name = "jinzhu" ORDER BY id LIMIT 1;

	var users []User
	// Not In
	db.Not(map[string]interface{}{"name": []string{"jinzhu", "jinzhu 2"}}).Find(&users)
	// SELECT * FROM users WHERE name NOT IN ("jinzhu", "jinzhu 2");

	// Struct
	db.Not(User{Name: "jinzhu", Age: 18}).First(&user)
	// SELECT * FROM users WHERE name <> "jinzhu" AND age <> 18 ORDER BY id LIMIT 1;

	// Not In slice of primary keys
	db.Not([]int64{1, 2, 3}).First(&user)
	// SELECT * FROM users WHERE id NOT IN (1,2,3) ORDER BY id LIMIT 1;
}

func orCondition() {
	var users []User
	db.Where("role = ?", "admin").Or("role = ?", "super_admin").Find(&users)
	// SELECT * FROM users WHERE role = 'admin' OR role = 'super_admin';

	// Struct
	db.Where("name = 'jinzhu'").Or(User{Name: "jinzhu 2", Age: 18}).Find(&users)
	// SELECT * FROM users WHERE name = 'jinzhu' OR (name = 'jinzhu 2' AND age = 18);

	// Map
	db.Where("name = 'jinzhu'").Or(map[string]interface{}{"name": "jinzhu 2", "age": 18}).Find(&users)
	// SELECT * FROM users WHERE name = 'jinzhu' OR (name = 'jinzhu 2' AND age = 18);
}

func selectingSpecificFields() {
	var users []User
	db.Select("name", "age").Find(&users)
	// SELECT name, age FROM users;

	db.Select([]string{"name", "age"}).Find(&users)
	// SELECT name, age FROM users;

	db.Table("users").Select("COALESCE(age,?)", 42).Rows()
	// SELECT COALESCE(age,'42') FROM users;
}

func orderAndDistinct() {
	var users []User
	db.Order("age desc, name").Find(&users)
	// SELECT * FROM users ORDER BY age desc, name;

	// Multiple orders
	db.Order("age desc").Order("name").Find(&users)
	// SELECT * FROM users ORDER BY age desc, name;

	db.Distinct("name", "age").Order("name, age desc").Find(&users)
}

func limitAndOffset() {
	// Refer to Pagination for details on how to make a paginator
	// https://gorm.io/docs/scopes.html#pagination

	var users []User
	db.Limit(3).Find(&users)
	fmt.Println(users)

	// // Cancel limit condition with -1
	db.Limit(-1).Find(&users)
	fmt.Println(users)

	// You have an error in your SQL syntax;
	// check the manual that corresponds to your MySQL server version
	// for the right syntax to use near 'OFFSET 3' at line 1
	db.Offset(3).Find(&users) // error

	db.Limit(5).Offset(3).Find(&users)
	fmt.Println(users)
}

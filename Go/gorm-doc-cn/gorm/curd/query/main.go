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

	retrievingSingleObj()
	retrievingObjWithPK()
	retrievingAllObjs()
}

func retrievingSingleObj() {
	firstUser1 := User{
		Name: "jinZhu1",
	}
	firstUser1.ID = 5

	// get the first record ordered by primary key ID
	db.First(&firstUser1)
	fmt.Println(firstUser1)

	firstUser2 := User{
		Name: "jinZhu1",
	}

	// get the first record ordered by primary key ID asc
	db.First(&firstUser2)
	fmt.Println(firstUser2)

	// Get one record, no specified order
	// but after test, if the ID isn't nil, the query will get the first record by ID
	// otherwise, the query will find the first record in mysql no specified order
	var takeUser User
	// takeUser.ID = 5
	// takeUser.Name = "JinZhu"
	// throw the debug sql, we can get the specified query sql
	db.Take(&takeUser)
	fmt.Println(takeUser)

	lastUser := User{
		Name: "JinZhu",
	}
	lastUser.ID = 5

	// Get last record, ordered by primary key desc
	db.Last(&lastUser)
	fmt.Println(lastUser)

	// or when the model is specified using db.Model()
	result := map[string]interface{}{}
	db.Model(&User{}).Last(&result)
	fmt.Println(result)

	// works because model is specified using `db.Model()`
	result = map[string]interface{}{}
	db.Model(&User{}).Last(result) // not pass a pointer, but still work, maybe because map
	fmt.Println(result)

	// the next demo is funny

	// doesn't work but not err
	result = map[string]interface{}{}
	db.Table("users").First(&result) // model value required: SELECT * FROM `users` ORDER BY `users`. LIMIT 1

	// works with Take
	result = map[string]interface{}{}
	db.Table("users").Take(&result) // cause take not have `order by`

	// no primary key defined, results will be ordered by first field (i.e., `Code`)
}

func retrievingObjWithPK() {
	var user User

	// 下面两条等价
	db.First(&user, 5)
	fmt.Println(user)

	db.First(&user, "5")
	fmt.Println(user)

	// 如果接受非数组，只会获得 id = 1 的record
	var users1 User
	db.Find(&users1, []int{1, 2, 3})
	fmt.Println(users1)

	// 数组，获得全部 user
	var users2 []User
	db.Find(&users2, []int{1, 2, 3})
	fmt.Println(users2)

	// When working with strings, extra care needs to be taken to avoid SQL injection
	// db.First(&user, "id = ?", "1b74413f-f3b8-409f-ac47-e8c062e3472a")

}

func retrievingAllObjs() {
	var users []User
	result := db.Find(&users) // db.Select("*").Scan(&users) also works, but duplicate
	fmt.Println(result.RowsAffected)
	fmt.Println(users)
}

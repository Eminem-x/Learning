package main

import (
	"fmt"

	validator "github.com/go-playground/validator/v10"
)

// Official: https://github.com/go-playground/validator
// Blog: https://darjun.github.io/2020/04/04/godailylib/validator/
func main() {
	basicValidate()
	crossStruct()
	customValidate()
}

type User struct {
	Name    string   `validate:"min=6,max=10"`
	Age     int      `validate:"min=1,max=100"`
	Sex     string   `validate:"oneof=male female"`
	Hobbies []string `validate:"unique"`
}

type RegisterForm struct {
	Name            string `validate:"min=2"`
	Age             int    `validate:"min=18"`
	Password        string `validate:"min=10"`
	ConfirmPassword string `validate:"eqfield=Password"`
}

func basicValidate() {
	validate := validator.New()
	user1 := User{
		Name:    "yuanhao",
		Age:     21,
		Sex:     "male",
		Hobbies: []string{"pingpong", "chess", "programming"},
	}
	err := validate.Struct(user1)
	fmt.Println(err)

	user2 := User{Name: "ycx", Age: 101, Sex: "man", Hobbies: []string{"pingpong", "pingpong"}}
	err = validate.Struct(user2)
	fmt.Println(err)
}

func crossStruct() {
	validate := validator.New()

	f1 := RegisterForm{
		Name:            "dj",
		Age:             18,
		Password:        "1234567890",
		ConfirmPassword: "1234567890",
	}
	err := validate.Struct(f1)
	if err != nil {
		fmt.Println(err)
	}

	f2 := RegisterForm{
		Name:            "dj",
		Age:             18,
		Password:        "1234567890",
		ConfirmPassword: "123",
	}
	err = validate.Struct(f2)
	if err != nil {
		fmt.Println(err)
	}
}

type UserForm struct {
	Name string `validate:"palindrome"`
	Age  int    `validate:"min=18"`
}

func customValidate() {
	validate := validator.New()
	validate.RegisterValidation("palindrome", CheckPalindrome)

	f1 := UserForm{
		Name: "djd",
		Age:  18,
	}
	err := validate.Struct(f1)
	if err != nil {
		fmt.Println(err)
	}

	f2 := UserForm{
		Name: "dj",
		Age:  18,
	}
	err = validate.Struct(f2)
	if err != nil {
		fmt.Println(err)
	}
}

func reverseString(s string) string {
	runes := []rune(s)
	for from, to := 0, len(runes)-1; from < to; from, to = from+1, to-1 {
		runes[from], runes[to] = runes[to], runes[from]
	}

	return string(runes)
}

func CheckPalindrome(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	return value == reverseString(value)
}

package service

import (
	"gorm-lock/model"
	"strings"
	"time"
)

func GetUserInfo() string {
	users := model.GetUsers()
	var nameLists []string
	for _, user := range users {
		nameLists = append(nameLists, user.Name)
	}

	return strings.Join(nameLists, "-")
}

func UpdateUserInfoWithoutLock() string {

	users := model.GetUsers()
	time.Sleep(10 * time.Second)
	res := model.UpdateUserWithoutLock(users[0])

	return res
}

func UpdateUserInfoWithLock() string {

	res := model.UpdateUserWithLock()

	return res
}

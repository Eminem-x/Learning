package models

import "gorm.io/gorm"

type Resource struct {
	gorm.Model
	Pid        uint   `gorm:"type:bigint;comment:PID"`
	Title      string `gorm:"type:varchar(255);comment:描述"`
	Type       int    `gorm:"type:bigint;comment:类型"`
	Component  string `gorm:"type:varchar(255);comment:组件"`
	Permission string `gorm:"type:varchar(255);comment:权限"`
	Hidden     bool   `gorm:"type:boolean;comment:隐藏"`
}

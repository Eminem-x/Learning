package models

import "gorm.io/gorm"

type SysRolesResource struct {
	gorm.Model
	Resource   Resource
	RoleID     uint `gorm:"type:bigint;comment:RoleId"`
	ResourceID uint `gorm:"type:bigint;comment:ResourceId"`
	RoleType   string
}

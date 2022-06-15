package models

import "gorm.io/gorm"

type Role struct {
	gorm.Model
	Name        string             `gorm:"unique;type:varchar(255);comment:角色名称"`
	Description string             `gorm:"type:longtext;comment:角色描述"`
	ResourceIDs []SysRolesResource `gorm:"polymorphic:Role"`
}

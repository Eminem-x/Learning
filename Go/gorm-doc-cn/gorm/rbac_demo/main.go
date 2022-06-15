package main

import (
	"fmt"
	"gorm.io/gorm"
	"gorm/config"
	"gorm/rbac_demo/models"
)

var db *gorm.DB

// 该 demo 非常糟糕，不适合处理多对多的关系，推荐使用 many2many
// 但是对于理解 Preload 却很有帮助，当然要结合官方文档
func main() {
	db = config.InitDB()

	// 模拟role 和 中间表
	role := models.Role{
		Name:        "gorm-poly---",
		Description: "gorm-poly",
		ResourceIDs: []models.SysRolesResource{
			models.SysRolesResource{
				ResourceID: 2,
			},
			models.SysRolesResource{
				ResourceID: 3,
			},
			models.SysRolesResource{
				ResourceID: 7,
			},
		},
	}

	// 创建 role 时同时也更新中间表
	db.Model(&models.Role{}).Create(&role)

	// 不会包含 resource
	db.Model(&models.Role{}).First(&role)
	for _, v := range role.ResourceIDs {
		fmt.Println(v.Resource.Title)
	}

	// 可以预加载出 resource
	db.Model(&models.Role{}).Preload("ResourceIDs").First(&role)
	for i, v := range role.ResourceIDs {
		var resource models.Resource
		db.Model(&models.Resource{}).First(&resource, v.ResourceID)
		role.ResourceIDs[i].Resource = resource
	}
	for _, v := range role.ResourceIDs {
		fmt.Println(v.Resource.Title)
	}

	// 删除中间表
	db.Model(&models.SysRolesResource{}).Where("role_id=?", role.ID).Unscoped().Delete(&models.SysRolesResource{})

	// 更新时也会更新中间表
	for i, v := range role.ResourceIDs {
		var resource models.Resource
		db.Model(&models.Resource{}).First(&resource, v.ResourceID)
		role.ResourceIDs[i].Resource = resource
	}
	db.Model(&models.Role{}).Updates(role)
}

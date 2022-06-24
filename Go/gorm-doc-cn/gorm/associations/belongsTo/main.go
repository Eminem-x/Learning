package main

import (
	"gorm.io/gorm"
	"gorm/config"
)

type Worker struct {
	gorm.Model
	Name      string
	CompanyID int
	// Foreign Key Constraints
	Company Company `gorm:"constraint:Onupdate:CASCADE,OnDelete:SET NULL;"`

	// Override Foreign Key: GORM provides a way to customize the foreign key.
	// CompanyRefer int
	// Company Company `gorm:"foreignKey:CompanyRefer"`

	// Override References: I think this is a bad operator which make code mess.
}

type Company struct {
	ID   int
	Name string
}

var db *gorm.DB

func main() {
	db = config.InitDB()

	if err := db.AutoMigrate(&Company{}); err != nil {
		panic("failed to auto migrate")
	}

	if err := db.AutoMigrate(&Worker{}); err != nil {
		panic("failed to auto migrate")
	}

	// CREATE TABLE `workers` (`id` bigint unsigned AUTO_INCREMENT,`created_at` datetime(3) NULL,`updated_at` datetime(3) NULL,
	// `deleted_at` datetime(3) NULL,`name` longtext,`company_id` bigint,PRIMARY KEY (`id`),INDEX idx_workers_deleted_at (`deleted_at`),
	// CONSTRAINT `fk_workers_company` FOREIGN KEY (`company_id`) REFERENCES `companies`(`id`) ON DELETE SET NULL ON UPDATE CASCADE)

	createCompany()
	updateCompany()
	deleteCompany()
}

func createCompany() {
	var company Company
	company.ID = 1
	company.Name = "ycx"
	db.Create(&company)

	worker := Worker{
		Name:      "ycx",
		CompanyID: 1,
		Company:   company,
	}
	db.Create(&worker)
}

func updateCompany() {
	// This will update Worker's companyID in the same time.
	db.Model(&Company{}).Where("name = ?", "ycx").Update("id", 2)
}

func deleteCompany() {
	// The worker's companyID filed is set as NULL
	db.Model(&Company{}).Where("name = ?", "ycx").Delete(&Company{})
}

package db

import "gorm.io/gorm"

type Todo struct {
	gorm.Model
	Name string
}

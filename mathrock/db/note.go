package db

import "gorm.io/gorm"

type Note struct {
	gorm.Model
	UserID  uint   `gorm:"uniqueIndex:idx_user_name"`
	Name    string `gorm:"type:varchar(255);uniqueIndex:idx_user_name" validate:"required,max=20"`
	Content string `validate:"max=50,required"`
	User    User   `gorm:"foreignKey:UserID"`
}

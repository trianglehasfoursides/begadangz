package db

import "gorm.io/gorm"

type Email struct {
	gorm.Model
	From       string  `json:"from" gorm:"not null"`
	To         string  `json:"to" gorm:"not null"`
	Mail       string  `json:"mail" gorm:"not null"`
	Subject    string  `json:"subject" gorm:"not null"`
	Attachment *string `json:"attachment"`
	UserID     int
	User       User `gorm:"foreignKey:UserID"`
}

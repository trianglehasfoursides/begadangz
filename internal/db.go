package internal

import (
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Setup() (err error) {
	dsn := os.Getenv("DB_URL")
	if DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{}); err != nil {
		return
	}

	return
}

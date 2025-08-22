package domain

import "gorm.io/gorm"

type Geo struct {
	gorm.Model
	Country  string `json:"country" validate:"country,omitempty"`
	Redirect string `json:"url" validate:"http_url,omitempty"`
	URLID    uint
}

package validate

import "github.com/go-playground/validator/v10"

var Valid *validator.Validate

func init() {
	Valid = validator.New()
}

package validate

import (
	"net/http"
	"regexp"
	"time"

	"github.com/go-playground/validator/v10"
)

var Valid *validator.Validate

func init() {
	Valid = validator.New()
	Valid.RegisterValidation("password", password)
	Valid.RegisterValidation("link", link)
	Valid.RegisterValidation("exp", exp)
	Valid.RegisterValidation("formatdate", format)
}

func password(fl validator.FieldLevel) bool {
	password := fl.Field().String()

	if len(password) < 8 || len(password) > 20 {
		return false
	}
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasNumber := regexp.MustCompile(`[0-9]`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[^a-zA-Z0-9]`).MatchString(password)

	return hasUpper && hasLower && hasNumber && hasSpecial
}

func link(fl validator.FieldLevel) bool {
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Head(fl.Field().String())
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

func exp(fl validator.FieldLevel) bool {
	dateStr := fl.Field().String()

	const layout = "02-01-2006"

	parsedDate, err := time.Parse(layout, dateStr)
	if err != nil {
		return false
	}

	today := time.Now()
	y1, m1, d1 := parsedDate.Date()
	y2, m2, d2 := today.Date()

	parsedOnly := time.Date(y1, m1, d1, 0, 0, 0, 0, time.Local)
	todayOnly := time.Date(y2, m2, d2, 0, 0, 0, 0, time.Local)

	return parsedOnly.After(todayOnly)
}

func format(fl validator.FieldLevel) bool {
	dateStr := fl.Field().String()

	const layout = "02-01-2006"

	_, err := time.Parse(layout, dateStr)
	if err != nil {
		return false
	}

	return true
}

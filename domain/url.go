package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/mileusna/useragent"
	"github.com/trianglehasfoursides/begadangz/auth"
	"github.com/trianglehasfoursides/begadangz/db"
	"github.com/trianglehasfoursides/begadangz/templ"
	"github.com/trianglehasfoursides/begadangz/validate"

	"gorm.io/gorm"
)

type UTMMap map[string]string

func (u UTMMap) Value() (driver.Value, error) {
	return json.Marshal(u)
}

func (u *UTMMap) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("Invalid UTMMap value format: %v", value)
	}
	return json.Unmarshal(bytes, &u)
}

type exp struct {
	Time string `json:"time,omitempty" validate:"omitempty,exp"`
	URL  string `json:"url,omitempty" validate:"omitempty,link"`
}

func (e exp) Value() (driver.Value, error) {
	return json.Marshal(e)
}

func (e *exp) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("Invalid expiration value format: %v", value)
	}
	return json.Unmarshal(bytes, &e)
}

type device struct {
	Android string `json:"android,omitempty" validate:"omitempty,link"`
	IOS     string `json:"ios,omitempty" validate:"omitempty,link"`
}

func (d device) Value() (driver.Value, error) {
	return json.Marshal(d)
}

func (d *device) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("Invalid device value format: %v", value)
	}
	return json.Unmarshal(bytes, &d)
}

type URL struct {
	gorm.Model
	UserID     string
	Name       string  `json:"name" gorm:"unique" validate:"required,max=50,alphanum"`
	Redirect   string  `json:"redirect" validate:"required,link"`
	Password   string  `json:"password,omitempty" validate:"omitempty,max=20,min=8"`
	Comments   string  `json:"comments,omitempty" validate:"max=300"`
	UTM        *UTMMap `json:"utm,omitempty"`
	Expiration *exp    `json:"exp,omitempty" validate:"omitempty"`
	Device     *device `json:"device,omitempty" validate:"omitempty"`
	Geos       []*Geo  `json:"geos,omitempty" gorm:"constraint:OnDelete:CASCADE;"`
}

// add new urls
func (u *URL) Add(ctx *gin.Context) {
	link := new(URL)
	if err := ctx.BindJSON(link); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body.",
		})
		return
	}

	if err := validate.Valid.Struct(link); err != nil {
		ers := []string{}
		if errs, ok := err.(validator.ValidationErrors); ok {
			for _, e := range errs {
				ers = append(ers, e.Error())
			}
		}

		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": strings.Join(ers, ";"),
		})
		return
	}

	query := url.Values{}
	for k, v := range *link.UTM {
		if err := validate.Valid.Var(k, "oneof=source medium campaign term content referral"); err != nil {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Unsupported UTM parameter: " + k,
			})
			return
		}
		if v == "" {
			continue
		}
		query.Set("utm_"+k, v)
	}

	if encoded := query.Encode(); encoded != "" {
		if strings.Contains(link.Redirect, "?") {
			link.Redirect += "&" + encoded
		} else {
			link.Redirect += "?" + encoded
		}
	}

	link.UserID = auth.UserId(ctx)
	if err := db.DB.Create(link).Error; err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			ctx.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error": "This URL name is already in use.",
			})
			return
		}

		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":  "Unable to create URL.",
			"detail": err.Error(), // bisa dibuang di prod
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "URL created successfully.",
		"data":    link,
	})
}

// view spesicif url
func (u *URL) View(ctx *gin.Context) {
	ip, err := netip.ParseAddr(ctx.ClientIP())
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid client IP address."})
		return
	}

	resp, err := http.Get("http://ip-api.com/json/" + ip.String())
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var info struct {
		Country string `json:"country"`
		Region  string `json:"regionName"`
		City    string `json:"city"`
		ISP     string `json:"isp"`
		Query   string `json:"query"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		panic(err)
	}

	host := ctx.Request.Host
	hostWithoutPort := strings.Split(host, ":")[0]
	name := strings.Split(hostWithoutPort, ".")
	if len(name) <= 2 {
		templ.View(ctx.Writer, "halo", map[string]any{
			"Name": "halo",
		})
	}

	link := new(URL)
	if err := db.DB.Preload("Geos").Where("name = ?", name[0]).First(link).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Requested URL not found."})
		return
	}

	if link.Expiration.Time != "" {
		exp, err := time.Parse("02-01-2006", link.Expiration.Time)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid expiration date format."})
			return
		}

		if !exp.IsZero() && time.Now().After(exp) {
			ctx.Redirect(http.StatusPermanentRedirect, link.Expiration.URL)
			return
		}
	}

	// password protection
	if link.Password != "" {
		if ctx.Query("password") != link.Password {
			templ.View(ctx.Writer, "password", map[string]any{
				"Name": name[0],
			})
			return
		}
	}

	ua := useragent.Parse(ctx.Request.UserAgent())
	args := struct {
		name      string
		timestamp time.Time
		country   string
		city      string
		region    string
		referer   string
		agent     string
		ip        string
		device    string
		os        string
		browser   string
	}{}

	args.name = link.Name
	args.timestamp = time.Now()
	args.country = info.Country
	args.city = info.City
	args.region = info.Region
	args.referer = ctx.GetHeader("referer")
	args.ip = ip.String()
	args.device = ua.Device
	args.os = ua.OS
	if ua.Bot {
		args.agent = "bot"
	}

	args.agent = "human"
	if ua.IsChrome() {
		args.browser = "Chrome"
	} else if ua.IsEdge() {
		args.browser = "Edge"
	} else if ua.IsFirefox() {
		args.browser = "Firefox"
	} else if ua.IsOpera() {
		args.browser = "Opera"
	} else if ua.IsOperaMini() {
		args.browser = "Opera Mini"
	} else if ua.IsSafari() {
		args.browser = "Safari"
	}

	if _, err := Click.Exec(
		`INSERT INTO clicks
        (name, timestamp, country, referer, user_agent, ip, device, os, browser, city, region)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		args.name,
		args.timestamp,
		args.country,
		args.referer,
		args.agent,
		args.ip,
		args.device,
		args.os,
		args.browser,
		args.city,
		args.region,
	); err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to record "})
	}

	if len(link.Geos) > 0 {
		for _, url := range link.Geos {
			if info.Country == url.Country {
				ctx.Redirect(http.StatusFound, url.Redirect)
				return
			}
		}
	}

	if ua.IsAndroid() && link.Device.Android != "" {
		ctx.Redirect(http.StatusTemporaryRedirect, link.Device.Android)
	} else if ua.IsIOS() && link.Device.IOS != "" {
		ctx.Redirect(http.StatusTemporaryRedirect, link.Device.IOS)
	}

	// default redirect
	ctx.Redirect(http.StatusPermanentRedirect, link.Redirect)
}

// get specific url
func (u *URL) Get(ctx *gin.Context) {
	name := ctx.Param("name")
	if name == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Missing URL name parameter.",
		})
		return
	}

	link := new(URL)
	if err := db.DB.Preload("Geos").Where("name = ? AND user_id = ?", name, auth.UserId(ctx)).First(link).Error; err != nil {
		fmt.Println("sjssj")
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch URL.",
		})
		return
	}

	ctx.JSON(http.StatusOK, link)
}

// edit specific url
func (u *URL) Edit(ctx *gin.Context) {
	name := ctx.Param("name")
	if name == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Missing URL name parameter.",
		})
		return
	}

	existing := new(URL)
	if err := db.DB.Where("name = ? AND user_id = ?", name, auth.UserId(ctx)).
		First(&existing).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": "URL not found.",
		})
		return
	}

	link := new(URL)
	if err := ctx.BindJSON(link); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body.",
		})
		return
	}

	if err := validate.Valid.Struct(link); err != nil {
		ers := []string{}
		if errs, ok := err.(validator.ValidationErrors); ok {
			for _, e := range errs {
				ers = append(ers, e.Error())
			}
		}
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": strings.Join(ers, ";"),
		})
		return
	}

	query := url.Values{}
	for k, v := range *link.UTM {
		if err := validate.Valid.Var(k, "oneof=source medium campaign term content referral"); err != nil {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Unsupported UTM parameter: " + k,
			})
			return
		}
		if v == "" {
			continue
		}
		query.Set("utm_"+k, v)
	}

	if encoded := query.Encode(); encoded != "" {
		if strings.Contains(link.Redirect, "?") {
			link.Redirect += "&" + encoded
		} else {
			link.Redirect += "?" + encoded
		}
	}

	err := db.DB.Model(existing).
		Where("name = ? AND user_id = ?", name, auth.UserId(ctx)).
		Updates(map[string]interface{}{
			"name":       link.Name,
			"redirect":   link.Redirect,
			"password":   link.Password,
			"comments":   link.Comments,
			"utm":        link.UTM,
			"expiration": link.Expiration,
			"device":     link.Device,
		}).Error
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":  "Unable to update URL.",
			"detail": err.Error(),
		})
		return
	}

	if err := db.DB.Model(&existing).
		Association("Geos").
		Replace(link.Geos); err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":  "Unable to update URL geolocation data.",
			"detail": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "URL updated successfully.",
	})
}

// delete specific url
func (u *URL) Delete(ctx *gin.Context) {
	name := ctx.Param("name")
	if name == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Missing URL name parameter.",
		})
		return
	}

	if err := db.DB.Unscoped().Where("name = ? AND user_id = ?", name, auth.UserId(ctx)).Delete(u).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete URL.",
		})
		return
	}

	var partitionName string
	err := Click.QueryRow(fmt.Sprintf(`SELECT partition
		FROM system.parts
		WHERE table = 'clicks'
		  AND database = 'default'
		  AND active = 1
		  AND partition = '%s'
		LIMIT 1`, name)).Scan(&partitionName)
	if err != nil {
		log.Fatal("Failed to retrieve partition:", err)
	}

	// Hapus partisi tersebut
	dropQuery := fmt.Sprintf(`ALTER TABLE clicks DROP PARTITION '%s'`, partitionName)
	_, err = Click.Exec(dropQuery)
	if err != nil {
		log.Fatal("Failed to drop partition:", err)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "URL deleted successfully.",
	})
}

func (u *URL) Clicks(ctx *gin.Context) {
	name := ctx.Param("name")
	if name == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Missing URL name parameter.",
		})
		return
	}

	if err := db.DB.Where("name = ? AND user_id = ?", name, auth.UserId(ctx)).First(u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{
				"error": "No URL found with name: " + name,
			})
			return
		}
	}

	var total int
	err := Click.QueryRow(`SELECT COUNT(*) FROM clicks WHERE name = ?`, name).Scan(&total)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve click count.",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"total": total,
	})
}

func (u *URL) Qr(ctx *gin.Context) {
	name := ctx.Param("name")
	if name == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Missing URL name parameter.",
		})
		return
	}

	if err := db.DB.Where("name = ? AND user_id = ?", name, auth.UserId(ctx)).First(u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{
				"error": "No URL found with name: " + name,
			})
			return
		}
	}

	png, err := qrcode.Encode("http://"+name+".localhost:8000", qrcode.High, 256)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.Data(http.StatusOK, "application/octet-stream", png)
}

package auth

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/google"
	"github.com/trianglehasfoursides/begadangz/db"
	"gorm.io/gorm"
)

var secret = os.Getenv("JWT_SECRET")

type User struct {
	ID    string
	Name  string
	Email string
}

func init() {
	goth.UseProviders(
		google.New(os.Getenv("GOOGLE_KEY"), os.Getenv("GOOGLE_SECRET"), "http://localhost:3000/auth/google/callback", "profile"),
		github.New(os.Getenv("GITHUB_KEY"), os.Getenv("GITHUB_SECRET"), "http://localhost:3000/auth/github/callback"),
	)
}

func Auth(ctx *gin.Context) {
	if ctx.GetHeader("Authorization") == "" {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	token := strings.Split(ctx.GetHeader("Authorization"), " ")[1]

	id, err := verify(token)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.Set("user_id", id)
	ctx.Next()
}

func UserId(ctx *gin.Context) string {
	id, _ := ctx.Get("user_id")
	return id.(string)
}

func Callback(ctx *gin.Context) {
	user, err := gothic.CompleteUserAuth(ctx.Writer, ctx.Request)
	if err != nil {
		slog.Error(err.Error())
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	var exist User

	result := db.DB.Where("email = ?", user.Email).First(&exist)

	var userID string
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			userID = uuid.NewString()
			newUser := User{
				ID:    userID,
				Name:  user.Name,
				Email: user.Email,
			}

			if err := db.DB.Create(&newUser).Error; err != nil {
				slog.Error(err.Error())
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
		} else {
			// Error lain selain record not found
			slog.Error(result.Error.Error())
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": result.Error.Error(),
			})
			return
		}
	} else {
		userID = exist.ID
	}

	token, err := create(user.Name, user.Email)
	if err != nil {
		slog.Error(err.Error())
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	url := fmt.Sprintf("http://localhost:7000/auth/callback?token=%s", token)
	ctx.Redirect(http.StatusTemporaryRedirect, url)
}

func create(username string, email string) (tkn string, err error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.MapClaims{
			"username": username,
			"email":    email,
			"exp":      time.Now().Add(time.Hour * 24).Unix(),
		})

	tkn, err = token.SignedString(secret)
	if err != nil {
		return
	}

	return
}

func Redirect(ctx *gin.Context) {
	q := ctx.Request.URL.Query()
	q.Add("provider", ctx.Param("provider"))
	ctx.Request.URL.RawQuery = q.Encode()
	gothic.BeginAuthHandler(ctx.Writer, ctx.Request)
}

func verify(token string) (id string, err error) {
	tkn, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		return secret, nil
	})

	if err != nil {
		return
	}

	if !tkn.Valid {
		return
	}

	claims := tkn.Claims.(jwt.MapClaims)
	id = claims["user_id"].(string)

	return
}

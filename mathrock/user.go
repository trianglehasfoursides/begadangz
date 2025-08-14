package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/google"
	"github.com/trianglehasfoursides/begadangz/mathrock/db"
	"gorm.io/gorm/clause"
)

type user struct {
	secret []byte
}

// singleton
var usr = new(user)

func (u *user) setup() {
	authKey, _ := base64.StdEncoding.DecodeString(os.Getenv("AUTH_KEY"))
	encryptKey, _ := base64.StdEncoding.DecodeString(os.Getenv("ENCRYPT_KEY"))
	gothic.Store = sessions.NewCookieStore(authKey, encryptKey)

	goth.UseProviders(
		google.New(os.Getenv("GOOGLE_ID"), os.Getenv("GOOGLE_SECRET"), "http://localhost:8000/auth/callback/google", "profile"),
		github.New(os.Getenv("GITHUB_ID"), os.Getenv("GITHUB_SECRET"), "http://localhost:8000/auth/callback/github"),
	)
}

func (u *user) verify(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		return u.secret, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("Invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("Invalid claims")
	}

	return claims, nil
}

func (u *user) authenticate() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Header("Content-Type", "application/json")
		thetoken := ctx.GetHeader("Authorization")
		if thetoken == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Missing authorization header",
			})
			return
		}
		thetoken = thetoken[len("Bearer "):]

		claims, err := u.verify(thetoken)
		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": jwt.ErrTokenExpired.Error(),
				})
				return
			}
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "An unexpected error occurred. Please try again.",
			})
			return
		}

		ctx.Set("user_id", claims["user_id"].(float64))
		ctx.Next()
	}
}

func (u *user) redirect(ctx *gin.Context) {
	query := ctx.Request.URL.Query()
	query.Add("provider", ctx.Param("provider"))
	ctx.Request.URL.RawQuery = query.Encode()

	gothic.BeginAuthHandler(ctx.Writer, ctx.Request)
}

func (u *user) callback(ctx *gin.Context) {
	query := ctx.Request.URL.Query()
	query.Add("provider", ctx.Param("provider"))
	ctx.Request.URL.RawQuery = query.Encode()

	meta, err := gothic.CompleteUserAuth(ctx.Writer, ctx.Request)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	user := &db.User{
		Name:  meta.Name,
		Email: meta.Email,
	}

	result := db.Db.
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(user)

	if result.Error != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Please try again latter",
		})
		return
	}

	if result.RowsAffected == 0 {
		db.Db.First(&user, "email = ?", user.Email)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.MapClaims{
			"user_id":  user.ID,
			"username": user.Name,
			"email":    user.Email,
			"exp":      time.Now().Add(time.Hour * 24).Unix(),
		})

	tokenString, err := token.SignedString(u.secret)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Please try again latter",
		})
		return
	}

	url := fmt.Sprintf("http://localhost:9000/auth/callback?token=%s&email=%s", tokenString, user.Email)
	ctx.Redirect(http.StatusTemporaryRedirect, url)
}

func (u *user) logout(ctx *gin.Context) {
	query := ctx.Request.URL.Query()
	query.Add("provider", ctx.Param("provider"))
	ctx.Request.URL.RawQuery = query.Encode()

	gothic.Logout(ctx.Writer, ctx.Request)
	ctx.Redirect(http.StatusTemporaryRedirect, "http://localhost:9000")
}

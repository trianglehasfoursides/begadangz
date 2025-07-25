package main

import (
	"encoding/base64"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/trianglehasfoursides/begadangz/mathrock/db"
)

func main() {
	// env
	godotenv.Load()

	// setup db
	db.Setup()

	// setup auth
	usr.setup()
	usr.secret, _ = base64.StdEncoding.DecodeString(os.Getenv("AUTH_KEY"))

	// // singleton for email
	// eml := &email{
	// 	client: resend.NewClient(os.Getenv("RESEND_API_KEY")),
	// }

	//singleton for note
	thenote := &note{}

	// migration
	mgrt := true
	if mgrt {
		if err := db.Db.
			AutoMigrate(&db.User{}, &db.Email{}, &db.Note{}); err != nil {
			panic(err.Error())
		}
	}

	router := gin.Default()
	router.Use(gin.Recovery())

	router.NoRoute(func(ctx *gin.Context) {})

	// router.GET("/auth/page", usr.page)
	router.GET("/auth/:provider", usr.redirect)
	router.GET("/auth/callback/:provider", usr.callback)
	router.GET("/auth/logout/:provider", usr.logout)

	router.Use(usr.authenticate())

	// email
	// router.POST("/email/send", eml.send)
	// router.GET("/email/history", eml.history)
	// router.GET("/email/:email_id", eml.view)
	// mail := router.Group("/email", gate("mail"))
	// mail.DELETE("/:email_id", eml.remove)

	// note
	router.POST("/note", thenote.add)
	router.GET("/note", thenote.list)
	note := router.Group("/note", gate("note"))
	note.GET("/:name", thenote.get)
	note.PUT("/:name", thenote.put)
	note.DELETE("/:name", thenote.remove)

	router.Run("localhost:8000")
}

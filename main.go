package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/trianglehasfoursides/begadangz/auth"
	"github.com/trianglehasfoursides/begadangz/click"
	"github.com/trianglehasfoursides/begadangz/db"
	"github.com/trianglehasfoursides/begadangz/geo"
	"github.com/trianglehasfoursides/begadangz/url"
)

func main() {
	// setup
	godotenv.Load()

	if err := db.Setup(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	click.Setup()
	if click.Click.Ping() != nil {
		fmt.Println("clickhousenya gak bisa")
		return
	}

	url, geo := new(url.URL), new(geo.Geo)

	if err := db.DB.AutoMigrate(url, geo); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	router := gin.Default()

	router.GET("/", url.View)

	// auth

	linkGroup := router.Group("/link")
	linkGroup.Use(auth.Auth)
	{
		linkGroup.POST("/add", url.Add)
		linkGroup.GET("/get/:name", url.Get)
		linkGroup.GET("/clicks/:name", url.Clicks)
		linkGroup.PUT("/put/:name", url.Edit)
		linkGroup.DELETE("/rm/:name", url.Delete)
	}

	router.Run("localhost:8000")
}

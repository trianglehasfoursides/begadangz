package event

import "github.com/gin-gonic/gin"

type Event struct {
	Title       string
	Description string
}

func Create(ctx *gin.Context) {
}

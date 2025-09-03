package internal

import "github.com/gin-gonic/gin"

func ErrResponse(msg string, err error) gin.H {
	if gin.EnvGinMode == "dev" {
		return gin.H{
			"error": err.Error(),
		}
	}

	return gin.H{
		"error": msg,
	}
}

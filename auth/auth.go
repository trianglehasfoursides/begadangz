package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trianglehasfoursides/cadwallader"
)

func Auth(ctx *gin.Context) {
	fmt.Println("awal auth")
	token := strings.Split(ctx.GetHeader("Authorization"), " ")[1]
	user, err := cadwallader.Verify("http://localhost:3000/jwks", token)

	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.Set("user_id", user.ID)
	fmt.Println("alhir auth auth")
	ctx.Next()
}

func UserId(ctx *gin.Context) string {
	id, _ := ctx.Get("user_id")
	return id.(string)
}

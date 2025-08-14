package main

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/trianglehasfoursides/begadangz/mathrock/db"
	"gorm.io/gorm"
)

var gates = make(map[string]gin.HandlerFunc)

func gate(name string) gin.HandlerFunc {
	return gates[name]
}

func init() {
	gates["mail"] = func(ctx *gin.Context) {
		idStr := ctx.Param("email_id")
		if idStr == "" {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Missing email_id"})
			return
		}

		id, err := strconv.Atoi(idStr)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid email_id"})
			return
		}

		var email db.Email
		if err := db.Db.Where("id = ? AND user_id = ?", id, int(ctx.GetFloat64("user_id"))).
			First(&email).
			Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Email not found"})
				return
			}
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		if email.UserID != int(ctx.GetFloat64("user_id")) {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		ctx.Next()
	}
	gates["note"] = func(ctx *gin.Context) {
		name := ctx.Param("name")
		if name == "" {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Missing note name"})
			return
		}

		var note db.Note
		if err := db.Db.Where("name = ? AND user_id = ?", name, int(ctx.GetFloat64("user_id"))).
			First(&note).
			Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Note not found"})
				return
			}
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		if note.UserID != uint(ctx.GetFloat64("user_id")) {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		ctx.Next()
	}
}

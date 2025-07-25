package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/trianglehasfoursides/begadangz/mathrock/db"
	"github.com/trianglehasfoursides/begadangz/validate"
	"gorm.io/gorm"
)

type note struct{}

func (n *note) add(ctx *gin.Context) {
	thenote := &db.Note{}
	ctx.BindJSON(thenote)
	thenote.UserID = uint(ctx.GetFloat64("user_id"))

	if err := validate.Valid.Struct(thenote); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	if err := db.Db.Create(thenote).Error; err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1062 {
			msg := fmt.Sprintf("Note with name '%s' already exists", thenote.Name)
			ctx.JSON(http.StatusBadRequest, gin.H{
				"error": msg,
			})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save your data. Please try again.",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status": "Succes",
	})
}

func (n *note) list(ctx *gin.Context) {
	userID := int(ctx.GetFloat64("user_id"))
	var notes []db.Note

	result := db.Db.Where("user_id = ?", userID).Find(&notes)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"error": "The requested item could not be found.",
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "An unexpected error occurred. Please try again.",
		})
		return
	}

	ctx.JSON(http.StatusOK, notes)
}

func (n *note) get(ctx *gin.Context) {
	name := ctx.Param("name")
	if name == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "missing email id"})
		return
	}

	note := &db.Note{}
	result := db.Db.
		Where("name = ? AND user_id = ?", name, int(ctx.GetFloat64("user_id"))).
		First(note)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, gin.H{
				"error": "The requested note could not be found.",
			})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error": "An unexpected error occurred. Please try again.",
			})
		}
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"ID":      note.ID,
		"title":   note.Name,
		"content": note.Content,
	})
}

func (n *note) put(ctx *gin.Context) {
	name := ctx.Param("name")
	userID := int(ctx.GetFloat64("user_id"))

	var existing db.Note
	err := db.Db.Where("name = ? AND user_id = ?", name, userID).First(&existing).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Note not found"})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	var payload struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := ctx.BindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	existing.Name = payload.Name
	existing.Content = payload.Content

	if err := db.Db.Save(&existing).Error; err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1062 {
			msg := fmt.Sprintf("Note with name '%s' already exists", existing.Name)
			ctx.JSON(http.StatusBadRequest, gin.H{
				"error": msg,
			})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save your data. Please try again.",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (n *note) remove(ctx *gin.Context) {
	name := ctx.Param("name")
	if name == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "missing note name"})
		return
	}

	note := &db.Note{}
	result := db.Db.Unscoped().
		Delete(note, "name = ? AND user_id = ?", name, uint(ctx.GetFloat64("user_id")))

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, gin.H{
				"error": "The requested note could not be found.",
			})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error": "An unexpected error occurred. Please try again.",
			})
		}
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status": "succes",
	})
}

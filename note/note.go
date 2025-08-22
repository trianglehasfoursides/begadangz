package note

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"

	"github.com/trianglehasfoursides/begadangz/auth"
	"github.com/trianglehasfoursides/begadangz/db"
	"github.com/trianglehasfoursides/begadangz/validate"
	"gorm.io/gorm"
)

type Note struct {
	gorm.Model
	UserID  string `gorm:"type:varchar(255);uniqueIndex:idx_user_name"`
	Name    string `gorm:"type:varchar(255);uniqueIndex:idx_user_name" validate:"required,max=20"`
	Content string `validate:"max=100,required"`
}

func (n *Note) Add(ctx *gin.Context) {
	ctx.BindJSON(n)
	n.UserID = auth.UserId(ctx)

	if err := validate.Valid.Struct(n); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	if err := db.DB.Create(n).Error; err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1062 {
			msg := fmt.Sprintf("Note with name '%s' already exists", n.Name)
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

func (n *Note) List(ctx *gin.Context) {
	userID := auth.UserId(ctx)
	var notes []Note

	result := db.DB.Where("user_id = ?", userID).Find(&notes)
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

func (n *Note) Get(ctx *gin.Context) {
	name := ctx.Param("name")
	if name == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "missing email id"})
		return
	}

	result := db.DB.
		Where("name = ? AND user_id = ?", name, auth.UserId(ctx)).
		First(n)

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
		"ID":      n.ID,
		"title":   n.Name,
		"content": n.Content,
	})
}

func (n *Note) Edit(ctx *gin.Context) {
	name := ctx.Param("name")
	userID := auth.UserId(ctx)

	var existing Note
	err := db.DB.Where("name = ? AND user_id = ?", name, userID).First(&existing).Error
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

	if err := db.DB.Save(&existing).Error; err != nil {
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

func (n *Note) Remove(ctx *gin.Context) {
	name := ctx.Param("name")
	if name == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "missing note name"})
		return
	}

	result := db.DB.Unscoped().
		Delete(n, "name = ? AND user_id = ?", name, auth.UserId(ctx))

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

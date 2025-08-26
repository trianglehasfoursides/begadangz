package tools

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trianglehasfoursides/begadangz/internal"
	"gorm.io/gorm"
)

type Todo struct {
	gorm.Model
	UserID      string `gorm:"type:varchar(255);uniqueIndex:idx_user_name"`
	Name        string `gorm:"type:varchar(255);uniqueIndex:idx_user_name" validate:"required,max=20"`
	Task        string `json:"task"`
	Done        *bool  `gorm:"default:false"`
	ExpiredAt   string `json:"expiredat" validate:"customdate,required"`
	CompletedAt string `json:"completedat"`
}

func (t *Todo) Add(ctx *gin.Context) {
	if err := ctx.BindJSON(t); err != nil {
		// TODO : errornya
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	if err := internal.Valid.Struct(t); err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	t.UserID = internal.UserId(ctx)
	if err := internal.DB.Create(t).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "Success"})
}

func (t *Todo) Check(ctx *gin.Context) {
	var existing Todo
	if err := internal.DB.Where("name = ? AND user_id = ?", ctx.Param("name"), internal.UserId(ctx)).First(&existing).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Todo not found"})
		return
	}

	// Pastikan pointer tidak nil
	if existing.Done == nil {
		existing.Done = new(bool)
	}

	if !*existing.Done {
		*existing.Done = true
		existing.CompletedAt = time.Now().Format("02-01-2006")
	} else {
		*existing.Done = false
		existing.CompletedAt = ""
	}

	if err := internal.DB.Save(&existing).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status": "Success",
	})
}

func (t *Todo) Get(ctx *gin.Context) {
	if err := internal.DB.Where("name = ? AND user_id = ?", ctx.Param("name"), internal.UserId(ctx)).First(t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "Todo not found",
			})
		} else {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to fetch todo",
			})
		}
		return
	}

	ctx.JSON(http.StatusOK, t)
}

func (t *Todo) Edit(ctx *gin.Context) {
	var existing Todo
	err := internal.DB.Where("name = ? AND user_id = ?", ctx.Param("name"), internal.UserId(ctx)).First(&existing).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Note not found"})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	ctx.BindJSON(t)
	if err := internal.Valid.Var(t.Task, "required"); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existing.Task = t.Task
	if err := internal.DB.Save(existing).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status": "Succes",
	})
}

func (t *Todo) Remove(ctx *gin.Context) {
	if err := internal.DB.Unscoped().Delete(t, "name = ? AND user_id = ?", ctx.Param("name"), internal.UserId(ctx)).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Unable to delete the item"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status": "Succes",
	})
}

func (t *Todo) List(ctx *gin.Context) {
	layout := "02-01-2006"

	isToday := func(dateStr string) bool {
		due, err := time.Parse(layout, dateStr)
		if err != nil {
			return false
		}
		y1, m1, d1 := due.Date()
		y2, m2, d2 := time.Now().Date()
		return y1 == y2 && m1 == m2 && d1 == d2
	}

	isOverdue := func(dateStr string) bool {
		due, err := time.Parse(layout, dateStr)
		if err != nil {
			return false
		}
		y1, m1, d1 := due.Date()
		y2, m2, d2 := time.Now().Date()
		dueDate := time.Date(y1, m1, d1, 0, 0, 0, 0, time.Local)
		today := time.Date(y2, m2, d2, 0, 0, 0, 0, time.Local)
		return dueDate.Before(today)
	}

	isUpcoming := func(dateStr string) bool {
		due, err := time.Parse(layout, dateStr)
		if err != nil {
			return false
		}
		y1, m1, d1 := due.Date()
		y2, m2, d2 := time.Now().Date()
		dueDate := time.Date(y1, m1, d1, 0, 0, 0, 0, time.Local)
		today := time.Date(y2, m2, d2, 0, 0, 0, 0, time.Local)
		return dueDate.After(today)
	}

	todos := []Todo{}
	if err := internal.DB.Find(&todos, "user_id = ?", internal.UserId(ctx)).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	filtered := []Todo{}
	filter := ctx.Param("filter")

	for _, td := range todos {
		if td.Done == nil {
			continue
		}

		switch filter {
		case "today":
			if !*td.Done && isToday(td.ExpiredAt) {
				filtered = append(filtered, td)
			}
		case "overdue":
			if !*td.Done && isOverdue(td.ExpiredAt) {
				filtered = append(filtered, td)
			}
		case "upcoming":
			if !*td.Done && isUpcoming(td.ExpiredAt) {
				filtered = append(filtered, td)
			}
		case "archived":
			if *td.Done {
				filtered = append(filtered, td)
			}
		}
	}

	ctx.JSON(http.StatusOK, filtered)
}

func (t *Todo) Clear(ctx *gin.Context) {
	if err := internal.DB.Where("done = ?", 1).Unscoped().Delete(t, "user_id = ?", internal.UserId(ctx)).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "All archived todos have been deleted.",
	})
}

func (t *Todo) Filter(ctx *gin.Context) {
	name := ctx.Param("name")

	var todos []Todo
	if err := internal.DB.Where("name LIKE ? AND user_id = ?", "%"+name+"%", internal.UserId(ctx)).
		Find(&todos).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, todos)
}

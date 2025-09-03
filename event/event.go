package event

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trianglehasfoursides/begadangz/internal"
	"gorm.io/gorm"
)

type status int

const (
	Published status = iota
	Ongoing
	End
	Cancelled
)

type Event struct {
	Id          string
	UserId      string
	Title       string `json:"title"`
	Description string `json:"desc"`
	Cover       string
	StartDate   time.Time `json:"start"`
	EndDate     time.Time `json:"end"`
	Category    bool      // if false offline else true online
	Location    string
	Address     string
	Coordinat   string
	Capacity    int
	Message     string // after participant fill the form
	Status      status
}

type Participant struct {
	gorm.Model
	Name  string
	Email string
	Code  string
}

func (e *Event) Create(ctx *gin.Context) {
	if err := ctx.BindJSON(e); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, internal.ErrResponse("", err))
		return
	}

	if err := internal.Valid.Struct(e); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, internal.ErrResponse("", err))
		return
	}

	e.Id = uuid.NewString()
	e.UserId = internal.UserId(ctx)
	if err := internal.DB.Create(e).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, internal.ErrResponse("", err))
		return
	}
}

func (e *Event) Get(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "id can't be empty",
		})
		return
	}

	if err := internal.DB.Where("id = ? AND user_id = ?", id, internal.UserId(ctx)).First(e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusNotFound, internal.ErrResponse("", err))
			return
		}

		ctx.AbortWithStatusJSON(http.StatusInternalServerError, internal.ErrResponse("", err))
		return
	}

	ctx.JSON(http.StatusOK, e)
}

func (e *Event) Edit(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "id is required",
		})
		return
	}

	if err := ctx.BindJSON(e); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, internal.ErrResponse("", err))
		return
	}

	if err := internal.Valid.Struct(e); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	if err := internal.DB.Where("id = ? AND user_id = ?", id, internal.UserId(ctx)).Updates(e).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, internal.ErrResponse("", err))
		return
	}

	ctx.Status(http.StatusOK)
}

func (e *Event) Delete(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "id is required",
		})
		return
	}

	if err := internal.DB.Where("id = ? AND user_id = ?", id, internal.UserId(ctx)).Delete(e).Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, internal.ErrResponse("", err))
		return
	}

	ctx.Status(http.StatusOK)
}

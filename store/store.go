package store

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/trianglehasfoursides/begadangz/internal"
	"gorm.io/gorm"
)

type Store struct {
	gorm.Model

	UserID string `json:"user_id" gorm:"not null;index"`

	Name        string `json:"name" gorm:"varchar(100);not null"`
	Description string `json:"description" gorm:"varchar(500)"`
	Domain      string
	Email       string `json:"email" gorm:"varchar(255);not null"`
	Country     string `json:"country" gorm:"varchar(2);not null"`
	Currency    string `json:"currency" gorm:"varchar(3);not null"`
	Status      string
}

func (s *Store) Create(ctx *gin.Context) {
	if err := ctx.BindJSON(s); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "can't",
		})
		return
	}

	if err := internal.Valid.Struct(s); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "can't",
		})
		return
	}

	_, err := internal.RDB.Get(ctx, "key2").Result()
	if err != redis.Nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "can't",
		})
		return
	}

	query := internal.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
		tx.AutoMigrate(s)
		tx.Create(s)
		return tx
	})

	if err := internal.RDB.Set(ctx, "store:"+s.Name, s.Name, 0).Err(); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "can't",
		})
		return
	}

	if err := internal.Pop.Create(s.Name, query); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "can't",
		})
		return
	}
}

func (s *Store) Get(ctx *gin.Context) {
	_, err := internal.RDB.Get(ctx, "key2").Result()
	if err == redis.Nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "can't",
		})
		return
	}

	query := internal.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("name = ?", ctx.Param("name")).Find(s)
	})

	result, err := internal.Pop.Query(ctx.Param("name"), query)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "can't",
		})
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (s *Store) Edit(ctx *gin.Context) {}

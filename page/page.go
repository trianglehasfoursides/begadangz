package page

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trianglehasfoursides/begadangz/internal"
	"gorm.io/gorm"
)

const (
	MaxLinksPerPage = 20
)

type Link struct {
	gorm.Model
	PageID   string `json:"page_id" gorm:"type:varchar(36);uniqueIndex:idx_page_link_name"`
	Name     string `json:"name" gorm:"type:varchar(100);uniqueIndex:idx_page_link_name" validate:"required,min=1,max=100"`
	Redirect string `json:"redirect" gorm:"type:varchar(500)" validate:"required,url,max=500"`
}

type Page struct {
	ID     string `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID string `json:"user_id" gorm:"type:varchar(36);uniqueIndex;not null" validate:"required"`
	Name   string `json:"name" gorm:"type:varchar(100);not null" validate:"required,min=1,max=100"`
	About  string `json:"about" gorm:"type:varchar(100);not null" validate:"required,min=10,max=100"`
	Links  []Link `json:"links,omitempty" gorm:"foreignKey:PageID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (p *Page) check(userID string) (bool, error) {
	if userID == "" {
		return false, errors.New("user ID cannot be empty")
	}

	err := internal.DB.Where("user_id = ?", userID).First(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check page existence: %w", err)
	}
	return true, nil
}

func (p *Page) validateLinkName(linkName string) error {
	if strings.TrimSpace(linkName) == "" {
		return errors.New("link name cannot be empty")
	}

	var count int64
	err := internal.DB.Model(&Link{}).Where("page_id = ? AND name = ?", p.ID, linkName).Count(&count).Error
	if err != nil {
		return fmt.Errorf("failed to validate link name: %w", err)
	}

	if count > 0 {
		return errors.New("link name already exists in this page")
	}

	return nil
}

func (p *Page) Create(ctx *gin.Context) {
	userID := internal.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized Error",
		})
		return
	}

	exists, err := p.check(userID)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	if exists {
		ctx.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"error": "You already have a page created.",
		})
		return
	}

	if err := ctx.BindJSON(p); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid data format. Please check your input.",
		})
		return
	}

	if err := internal.Valid.Struct(p); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Please fill in all required fields correctly.",
		})
		return
	}

	p.ID = uuid.New().String()
	p.UserID = userID

	tx := internal.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(p).Error; err != nil {
		tx.Rollback()
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create page. Please try again.",
		})
		return
	}

	if err := tx.Commit().Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	if err := internal.RDB.Set(ctx, p.Name, "link", 0).Err(); err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":  "Unable to create URL.",
			"detail": err.Error(), // bisa dibuang di prod
		})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": "Page created successfully",
		"data":    p,
	})
}

func (p *Page) GetPage(ctx *gin.Context) {
	userID := internal.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized access",
		})
		return
	}

	err := internal.DB.Preload("Links").Where("user_id = ?", userID).First(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "Page not found",
			})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   p,
	})
}

func (p *Page) AddLink(ctx *gin.Context) {
	userID := internal.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized access",
		})
		return
	}

	err := internal.DB.Preload("Links").Where("user_id = ?", userID).First(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "Page not found",
			})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	if len(p.Links) >= MaxLinksPerPage {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("You can only have a maximum of %d links per page.", MaxLinksPerPage),
		})
		return
	}

	link := &Link{}
	if err := ctx.BindJSON(link); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid data format. Please check your input.",
		})
		return
	}

	if err := internal.Valid.Struct(link); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Please fill in all required fields correctly.",
		})
		return
	}

	if err := p.validateLinkName(link.Name); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "A link with this name already exists on your page.",
		})
		return
	}

	link.PageID = p.ID

	tx := internal.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(link).Error; err != nil {
		tx.Rollback()
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to add link. Please try again.",
		})
		return
	}

	if err := tx.Commit().Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": "Link added successfully",
		"data":    link,
	})
}

// EditLink updates an existing link
func (p *Page) EditLink(ctx *gin.Context) {
	userID := internal.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized access",
		})
		return
	}

	linkName := ctx.Param("name")
	if strings.TrimSpace(linkName) == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Link name is required",
		})
		return
	}

	// Get page
	err := internal.DB.Where("user_id = ?", userID).First(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "Page not found",
			})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	// Bind and validate new link data
	newLinkData := &Link{}
	if err := ctx.BindJSON(newLinkData); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid data format. Please check your input.",
		})
		return
	}

	if err := internal.Valid.Struct(newLinkData); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Please fill in all required fields correctly.",
		})
		return
	}

	// Check if new name conflicts with existing links (except current one)
	if newLinkData.Name != linkName {
		var count int64
		err := internal.DB.Model(&Link{}).
			Where("page_id = ? AND name = ? AND name != ?", p.ID, newLinkData.Name, linkName).
			Count(&count).Error
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Something went wrong. Please try again later.",
			})
			return
		}

		if count > 0 {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "A link with this name already exists on your page.",
			})
			return
		}
	}

	// Update link
	result := internal.DB.Model(&Link{}).
		Where("page_id = ? AND name = ?", p.ID, linkName).
		Updates(map[string]any{
			"name":     newLinkData.Name,
			"redirect": newLinkData.Redirect,
		})

	if result.Error != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update link. Please try again.",
		})
		return
	}

	if result.RowsAffected == 0 {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": "Link not found",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Link updated successfully",
	})
}

// DeleteLink removes a link from the page
func (p *Page) DeleteLink(ctx *gin.Context) {
	userID := internal.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized access",
		})
		return
	}

	linkName := ctx.Param("name")
	if strings.TrimSpace(linkName) == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Link name is required",
		})
		return
	}

	// Get page
	err := internal.DB.Where("user_id = ?", userID).First(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "Page not found",
			})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	// Delete link (hard delete with Unscoped)
	result := internal.DB.Unscoped().
		Where("page_id = ? AND name = ?", p.ID, linkName).
		Delete(&Link{})

	if result.Error != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete link. Please try again.",
		})
		return
	}

	if result.RowsAffected == 0 {
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": "Link not found",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Link deleted successfully",
	})
}

// DeletePage removes the entire page and all its links
func (p *Page) DeletePage(ctx *gin.Context) {
	userID := internal.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized access",
		})
		return
	}

	// Get page
	err := internal.DB.Where("user_id = ?", userID).First(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "Page not found",
			})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	tx := internal.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete all links first (due to foreign key constraint)
	if err := tx.Unscoped().Where("page_id = ?", p.ID).Delete(&Link{}).Error; err != nil {
		tx.Rollback()
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete page. Please try again.",
		})
		return
	}

	// Delete page
	if err := tx.Unscoped().Delete(p).Error; err != nil {
		tx.Rollback()
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete page. Please try again.",
		})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Page deleted successfully",
	})
}

// GetLinkStats returns statistics about links in the page
func (p *Page) GetLinkStats(ctx *gin.Context) {
	userID := internal.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized access",
		})
		return
	}

	// Get page with links
	err := internal.DB.Preload("Links").Where("user_id = ?", userID).First(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "Page not found",
			})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	stats := map[string]any{
		"total_links":     len(p.Links),
		"remaining_slots": MaxLinksPerPage - len(p.Links),
		"max_links":       MaxLinksPerPage,
		"page_id":         p.ID,
		"page_name":       p.Name,
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   stats,
	})
}

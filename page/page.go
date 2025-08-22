package page

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trianglehasfoursides/begadangz/auth"
	"github.com/trianglehasfoursides/begadangz/db"
	"github.com/trianglehasfoursides/begadangz/validate"
	"gorm.io/gorm"
)

const (
	MaxLinksPerPage = 20
)

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// SuccessResponse represents a standard success response
type SuccessResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// Link represents a link within a page
type Link struct {
	gorm.Model
	PageID   string `json:"page_id" gorm:"type:varchar(36);uniqueIndex:idx_page_link_name"`
	Name     string `json:"name" gorm:"type:varchar(100);uniqueIndex:idx_page_link_name" validate:"required,min=1,max=100"`
	Redirect string `json:"redirect" gorm:"type:varchar(500)" validate:"required,url,max=500"`
}

// Page represents a user page containing multiple links
type Page struct {
	ID     string `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID string `json:"user_id" gorm:"type:varchar(36);uniqueIndex;not null" validate:"required"`
	Name   string `json:"name" gorm:"type:varchar(100);not null" validate:"required,min=1,max=100"`
	Links  []Link `json:"links,omitempty" gorm:"foreignKey:PageID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// TableName sets the table name for Page model
func (Page) TableName() string {
	return "pages"
}

// TableName sets the table name for Link model
func (Link) TableName() string {
	return "links"
}

// check verifies if a page exists for the given user ID
func (p *Page) check(userID string) (bool, error) {
	if userID == "" {
		return false, errors.New("user ID cannot be empty")
	}

	err := db.DB.Where("user_id = ?", userID).First(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check page existence: %w", err)
	}
	return true, nil
}

// validateLinkName checks if link name is valid and unique within the page
func (p *Page) validateLinkName(linkName string) error {
	if strings.TrimSpace(linkName) == "" {
		return errors.New("link name cannot be empty")
	}

	var count int64
	err := db.DB.Model(&Link{}).Where("page_id = ? AND name = ?", p.ID, linkName).Count(&count).Error
	if err != nil {
		return fmt.Errorf("failed to validate link name: %w", err)
	}

	if count > 0 {
		return errors.New("link name already exists in this page")
	}

	return nil
}

// Create creates a new page for the user
func (p *Page) Create(ctx *gin.Context) {
	userID := auth.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{
			Error: "Unauthorized access",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	// Check if page already exists
	exists, err := p.check(userID)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to check page existence",
			Code:    "INTERNAL_ERROR",
			Details: err.Error(),
		})
		return
	}

	if exists {
		ctx.AbortWithStatusJSON(http.StatusConflict, ErrorResponse{
			Error: "Page already exists for this user",
			Code:  "PAGE_EXISTS",
		})
		return
	}

	// Bind JSON request
	if err := ctx.BindJSON(p); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid JSON format",
			Code:    "INVALID_JSON",
			Details: err.Error(),
		})
		return
	}

	// Validate struct
	if err := validate.Valid.Struct(p); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Validation failed",
			Code:    "VALIDATION_ERROR",
			Details: err.Error(),
		})
		return
	}

	// Set page properties
	p.ID = uuid.New().String()
	p.UserID = userID

	// Start transaction
	tx := db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create page
	if err := tx.Create(p).Error; err != nil {
		tx.Rollback()
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to create page",
			Code:    "CREATE_ERROR",
			Details: err.Error(),
		})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to commit transaction",
			Code:    "COMMIT_ERROR",
			Details: err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusCreated, SuccessResponse{
		Status:  "success",
		Message: "Page created successfully",
		Data:    p,
	})
}

// GetPage retrieves user's page with links
func (p *Page) GetPage(ctx *gin.Context) {
	userID := auth.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{
			Error: "Unauthorized access",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	err := db.DB.Preload("Links").Where("user_id = ?", userID).First(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusNotFound, ErrorResponse{
				Error: "Page not found",
				Code:  "PAGE_NOT_FOUND",
			})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve page",
			Code:    "RETRIEVAL_ERROR",
			Details: err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, SuccessResponse{
		Status: "success",
		Data:   p,
	})
}

// AddLink adds a new link to the page
func (p *Page) AddLink(ctx *gin.Context) {
	userID := auth.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{
			Error: "Unauthorized access",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	// Get page with links count
	err := db.DB.Preload("Links").Where("user_id = ?", userID).First(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusNotFound, ErrorResponse{
				Error: "Page not found",
				Code:  "PAGE_NOT_FOUND",
			})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve page",
			Code:    "RETRIEVAL_ERROR",
			Details: err.Error(),
		})
		return
	}

	// Check link limit
	if len(p.Links) >= MaxLinksPerPage {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, ErrorResponse{
			Error:   fmt.Sprintf("Maximum number of links (%d) reached", MaxLinksPerPage),
			Code:    "LINK_LIMIT_EXCEEDED",
			Details: fmt.Sprintf("Current links: %d, Maximum allowed: %d", len(p.Links), MaxLinksPerPage),
		})
		return
	}

	// Bind and validate link
	link := &Link{}
	if err := ctx.BindJSON(link); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid JSON format",
			Code:    "INVALID_JSON",
			Details: err.Error(),
		})
		return
	}

	if err := validate.Valid.Struct(link); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Validation failed",
			Code:    "VALIDATION_ERROR",
			Details: err.Error(),
		})
		return
	}

	// Validate link name uniqueness
	if err := p.validateLinkName(link.Name); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, ErrorResponse{
			Error: err.Error(),
			Code:  "DUPLICATE_LINK_NAME",
		})
		return
	}

	// Set link properties
	link.PageID = p.ID

	// Start transaction
	tx := db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create link
	if err := tx.Create(link).Error; err != nil {
		tx.Rollback()
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to create link",
			Code:    "CREATE_ERROR",
			Details: err.Error(),
		})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to commit transaction",
			Code:    "COMMIT_ERROR",
			Details: err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusCreated, SuccessResponse{
		Status:  "success",
		Message: "Link added successfully",
		Data:    link,
	})
}

// EditLink updates an existing link
func (p *Page) EditLink(ctx *gin.Context) {
	userID := auth.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{
			Error: "Unauthorized access",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	linkName := ctx.Param("name")
	if strings.TrimSpace(linkName) == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, ErrorResponse{
			Error: "Link name parameter is required",
			Code:  "MISSING_PARAMETER",
		})
		return
	}

	// Get page
	err := db.DB.Where("user_id = ?", userID).First(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusNotFound, ErrorResponse{
				Error: "Page not found",
				Code:  "PAGE_NOT_FOUND",
			})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve page",
			Code:    "RETRIEVAL_ERROR",
			Details: err.Error(),
		})
		return
	}

	// Bind and validate new link data
	newLinkData := &Link{}
	if err := ctx.BindJSON(newLinkData); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid JSON format",
			Code:    "INVALID_JSON",
			Details: err.Error(),
		})
		return
	}

	if err := validate.Valid.Struct(newLinkData); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Validation failed",
			Code:    "VALIDATION_ERROR",
			Details: err.Error(),
		})
		return
	}

	// Check if new name conflicts with existing links (except current one)
	if newLinkData.Name != linkName {
		var count int64
		err := db.DB.Model(&Link{}).
			Where("page_id = ? AND name = ? AND name != ?", p.ID, newLinkData.Name, linkName).
			Count(&count).Error
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Failed to validate link name",
				Code:    "VALIDATION_ERROR",
				Details: err.Error(),
			})
			return
		}

		if count > 0 {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, ErrorResponse{
				Error: "Link name already exists in this page",
				Code:  "DUPLICATE_LINK_NAME",
			})
			return
		}
	}

	// Update link
	result := db.DB.Model(&Link{}).
		Where("page_id = ? AND name = ?", p.ID, linkName).
		Updates(map[string]interface{}{
			"name":     newLinkData.Name,
			"redirect": newLinkData.Redirect,
		})

	if result.Error != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update link",
			Code:    "UPDATE_ERROR",
			Details: result.Error.Error(),
		})
		return
	}

	if result.RowsAffected == 0 {
		ctx.AbortWithStatusJSON(http.StatusNotFound, ErrorResponse{
			Error: "Link not found",
			Code:  "LINK_NOT_FOUND",
		})
		return
	}

	ctx.JSON(http.StatusOK, SuccessResponse{
		Status:  "success",
		Message: "Link updated successfully",
	})
}

// DeleteLink removes a link from the page
func (p *Page) DeleteLink(ctx *gin.Context) {
	userID := auth.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{
			Error: "Unauthorized access",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	linkName := ctx.Param("name")
	if strings.TrimSpace(linkName) == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, ErrorResponse{
			Error: "Link name parameter is required",
			Code:  "MISSING_PARAMETER",
		})
		return
	}

	// Get page
	err := db.DB.Where("user_id = ?", userID).First(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusNotFound, ErrorResponse{
				Error: "Page not found",
				Code:  "PAGE_NOT_FOUND",
			})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve page",
			Code:    "RETRIEVAL_ERROR",
			Details: err.Error(),
		})
		return
	}

	// Delete link (hard delete with Unscoped)
	result := db.DB.Unscoped().
		Where("page_id = ? AND name = ?", p.ID, linkName).
		Delete(&Link{})

	if result.Error != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to delete link",
			Code:    "DELETE_ERROR",
			Details: result.Error.Error(),
		})
		return
	}

	if result.RowsAffected == 0 {
		ctx.AbortWithStatusJSON(http.StatusNotFound, ErrorResponse{
			Error: "Link not found",
			Code:  "LINK_NOT_FOUND",
		})
		return
	}

	ctx.JSON(http.StatusOK, SuccessResponse{
		Status:  "success",
		Message: "Link deleted successfully",
	})
}

// DeletePage removes the entire page and all its links
func (p *Page) DeletePage(ctx *gin.Context) {
	userID := auth.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{
			Error: "Unauthorized access",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	// Get page
	err := db.DB.Where("user_id = ?", userID).First(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusNotFound, ErrorResponse{
				Error: "Page not found",
				Code:  "PAGE_NOT_FOUND",
			})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve page",
			Code:    "RETRIEVAL_ERROR",
			Details: err.Error(),
		})
		return
	}

	// Start transaction
	tx := db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete all links first (due to foreign key constraint)
	if err := tx.Unscoped().Where("page_id = ?", p.ID).Delete(&Link{}).Error; err != nil {
		tx.Rollback()
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to delete page links",
			Code:    "DELETE_ERROR",
			Details: err.Error(),
		})
		return
	}

	// Delete page
	if err := tx.Unscoped().Delete(p).Error; err != nil {
		tx.Rollback()
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to delete page",
			Code:    "DELETE_ERROR",
			Details: err.Error(),
		})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to commit transaction",
			Code:    "COMMIT_ERROR",
			Details: err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, SuccessResponse{
		Status:  "success",
		Message: "Page deleted successfully",
	})
}

// GetLinkStats returns statistics about links in the page
func (p *Page) GetLinkStats(ctx *gin.Context) {
	userID := auth.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{
			Error: "Unauthorized access",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	// Get page with links
	err := db.DB.Preload("Links").Where("user_id = ?", userID).First(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusNotFound, ErrorResponse{
				Error: "Page not found",
				Code:  "PAGE_NOT_FOUND",
			})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to retrieve page",
			Code:    "RETRIEVAL_ERROR",
			Details: err.Error(),
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

	ctx.JSON(http.StatusOK, SuccessResponse{
		Status: "success",
		Data:   stats,
	})
}

func (p *Page) View(ctx *gin.Context) {
}

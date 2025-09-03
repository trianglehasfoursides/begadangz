package form

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trianglehasfoursides/begadangz/internal"
	"gorm.io/gorm"
)

type Form struct {
	ID        string    `json:"id" gorm:"type:uuid;primary_key"`
	FormID    string    `json:"form_id" gorm:"type:uuid" validate:"required"`
	Name      string    `json:"name" gorm:"type:varchar(100)" validate:"required,min=1,max=100"`
	Type      string    `json:"type" gorm:"type:varchar(50)" validate:"required,oneof=text select radio checkbox"`
	Required  bool      `json:"required" gorm:"default:false"`
	Options   []Option  `json:"options,omitempty" gorm:"foreignKey:FieldID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	UserId    string    `json:"user_id" gorm:"type:uuid" validate:"required"`
}

type Option struct {
	ID        string    `json:"id" gorm:"type:uuid;primary_key"`
	FieldID   string    `json:"field_id" gorm:"type:uuid" validate:"required"`
	Value     string    `json:"value" gorm:"type:varchar(200)" validate:"required,min=1,max=200"`
	IsDefault bool      `json:"is_default" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type FormSubmission struct {
	ID          string       `json:"id" gorm:"type:uuid;primary_key"`
	FormID      string       `json:"form_id" gorm:"type:uuid" validate:"required"`
	UserID      string       `json:"user_id" gorm:"type:uuid"`
	Email       string       `json:"email" gorm:"type:varchar(255)"`
	Name        string       `json:"name" gorm:"type:varchar(100)"`
	Answers     []FormAnswer `json:"answers,omitempty" gorm:"foreignKey:SubmissionID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	SubmittedAt time.Time    `json:"submitted_at"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	OwnerID     string       `json:"owner_id" gorm:"type:uuid"` // ID pemilik form (yang membuat form)
}

type FormAnswer struct {
	ID           string    `json:"id" gorm:"type:uuid;primary_key"`
	SubmissionID string    `json:"submission_id" gorm:"type:uuid" validate:"required"`
	FieldID      string    `json:"field_id" gorm:"type:uuid" validate:"required"`
	Value        string    `json:"value" gorm:"type:text"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (f *Form) Create(ctx *gin.Context) {
	// Check authentication
	userID := internal.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
		})
		return
	}

	if err := ctx.BindJSON(f); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid data format. Please check your input.",
		})
		return
	}

	// Set user ID dari authenticated user
	f.UserId = userID

	if err := internal.Valid.Struct(f); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Please fill in all required fields correctly.",
		})
		return
	}

	f.ID = uuid.New().String()

	tx := internal.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(f).Error; err != nil {
		tx.Rollback()
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create form. Please try again.",
		})
		return
	}

	if len(f.Options) > 0 {
		for i := range f.Options {
			f.Options[i].ID = uuid.New().String()
			f.Options[i].FieldID = f.ID
		}

		if err := tx.Create(&f.Options).Error; err != nil {
			tx.Rollback()
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to create form options. Please try again.",
			})
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	if err := internal.RDB.Set(ctx, f.Name, "link", 0).Err(); err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":  "Unable to create URL.",
			"detail": err.Error(), // bisa dibuang di prod
		})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": "Form created successfully",
		"data":    f,
	})
}

func (f *Form) List(ctx *gin.Context) {
	userID := internal.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
		})
		return
	}

	var forms []Form
	// Filter berdasarkan user_id untuk memastikan user hanya melihat form miliknya
	err := internal.DB.Preload("Options").Where("user_id = ?", userID).Find(&forms).Error
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   forms,
	})
}

func (f *Form) Update(ctx *gin.Context) {
	userID := internal.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
		})
		return
	}

	id := ctx.Param("id")
	if strings.TrimSpace(id) == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Form field ID is required",
		})
		return
	}

	var existing Form
	err := internal.DB.Where("id = ? AND user_id = ?", id, userID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Access denied. You can only update your own forms.",
			})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	if err := ctx.BindJSON(f); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid data format. Please check your input.",
		})
		return
	}

	f.UserId = existing.UserId

	if err := internal.Valid.Struct(f); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Please fill in all required fields correctly.",
		})
		return
	}

	tx := internal.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	result := tx.Model(&existing).Updates(map[string]any{
		"name":     f.Name,
		"type":     f.Type,
		"required": f.Required,
	})

	if result.Error != nil {
		tx.Rollback()
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update form field. Please try again.",
		})
		return
	}

	if len(f.Options) > 0 {
		if err := tx.Where("field_id = ?", id).Delete(&Option{}).Error; err != nil {
			tx.Rollback()
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to update form options. Please try again.",
			})
			return
		}

		for i := range f.Options {
			f.Options[i].ID = uuid.New().String()
			f.Options[i].FieldID = id
		}

		if err := tx.Create(&f.Options).Error; err != nil {
			tx.Rollback()
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to update form options. Please try again.",
			})
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Form field updated successfully",
	})
}

func (f *Form) Delete(ctx *gin.Context) {
	// Check authentication
	userID := internal.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
		})
		return
	}

	id := ctx.Param("id")
	if strings.TrimSpace(id) == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Form field ID is required",
		})
		return
	}

	var existing Form
	// Check ownership - user hanya bisa delete form miliknya
	err := internal.DB.Where("id = ? AND user_id = ?", id, userID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Access denied. You can only delete your own forms.",
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

	if err := tx.Where("field_id = ?", id).Delete(&Option{}).Error; err != nil {
		tx.Rollback()
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete form field. Please try again.",
		})
		return
	}

	if err := tx.Delete(&existing).Error; err != nil {
		tx.Rollback()
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete form field. Please try again.",
		})
		return
	}

	if err := tx.Commit().Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Form field deleted successfully",
	})
}

func (fs *FormSubmission) Submit(ctx *gin.Context) {
	if err := ctx.BindJSON(fs); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid data format. Please check your input.",
		})
		return
	}

	var form Form
	err := internal.DB.Select("user_id").Where("form_id = ?", fs.FormID).First(&form).Error
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Form not found",
		})
		return
	}

	fs.OwnerID = form.UserId

	if err := internal.Valid.Struct(fs); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Please fill in all required fields correctly.",
		})
		return
	}

	fs.ID = uuid.New().String()
	fs.SubmittedAt = time.Now()

	tx := internal.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(fs).Error; err != nil {
		tx.Rollback()
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to submit form. Please try again.",
		})
		return
	}

	if len(fs.Answers) > 0 {
		for i := range fs.Answers {
			fs.Answers[i].ID = uuid.New().String()
			fs.Answers[i].SubmissionID = fs.ID
		}

		if err := tx.Create(&fs.Answers).Error; err != nil {
			tx.Rollback()
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to save form answers. Please try again.",
			})
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": "Form submitted successfully",
		"data":    fs,
	})
}

func (fs *FormSubmission) ListSubmissions(ctx *gin.Context) {
	userID := internal.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
		})
		return
	}

	formID := ctx.Query("form_id")
	if strings.TrimSpace(formID) == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Form ID is required",
		})
		return
	}

	var form Form
	err := internal.DB.Select("user_id").Where("form_id = ?", formID).First(&form).Error
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Form not found",
		})
		return
	}

	if form.UserId != userID {
		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "Access denied. You can only view submissions for your own forms.",
		})
		return
	}

	var submissions []FormSubmission
	err = internal.DB.Preload("Answers").Where("form_id = ?", formID).Find(&submissions).Error
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   submissions,
	})
}

func (fs *FormSubmission) GetSubmission(ctx *gin.Context) {
	userID := internal.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
		})
		return
	}

	id := ctx.Param("id")
	if strings.TrimSpace(id) == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Submission ID is required",
		})
		return
	}

	err := internal.DB.Preload("Answers").Where("id = ? AND owner_id = ?", id, userID).First(fs).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Access denied. You can only view submissions for your own forms.",
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
		"data":   fs,
	})
}

func (fs *FormSubmission) DeleteSubmission(ctx *gin.Context) {
	// Check authentication
	userID := internal.UserId(ctx)
	if userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
		})
		return
	}

	id := ctx.Param("id")
	if strings.TrimSpace(id) == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Submission ID is required",
		})
		return
	}

	var existing FormSubmission
	// Check ownership - user hanya bisa delete submission dari form miliknya
	err := internal.DB.Where("id = ? AND owner_id = ?", id, userID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Access denied. You can only delete submissions for your own forms.",
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

	if err := tx.Where("submission_id = ?", id).Delete(&FormAnswer{}).Error; err != nil {
		tx.Rollback()
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete form submission. Please try again.",
		})
		return
	}

	if err := tx.Delete(&existing).Error; err != nil {
		tx.Rollback()
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete form submission. Please try again.",
		})
		return
	}

	if err := tx.Commit().Error; err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Something went wrong. Please try again later.",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Form submission deleted successfully",
	})
}

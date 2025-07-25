package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/resend/resend-go/v2"
	"github.com/trianglehasfoursides/begadangz/mathrock/db"
	"github.com/trianglehasfoursides/begadangz/validate"
	"github.com/yuin/goldmark"
	"gorm.io/gorm"
)

type email struct {
	client *resend.Client
}

func (e *email) send(ctx *gin.Context) {
	// check all required form
	err := func(payload ...string) error {
		for _, str := range payload {
			if err := validate.Valid.Var(str, "required"); err != nil {
				return err
			}
		}
		return nil
	}(ctx.PostForm("from"), ctx.PostForm("to"), ctx.PostForm("subject"), ctx.PostForm("mail"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
	}

	req := &resend.SendEmailRequest{
		From:    ctx.PostForm("from"),
		To:      []string{ctx.PostForm("to")},
		Subject: ctx.PostForm("subject"),
	}

	var mail bytes.Buffer
	content := ctx.PostForm("mail")
	if err := goldmark.Convert([]byte(content), &mail); err != nil {
		req.Text = content
	}
	req.Html = mail.String()

	fh, err := ctx.FormFile("attachment")
	isattach := err == nil
	if err != nil && err != http.ErrMissingFile {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to process attachments file",
		})
		return
	}

	var filename string
	if isattach {
		if fh.Size > 10*1024*1024 {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"error": "The uploaded file is too large. Maximum size is 10MB.",
			})
			return
		}

		file, err := fh.Open()
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"error": "Failed to process attachments file",
			})
			return
		}

		buff, err := io.ReadAll(file)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"error": "Failed to process attachments file",
			})
			return
		}

		filename = fh.Filename
		req.Attachments = []*resend.Attachment{
			&resend.Attachment{
				Content:     buff,
				Filename:    fh.Filename,
				ContentType: http.DetectContentType(buff),
			},
		}
	}

	txn := db.Db.Begin()
	defer txn.Rollback()
	if err := txn.Create(&db.Email{
		From:       ctx.PostForm("from"),
		To:         ctx.PostForm("to"),
		Subject:    ctx.PostForm("subject"),
		Mail:       ctx.PostForm("mail"),
		Attachment: &filename,
		UserID:     int(ctx.GetFloat64("user_id")),
	}).Error; err != nil {
		txn.Rollback()
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to process the request. Please try again.",
		})
		return
	}

	_, err = e.client.Emails.Send(req)
	if err != nil {
		txn.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "We couldn't send the email. Please try again later.",
		})
		return
	}

	txn.Commit()
	ctx.JSON(http.StatusOK, gin.H{
		"status": "Succes",
	})
}

func (e *email) history(ctx *gin.Context) {
	userID := int(ctx.GetFloat64("user_id"))

	all := make([]*db.Email, 0)
	result := db.Db.
		Where("user_id = ?", userID).
		Find(&all)

	if err := result.Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "An unexpected error occurred. Please try again.",
		})
		return
	}

	if result.RowsAffected == 0 {
		fmt.Println(userID)
		ctx.JSON(http.StatusNotFound, gin.H{
			"error": "No emails found for this user.",
		})
		return
	}

	ctx.JSON(http.StatusOK, all)
}

func (e *email) view(ctx *gin.Context) {
	emailIdStr := ctx.Param("email_id")
	if emailIdStr == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "missing email id"})
		return
	}

	emailId, err := strconv.Atoi(emailIdStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid email id"})
		return
	}

	email := &db.Email{}
	result := db.Db.
		Where("id = ? AND user_id = ?", emailId, int(ctx.GetFloat64("user_id"))).
		First(email)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, gin.H{
				"error": "The requested email could not be found.",
			})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error": "An unexpected error occurred. Please try again.",
			})
		}
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"ID":         email.ID,
		"From":       email.From,
		"To":         email.To,
		"Subject":    email.Subject,
		"Mail":       email.Mail,
		"Attachment": email.Attachment,
	})
}

func (e *email) remove(ctx *gin.Context) {
	emailIdStr := ctx.Param("email_id")
	if emailIdStr == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "missing email id"})
		return
	}

	emailId, err := strconv.Atoi(emailIdStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid email id"})
		return
	}

	email := &db.Email{}
	result := db.Db.
		Delete(email, "id = ?", emailId)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, gin.H{
				"error": "The requested email could not be found.",
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

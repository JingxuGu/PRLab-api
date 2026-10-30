package model

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var ErrApplicationEmailRegistered = errors.New("application email already registered")
var ErrApplicationEmailPending = errors.New("application email already pending")

// An application reserves only an email. Credentials do not exist until approval.
type AccountApplication struct {
	Id                       int     `json:"id"`
	Email                    string  `json:"email" gorm:"type:varchar(50);not null"`
	PendingEmail             *string `json:"-" gorm:"type:varchar(50);uniqueIndex"`
	Status                   string  `json:"status" gorm:"type:varchar(16);index"`
	CreatedAt                int64   `json:"created_at"`
	ReviewedAt               int64   `json:"reviewed_at"`
	ReviewedBy               int     `json:"reviewed_by"`
	UserId                   int     `json:"user_id"`
	Username                 string  `json:"username" gorm:"type:varchar(20)"`
	NotificationStatus       string  `json:"notification_status" gorm:"type:varchar(16)"`
	CredentialDeliveryStatus string  `json:"credential_delivery_status" gorm:"type:varchar(16)"`
	DeliveryAttemptAt        int64   `json:"delivery_attempt_at"`
}

func SubmitAccountApplication(email string) (*AccountApplication, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var count int64
	if err := DB.Unscoped().Model(&User{}).Where("LOWER(email) = ?", email).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrApplicationEmailRegistered
	}
	if err := DB.Model(&AccountApplication{}).Where("pending_email = ?", email).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrApplicationEmailPending
	}
	application := &AccountApplication{Email: email, PendingEmail: &email, Status: "pending", CreatedAt: time.Now().Unix(), NotificationStatus: "pending", CredentialDeliveryStatus: "not_applicable"}
	// A unique nullable email reservation prevents concurrent duplicate applications.
	if err := DB.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Create(application).Error; err != nil {
		var pending int64
		if DB.Model(&AccountApplication{}).Where("pending_email = ?", email).Count(&pending).Error == nil && pending > 0 {
			return nil, ErrApplicationEmailPending
		}
		return nil, err
	}
	return application, nil
}

// Return the one-time plaintext password only to the mail-delivery service, never an API response.
func ReviewAccountApplication(id, reviewer, quota int, approve bool) (*AccountApplication, string, error) {
	if quota < 0 || quota > 1000000000 {
		return nil, "", ErrRegistrationConflict
	}
	var application AccountApplication
	password := ""
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&application, id).Error; err != nil {
			return err
		}
		if application.Status != "pending" {
			return ErrRegistrationConflict
		}
		status := "rejected"
		if approve {
			status = "approved"
		}
		now := time.Now().Unix()
		r := tx.Model(&AccountApplication{}).Where("id = ? AND status = ?", id, "pending").Updates(map[string]any{"status": status, "pending_email": nil, "reviewed_by": reviewer, "reviewed_at": now})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return ErrRegistrationConflict
		}
		if approve {
			var count int64
			if err := tx.Unscoped().Model(&User{}).Where("LOWER(email) = ?", application.Email).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrRegistrationConflict
			}
			random := make([]byte, 32)
			if _, err := rand.Read(random); err != nil {
				return err
			}
			username := "acct_" + hex.EncodeToString(random[:5])
			password = base64.RawURLEncoding.EncodeToString(random[5:17])
			hash, err := common.Password2Hash(password)
			if err != nil {
				return err
			}
			user := User{Username: username, Email: application.Email, Password: hash, DisplayName: username, Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: quota, Group: "default", AffCode: hex.EncodeToString(random[17:])}
			if err := tx.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Create(&user).Error; err != nil {
				return err
			}
			if err := tx.Model(&application).Updates(map[string]any{"user_id": user.Id, "username": username, "credential_delivery_status": "sending", "delivery_attempt_at": now}).Error; err != nil {
				return err
			}
		}
		return tx.First(&application, id).Error
	})
	if err != nil {
		return nil, "", err
	}
	return &application, password, nil
}

// Failed delivery can be retried before first login. Never reset an established user's password.
func RetryApplicationCredentials(id int) (*AccountApplication, string, error) {
	var application AccountApplication
	password := ""
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&application, id).Error; err != nil {
			return err
		}
		now := time.Now().Unix()
		if application.Status != "approved" || (application.CredentialDeliveryStatus != "failed" && !(application.CredentialDeliveryStatus == "sending" && application.DeliveryAttemptAt < now-120)) {
			return ErrRegistrationConflict
		}
		r := tx.Model(&AccountApplication{}).Where("id = ? AND credential_delivery_status = ? AND delivery_attempt_at = ?", id, application.CredentialDeliveryStatus, application.DeliveryAttemptAt).Updates(map[string]any{"credential_delivery_status": "sending", "delivery_attempt_at": now})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return ErrRegistrationConflict
		}
		random := make([]byte, 12)
		if _, err := rand.Read(random); err != nil {
			return err
		}
		password = base64.RawURLEncoding.EncodeToString(random)
		hash, err := common.Password2Hash(password)
		if err != nil {
			return err
		}
		r = tx.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Model(&User{}).Where("id = ? AND last_login_at = 0 AND status = ?", application.UserId, common.UserStatusEnabled).Update("password", hash)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return ErrRegistrationConflict
		}
		return tx.First(&application, id).Error
	})
	if err != nil {
		return nil, "", err
	}
	return &application, password, nil
}

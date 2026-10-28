package model

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var ErrInvitationUnavailable = errors.New("invitation unavailable")
var ErrRegistrationConflict = errors.New("registration already exists or has been reviewed")

type RegistrationInvitation struct {
	Id           int    `json:"id"`
	TokenHash    string `json:"-" gorm:"type:char(64);uniqueIndex;not null"`
	Email        string `json:"email" gorm:"type:varchar(50);not null"`
	CreatedBy    int    `json:"created_by"`
	CreatedAt    int64  `json:"created_at"`
	ExpiresAt    int64  `json:"expires_at"`
	ConsumedAt   int64  `json:"consumed_at"`
	RevokedAt    int64  `json:"revoked_at"`
	InitialQuota int    `json:"initial_quota"`
}

type RegistrationRequest struct {
	Id                 int     `json:"id"`
	InvitationId       int     `json:"invitation_id" gorm:"uniqueIndex"`
	Username           string  `json:"username" gorm:"type:varchar(20)"`
	Email              string  `json:"email" gorm:"type:varchar(50)"`
	PasswordHash       string  `json:"-" gorm:"type:varchar(128)"`
	PendingUsername    *string `json:"-" gorm:"type:varchar(20);uniqueIndex"`
	PendingEmail       *string `json:"-" gorm:"type:varchar(50);uniqueIndex"`
	Status             string  `json:"status" gorm:"type:varchar(16);index"`
	CreatedAt          int64   `json:"created_at"`
	ReviewedAt         int64   `json:"reviewed_at"`
	ReviewedBy         int     `json:"reviewed_by"`
	UserId             int     `json:"user_id"`
	NotificationStatus string  `json:"notification_status" gorm:"type:varchar(16)"`
}

func invitationHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func CreateRegistrationInvitation(email string, creator, quota int, expires int64) (*RegistrationInvitation, string, error) {
	if expires <= time.Now().Unix() || quota < 0 || quota > 1000000000 {
		return nil, "", ErrInvitationUnavailable
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, "", err
	}
	token := "prlab_inv_" + base64.RawURLEncoding.EncodeToString(b)
	inv := &RegistrationInvitation{TokenHash: invitationHash(token), Email: strings.ToLower(strings.TrimSpace(email)), CreatedBy: creator, InitialQuota: quota, CreatedAt: time.Now().Unix(), ExpiresAt: expires}
	return inv, token, DB.Create(inv).Error
}

func registrationUserExists(tx *gorm.DB, username, email string) (bool, error) {
	var count int64
	err := tx.Unscoped().Model(&User{}).Where("LOWER(username) = ? OR LOWER(email) = ?", strings.ToLower(username), strings.ToLower(email)).Count(&count).Error
	return count > 0, err
}

// Claim the invitation and persist the application atomically. No User exists yet.
func SubmitInvitationRegistration(token, username, email, password string) (*RegistrationRequest, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	passwordHash, err := common.Password2Hash(password)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	pendingUsername := strings.ToLower(username)
	req := &RegistrationRequest{Username: username, Email: email, PasswordHash: passwordHash, PendingUsername: &pendingUsername, PendingEmail: &email, Status: "pending", CreatedAt: now, NotificationStatus: "pending"}
	err = DB.Transaction(func(tx *gorm.DB) error {
		var inv RegistrationInvitation
		if err := tx.Where("token_hash = ? AND email = ? AND consumed_at = 0 AND revoked_at = 0 AND expires_at > ?", invitationHash(token), email, now).First(&inv).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvitationUnavailable
			}
			return err
		}
		exists, err := registrationUserExists(tx, username, email)
		if err != nil {
			return err
		}
		if exists {
			return ErrRegistrationConflict
		}
		claim := tx.Model(&RegistrationInvitation{}).Where("id = ? AND consumed_at = 0 AND revoked_at = 0 AND expires_at > ?", inv.Id, now).Update("consumed_at", now)
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected != 1 {
			return ErrInvitationUnavailable
		}
		req.InvitationId = inv.Id
		return tx.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Create(req).Error
	})
	return req, err
}

// The conditional transition serializes approve/reject. A failed User insert rolls it back.
func ReviewInvitationRegistration(id, reviewer int, approve bool) (*RegistrationRequest, error) {
	var req RegistrationRequest
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&req, id).Error; err != nil {
			return err
		}
		if req.Status != "pending" {
			return ErrRegistrationConflict
		}
		status := "rejected"
		if approve {
			status = "approved"
		}
		r := tx.Model(&RegistrationRequest{}).Where("id = ? AND status = ?", id, "pending").Updates(map[string]interface{}{"status": status, "reviewed_by": reviewer, "reviewed_at": time.Now().Unix(), "password_hash": "", "pending_username": nil, "pending_email": nil})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return ErrRegistrationConflict
		}
		if approve {
			exists, err := registrationUserExists(tx, req.Username, req.Email)
			if err != nil {
				return err
			}
			if exists {
				return ErrRegistrationConflict
			}
			var inv RegistrationInvitation
			if err := tx.First(&inv, req.InvitationId).Error; err != nil {
				return err
			}
			b := make([]byte, 16)
			if _, err := rand.Read(b); err != nil {
				return err
			}
			user := User{Username: req.Username, Email: req.Email, Password: req.PasswordHash, DisplayName: req.Username, Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: inv.InitialQuota, Group: "default", AffCode: hex.EncodeToString(b), InviterId: inv.CreatedBy}
			// The password is already bcrypt-hashed. Insert must not hash it a second time.
			if err := tx.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Create(&user).Error; err != nil {
				return err
			}
			if err := tx.Model(&req).Update("user_id", user.Id).Error; err != nil {
				return err
			}
		}
		return tx.First(&req, id).Error
	})
	return &req, err
}

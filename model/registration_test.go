package model

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func registrationDB(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&RegistrationInvitation{}, &RegistrationRequest{}))
	truncateTables(t)
	t.Cleanup(func() { DB.Exec("DELETE FROM registration_requests"); DB.Exec("DELETE FROM registration_invitations") })
}
func invitationFixture(t *testing.T) (*RegistrationInvitation, string) {
	t.Helper()
	inv, code, err := CreateRegistrationInvitation("invited@example.org", 1, 100, time.Now().Add(time.Hour).Unix())
	require.NoError(t, err)
	return inv, code
}
func TestInvitationPendingIsNotAnAccount(t *testing.T) {
	registrationDB(t)
	_, code := invitationFixture(t)
	req, err := SubmitInvitationRegistration(code, "applicant", "invited@example.org", "SafePassword123")
	require.NoError(t, err)
	require.Equal(t, "pending", req.Status)
	require.NotEqual(t, "SafePassword123", req.PasswordHash)
	require.True(t, common.ValidatePasswordAndHash("SafePassword123", req.PasswordHash))
	var count int64
	require.NoError(t, DB.Model(&User{}).Count(&count).Error)
	require.Zero(t, count)
	user := User{Username: "applicant", Password: "SafePassword123"}
	require.Error(t, user.ValidateAndFill())
	b, err := common.Marshal(req)
	require.NoError(t, err)
	require.NotContains(t, string(b), req.PasswordHash)
	require.NotContains(t, string(b), "password")
}
func TestInvitationRejectsWrongEmailExpiredAndRevoked(t *testing.T) {
	registrationDB(t)
	inv, code := invitationFixture(t)
	_, err := SubmitInvitationRegistration(code, "applicant", "other@example.org", "SafePassword123")
	require.ErrorIs(t, err, ErrInvitationUnavailable)
	require.NoError(t, DB.Model(inv).Update("expires_at", time.Now().Add(-time.Second).Unix()).Error)
	_, err = SubmitInvitationRegistration(code, "applicant", "invited@example.org", "SafePassword123")
	require.ErrorIs(t, err, ErrInvitationUnavailable)
	require.NoError(t, DB.Model(inv).Updates(map[string]interface{}{"expires_at": time.Now().Add(time.Hour).Unix(), "revoked_at": time.Now().Unix()}).Error)
	_, err = SubmitInvitationRegistration(code, "applicant", "invited@example.org", "SafePassword123")
	require.ErrorIs(t, err, ErrInvitationUnavailable)
}
func TestInvitationConsumedOnlyOnceConcurrently(t *testing.T) {
	registrationDB(t)
	_, code := invitationFixture(t)
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := SubmitInvitationRegistration(code, "applicant", "invited@example.org", "SafePassword123")
			if err == nil {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, accepted.Load())
	var count int64
	require.NoError(t, DB.Model(&RegistrationRequest{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}
func TestRegistrationApprovalIsAtomicAndIdempotent(t *testing.T) {
	registrationDB(t)
	_, code := invitationFixture(t)
	req, err := SubmitInvitationRegistration(code, "applicant", "invited@example.org", "SafePassword123")
	require.NoError(t, err)
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := ReviewInvitationRegistration(req.Id, 1, true)
			if err == nil {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, accepted.Load())
	var user User
	require.NoError(t, DB.Where("username = ?", "applicant").First(&user).Error)
	require.Equal(t, common.RoleCommonUser, user.Role)
	require.Equal(t, common.UserStatusEnabled, user.Status)
	require.Equal(t, 100, user.Quota)
	require.True(t, common.ValidatePasswordAndHash("SafePassword123", user.Password))
	require.NoError(t, DB.First(req, req.Id).Error)
	require.Empty(t, req.PasswordHash)
	require.Nil(t, req.PendingEmail)
	require.Equal(t, user.Id, req.UserId)
}
func TestRegistrationRejectClearsPasswordAndAllowsNewInvitation(t *testing.T) {
	registrationDB(t)
	_, code := invitationFixture(t)
	req, err := SubmitInvitationRegistration(code, "applicant", "invited@example.org", "SafePassword123")
	require.NoError(t, err)
	req, err = ReviewInvitationRegistration(req.Id, 1, false)
	require.NoError(t, err)
	require.Equal(t, "rejected", req.Status)
	require.Empty(t, req.PasswordHash)
	var count int64
	DB.Model(&User{}).Count(&count)
	require.Zero(t, count)
	_, code = invitationFixture(t)
	_, err = SubmitInvitationRegistration(code, "applicant", "invited@example.org", "SafePassword123")
	require.NoError(t, err)
}
func TestRegistrationApprovalConflictRollsBack(t *testing.T) {
	registrationDB(t)
	_, code := invitationFixture(t)
	req, err := SubmitInvitationRegistration(code, "applicant", "invited@example.org", "SafePassword123")
	require.NoError(t, err)
	require.NoError(t, DB.Create(&User{Username: "applicant", AffCode: "existing-aff-code"}).Error)
	_, err = ReviewInvitationRegistration(req.Id, 1, true)
	require.ErrorIs(t, err, ErrRegistrationConflict)
	require.NoError(t, DB.First(req, req.Id).Error)
	require.Equal(t, "pending", req.Status)
	require.NotEmpty(t, req.PasswordHash)
}

func TestRegistrationDuplicatePendingRollsBackInvitationClaim(t *testing.T) {
	registrationDB(t)
	_, code := invitationFixture(t)
	_, err := SubmitInvitationRegistration(code, "applicant", "invited@example.org", "SafePassword123")
	require.NoError(t, err)
	inv, code := invitationFixture(t)
	_, err = SubmitInvitationRegistration(code, "othername", "invited@example.org", "SafePassword123")
	require.Error(t, err)
	require.NoError(t, DB.First(inv, inv.Id).Error)
	require.Zero(t, inv.ConsumedAt)
}

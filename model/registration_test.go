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
	require.NoError(t, DB.AutoMigrate(&RegistrationInvitation{}, &RegistrationRequest{}, &AccountApplication{}))
	truncateTables(t)
	t.Cleanup(func() {
		DB.Exec("DELETE FROM account_applications")
		DB.Exec("DELETE FROM registration_requests")
		DB.Exec("DELETE FROM registration_invitations")
	})
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

func TestEmailApplicationNeedsOnlyEmailAndRejectsDuplicates(t *testing.T) {
	registrationDB(t)
	req, err := SubmitAccountApplication(" APPLICANT@EXAMPLE.ORG ")
	require.NoError(t, err)
	require.Equal(t, "applicant@example.org", req.Email)
	require.Equal(t, "pending", req.Status)
	require.Empty(t, req.Username)
	require.Zero(t, req.UserId)
	var count int64
	require.NoError(t, DB.Model(&User{}).Count(&count).Error)
	require.Zero(t, count)
	_, err = SubmitAccountApplication("applicant@example.org")
	require.ErrorIs(t, err, ErrApplicationEmailPending)
	rejected, password, err := ReviewAccountApplication(req.Id, 1, 0, false)
	require.NoError(t, err)
	require.Empty(t, password)
	require.Equal(t, "rejected", rejected.Status)
	require.Nil(t, rejected.PendingEmail)
	_, err = SubmitAccountApplication(req.Email)
	require.NoError(t, err)
}

func TestEmailApprovalGeneratesOrdinaryCredentialsOnlyOnce(t *testing.T) {
	registrationDB(t)
	req, err := SubmitAccountApplication("approved@example.org")
	require.NoError(t, err)
	approved, password, err := ReviewAccountApplication(req.Id, 1, 321, true)
	require.NoError(t, err)
	require.Len(t, password, 16)
	require.Equal(t, req.Email, approved.Username)
	require.Equal(t, "sending", approved.CredentialDeliveryStatus)
	var user User
	require.NoError(t, DB.First(&user, approved.UserId).Error)
	require.Equal(t, common.RoleCommonUser, user.Role)
	require.Equal(t, 321, user.Quota)
	require.True(t, common.ValidatePasswordAndHash(password, user.Password))
	require.NotEqual(t, password, user.Password)
	encoded, err := common.Marshal(approved)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), password)
	require.NotContains(t, string(encoded), user.Password)
	_, again, err := ReviewAccountApplication(req.Id, 1, 321, true)
	require.Error(t, err)
	require.Empty(t, again)
	_, err = SubmitAccountApplication(req.Email)
	require.ErrorIs(t, err, ErrApplicationEmailRegistered)
}

func TestEmailApprovalSerializesConcurrentReviews(t *testing.T) {
	registrationDB(t)
	req, err := SubmitAccountApplication("race@example.org")
	require.NoError(t, err)
	var approved int32
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := ReviewAccountApplication(req.Id, 1, 10, true); err == nil {
				atomic.AddInt32(&approved, 1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, approved)
	var count int64
	require.NoError(t, DB.Model(&User{}).Where("email = ?", req.Email).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestCredentialRetryCannotResetAnEstablishedAccount(t *testing.T) {
	registrationDB(t)
	req, err := SubmitAccountApplication("retry@example.org")
	require.NoError(t, err)
	approved, first, err := ReviewAccountApplication(req.Id, 1, 100, true)
	require.NoError(t, err)
	require.NoError(t, DB.Model(approved).Update("credential_delivery_status", "failed").Error)
	retried, second, err := RetryApplicationCredentials(req.Id)
	require.NoError(t, err)
	require.NotEqual(t, first, second)
	var user User
	require.NoError(t, DB.First(&user, retried.UserId).Error)
	require.True(t, common.ValidatePasswordAndHash(second, user.Password))
	require.False(t, common.ValidatePasswordAndHash(first, user.Password))
	require.NoError(t, DB.Model(retried).Update("credential_delivery_status", "failed").Error)
	require.NoError(t, DB.Model(&user).Update("last_login_at", time.Now().Unix()).Error)
	_, password, err := RetryApplicationCredentials(req.Id)
	require.Error(t, err)
	require.Empty(t, password)
	var after User
	require.NoError(t, DB.First(&after, user.Id).Error)
	require.Equal(t, user.Password, after.Password)
	var application AccountApplication
	require.NoError(t, DB.First(&application, req.Id).Error)
	require.Equal(t, "failed", application.CredentialDeliveryStatus)
}

func TestLegacyApplicationUsernameMigrationPreservesIdentityAndIsIdempotent(t *testing.T) {
	registrationDB(t)
	hash, err := common.Password2Hash("FixturePassword123")
	require.NoError(t, err)
	user := User{Username: "acct_legacy", DisplayName: "acct_legacy", Email: "long.registration.email@example.org", Password: hash, Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 12345, Group: "default", AffCode: "migration-fixture"}
	require.NoError(t, DB.Create(&user).Error)
	application := AccountApplication{Email: user.Email, Username: user.Username, UserId: user.Id, Status: "approved", CredentialDeliveryStatus: "sent"}
	require.NoError(t, DB.Create(&application).Error)
	require.NoError(t, MigrateApplicationEmailUsernames())
	require.NoError(t, MigrateApplicationEmailUsernames())
	var after User
	require.NoError(t, DB.First(&after, user.Id).Error)
	require.Equal(t, user.Email, after.Username)
	require.Equal(t, user.Email, after.DisplayName)
	require.Equal(t, hash, after.Password)
	require.Equal(t, user.Quota, after.Quota)
	require.Equal(t, user.Role, after.Role)
	require.Equal(t, user.Id, after.Id)
	after.Password = "FixturePassword123" // Validate submitted plaintext fields, not a stored bcrypt hash.
	require.NoError(t, common.Validate.Struct(after))
	require.NoError(t, DB.First(&application, application.Id).Error)
	require.Equal(t, after.Username, application.Username)
}

func TestEmailUsernameCollisionDoesNotApproveOrOverwriteAnotherAccount(t *testing.T) {
	registrationDB(t)
	existing := User{Username: "collision@example.org", Email: "another@example.org", Password: "fixture-hash", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "collision-owner"}
	require.NoError(t, DB.Create(&existing).Error)
	_, err := SubmitAccountApplication("collision@example.org")
	require.ErrorIs(t, err, ErrApplicationEmailRegistered)
	req := AccountApplication{Email: "collision@example.org", Status: "pending"}
	require.NoError(t, DB.Create(&req).Error)
	_, password, err := ReviewAccountApplication(req.Id, 1, 100, true)
	require.Error(t, err)
	require.Empty(t, password)
	var after AccountApplication
	require.NoError(t, DB.First(&after, req.Id).Error)
	require.Equal(t, "pending", after.Status)
	var untouched User
	require.NoError(t, DB.First(&untouched, existing.Id).Error)
	require.Equal(t, existing.Email, untouched.Email)
	require.Equal(t, existing.Password, untouched.Password)
}

func TestEmailUsernameMigrationRollsBackOnCollisionAndPreservesCustomNames(t *testing.T) {
	registrationDB(t)
	legacy := User{Username: "acct_first", DisplayName: "acct_first", Email: "first@example.org", Role: common.RoleCommonUser, AffCode: "first"}
	blocked := User{Username: "acct_blocked", Email: "blocked@example.org", Role: common.RoleCommonUser, AffCode: "blocked"}
	collision := User{Username: blocked.Email, Email: "other@example.org", Role: common.RoleCommonUser, AffCode: "other"}
	custom := User{Username: "custom", DisplayName: "Custom Name", Email: "custom@example.org", Role: common.RoleCommonUser, AffCode: "custom"}
	admin := User{Username: "acct_admin", Email: "admin@example.org", Role: common.RoleRootUser, AffCode: "admin"}
	for _, user := range []*User{&legacy, &blocked, &collision, &custom, &admin} {
		require.NoError(t, DB.Create(user).Error)
	}
	for _, user := range []User{legacy, blocked, custom, admin} {
		require.NoError(t, DB.Create(&AccountApplication{Email: user.Email, Username: user.Username, UserId: user.Id, Status: "approved"}).Error)
	}
	require.ErrorIs(t, MigrateApplicationEmailUsernames(), ErrRegistrationConflict)
	var after User
	require.NoError(t, DB.First(&after, legacy.Id).Error)
	require.Equal(t, legacy.Username, after.Username)
	require.NoError(t, DB.Unscoped().Delete(&collision).Error)
	require.NoError(t, MigrateApplicationEmailUsernames())
	for _, user := range []User{custom, admin} {
		var retained User
		require.NoError(t, DB.First(&retained, user.Id).Error)
		require.Equal(t, user.Username, retained.Username)
		require.Equal(t, user.DisplayName, retained.DisplayName)
	}
}

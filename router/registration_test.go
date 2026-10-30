package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRegistrationRoutesRequireRootAndPreservePendingState(t *testing.T) {
	t.Setenv("INVITATION_REGISTRATION_ENABLED", "true")
	t.Setenv("PRLAB_APPROVAL_EMAIL", "")
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.UsingSQLite = true
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}, &model.RegistrationInvitation{}, &model.RegistrationRequest{}))
	oldRegister, oldPassword, oldVerification := common.RegisterEnabled, common.PasswordRegisterEnabled, common.EmailVerificationEnabled
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	t.Cleanup(func() {
		common.RegisterEnabled = oldRegister
		common.PasswordRegisterEnabled = oldPassword
		common.EmailVerificationEnabled = oldVerification
	})
	r := gin.New()
	r.Use(sessions.Sessions("test-session", cookie.NewStore([]byte("registration-test-cookie-secret"))))
	r.Use(func(c *gin.Context) {
		if role, err := strconv.Atoi(c.GetHeader("X-Test-Role")); err == nil {
			s := sessions.Default(c)
			s.Set("username", "operator")
			s.Set("id", 1)
			s.Set("role", role)
			s.Set("status", common.UserStatusEnabled)
		}
	})
	SetApiRouter(r)
	call := func(method, path string, body interface{}, role int) *httptest.ResponseRecorder {
		b, e := common.Marshal(body)
		require.NoError(t, e)
		q := httptest.NewRequest(method, path, bytes.NewReader(b))
		q.Header.Set("Content-Type", "application/json")
		if role > 0 {
			q.Header.Set("X-Test-Role", strconv.Itoa(role))
			q.Header.Set("New-Api-User", "1")
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, q)
		return w
	}
	for _, role := range []int{0, common.RoleCommonUser, common.RoleAdminUser} {
		for _, path := range []string{"/api/registration/invitations", "/api/registration/requests/1/review", "/api/registration/requests/1/notify", "/api/registration/invitations/1/revoke"} {
			w := call(http.MethodPost, path, map[string]interface{}{"action": "approve"}, role)
			require.NotContains(t, w.Body.String(), `"success":true`)
		}
		w := call(http.MethodGet, "/api/registration/", nil, role)
		require.NotContains(t, w.Body.String(), `"success":true`)
	}
	w := call(http.MethodPost, "/api/user/register", map[string]string{"username": "applicant", "email": "invited@example.org", "password": "SafePassword123"}, 0)
	require.Equal(t, 400, w.Code)
	_, code, err := model.CreateRegistrationInvitation("invited@example.org", 1, 100, time.Now().Add(time.Hour).Unix())
	require.NoError(t, err)
	w = call(http.MethodPost, "/api/user/register", map[string]interface{}{"username": "applicant", "email": "invited@example.org", "password": "SafePassword123", "invite_code": code, "role": 100, "status": 1}, 0)
	require.Equal(t, 202, w.Code)
	require.Contains(t, w.Body.String(), `"status":"pending"`)
	var count int64
	db.Model(&model.User{}).Count(&count)
	require.Zero(t, count)
	w = call(http.MethodGet, "/api/registration/", nil, common.RoleRootUser)
	require.Equal(t, 200, w.Code)
	require.NotContains(t, w.Body.String(), "password_hash")
	require.NotContains(t, w.Body.String(), "token_hash")
	require.NotContains(t, w.Body.String(), code)
	w = call(http.MethodPost, "/api/registration/requests/1/review", map[string]string{"action": "approve"}, common.RoleRootUser)
	require.Equal(t, 200, w.Code)
	var user model.User
	require.NoError(t, db.First(&user).Error)
	require.Equal(t, common.RoleCommonUser, user.Role)
	w = call(http.MethodPost, "/api/registration/requests/1/review", map[string]string{"action": "approve"}, common.RoleRootUser)
	require.Equal(t, 409, w.Code)
}

package controller

import (
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

var registrationUsername = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,20}$`)

func validInvitationEmail(email string) bool {
	a, err := mail.ParseAddress(email)
	return err == nil && a.Address == email && len(email) <= 50
}

func RegisterWithInvitation(c *gin.Context) {
	if !common.RegisterEnabled || !common.PasswordRegisterEnabled {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "注册申请已关闭"})
		return
	}
	var input struct {
		Username         string `json:"username"`
		Email            string `json:"email"`
		Password         string `json:"password"`
		InviteCode       string `json:"invite_code"`
		VerificationCode string `json:"verification_code"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	if common.DecodeJson(c.Request.Body, &input) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "申请信息无效"})
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.InviteCode = strings.TrimSpace(input.InviteCode)
	if !registrationUsername.MatchString(input.Username) || !validInvitationEmail(input.Email) || common.Validate.Var(input.Password, "min=8,max=20") != nil || len(input.InviteCode) != 53 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请检查用户名、邮箱、8–20 位密码和邀请码"})
		return
	}
	if common.EmailVerificationEnabled && !common.VerifyCodeWithKey(input.Email, input.VerificationCode, common.EmailVerificationPurpose) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "邮箱验证码无效"})
		return
	}
	req, err := model.SubmitInvitationRegistration(input.InviteCode, input.Username, input.Email, input.Password)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "邀请无效、已使用、已过期，或账号已有申请"})
		return
	}
	// Failure to notify must not lose the persisted application.
	service.NotifyRegistrationRequest(req.Id)
	c.JSON(http.StatusAccepted, gin.H{"success": true, "message": "申请已提交，等待管理员审批。批准前无法登录或使用 API。", "data": gin.H{"status": "pending"}})
}

func GetRegistrationApprovals(c *gin.Context) {
	status := c.DefaultQuery("status", "pending")
	if status != "pending" && status != "approved" && status != "rejected" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "申请状态无效"})
		return
	}
	var requests []model.RegistrationRequest
	var invitations []model.RegistrationInvitation
	if model.DB.Where("status = ?", status).Order("id desc").Limit(100).Find(&requests).Error != nil || model.DB.Order("id desc").Limit(100).Find(&invitations).Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取申请失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"enabled": common.InvitationRegistrationEnabled(), "notification_ready": service.RegistrationNotificationReady(), "requests": requests, "invitations": invitations}})
}

func CreateRegistrationInvitation(c *gin.Context) {
	var input struct {
		Email        string `json:"email"`
		ValidDays    int    `json:"valid_days"`
		InitialQuota int    `json:"initial_quota"`
	}
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(400, gin.H{"success": false, "message": "邀请参数无效"})
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !validInvitationEmail(input.Email) || input.ValidDays < 1 || input.ValidDays > 30 || input.InitialQuota < 0 || input.InitialQuota > 1000000000 {
		c.JSON(400, gin.H{"success": false, "message": "请输入有效邮箱、1–30 天有效期和非负额度"})
		return
	}
	inv, token, err := model.CreateRegistrationInvitation(input.Email, c.GetInt("id"), input.InitialQuota, time.Now().Add(time.Duration(input.ValidDays)*24*time.Hour).Unix())
	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "创建邀请失败"})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": gin.H{"invitation": inv, "invite_code": token}})
}

func RevokeRegistrationInvitation(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		c.JSON(400, gin.H{"success": false, "message": "邀请编号无效"})
		return
	}
	r := model.DB.Model(&model.RegistrationInvitation{}).Where("id = ? AND consumed_at = 0 AND revoked_at = 0", id).Update("revoked_at", time.Now().Unix())
	if r.Error != nil || r.RowsAffected != 1 {
		c.JSON(409, gin.H{"success": false, "message": "邀请已使用、已撤销或不存在"})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func ReviewRegistrationRequest(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	var input struct {
		Action string `json:"action"`
	}
	if err != nil || id < 1 || c.ShouldBindJSON(&input) != nil || (input.Action != "approve" && input.Action != "reject") {
		c.JSON(400, gin.H{"success": false, "message": "审批参数无效"})
		return
	}
	req, err := model.ReviewInvitationRegistration(id, c.GetInt("id"), input.Action == "approve")
	if err != nil {
		c.JSON(409, gin.H{"success": false, "message": "申请已处理、不存在，或账号已存在"})
		return
	}
	model.RecordLog(c.GetInt("id"), model.LogTypeManage, "Registration request "+strconv.Itoa(id)+": "+input.Action)
	c.JSON(200, gin.H{"success": true, "data": req})
}

func NotifyRegistrationRequest(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		c.JSON(400, gin.H{"success": false, "message": "申请编号无效"})
		return
	}
	status := service.NotifyRegistrationRequest(id)
	c.JSON(200, gin.H{"success": status == "sent", "message": map[string]string{"sent": "通知已发送", "unconfigured": "请先配置审批收件邮箱和 SMTP", "failed": "通知发送失败，请检查 SMTP 与网络", "reviewed": "申请已处理或不存在"}[status]})
}

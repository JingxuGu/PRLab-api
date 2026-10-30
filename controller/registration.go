package controller

import (
	"errors"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func validApplicationEmail(email string) bool {
	a, err := mail.ParseAddress(email)
	return err == nil && a.Address == email && len(email) <= 50
}

func RegisterAccountApplication(c *gin.Context) {
	if !common.RegisterEnabled || !common.PasswordRegisterEnabled {
		c.JSON(403, gin.H{"success": false, "message": "账号申请已关闭"})
		return
	}
	var input struct {
		Email string `json:"email"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	if common.DecodeJson(c.Request.Body, &input) != nil {
		c.JSON(400, gin.H{"success": false, "message": "请输入有效邮箱"})
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !validApplicationEmail(input.Email) {
		c.JSON(400, gin.H{"success": false, "message": "请输入有效邮箱"})
		return
	}
	application, err := model.SubmitAccountApplication(input.Email)
	if errors.Is(err, model.ErrApplicationEmailRegistered) {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "该邮箱已注册，请登录或联系管理员。", "data": gin.H{"status": "registered"}})
		return
	}
	if errors.Is(err, model.ErrApplicationEmailPending) {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "该邮箱已提交申请，正在等待管理员审核，请勿重复提交。", "data": gin.H{"status": "pending"}})
		return
	}
	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "申请提交失败，请稍后重试"})
		return
	}
	service.NotifyAccountApplication(application.Id)
	c.JSON(202, gin.H{"success": true, "message": "申请已收到。审核通过后，账号和初始密码会发送到申请邮箱。", "data": gin.H{"status": "pending"}})
}

func GetRegistrationApprovals(c *gin.Context) {
	status := c.DefaultQuery("status", "pending")
	if status != "pending" && status != "approved" && status != "rejected" {
		c.JSON(400, gin.H{"success": false, "message": "申请状态无效"})
		return
	}
	var applications []model.AccountApplication
	if model.DB.Where("status = ?", status).Order("id desc").Limit(100).Find(&applications).Error != nil {
		c.JSON(500, gin.H{"success": false, "message": "读取申请失败"})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": gin.H{"enabled": common.ApprovalRegistrationEnabled(), "notification_ready": service.RegistrationNotificationReady(), "requests": applications}})
}

func ReviewRegistrationRequest(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	var input struct {
		Action       string `json:"action"`
		InitialQuota *int   `json:"initial_quota"`
	}
	if err != nil || id < 1 || c.ShouldBindJSON(&input) != nil || (input.Action != "approve" && input.Action != "reject") {
		c.JSON(400, gin.H{"success": false, "message": "审批参数无效"})
		return
	}
	quota := 100000000
	if input.InitialQuota != nil {
		quota = *input.InitialQuota
	}
	if quota < 0 || quota > 1000000000 {
		c.JSON(400, gin.H{"success": false, "message": "初始额度无效"})
		return
	}
	// Avoid creating an account whose credentials cannot be delivered at all.
	if input.Action == "approve" && !service.RegistrationNotificationReady() {
		c.JSON(409, gin.H{"success": false, "message": "请先配置邮件服务"})
		return
	}
	application, password, err := model.ReviewAccountApplication(id, c.GetInt("id"), quota, input.Action == "approve")
	if err != nil {
		c.JSON(409, gin.H{"success": false, "message": "申请已处理、不存在，或邮箱已有账号"})
		return
	}
	if input.Action == "approve" {
		application.CredentialDeliveryStatus = service.DeliverApplicationCredentials(application, password)
	}
	model.RecordLog(c.GetInt("id"), model.LogTypeManage, "Account application "+strconv.Itoa(id)+": "+input.Action)
	c.JSON(200, gin.H{"success": true, "data": application})
}

func NotifyRegistrationRequest(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		c.JSON(400, gin.H{"success": false, "message": "申请编号无效"})
		return
	}
	status := service.NotifyAccountApplication(id)
	c.JSON(200, gin.H{"success": status == "sent", "message": map[string]string{"sent": "通知已发送", "failed": "通知发送失败，请检查邮件服务", "unconfigured": "请先配置邮件服务", "reviewed": "申请已处理或不存在"}[status]})
}

func RetryRegistrationCredentials(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		c.JSON(400, gin.H{"success": false, "message": "申请编号无效"})
		return
	}
	if !service.RegistrationNotificationReady() {
		c.JSON(409, gin.H{"success": false, "message": "请先配置邮件服务"})
		return
	}
	application, password, err := model.RetryApplicationCredentials(id)
	if err != nil {
		c.JSON(409, gin.H{"success": false, "message": "仅可在首次登录前重试失败的账号邮件"})
		return
	}
	status := service.DeliverApplicationCredentials(application, password)
	application.CredentialDeliveryStatus = status
	c.JSON(200, gin.H{"success": status == "sent", "data": application, "message": map[string]string{"sent": "账号邮件已发送", "failed": "账号邮件发送失败，可在首次登录前重试"}[status]})
}

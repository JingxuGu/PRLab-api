package service

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRegistrationNotificationPersistsWithoutSMTPAndRetries(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB })
	require.NoError(t, db.AutoMigrate(&model.RegistrationRequest{}))
	req := model.RegistrationRequest{Username: "<unsafe>", Email: "invited@example.org", Status: "pending", PasswordHash: "do-not-send-this-hash"}
	require.NoError(t, db.Create(&req).Error)
	t.Setenv("PRLAB_APPROVAL_EMAIL", "")
	require.Equal(t, "unconfigured", NotifyRegistrationRequest(req.Id))
	require.NoError(t, db.First(&req, req.Id).Error)
	require.Equal(t, "pending", req.Status)
	require.Equal(t, "unconfigured", req.NotificationStatus)
	oldHost, oldFrom, oldPort, oldBase := common.SMTPServer, common.SMTPFrom, common.SMTPPort, system_setting.ServerAddress
	oldSender := sendRegistrationEmail
	t.Cleanup(func() {
		common.SMTPServer = oldHost
		common.SMTPFrom = oldFrom
		common.SMTPPort = oldPort
		system_setting.ServerAddress = oldBase
		sendRegistrationEmail = oldSender
	})
	common.SMTPServer = "smtp.example.org"
	common.SMTPFrom = "sender@example.org"
	common.SMTPPort = 465
	system_setting.ServerAddress = "https://api.example.org"
	t.Setenv("PRLAB_APPROVAL_EMAIL", "owner@example.org")
	t.Setenv("SMTP_FROM_NAME", "")
	require.Equal(t, common.SystemName, common.EmailSenderName())
	t.Setenv("SMTP_FROM_NAME", " example.org ")
	require.Equal(t, "example.org", common.EmailSenderName())
	sendRegistrationEmail = func(c registrationMailConfig, body string) error {
		require.Equal(t, "example.org", c.SenderName)
		require.Equal(t, "owner@example.org", c.Recipient)
		require.Contains(t, body, "&lt;unsafe&gt;")
		require.NotContains(t, body, req.PasswordHash)
		require.NotContains(t, body, "<unsafe>")
		require.Contains(t, body, "https://api.example.org/console/user")
		return errors.New("test delivery failure")
	}
	require.Equal(t, "failed", NotifyRegistrationRequest(req.Id))
	sendRegistrationEmail = func(c registrationMailConfig, body string) error { return nil }
	require.Equal(t, "sent", NotifyRegistrationRequest(req.Id))
	require.NoError(t, db.Model(&req).Update("status", "rejected").Error)
	require.Equal(t, "reviewed", NotifyRegistrationRequest(req.Id))
}

func TestRegistrationSMTPRejectsPlaintextBeforeAuthentication(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	commands := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			commands <- ""
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		_, _ = fmt.Fprint(conn, "220 example.org ESMTP\r\n")
		var seen strings.Builder
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := scanner.Text()
			seen.WriteString(line + "\n")
			_, _ = fmt.Fprint(conn, "250 example.org\r\n")
		}
		commands <- seen.String()
	}()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	n, err := strconv.Atoi(port)
	require.NoError(t, err)
	err = sendApprovalEmail(registrationMailConfig{Host: host, Port: n, Account: "test-sender", Password: "test-password"}, "test")
	require.Error(t, err)
	require.NotContains(t, strings.ToUpper(<-commands), "AUTH")
}

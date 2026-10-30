package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"html"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

type registrationMailConfig struct {
	Host, Account, Password, From, Recipient, SenderName, ReviewURL, Subject string
	Port                                                                     int
	ImplicitTLS, LoginAuth                                                   bool
}

func registrationMailSettings() (registrationMailConfig, bool) {
	common.OptionMapRWMutex.RLock()
	c := registrationMailConfig{Host: common.SMTPServer, Port: common.SMTPPort, Account: common.SMTPAccount, Password: common.SMTPToken, From: common.SMTPFrom, SenderName: common.EmailSenderName(), ImplicitTLS: common.SMTPSSLEnabled || common.SMTPPort == 465, LoginAuth: common.SMTPForceAuthLogin}
	base := system_setting.ServerAddress
	common.OptionMapRWMutex.RUnlock()
	c.Recipient = strings.TrimSpace(os.Getenv("PRLAB_APPROVAL_EMAIL"))
	if c.From == "" {
		c.From = c.Account
	}
	recipient, err := mail.ParseAddress(c.Recipient)
	if err != nil || recipient.Address != c.Recipient {
		return c, false
	}
	from, err := mail.ParseAddress(c.From)
	if err != nil || from.Address != c.From {
		return c, false
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return c, false
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/console/user"
	u.RawQuery = ""
	u.Fragment = ""
	c.ReviewURL = u.String()
	return c, c.Host != "" && c.Port > 0 && c.Port <= 65535
}

func RegistrationNotificationReady() bool {
	_, ready := registrationMailSettings()
	return ready
}

// Use verified TLS and a single bounded deadline. Do not log SMTP credentials or errors.
func sendApprovalEmail(c registrationMailConfig, body string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(c.Host, strconv.Itoa(c.Port)))
	if err != nil {
		return err
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}
	tlsConfig := &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12}
	if c.ImplicitTLS {
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return err
		}
		conn = tlsConn
	}
	client, err := smtp.NewClient(conn, c.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	if !c.ImplicitTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("SMTP requires TLS")
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return err
		}
	}
	if c.Account != "" {
		var auth smtp.Auth = smtp.PlainAuth("", c.Account, c.Password, c.Host)
		if c.LoginAuth {
			auth = common.LoginAuth(c.Account, c.Password)
		}
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(c.From); err != nil {
		return err
	}
	if err := client.Rcpt(c.Recipient); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	from := (&mail.Address{Name: c.SenderName, Address: c.From}).String()
	subject := c.Subject
	if subject == "" {
		subject = "Registration approval requested"
	}
	message := "From: " + from + "\r\nTo: " + c.Recipient + "\r\nSubject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\nDate: " + time.Now().Format(time.RFC1123Z) + "\r\nAuto-Submitted: auto-generated\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" + body
	if _, err := w.Write([]byte(message)); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

var sendRegistrationEmail = sendApprovalEmail

// The durable request remains actionable when SMTP is unavailable. Root can retry later.
func NotifyRegistrationRequest(id int) string {
	var req model.RegistrationRequest
	if model.DB.First(&req, id).Error != nil || req.Status != "pending" {
		return "reviewed"
	}
	c, ready := registrationMailSettings()
	status := "unconfigured"
	if ready {
		body := fmt.Sprintf("<p>A registration request is awaiting your review.</p><p>Request: %d<br>Username: %s<br>Email: %s</p><p><a href=\"%s\">Sign in as administrator to approve or reject</a></p><p>Opening this link does not approve the account.</p>", req.Id, html.EscapeString(req.Username), html.EscapeString(req.Email), html.EscapeString(c.ReviewURL))
		status = "sent"
		if sendRegistrationEmail(c, body) != nil {
			status = "failed"
		}
	}
	model.DB.Model(&model.RegistrationRequest{}).Where("id = ?", id).Update("notification_status", status)
	return status
}

// Notify only the administrator when an anonymous email application is received.
func NotifyAccountApplication(id int) string {
	var application model.AccountApplication
	if model.DB.First(&application, id).Error != nil || application.Status != "pending" {
		return "reviewed"
	}
	c, ready := registrationMailSettings()
	status := "unconfigured"
	if ready {
		body := fmt.Sprintf("<p>An account application is awaiting your review.</p><p>Request: %d<br>Email: %s</p><p><a href=\"%s\">Sign in as administrator to approve or reject</a></p><p>Approval creates an ordinary account and emails its credentials to the applicant. Opening this link does not approve the application.</p>", application.Id, html.EscapeString(application.Email), html.EscapeString(c.ReviewURL))
		status = "sent"
		if sendRegistrationEmail(c, body) != nil {
			status = "failed"
		}
	}
	model.DB.Model(&model.AccountApplication{}).Where("id = ?", id).Update("notification_status", status)
	return status
}

// The generated plaintext password exists only for this mail attempt, not in DB or logs.
func DeliverApplicationCredentials(application *model.AccountApplication, password string) string {
	c, ready := registrationMailSettings()
	status := "failed"
	if ready && password != "" {
		c.Recipient = application.Email
		c.Subject = "Your account is ready"
		loginURL := strings.TrimSuffix(c.ReviewURL, "/console/user") + "/login"
		body := fmt.Sprintf("<p>Your account application has been approved.</p><p>Username: <strong>%s</strong><br>Initial password: <strong>%s</strong></p><p><a href=\"%s\">Sign in</a></p><p>You can change this initial password in your personal settings after signing in. Keep this email private and do not forward these credentials.</p>", html.EscapeString(application.Username), html.EscapeString(password), html.EscapeString(loginURL))
		if sendRegistrationEmail(c, body) == nil {
			status = "sent"
		}
	}
	model.DB.Model(&model.AccountApplication{}).Where("id = ? AND credential_delivery_status = ?", application.Id, "sending").Update("credential_delivery_status", status)
	return status
}

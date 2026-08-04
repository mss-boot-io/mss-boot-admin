package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/mail"
	"net/smtp"
	"sort"
	"time"
)

//go:embed *.html
var FS embed.FS

type SendType string

const (
	RegisterSender      SendType = "register"
	LoginSender         SendType = "login"
	ResetPasswordSender SendType = "resetPassword"
)

func (s SendType) String() string {
	return string(s)
}

type VerifyCodeSender func(smtpHost, smtpPort, from, password, username, to, code, organization string) error

var Sender = map[SendType]VerifyCodeSender{
	RegisterSender:      SendRegisterVerifyCode,
	LoginSender:         SendLoginVerifyCode,
	ResetPasswordSender: SendResetPasswordVerifyCode,
}

var smtpSendMail = smtp.SendMail

// SendRegisterVerifyCode sends the registration verification template.
func SendRegisterVerifyCode(smtpHost, smtpPort, from, password, username, to, code, organization string) error {
	return sendVerifyCode("register_verify_code.html", smtpHost, smtpPort, from, password, username, to, code, organization)
}

// SendLoginVerifyCode sends the login verification template.
func SendLoginVerifyCode(smtpHost, smtpPort, from, password, username, to, code, organization string) error {
	return sendVerifyCode("login_verify_code.html", smtpHost, smtpPort, from, password, username, to, code, organization)
}

// SendResetPasswordVerifyCode sends the password-reset verification template.
func SendResetPasswordVerifyCode(smtpHost, smtpPort, from, password, username, to, code, organization string) error {
	return sendVerifyCode("password_reset_code.html", smtpHost, smtpPort, from, password, username, to, code, organization)
}

func sendVerifyCode(templateName, smtpHost, smtpPort, from, password, username, to, code, organization string) error {
	templateBytes, err := FS.ReadFile(templateName)
	if err != nil {
		return fmt.Errorf("read verification email template %q: %w", templateName, err)
	}
	tmpl, err := template.New("email").Parse(string(templateBytes))
	if err != nil {
		return fmt.Errorf("parse verification email template %q: %w", templateName, err)
	}
	data := map[string]any{
		"Code":         code,
		"Year":         time.Now().Year(),
		"Organization": organization,
	}
	var body bytes.Buffer
	if err = tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("render verification email template %q: %w", templateName, err)
	}

	fromAddress := mail.Address{Name: organization, Address: from}
	toAddress := mail.Address{Name: username, Address: to}
	headers := map[string]string{
		"From":         fromAddress.String(),
		"To":           toAddress.String(),
		"Subject":      "Your verification code is " + code + " (valid for 5 minutes)",
		"Date":         time.Now().Format(time.RFC1123Z),
		"MIME-Version": "1.0",
		"Content-Type": `text/html; charset="UTF-8"`,
	}

	var message bytes.Buffer
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err = fmt.Fprintf(&message, "%s: %s\r\n", key, headers[key]); err != nil {
			return err
		}
	}
	if _, err = fmt.Fprintf(&message, "\r\n%s", body.String()); err != nil {
		return err
	}

	auth := smtp.PlainAuth("", fromAddress.Address, password, smtpHost)
	if err = smtpSendMail(
		smtpHost+":"+smtpPort,
		auth,
		fromAddress.Address,
		[]string{toAddress.Address},
		message.Bytes(),
	); err != nil {
		slog.Error("verification email send failed", "error", err)
		return err
	}
	slog.Info("verification email sent")
	return nil
}

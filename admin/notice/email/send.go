package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net"
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

type VerifyCodeSender func(context.Context, string, string, string, string, string, string, string, string) error

var Sender = map[SendType]VerifyCodeSender{
	RegisterSender:      SendRegisterVerifyCode,
	LoginSender:         SendLoginVerifyCode,
	ResetPasswordSender: SendResetPasswordVerifyCode,
}

var smtpSendMail = sendMailContext

const verificationEmailTimeout = 10 * time.Second

// SendRegisterVerifyCode sends the registration verification template.
func SendRegisterVerifyCode(ctx context.Context, smtpHost, smtpPort, from, password, username, to, code, organization string) error {
	return sendVerifyCode(ctx, "register_verify_code.html", smtpHost, smtpPort, from, password, username, to, code, organization)
}

// SendLoginVerifyCode sends the login verification template.
func SendLoginVerifyCode(ctx context.Context, smtpHost, smtpPort, from, password, username, to, code, organization string) error {
	return sendVerifyCode(ctx, "login_verify_code.html", smtpHost, smtpPort, from, password, username, to, code, organization)
}

// SendResetPasswordVerifyCode sends the password-reset verification template.
func SendResetPasswordVerifyCode(ctx context.Context, smtpHost, smtpPort, from, password, username, to, code, organization string) error {
	return sendVerifyCode(ctx, "password_reset_code.html", smtpHost, smtpPort, from, password, username, to, code, organization)
}

func sendVerifyCode(ctx context.Context, templateName, smtpHost, smtpPort, from, password, username, to, code, organization string) error {
	if ctx == nil {
		return errors.New("verification email context is required")
	}
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
		"Subject":      "Your verification code (valid for 5 minutes)",
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
	sendCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		sendCtx, cancel = context.WithTimeout(ctx, verificationEmailTimeout)
		defer cancel()
	}
	if err = smtpSendMail(
		sendCtx,
		smtpHost+":"+smtpPort,
		auth,
		fromAddress.Address,
		[]string{toAddress.Address},
		message.Bytes(),
	); err != nil {
		// SMTP errors can echo the recipient address or message content. Keep
		// challenge subjects and codes out of process logs.
		slog.Error("verification email send failed")
		return err
	}
	slog.Info("verification email sent")
	return nil
}

func sendMailContext(
	ctx context.Context,
	address string,
	auth smtp.Auth,
	from string,
	to []string,
	message []byte,
) error {
	if ctx == nil {
		return errors.New("SMTP context is required")
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer connection.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err = connection.SetDeadline(deadline); err != nil {
			return err
		}
	}

	closed := make(chan struct{})
	defer close(closed)
	go func() {
		select {
		case <-ctx.Done():
			_ = connection.Close()
		case <-closed:
		}
	}()

	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return err
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err = client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if auth != nil {
		if ok, _ := client.Extension("AUTH"); !ok {
			return errors.New("SMTP server does not support authentication")
		}
		if err = client.Auth(auth); err != nil {
			return err
		}
	}
	if err = client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = writer.Write(message); err != nil {
		_ = writer.Close()
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

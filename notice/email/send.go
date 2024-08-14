package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/mail"
	"net/smtp"
	"time"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/8/13 17:04:03
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/8/13 17:04:03
 */

//go:embed *.html
var FS embed.FS

func SendLoginVerifyCode(smtpHost, smtpPort, from, password, username, to, code, organization string) error {
	return sendVerifyCode("login_verify_code.html", smtpHost, smtpPort, from, password, username, to, code, organization)
}

func SendResetPasswordVerifyCode(smtpHost, smtpPort, from, password, username, to, code, organization string) error {
	return sendVerifyCode("password_reset_code.html", smtpHost, smtpPort, from, password, username, to, code, organization)
}

func sendVerifyCode(temp, smtpHost, smtpPort, from, password, username, to, code, organization string) error {
	rb, err := FS.ReadFile(temp)
	if err != nil {
		return err
	}
	// html template parse
	tmpl, err := template.New("email").Parse(string(rb))
	if err != nil {
		return err
	}
	data := map[string]any{
		"Code":         code,
		"Year":         time.Now().Year(),
		"Organization": organization,
	}
	var body bytes.Buffer
	if err = tmpl.Execute(&body, data); err != nil {
		return err
	}
	// 发件人信息
	fromAddress := mail.Address{Name: organization, Address: from}
	// 收件人信息
	toAddress := mail.Address{Name: username, Address: to}

	// 邮件头信息
	headers := make(map[string]string)
	headers["From"] = fromAddress.String()
	headers["To"] = toAddress.String()
	headers["Subject"] = "Your verification code is " + code + " (valid for 5 minutes)"
	headers["Date"] = time.Now().Format(time.RFC1123Z)
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = `text/html; charset="UTF-8"`
	// 构建邮件内容
	var msg bytes.Buffer
	for k, v := range headers {
		_, err = fmt.Fprintf(&msg, "%s: %s\r\n", k, v)
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(&msg, "\r\n%s", body.String())
	if err != nil {
		return err
	}
	// SMTP 配置
	auth := smtp.PlainAuth("", fromAddress.Address, password, smtpHost)

	// 发送邮件
	err = smtp.SendMail(smtpHost+":"+smtpPort, auth, fromAddress.Address, []string{toAddress.Address}, msg.Bytes())
	if err != nil {
		slog.Error("Failed to send email", slog.Any("error", err))
		return err
	}
	slog.Info("Email sent successfully!")
	return nil
}

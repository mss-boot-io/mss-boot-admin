package email

import (
	"errors"
	"net/smtp"
	"strings"
	"testing"
)

type capturedMail struct {
	address string
	from    string
	to      []string
	message string
}

func TestVerificationEmailSendersRenderAndSend(t *testing.T) {
	previous := smtpSendMail
	t.Cleanup(func() { smtpSendMail = previous })

	var captured capturedMail
	smtpSendMail = func(address string, _ smtp.Auth, from string, to []string, message []byte) error {
		captured = capturedMail{
			address: address,
			from:    from,
			to:      append([]string(nil), to...),
			message: string(message),
		}
		return nil
	}

	tests := []struct {
		name       string
		senderType SendType
		sender     VerifyCodeSender
		marker     string
	}{
		{name: "register", senderType: RegisterSender, sender: SendRegisterVerifyCode, marker: "verification"},
		{name: "login", senderType: LoginSender, sender: SendLoginVerifyCode, marker: "login"},
		{name: "reset password", senderType: ResetPasswordSender, sender: SendResetPasswordVerifyCode, marker: "reset"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			captured = capturedMail{}
			if test.senderType.String() == "" || Sender[test.senderType] == nil {
				t.Fatalf("sender registry missing %q", test.senderType)
			}
			if err := test.sender(
				"smtp.example.com",
				"2525",
				"noreply@example.com",
				"secret",
				"Example User",
				"user@example.com",
				"123456",
				"Example Org",
			); err != nil {
				t.Fatalf("send verification email: %v", err)
			}
			if captured.address != "smtp.example.com:2525" || captured.from != "noreply@example.com" {
				t.Fatalf("unexpected SMTP envelope: %#v", captured)
			}
			if len(captured.to) != 1 || captured.to[0] != "user@example.com" {
				t.Fatalf("unexpected recipients: %#v", captured.to)
			}
			for _, value := range []string{
				"Subject: Your verification code is 123456",
				"Example Org",
				"123456",
				"Content-Type: text/html",
			} {
				if !strings.Contains(captured.message, value) {
					t.Fatalf("message does not contain %q:\n%s", value, captured.message)
				}
			}
			if !strings.Contains(strings.ToLower(captured.message), test.marker) {
				t.Fatalf("message does not contain template marker %q", test.marker)
			}
		})
	}
}

func TestVerificationEmailPropagatesTransportAndTemplateErrors(t *testing.T) {
	previous := smtpSendMail
	t.Cleanup(func() { smtpSendMail = previous })

	transportError := errors.New("SMTP unavailable")
	smtpSendMail = func(string, smtp.Auth, string, []string, []byte) error {
		return transportError
	}
	if err := SendLoginVerifyCode(
		"smtp.example.com", "25", "from@example.com", "secret",
		"User", "to@example.com", "999999", "Example",
	); !errors.Is(err, transportError) {
		t.Fatalf("transport error = %v", err)
	}

	if err := sendVerifyCode(
		"missing-template.html",
		"smtp.example.com", "25", "from@example.com", "secret",
		"User", "to@example.com", "999999", "Example",
	); err == nil || !strings.Contains(err.Error(), "missing-template.html") {
		t.Fatalf("template error = %v", err)
	}
}

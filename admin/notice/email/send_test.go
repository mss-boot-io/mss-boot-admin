package email

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/smtp"
	"strings"
	"testing"
	"time"
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
	smtpSendMail = func(_ context.Context, address string, _ smtp.Auth, from string, to []string, message []byte) error {
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
				context.Background(),
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
				"Subject: Your verification code (valid for 5 minutes)",
				"Example Org",
				"123456",
				"Content-Type: text/html",
			} {
				if !strings.Contains(captured.message, value) {
					t.Fatalf("message does not contain %q:\n%s", value, captured.message)
				}
			}
			headers := strings.SplitN(captured.message, "\r\n\r\n", 2)[0]
			if strings.Contains(headers, "123456") {
				t.Fatalf("message headers leaked the verification code:\n%s", headers)
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

	transportError := errors.New("SMTP unavailable for to@example.com code=999999")
	var logBuffer bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuffer, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })
	smtpSendMail = func(ctx context.Context, _ string, _ smtp.Auth, _ string, _ []string, _ []byte) error {
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > verificationEmailTimeout {
			t.Fatalf("SMTP context deadline = %v, %v; want bounded deadline", deadline, ok)
		}
		return transportError
	}
	if err := SendLoginVerifyCode(
		context.Background(),
		"smtp.example.com", "25", "from@example.com", "secret",
		"User", "to@example.com", "999999", "Example",
	); !errors.Is(err, transportError) {
		t.Fatalf("transport error = %v", err)
	}
	if logText := logBuffer.String(); strings.Contains(logText, "to@example.com") || strings.Contains(logText, "999999") || strings.Contains(logText, "SMTP unavailable") {
		t.Fatalf("transport log leaked challenge material: %q", logText)
	}

	if err := sendVerifyCode(
		context.Background(),
		"missing-template.html",
		"smtp.example.com", "25", "from@example.com", "secret",
		"User", "to@example.com", "999999", "Example",
	); err == nil || !strings.Contains(err.Error(), "missing-template.html") {
		t.Fatalf("template error = %v", err)
	}
}

func TestVerificationEmailSenderHonorsCancellation(t *testing.T) {
	previous := smtpSendMail
	t.Cleanup(func() { smtpSendMail = previous })

	smtpSendMail = func(ctx context.Context, _ string, _ smtp.Auth, _ string, _ []string, _ []byte) error {
		<-ctx.Done()
		return ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := SendLoginVerifyCode(
		ctx,
		"smtp.example.com", "25", "from@example.com", "secret",
		"User", "to@example.com", "123456", "Example",
	); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled transport error = %v, want context.Canceled", err)
	}
}

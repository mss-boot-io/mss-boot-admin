package email

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
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

type smtpServerResult struct {
	mail capturedMail
	err  error
}

func startSMTPTestServer(t *testing.T) (string, <-chan smtpServerResult) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for SMTP test server: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	results := make(chan smtpServerResult, 1)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				results <- smtpServerResult{err: fmt.Errorf("accept SMTP connection: %w", err)}
			}
			return
		}
		defer connection.Close()

		reader := bufio.NewReader(connection)
		writer := bufio.NewWriter(connection)
		writeResponse := func(response string) error {
			if _, err := writer.WriteString(response); err != nil {
				return err
			}
			return writer.Flush()
		}
		fail := func(format string, args ...any) {
			results <- smtpServerResult{err: fmt.Errorf(format, args...)}
		}

		if err := writeResponse("220 smtp.test ESMTP\r\n"); err != nil {
			fail("write SMTP greeting: %v", err)
			return
		}

		var captured capturedMail
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if errors.Is(err, io.EOF) {
					results <- smtpServerResult{mail: captured}
				} else {
					fail("read SMTP command: %v", err)
				}
				return
			}
			command := strings.TrimSpace(line)
			upperCommand := strings.ToUpper(command)
			switch {
			case strings.HasPrefix(upperCommand, "EHLO "), strings.HasPrefix(upperCommand, "HELO "):
				err = writeResponse("250-smtp.test\r\n250 PIPELINING\r\n")
			case strings.HasPrefix(upperCommand, "MAIL FROM:"):
				captured.from = strings.TrimSpace(command[len("MAIL FROM:"):])
				err = writeResponse("250 OK\r\n")
			case strings.HasPrefix(upperCommand, "RCPT TO:"):
				captured.to = append(captured.to, strings.TrimSpace(command[len("RCPT TO:"):]))
				err = writeResponse("250 OK\r\n")
			case upperCommand == "DATA":
				if err = writeResponse("354 End data with <CR><LF>.<CR><LF>\r\n"); err == nil {
					var message strings.Builder
					for {
						dataLine, readErr := reader.ReadString('\n')
						if readErr != nil {
							fail("read SMTP data: %v", readErr)
							return
						}
						if dataLine == ".\r\n" || dataLine == ".\n" {
							break
						}
						message.WriteString(dataLine)
					}
					captured.message = message.String()
					err = writeResponse("250 Queued\r\n")
				}
			case upperCommand == "QUIT":
				if err = writeResponse("221 Bye\r\n"); err == nil {
					results <- smtpServerResult{mail: captured}
				}
				return
			default:
				fail("unexpected SMTP command %q", command)
				return
			}
			if err != nil {
				fail("write SMTP response: %v", err)
				return
			}
		}
	}()

	return listener.Addr().String(), results
}

func awaitSMTPResult(t *testing.T, results <-chan smtpServerResult) capturedMail {
	t.Helper()
	select {
	case result := <-results:
		if result.err != nil {
			t.Fatalf("SMTP test server: %v", result.err)
		}
		return result.mail
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SMTP test server")
		return capturedMail{}
	}
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

func TestSendMailContextDeliversMessage(t *testing.T) {
	address, results := startSMTPTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	message := []byte("Subject: focused coverage\r\n\r\nhello from the SMTP test")
	if err := sendMailContext(
		ctx,
		address,
		nil,
		"from@example.com",
		[]string{"first@example.com", "second@example.com"},
		message,
	); err != nil {
		t.Fatalf("send mail through test server: %v", err)
	}

	captured := awaitSMTPResult(t, results)
	if captured.from != "<from@example.com>" {
		t.Fatalf("MAIL FROM = %q, want %q", captured.from, "<from@example.com>")
	}
	if got := strings.Join(captured.to, ","); got != "<first@example.com>,<second@example.com>" {
		t.Fatalf("RCPT TO = %q", got)
	}
	if !strings.Contains(captured.message, "Subject: focused coverage") ||
		!strings.Contains(captured.message, "hello from the SMTP test") {
		t.Fatalf("SMTP message = %q", captured.message)
	}
}

func TestSendMailContextRejectsInvalidInputs(t *testing.T) {
	if err := sendMailContext(nil, "smtp.example.com:25", nil, "from@example.com", nil, nil); err == nil ||
		!strings.Contains(err.Error(), "context is required") {
		t.Fatalf("nil context error = %v", err)
	}
	if err := sendMailContext(context.Background(), "missing-port", nil, "from@example.com", nil, nil); err == nil {
		t.Fatal("address without port unexpectedly succeeded")
	}
}

func TestSendMailContextRejectsMissingAuthExtension(t *testing.T) {
	address, results := startSMTPTestServer(t)
	auth := smtp.PlainAuth("", "from@example.com", "secret", "127.0.0.1")
	err := sendMailContext(
		context.Background(),
		address,
		auth,
		"from@example.com",
		[]string{"to@example.com"},
		[]byte("Subject: unused\r\n\r\nunused"),
	)
	if err == nil || !strings.Contains(err.Error(), "does not support authentication") {
		t.Fatalf("missing AUTH extension error = %v", err)
	}
	_ = awaitSMTPResult(t, results)
}

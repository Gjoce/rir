package email

import (
	"bytes"
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/a-h/templ"
)

type Service interface {
	Send(ctx context.Context, to []EmailAddress, subject string, content templ.Component) error
}

type Emailer struct {
	Address string
	Sender  EmailAddress
	Auth    smtp.Auth
}

type EmailAddress struct {
	Name  string
	Email string
}

func (r EmailAddress) ToRCPT() string {
	return fmt.Sprintf("%s <%s>", r.Name, r.Email)
}

func (e Emailer) Send(ctx context.Context, recipients []EmailAddress, subject string, content templ.Component) error {
	toEmails := make([]string, len(recipients))
	toHeader := make([]string, len(recipients))

	for i, recipient := range recipients {
		toEmails[i] = recipient.Email
		toHeader[i] = recipient.ToRCPT()
	}

	var body bytes.Buffer
	if err := content.Render(ctx, &body); err != nil {
		return fmt.Errorf("failed to render email content: %w", err)
	}

	bodyStr := body.String()
	bodyStrCleanedUp := strings.ReplaceAll(bodyStr, `\n `, "\n")
	bodyStrCleanedUp = strings.ReplaceAll(bodyStrCleanedUp, `\n`, "\n")

	headers := []string{
		"To: " + strings.Join(toHeader, ", "),
		"From: " + e.Sender.ToRCPT(),
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=\"UTF-8\"",
	}

	message := strings.Join(headers, "\r\n") + "\r\n\r\n" + bodyStrCleanedUp

	return smtp.SendMail(e.Address, e.Auth, e.Sender.Email, toEmails, []byte(message))
}

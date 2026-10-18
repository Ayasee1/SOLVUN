package notification

import (
	"fmt"

	"github.com/resend/resend-go/v4"
)

// ResendSender delivers emails through the Resend API. It reads better in
// production than raw SMTP (managed SPF/DKIM, delivery events) and supports
// attachments for the invoice PDF.
type ResendSender struct {
	emails resend.EmailsSvc
	from   string
}

var _ Sender = (*ResendSender)(nil)

func NewResendSender(apiKey, from string) *ResendSender {
	return newResendSender(resend.NewClient(apiKey).Emails, from)
}

// newResendSender lets tests inject a fake client.
func newResendSender(emails resend.EmailsSvc, from string) *ResendSender {
	return &ResendSender{emails: emails, from: from}
}

func (s *ResendSender) Send(msg Message) error {
	attachments := make([]*resend.Attachment, 0, len(msg.Attachments))
	for _, attachment := range msg.Attachments {
		attachments = append(attachments, &resend.Attachment{
			Content:     attachment.Data,
			Filename:    attachment.Filename,
			ContentType: attachment.ContentType,
		})
	}

	params := &resend.SendEmailRequest{
		From:        s.from,
		To:          []string{msg.To},
		Subject:     msg.Subject,
		Html:        msg.HTML,
		Attachments: attachments,
	}

	if _, err := s.emails.Send(params); err != nil {
		return fmt.Errorf("resend: %w", err)
	}

	return nil
}

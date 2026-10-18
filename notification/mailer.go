// Package notification sends transactional emails (order received, payment
// confirmed). The Sender interface keeps the provider swappable: SMTP works
// with Mailtrap/sandbox today and any provider in production.
package notification

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/smtp"
)

type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

type Message struct {
	To          string
	Subject     string
	HTML        string
	Attachments []Attachment
}

// Sender delivers a message. Implementations must not panic; they return an
// error so callers can log it without breaking the request.
type Sender interface {
	Send(msg Message) error
}

// LogSender writes the email to the log instead of sending it. Used when SMTP
// is not configured, so development keeps working without credentials.
type LogSender struct{}

func (LogSender) Send(msg Message) error {
	log.Printf("[email] skipped (SMTP not configured) to=%s subject=%q", msg.To, msg.Subject)
	return nil
}

// SMTPSender sends through any SMTP server (Mailtrap, Resend, SendGrid, ...).
type SMTPSender struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

func (s SMTPSender) Send(msg Message) error {
	auth := smtp.PlainAuth("", s.User, s.Pass, s.Host)
	addr := net.JoinHostPort(s.Host, s.Port)

	return smtp.SendMail(addr, auth, s.From, []string{msg.To}, s.buildMIME(msg))
}

func (s SMTPSender) buildMIME(msg Message) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", s.From)
	fmt.Fprintf(&buf, "To: %s\r\n", msg.To)
	fmt.Fprintf(&buf, "Subject: %s\r\n", msg.Subject)
	buf.WriteString("MIME-Version: 1.0\r\n")

	if len(msg.Attachments) == 0 {
		buf.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
		buf.WriteString("\r\n")
		buf.WriteString(msg.HTML)
		return buf.Bytes()
	}

	const boundary = "golearn-8f3d1a"
	fmt.Fprintf(&buf, "Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary)
	buf.WriteString("\r\n")

	// HTML body part
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	buf.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(msg.HTML)
	buf.WriteString("\r\n")

	// attachments, base64 encoded and wrapped at 76 characters
	for _, attachment := range msg.Attachments {
		fmt.Fprintf(&buf, "--%s\r\n", boundary)
		fmt.Fprintf(&buf, "Content-Type: %s; name=\"%s\"\r\n", attachment.ContentType, attachment.Filename)
		buf.WriteString("Content-Transfer-Encoding: base64\r\n")
		fmt.Fprintf(&buf, "Content-Disposition: attachment; filename=\"%s\"\r\n", attachment.Filename)
		buf.WriteString("\r\n")

		encoded := base64.StdEncoding.EncodeToString(attachment.Data)
		for len(encoded) > 76 {
			buf.WriteString(encoded[:76])
			buf.WriteString("\r\n")
			encoded = encoded[76:]
		}
		buf.WriteString(encoded)
		buf.WriteString("\r\n")
	}

	fmt.Fprintf(&buf, "--%s--\r\n", boundary)
	return buf.Bytes()
}

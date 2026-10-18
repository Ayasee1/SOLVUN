package notification

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestBuildMIMEWithoutAttachment(t *testing.T) {
	sender := SMTPSender{From: "Store <no-reply@test.local>"}

	raw := string(sender.buildMIME(Message{To: "budi@example.com", Subject: "Hi", HTML: "<p>hello</p>"}))

	if !strings.Contains(raw, "text/html") {
		t.Error("missing html content type")
	}
	if strings.Contains(raw, "multipart") {
		t.Error("a message without attachments must not be multipart")
	}
	if !strings.Contains(raw, "<p>hello</p>") {
		t.Error("html body missing")
	}
}

func TestBuildMIMEWithAttachment(t *testing.T) {
	sender := SMTPSender{From: "Store <no-reply@test.local>"}
	data := []byte("%PDF-1.4 fake invoice")

	raw := sender.buildMIME(Message{
		To:      "budi@example.com",
		Subject: "Invoice",
		HTML:    "<p>hello</p>",
		Attachments: []Attachment{
			{Filename: "invoice-1234.pdf", ContentType: "application/pdf", Data: data},
		},
	})
	text := string(raw)

	if !strings.Contains(text, "multipart/mixed") {
		t.Error("missing multipart content type")
	}
	if !strings.Contains(text, "invoice-1234.pdf") {
		t.Error("missing attachment filename")
	}
	if !strings.Contains(text, "application/pdf") {
		t.Error("missing attachment content type")
	}
	if !strings.Contains(text, "base64") {
		t.Error("missing base64 encoding header")
	}

	// base64 lines are wrapped, so compare after removing line breaks
	encoded := base64.StdEncoding.EncodeToString(data)
	if !strings.Contains(strings.ReplaceAll(text, "\r\n", ""), encoded) {
		t.Error("attachment payload was not base64-encoded into the message")
	}
}

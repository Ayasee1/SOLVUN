package notification

import (
	"errors"
	"testing"

	"github.com/resend/resend-go/v4"
)

// fakeEmails implements resend.EmailsSvc by embedding the interface, so only
// Send needs an implementation for these tests.
type fakeEmails struct {
	resend.EmailsSvc
	sent *resend.SendEmailRequest
	err  error
}

func (f *fakeEmails) Send(params *resend.SendEmailRequest) (*resend.SendEmailResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.sent = params
	return &resend.SendEmailResponse{Id: "test-id"}, nil
}

func TestResendSenderMapsMessage(t *testing.T) {
	fake := &fakeEmails{}
	sender := newResendSender(fake, "Golearn Store <no-reply@test.local>")

	err := sender.Send(Message{
		To:      "budi@example.com",
		Subject: "Invoice",
		HTML:    "<p>hi</p>",
		Attachments: []Attachment{
			{Filename: "invoice-1.pdf", ContentType: "application/pdf", Data: []byte("%PDF")},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.sent == nil {
		t.Fatal("nothing was sent")
	}
	if fake.sent.From != "Golearn Store <no-reply@test.local>" {
		t.Errorf("from = %q", fake.sent.From)
	}
	if len(fake.sent.To) != 1 || fake.sent.To[0] != "budi@example.com" {
		t.Errorf("to = %v", fake.sent.To)
	}
	if fake.sent.Subject != "Invoice" || fake.sent.Html != "<p>hi</p>" {
		t.Errorf("subject/html = %q / %q", fake.sent.Subject, fake.sent.Html)
	}
	if len(fake.sent.Attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(fake.sent.Attachments))
	}

	got := fake.sent.Attachments[0]
	if got.Filename != "invoice-1.pdf" || got.ContentType != "application/pdf" || string(got.Content) != "%PDF" {
		t.Errorf("attachment = %+v", got)
	}
}

func TestResendSenderReturnsError(t *testing.T) {
	sender := newResendSender(&fakeEmails{err: errors.New("boom")}, "from@test.local")

	if err := sender.Send(Message{To: "budi@example.com"}); err == nil {
		t.Error("expected an error, got nil")
	}
}

package mailer

import (
	"errors"
	"testing"
)

func TestSMTPMailerRequiresConfig(t *testing.T) {
	m := NewSMTP(Config{})

	err := m.SendOTP("user@example.com", "123456")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestDiscardMailerDoesNotSend(t *testing.T) {
	m := NewDiscard()

	if err := m.SendPasswordReset("user@example.com", "http://localhost:3000/reset-password?token=t"); err != nil {
		t.Fatalf("discard mailer returned error: %v", err)
	}
}

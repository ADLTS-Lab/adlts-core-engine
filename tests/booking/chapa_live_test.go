package booking_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"adlts/internal/booking"

	"github.com/google/uuid"
)

func loadDotEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	var path string
	for {
		candidate := filepath.Join(dir, ".env")
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"'")
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}

func TestChapaProvider_LiveSmoke(t *testing.T) {
	loadDotEnv()
	secret := strings.TrimSpace(os.Getenv("CHAPA_SECRET_KEY"))
	if secret == "" {
		t.Skip("CHAPA_SECRET_KEY not set; skipping live Chapa smoke test")
	}

	webhookSecret := strings.TrimSpace(os.Getenv("CHAPA_WEBHOOK_SECRET"))
	baseURL := strings.TrimSpace(os.Getenv("CHAPA_BASE_URL"))
	p := booking.NewChapaProvider(secret, webhookSecret, baseURL)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	txRef := "adlts_smoke_" + uuid.NewString()

	initRes, err := p.InitiatePayment(ctx, booking.PaymentInitRequest{
		TxRef:       txRef,
		AmountCents: 100,
		Currency:    "ETB",
		Email:       "smoke-test@adlts.et",
		FirstName:   "Smoke",
		LastName:    "Test",
		Phone:       "0900000000",
		CallbackURL: "https://example.com/api/v1/bookings/00000000-0000-0000-0000-000000000000/payments/callback",
		ReturnURL:   "https://example.com/payment/return",
	})
	if err != nil {
		t.Fatalf("initiate payment failed: %v", err)
	}
	if initRes.CheckoutURL == "" {
		t.Fatalf("expected checkout_url, got empty")
	}

	verifyRes, err := p.VerifyTransaction(ctx, initRes.TxRef)
	if err != nil {
		t.Fatalf("verify transaction failed: %v", err)
	}
	if verifyRes.TxRef == "" {
		t.Fatalf("expected tx_ref in verify response, got empty")
	}
	if verifyRes.Status == "" {
		t.Fatalf("expected status in verify response, got empty")
	}
}

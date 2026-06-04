package booking

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChapaProviderInitiatePaymentNormalizesPhone(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/transaction/initialize" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"checkout_url": "https://checkout.chapa.co/pay/test",
			},
		})
	}))
	defer server.Close()

	provider := NewChapaProvider("secret", "webhook-secret", server.URL)
	_, err := provider.InitiatePayment(context.Background(), PaymentInitRequest{
		TxRef:       "tx-1",
		AmountCents: 10000,
		Currency:    "ETB",
		Email:       "candidate@example.com",
		FirstName:   "Jane",
		LastName:    "Doe",
		Phone:       "+251900000000",
		CallbackURL: "https://api.example.com/callback",
		ReturnURL:   "https://app.example.com/return",
	})
	if err != nil {
		t.Fatalf("InitiatePayment returned error: %v", err)
	}
	if got := captured["phone_number"]; got != "0900000000" {
		t.Fatalf("expected normalized phone, got %#v", got)
	}
}

func TestChapaProviderVerifyTransactionParsesStringAmountAndCurrency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/transaction/verify/tx-1" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"status":   "success",
				"tx_ref":   "tx-1",
				"amount":   "100.00",
				"currency": "etb",
			},
		})
	}))
	defer server.Close()

	provider := NewChapaProvider("secret", "webhook-secret", server.URL)
	got, err := provider.VerifyTransaction(context.Background(), "tx-1")
	if err != nil {
		t.Fatalf("VerifyTransaction returned error: %v", err)
	}
	if got.AmountCents != 10000 {
		t.Fatalf("expected 10000 cents, got %d", got.AmountCents)
	}
	if got.Currency != "ETB" {
		t.Fatalf("expected ETB currency, got %q", got.Currency)
	}
}

func TestChapaProviderValidateWebhookSignatureAcceptsEitherChapaHeader(t *testing.T) {
	payload := []byte(`{"tx_ref":"tx-1","status":"success"}`)
	provider := NewChapaProvider("secret", "webhook-secret", "https://api.chapa.co/v1")

	payloadSignature := hmacSHA256Hex([]byte("webhook-secret"), payload)
	secretSignature := hmacSHA256Hex([]byte("webhook-secret"), []byte("webhook-secret"))

	if !provider.ValidateWebhookSignature(payload, payloadSignature) {
		t.Fatalf("expected payload signature to validate")
	}
	if !provider.ValidateWebhookSignature(payload, "bad-signature", secretSignature) {
		t.Fatalf("expected fallback chapa-signature to validate")
	}
	if provider.ValidateWebhookSignature(payload, "bad-signature") {
		t.Fatalf("expected invalid signature to fail")
	}
}

func TestNormalizeChapaPhoneOmitsInvalidPhone(t *testing.T) {
	if got := normalizeChapaPhone("911000111"); got != "0911000111" {
		t.Fatalf("expected local Ethiopian phone normalization, got %q", got)
	}
	if got := normalizeChapaPhone("+1-555-0100"); got != "" {
		t.Fatalf("expected invalid phone to be omitted, got %q", got)
	}
}

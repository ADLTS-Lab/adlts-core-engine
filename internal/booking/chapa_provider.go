package booking

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ChapaProvider implements PaymentProvider using the Chapa payment gateway.
type ChapaProvider struct {
	secretKey     string
	webhookSecret string
	baseURL       string
	httpClient    *http.Client
}

// NewChapaProvider constructs a ready-to-use ChapaProvider.
func NewChapaProvider(secretKey, webhookSecret, baseURL string) *ChapaProvider {
	if baseURL == "" {
		baseURL = "https://api.chapa.co/v1"
	}
	return &ChapaProvider{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
		baseURL:       baseURL,
		httpClient:    &http.Client{Timeout: 15 * time.Second},
	}
}

// InitiatePayment calls POST /transaction/initialize on Chapa.
func (c *ChapaProvider) InitiatePayment(ctx context.Context, req PaymentInitRequest) (PaymentInitResult, error) {
	body := map[string]any{
		"amount":       fmt.Sprintf("%.2f", float64(req.AmountCents)/100.0),
		"currency":     req.Currency,
		"email":        req.Email,
		"first_name":   req.FirstName,
		"last_name":    req.LastName,
		"phone_number": req.Phone,
		"tx_ref":       req.TxRef,
		"callback_url": req.CallbackURL,
		"return_url":   req.ReturnURL,
	}

	rawBody, err := json.Marshal(body)
	if err != nil {
		return PaymentInitResult{}, fmt.Errorf("chapa: marshal init body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/transaction/initialize", bytes.NewReader(rawBody))
	if err != nil {
		return PaymentInitResult{}, fmt.Errorf("chapa: build init request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.secretKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return PaymentInitResult{}, fmt.Errorf("chapa: init http call: %w", err)
	}
	defer resp.Body.Close()

	var chapaResp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    struct {
			CheckoutURL string `json:"checkout_url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chapaResp); err != nil {
		return PaymentInitResult{}, fmt.Errorf("chapa: decode init response: %w", err)
	}
	if chapaResp.Status != "success" {
		return PaymentInitResult{}, fmt.Errorf("chapa: init failed: %s", chapaResp.Message)
	}

	return PaymentInitResult{CheckoutURL: chapaResp.Data.CheckoutURL, TxRef: req.TxRef}, nil
}

// VerifyTransaction calls GET /transaction/verify/{tx_ref} on Chapa.
func (c *ChapaProvider) VerifyTransaction(ctx context.Context, txRef string) (PaymentVerifyResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/transaction/verify/"+txRef, nil)
	if err != nil {
		return PaymentVerifyResult{}, fmt.Errorf("chapa: build verify request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.secretKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return PaymentVerifyResult{}, fmt.Errorf("chapa: verify http call: %w", err)
	}
	defer resp.Body.Close()

	var chapaResp struct {
		Status string `json:"status"`
		Data   struct {
			Status string  `json:"status"`
			TxRef  string  `json:"tx_ref"`
			Amount float64 `json:"amount"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chapaResp); err != nil {
		return PaymentVerifyResult{}, fmt.Errorf("chapa: decode verify response: %w", err)
	}

	return PaymentVerifyResult{
		TxRef:       chapaResp.Data.TxRef,
		Status:      chapaResp.Data.Status,
		AmountCents: int(chapaResp.Data.Amount * 100),
	}, nil
}

// ValidateWebhookSignature verifies the x-chapa-signature header.
func (c *ChapaProvider) ValidateWebhookSignature(payload []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// generateTxRef produces a unique, traceable transaction reference.
func generateTxRef(bookingID string) string {
	return fmt.Sprintf("adlts_booking_%s_%d", bookingID, time.Now().Unix())
}

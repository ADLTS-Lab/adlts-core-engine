package booking

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
		baseURL:       strings.TrimRight(baseURL, "/"),
		httpClient:    &http.Client{Timeout: 15 * time.Second},
	}
}

// InitiatePayment calls POST /transaction/initialize on Chapa.
func (c *ChapaProvider) InitiatePayment(ctx context.Context, req PaymentInitRequest) (PaymentInitResult, error) {
	if strings.TrimSpace(c.secretKey) == "" {
		return PaymentInitResult{}, fmt.Errorf("chapa: secret key is not configured")
	}

	body := map[string]any{
		"amount":       fmt.Sprintf("%.2f", float64(req.AmountCents)/100.0),
		"currency":     req.Currency,
		"email":        req.Email,
		"first_name":   req.FirstName,
		"last_name":    req.LastName,
		"tx_ref":       req.TxRef,
		"callback_url": req.CallbackURL,
		"return_url":   req.ReturnURL,
	}
	if phone := normalizeChapaPhone(req.Phone); phone != "" {
		body["phone_number"] = phone
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return PaymentInitResult{}, fmt.Errorf("chapa: init http status %d: %s", resp.StatusCode, string(b))
	}

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
	if chapaResp.Data.CheckoutURL == "" {
		return PaymentInitResult{}, fmt.Errorf("chapa: init failed: missing checkout_url")
	}

	return PaymentInitResult{CheckoutURL: chapaResp.Data.CheckoutURL, TxRef: req.TxRef}, nil
}

// VerifyTransaction calls GET /transaction/verify/{tx_ref} on Chapa.
func (c *ChapaProvider) VerifyTransaction(ctx context.Context, txRef string) (PaymentVerifyResult, error) {
	if strings.TrimSpace(c.secretKey) == "" {
		return PaymentVerifyResult{}, fmt.Errorf("chapa: secret key is not configured")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/transaction/verify/"+url.PathEscape(txRef), nil)
	if err != nil {
		return PaymentVerifyResult{}, fmt.Errorf("chapa: build verify request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.secretKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return PaymentVerifyResult{}, fmt.Errorf("chapa: verify http call: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return PaymentVerifyResult{}, fmt.Errorf("chapa: verify http status %d: %s", resp.StatusCode, string(b))
	}

	var chapaResp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    struct {
			Status   string          `json:"status"`
			TxRef    string          `json:"tx_ref"`
			Amount   json.RawMessage `json:"amount"`
			Currency string          `json:"currency"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chapaResp); err != nil {
		return PaymentVerifyResult{}, fmt.Errorf("chapa: decode verify response: %w", err)
	}
	if chapaResp.Status != "success" {
		return PaymentVerifyResult{}, fmt.Errorf("chapa: verify failed: %s", chapaResp.Message)
	}
	if chapaResp.Data.TxRef == "" {
		return PaymentVerifyResult{}, fmt.Errorf("chapa: verify failed: missing tx_ref")
	}
	amountCents, err := parseChapaAmountCents(chapaResp.Data.Amount)
	if err != nil {
		return PaymentVerifyResult{}, fmt.Errorf("chapa: verify failed: invalid amount: %w", err)
	}

	return PaymentVerifyResult{
		TxRef:       chapaResp.Data.TxRef,
		Status:      strings.ToLower(chapaResp.Data.Status),
		AmountCents: amountCents,
		Currency:    strings.ToUpper(chapaResp.Data.Currency),
	}, nil
}

// ValidateWebhookSignature verifies Chapa webhook signatures.
func (c *ChapaProvider) ValidateWebhookSignature(payload []byte, signatures ...string) bool {
	secret := strings.TrimSpace(c.webhookSecret)
	if secret == "" {
		return false
	}

	expectedPayloadSignature := hmacSHA256Hex([]byte(secret), payload)
	expectedSecretSignature := hmacSHA256Hex([]byte(secret), []byte(secret))
	for _, signature := range signatures {
		signature = strings.TrimSpace(signature)
		if signature == "" {
			continue
		}
		if hmac.Equal([]byte(expectedPayloadSignature), []byte(signature)) ||
			hmac.Equal([]byte(expectedSecretSignature), []byte(signature)) {
			return true
		}
	}
	return false
}

// generateTxRef produces a unique, traceable transaction reference.
// Chapa requires tx_ref to be at most 50 characters.
func generateTxRef(bookingID string) string {
	compactBookingID := strings.ReplaceAll(bookingID, "-", "")
	if len(compactBookingID) > 24 {
		compactBookingID = compactBookingID[:24]
	}
	nonce := strconv.FormatInt(time.Now().UnixNano(), 36)
	return fmt.Sprintf("adlts_%s_%s", compactBookingID, nonce)
}

func hmacSHA256Hex(secret, data []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

func parseChapaAmountCents(raw json.RawMessage) (int, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return 0, nil
	}
	s = strings.Trim(s, `"`)
	amount, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int(math.Round(amount * 100)), nil
}

func normalizeChapaPhone(phone string) string {
	p := strings.TrimSpace(phone)
	if p == "" {
		return ""
	}
	replacer := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "")
	p = replacer.Replace(p)

	switch {
	case strings.HasPrefix(p, "+251"):
		p = "0" + strings.TrimPrefix(p, "+251")
	case strings.HasPrefix(p, "251"):
		p = "0" + strings.TrimPrefix(p, "251")
	case (strings.HasPrefix(p, "9") || strings.HasPrefix(p, "7")) && len(p) == 9:
		p = "0" + p
	}

	if len(p) != 10 {
		return ""
	}
	if !strings.HasPrefix(p, "09") && !strings.HasPrefix(p, "07") {
		return ""
	}
	for _, r := range p {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return p
}

package booking

import "context"

// PaymentProvider abstracts any payment gateway.
// Current implementation: Chapa.
type PaymentProvider interface {
	// InitiatePayment creates a hosted payment session with the provider.
	InitiatePayment(ctx context.Context, req PaymentInitRequest) (PaymentInitResult, error)

	// VerifyTransaction confirms a tx_ref was completed.
	VerifyTransaction(ctx context.Context, txRef string) (PaymentVerifyResult, error)

	// ValidateWebhookSignature verifies the provider webhook signature.
	ValidateWebhookSignature(payload []byte, signature string) bool
}

// PaymentInitRequest contains everything needed to start a payment session.
type PaymentInitRequest struct {
	TxRef       string
	AmountCents int
	Currency    string
	Email       string
	FirstName   string
	LastName    string
	Phone       string
	CallbackURL string
	ReturnURL   string
}

// PaymentInitResult is returned by InitiatePayment.
type PaymentInitResult struct {
	CheckoutURL string
	TxRef       string
}

// PaymentVerifyResult is returned by VerifyTransaction.
type PaymentVerifyResult struct {
	TxRef       string
	Status      string
	AmountCents int
}

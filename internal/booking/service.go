package booking

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"adlts/internal/domain"
	"adlts/internal/platform/mailer"
	"adlts/internal/platform/security"

	"github.com/google/uuid"
)

const maxPaymentAttempts = 3

type Service struct {
	repo            *Repository
	provider        PaymentProvider
	mailer          *mailer.Mailer
	baseURL         string
	frontendBaseURL string
	internalAPIKey  string
	httpClient      *http.Client
}

func NewService(repo *Repository, provider PaymentProvider, mailer *mailer.Mailer, baseURL, frontendBaseURL string, internalAPIKey ...string) *Service {
	key := ""
	if len(internalAPIKey) > 0 {
		key = internalAPIKey[0]
	}
	return &Service{
		repo:            repo,
		provider:        provider,
		mailer:          mailer,
		baseURL:         baseURL,
		frontendBaseURL: frontendBaseURL,
		internalAPIKey:  key,
		httpClient:      &http.Client{Timeout: 15 * time.Second},
	}
}

// -------------------------------------------------------
// Booking lifecycle
// -------------------------------------------------------

func (s *Service) CreateBooking(ctx context.Context, candidateID uuid.UUID, req CreateBookingRequest) (domain.Booking, error) {
	if err := req.Validate(); err != nil {
		return domain.Booking{}, err
	}
	instituteID, err := uuid.Parse(req.InstituteID)
	if err != nil {
		return domain.Booking{}, ErrMissingInstituteID
	}
	exists, err := s.repo.InstituteExists(ctx, instituteID)
	if err != nil {
		return domain.Booking{}, err
	}
	if !exists {
		return domain.Booking{}, ErrInstituteNotFound
	}

	requiresVerification := false
	status := domain.BookingVerified
	if requiresVerification {
		status = domain.BookingPendingVerification
	}

	b := domain.Booking{
		CandidateID:          candidateID,
		InstituteID:          instituteID,
		Status:               status,
		RequiresVerification: requiresVerification,
		PaymentStatus:        "unpaid",
		PaymentAttempts:      0,
	}

	created, err := s.repo.CreateBooking(ctx, b, candidateID)
	if err != nil {
		return domain.Booking{}, err
	}

	return created, nil
}

func (s *Service) GetBooking(ctx context.Context, auth *security.AuthContext, bookingID uuid.UUID) (domain.Booking, error) {
	b, err := s.repo.BookingByID(ctx, bookingID)
	if err != nil {
		return domain.Booking{}, ErrBookingNotFound
	}
	if err := s.authorizeBookingAccess(auth, b); err != nil {
		return domain.Booking{}, err
	}
	return b, nil
}

func (s *Service) ListBookings(ctx context.Context, auth *security.AuthContext, statusFilter *domain.BookingStatus, page int) ([]domain.Booking, int, error) {
	f := BookingFilter{}
	switch auth.EntityType {
	case security.EntityCandidate:
		f.CandidateID = &auth.SubjectID
	case security.EntityInstitute:
		f.InstituteID = &auth.SubjectID
	case security.EntityAdmin, security.EntitySuperAdmin:
		// full access
	default:
		return nil, 0, ErrForbidden
	}
	if statusFilter != nil {
		f.Status = statusFilter
	}
	return s.repo.ListBookings(ctx, f, page)
}

func (s *Service) VerifyBooking(ctx context.Context, auth *security.AuthContext, bookingID uuid.UUID, req VerifyBookingRequest) (domain.Booking, error) {
	if err := req.Validate(); err != nil {
		return domain.Booking{}, err
	}
	if auth.EntityType != security.EntityAdmin && auth.EntityType != security.EntitySuperAdmin && auth.EntityType != security.EntityInstitute {
		return domain.Booking{}, ErrForbidden
	}

	b, err := s.repo.BookingByID(ctx, bookingID)
	if err != nil {
		return domain.Booking{}, ErrBookingNotFound
	}
	if b.Status != domain.BookingPendingVerification {
		if req.Action == "approve" && b.Status == domain.BookingVerified {
			return b, nil
		}
		if req.Action == "reject" && b.Status == domain.BookingRejected {
			return b, nil
		}
		return domain.Booking{}, ErrInvalidStatusForAction
	}

	fields := map[string]any{}
	if req.Action == "approve" {
		fields["status"] = string(domain.BookingVerified)
		fields["verified_by"] = auth.SubjectID
		fields["verified_at"] = time.Now().UTC()
	} else {
		fields["status"] = string(domain.BookingRejected)
		if strings.TrimSpace(req.RejectionReason) == "" {
			req.RejectionReason = "Rejected by reviewer"
		}
		fields["rejection_reason"] = req.RejectionReason
	}

	if err := s.repo.UpdateBookingFields(ctx, bookingID, fields, auth.SubjectID); err != nil {
		return domain.Booking{}, err
	}
	return s.repo.BookingByID(ctx, bookingID)
}

func (s *Service) ScheduleBooking(ctx context.Context, auth *security.AuthContext, bookingID uuid.UUID, req ScheduleBookingRequest) (domain.Booking, error) {
	if err := req.Validate(); err != nil {
		return domain.Booking{}, err
	}
	if auth.EntityType != security.EntityAdmin && auth.EntityType != security.EntitySuperAdmin {
		return domain.Booking{}, ErrForbidden
	}

	b, err := s.repo.BookingByID(ctx, bookingID)
	if err != nil {
		return domain.Booking{}, ErrBookingNotFound
	}
	if b.Status != domain.BookingVerified {
		return domain.Booking{}, ErrInvalidStatusForAction
	}

	candidateStatus, err := s.repo.CandidateStatusByID(ctx, b.CandidateID)
	if err != nil {
		return domain.Booking{}, err
	}
	if candidateStatus != string(domain.UserStatusActive) {
		return domain.Booking{}, errors.New("candidate is not active")
	}

	slotID, err := uuid.Parse(req.SlotID)
	if err != nil {
		return domain.Booking{}, ErrMissingSlotID
	}
	slot, err := s.repo.SlotByID(ctx, slotID)
	if err != nil {
		return domain.Booking{}, ErrSlotNotFound
	}
	if slot.BookedCount >= slot.Capacity {
		return domain.Booking{}, ErrSlotFull
	}

	if err := s.repo.IncrementSlotBookedCount(ctx, slotID); err != nil {
		return domain.Booking{}, err
	}

	fields := map[string]any{
		"slot_id":      slotID,
		"scheduled_at": slot.StartsAt,
		"status":       string(domain.BookingScheduled),
	}
	if err := s.repo.UpdateBookingFields(ctx, bookingID, fields, auth.SubjectID); err != nil {
		_ = s.repo.DecrementSlotBookedCount(ctx, slotID)
		return domain.Booking{}, err
	}

	return s.repo.BookingByID(ctx, bookingID)
}

func (s *Service) RescheduleBooking(ctx context.Context, auth *security.AuthContext, bookingID uuid.UUID, req RescheduleBookingRequest) (domain.Booking, error) {
	if err := req.Validate(); err != nil {
		return domain.Booking{}, err
	}

	b, err := s.repo.BookingByID(ctx, bookingID)
	if err != nil {
		return domain.Booking{}, ErrBookingNotFound
	}
	if auth.EntityType == security.EntityCandidate && b.CandidateID != auth.SubjectID {
		return domain.Booking{}, ErrForbidden
	}
	if auth.EntityType != security.EntityAdmin && auth.EntityType != security.EntitySuperAdmin && auth.EntityType != security.EntityCandidate {
		return domain.Booking{}, ErrForbidden
	}

	allowed := map[domain.BookingStatus]bool{
		domain.BookingScheduled: true,
		domain.BookingConfirmed: true,
	}
	if !allowed[b.Status] {
		return domain.Booking{}, ErrInvalidStatusForAction
	}

	newSlotID, err := uuid.Parse(req.SlotID)
	if err != nil {
		return domain.Booking{}, ErrMissingSlotID
	}
	newSlot, err := s.repo.SlotByID(ctx, newSlotID)
	if err != nil {
		return domain.Booking{}, ErrSlotNotFound
	}
	if newSlot.BookedCount >= newSlot.Capacity {
		return domain.Booking{}, ErrSlotFull
	}

	if b.SlotID != nil {
		_ = s.repo.DecrementSlotBookedCount(ctx, *b.SlotID)
	}
	if err := s.repo.IncrementSlotBookedCount(ctx, newSlotID); err != nil {
		return domain.Booking{}, err
	}

	fields := map[string]any{
		"slot_id":      newSlotID,
		"scheduled_at": newSlot.StartsAt,
		"status":       string(domain.BookingScheduled),
	}
	if err := s.repo.UpdateBookingFields(ctx, bookingID, fields, auth.SubjectID); err != nil {
		_ = s.repo.DecrementSlotBookedCount(ctx, newSlotID)
		return domain.Booking{}, err
	}

	return s.repo.BookingByID(ctx, bookingID)
}

func (s *Service) DeleteBooking(ctx context.Context, auth *security.AuthContext, bookingID uuid.UUID) error {
	b, err := s.repo.BookingByID(ctx, bookingID)
	if err != nil {
		return ErrBookingNotFound
	}

	if auth.EntityType == security.EntityCandidate {
		if b.CandidateID != auth.SubjectID {
			return ErrForbidden
		}
		if b.Status != domain.BookingDrafted {
			return ErrInvalidStatusForAction
		}
	} else if auth.EntityType != security.EntityAdmin && auth.EntityType != security.EntitySuperAdmin {
		return ErrForbidden
	}

	if b.SlotID != nil {
		_ = s.repo.DecrementSlotBookedCount(ctx, *b.SlotID)
	}
	return s.repo.DeleteBooking(ctx, bookingID)
}

func (s *Service) ArchiveBooking(ctx context.Context, bookingID uuid.UUID) error {
	fields := map[string]any{
		"status":      string(domain.BookingArchived),
		"archived_at": time.Now().UTC(),
	}
	return s.repo.UpdateBookingFields(ctx, bookingID, fields, uuid.Nil)
}

// -------------------------------------------------------
// Payment lifecycle
// -------------------------------------------------------

func (s *Service) InitiatePayment(ctx context.Context, auth *security.AuthContext, bookingID uuid.UUID, req InitiatePaymentRequest) (InitiatePaymentResponse, error) {
	if err := req.Validate(); err != nil {
		return InitiatePaymentResponse{}, err
	}

	b, err := s.repo.BookingByID(ctx, bookingID)
	if err != nil {
		return InitiatePaymentResponse{}, ErrBookingNotFound
	}
	if b.CandidateID != auth.SubjectID {
		return InitiatePaymentResponse{}, ErrForbidden
	}
	if b.PaymentStatus == "paid" {
		return InitiatePaymentResponse{}, ErrAlreadyProcessed
	}
	if b.Status != domain.BookingVerified &&
		b.Status != domain.BookingScheduled &&
		b.Status != domain.BookingPaymentFailed {
		return InitiatePaymentResponse{}, ErrInvalidStatusForAction
	}

	lastAttempt, err := s.repo.LatestPaymentAttemptNumber(ctx, bookingID)
	if err != nil {
		return InitiatePaymentResponse{}, err
	}
	attemptNumber := lastAttempt + 1
	if attemptNumber > maxPaymentAttempts {
		return InitiatePaymentResponse{}, ErrMaxPaymentAttempts
	}

	contact, err := s.repo.CandidateContactByID(ctx, auth.SubjectID)
	if err != nil {
		return InitiatePaymentResponse{}, err
	}

	txRef := generateTxRef(bookingID.String())
	base := strings.TrimRight(s.baseURL, "/")
	front := strings.TrimRight(s.frontendBaseURL, "/")
	callbackURL := fmt.Sprintf("%s/api/v1/bookings/%s/payments/callback", base, bookingID)
	returnURL := fmt.Sprintf("%s/bookings/%s/payment/success", front, bookingID)

	payment, err := s.repo.CreatePayment(ctx, domain.Payment{
		BookingID:     bookingID,
		AmountCents:   req.AmountCents,
		Currency:      req.Currency,
		Status:        "pending",
		Provider:      "chapa",
		ProviderRef:   &txRef,
		AttemptNumber: attemptNumber,
	})
	if err != nil {
		return InitiatePaymentResponse{}, err
	}

	if err := s.repo.UpdateBookingFields(ctx, bookingID, map[string]any{
		"status":               string(domain.BookingPaymentPending),
		"payment_status":       "pending",
		"payment_amount_cents": req.AmountCents,
		"payment_attempts":     attemptNumber,
	}, auth.SubjectID); err != nil {
		_ = s.repo.UpdatePaymentFields(ctx, payment.ID, map[string]any{"status": "failed"})
		return InitiatePaymentResponse{}, err
	}

	result, err := s.provider.InitiatePayment(ctx, PaymentInitRequest{
		TxRef:       txRef,
		AmountCents: req.AmountCents,
		Currency:    req.Currency,
		Email:       contact.Email,
		FirstName:   contact.FirstName,
		LastName:    contact.LastName,
		Phone:       contact.Phone,
		CallbackURL: callbackURL,
		ReturnURL:   returnURL,
	})
	if err != nil {
		_ = s.repo.UpdatePaymentFields(ctx, payment.ID, map[string]any{"status": "failed"})
		_ = s.repo.UpdateBookingFields(ctx, bookingID, map[string]any{
			"status":         string(domain.BookingPaymentFailed),
			"payment_status": "failed",
		}, auth.SubjectID)
		return InitiatePaymentResponse{}, fmt.Errorf("%w: initiate payment: %v", ErrPaymentProvider, err)
	}

	return InitiatePaymentResponse{
		PaymentID:   payment.ID.String(),
		CheckoutURL: result.CheckoutURL,
		TxRef:       result.TxRef,
	}, nil
}

func (s *Service) HandleChapaTransaction(ctx context.Context, bookingID uuid.UUID, txRef string) error {
	txRef = strings.TrimSpace(txRef)
	if txRef == "" {
		return ErrPaymentNotFound
	}

	payment, err := s.repo.PaymentByProviderRef(ctx, txRef)
	if err != nil {
		return ErrPaymentNotFound
	}
	if payment.BookingID != bookingID {
		return ErrPaymentNotFound
	}
	if payment.Status == "success" {
		return s.confirmPayment(ctx, payment, txRef)
	}

	verified, err := s.provider.VerifyTransaction(ctx, txRef)
	if err != nil {
		return fmt.Errorf("%w: verify transaction: %v", ErrPaymentProvider, err)
	}
	if !strings.EqualFold(verified.TxRef, txRef) {
		return ErrPaymentVerificationMismatch
	}
	if verified.Currency != "" && !strings.EqualFold(verified.Currency, payment.Currency) {
		return ErrPaymentVerificationMismatch
	}
	if verified.AmountCents > 0 && verified.AmountCents != payment.AmountCents {
		return ErrPaymentVerificationMismatch
	}

	switch normalizeChapaStatus(verified.Status) {
	case "success":
		return s.confirmPayment(ctx, payment, txRef)
	case "failed":
		return s.failPayment(ctx, payment)
	case "pending":
		return s.keepPaymentPending(ctx, payment)
	default:
		return nil
	}
}

func (s *Service) confirmPayment(ctx context.Context, payment domain.Payment, txRef string) error {
	if payment.Status != "success" {
		if err := s.repo.UpdatePaymentFields(ctx, payment.ID, map[string]any{"status": "success"}); err != nil {
			return err
		}
	}
	if err := s.repo.UpdateBookingFields(ctx, payment.BookingID, map[string]any{
		"status":         string(domain.BookingConfirmed),
		"payment_status": "paid",
		"payment_ref":    txRef,
	}, uuid.Nil); err != nil {
		return err
	}
	if err := s.triggerTestCreation(ctx, payment.BookingID); err != nil {
		return fmt.Errorf("booking.triggerTestCreation booking=%s: %w", payment.BookingID, err)
	}
	return nil
}

func (s *Service) failPayment(ctx context.Context, payment domain.Payment) error {
	if err := s.repo.UpdatePaymentFields(ctx, payment.ID, map[string]any{"status": "failed"}); err != nil {
		return err
	}
	return s.repo.UpdateBookingFields(ctx, payment.BookingID, map[string]any{
		"status":         string(domain.BookingPaymentFailed),
		"payment_status": "failed",
	}, uuid.Nil)
}

func (s *Service) keepPaymentPending(ctx context.Context, payment domain.Payment) error {
	if payment.Status != "pending" {
		if err := s.repo.UpdatePaymentFields(ctx, payment.ID, map[string]any{"status": "pending"}); err != nil {
			return err
		}
	}
	return s.repo.UpdateBookingFields(ctx, payment.BookingID, map[string]any{
		"status":         string(domain.BookingPaymentPending),
		"payment_status": "pending",
	}, uuid.Nil)
}

func (s *Service) triggerTestCreation(ctx context.Context, bookingID uuid.UUID) error {
	if strings.TrimSpace(s.internalAPIKey) == "" {
		return nil
	}

	details, err := s.repo.TestCreationDetails(ctx, bookingID)
	if err != nil {
		return err
	}

	payload := map[string]string{
		"booking_id":      details.BookingID.String(),
		"candidate_id":    details.CandidateID.String(),
		"test_center_id":  details.TestCenterID.String(),
		"test_level_code": details.TestLevelCode,
	}
	body, _ := json.Marshal(payload)

	testingURL := strings.TrimRight(s.baseURL, "/") + "/internal/tests"
	req, err := http.NewRequestWithContext(ctx, "POST", testingURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", s.internalAPIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("testing module returned %d: %s", resp.StatusCode, string(b))
	}

	// Read response to get test_id
	var responseData struct {
		Data struct {
			TestID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&responseData); err == nil && responseData.Data.TestID != "" {
		if testID, err := uuid.Parse(responseData.Data.TestID); err == nil {
			_ = s.repo.UpdateBookingFields(ctx, bookingID, map[string]any{
				"test_id": testID,
			}, uuid.Nil)
		}
	}

	return nil
}

func (s *Service) RetryPayment(ctx context.Context, auth *security.AuthContext, bookingID uuid.UUID, req InitiatePaymentRequest) (InitiatePaymentResponse, error) {
	b, err := s.repo.BookingByID(ctx, bookingID)
	if err != nil {
		return InitiatePaymentResponse{}, ErrBookingNotFound
	}
	if b.CandidateID != auth.SubjectID {
		return InitiatePaymentResponse{}, ErrForbidden
	}
	if b.Status != domain.BookingPaymentFailed {
		return InitiatePaymentResponse{}, ErrInvalidStatusForAction
	}
	if b.PaymentAttempts >= maxPaymentAttempts {
		return InitiatePaymentResponse{}, ErrMaxPaymentAttempts
	}

	if err := s.repo.UpdateBookingFields(ctx, bookingID, map[string]any{
		"status": string(domain.BookingScheduled),
	}, auth.SubjectID); err != nil {
		return InitiatePaymentResponse{}, err
	}

	return s.InitiatePayment(ctx, auth, bookingID, req)
}

func (s *Service) ListPayments(ctx context.Context, auth *security.AuthContext, bookingID uuid.UUID) ([]domain.Payment, error) {
	b, err := s.repo.BookingByID(ctx, bookingID)
	if err != nil {
		return nil, ErrBookingNotFound
	}
	if err := s.authorizeBookingAccess(auth, b); err != nil {
		return nil, err
	}
	return s.repo.ListPaymentsByBooking(ctx, bookingID)
}

// -------------------------------------------------------
// Slot management
// -------------------------------------------------------

func (s *Service) CreateSlot(ctx context.Context, auth *security.AuthContext, req CreateSlotRequest) (domain.Slot, error) {
	if err := req.Validate(); err != nil {
		return domain.Slot{}, err
	}
	if auth.EntityType != security.EntityAdmin && auth.EntityType != security.EntitySuperAdmin {
		return domain.Slot{}, ErrForbidden
	}

	instituteID, err := uuid.Parse(req.InstituteID)
	if err != nil {
		return domain.Slot{}, ErrMissingInstituteID
	}
	exists, err := s.repo.InstituteExists(ctx, instituteID)
	if err != nil {
		return domain.Slot{}, err
	}
	if !exists {
		return domain.Slot{}, ErrInstituteNotFound
	}

	var testCenterID *uuid.UUID
	if auth.TestCenterID != nil {
		tc := *auth.TestCenterID
		testCenterID = &tc
	}

	return s.repo.CreateSlot(ctx, domain.Slot{
		InstituteID:  instituteID,
		TestCenterID: testCenterID,
		StartsAt:     req.StartsAt,
		EndsAt:       req.EndsAt,
		Capacity:     req.Capacity,
		BookedCount:  0,
	}, auth.SubjectID)
}

func (s *Service) ListSlots(ctx context.Context, instituteID uuid.UUID, page int) ([]domain.Slot, int, error) {
	return s.repo.ListSlots(ctx, instituteID, page)
}

func (s *Service) GetSlot(ctx context.Context, slotID uuid.UUID) (domain.Slot, error) {
	return s.repo.SlotByID(ctx, slotID)
}

// -------------------------------------------------------
// Internal helpers
// -------------------------------------------------------

func (s *Service) authorizeBookingAccess(auth *security.AuthContext, b domain.Booking) error {
	switch auth.EntityType {
	case security.EntityCandidate:
		if b.CandidateID != auth.SubjectID {
			return ErrForbidden
		}
	case security.EntityInstitute:
		if b.InstituteID != auth.SubjectID {
			return ErrForbidden
		}
	case security.EntityAdmin, security.EntitySuperAdmin:
		return nil
	default:
		return ErrForbidden
	}
	return nil
}

func normalizeChapaStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch {
	case status == "success" || status == "successful":
		return "success"
	case status == "pending":
		return "pending"
	case status == "failed" || status == "cancelled" || status == "canceled" ||
		status == "failed/cancelled" || status == "failed/canceled" ||
		status == "reversed" || status == "refunded":
		return "failed"
	default:
		return status
	}
}

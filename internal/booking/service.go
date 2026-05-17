package booking

import (
	"context"
	"fmt"
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
}

func NewService(repo *Repository, provider PaymentProvider, mailer *mailer.Mailer, baseURL, frontendBaseURL string) *Service {
	return &Service{
		repo:            repo,
		provider:        provider,
		mailer:          mailer,
		baseURL:         baseURL,
		frontendBaseURL: frontendBaseURL,
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
		return domain.Booking{}, ErrInvalidStatusForAction
	}

	fields := map[string]any{}
	if req.Action == "approve" {
		fields["status"] = string(domain.BookingVerified)
		fields["verified_by"] = auth.SubjectID
		fields["verified_at"] = time.Now().UTC()
	} else {
		fields["status"] = string(domain.BookingRejected)
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
	if b.Status != domain.BookingScheduled {
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
		return InitiatePaymentResponse{}, fmt.Errorf("initiate payment: %w", err)
	}

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
		return InitiatePaymentResponse{}, err
	}

	return InitiatePaymentResponse{
		PaymentID:   payment.ID.String(),
		CheckoutURL: result.CheckoutURL,
		TxRef:       result.TxRef,
	}, nil
}

func (s *Service) HandleChapaWebhook(ctx context.Context, txRef string) error {
	payment, err := s.repo.PaymentByProviderRef(ctx, txRef)
	if err != nil {
		return ErrPaymentNotFound
	}

	if payment.Status == "success" || payment.Status == "failed" {
		return nil
	}

	verified, err := s.provider.VerifyTransaction(ctx, txRef)
	if err != nil {
		return fmt.Errorf("verify transaction: %w", err)
	}

	if verified.Status == "success" {
		_ = s.repo.UpdatePaymentFields(ctx, payment.ID, map[string]any{"status": "success"})
		_ = s.repo.UpdateBookingFields(ctx, payment.BookingID, map[string]any{
			"status":         string(domain.BookingConfirmed),
			"payment_status": "paid",
			"payment_ref":    txRef,
		}, uuid.Nil)
		return nil
	}

	_ = s.repo.UpdatePaymentFields(ctx, payment.ID, map[string]any{"status": "failed"})
	_ = s.repo.UpdateBookingFields(ctx, payment.BookingID, map[string]any{
		"status":         string(domain.BookingPaymentFailed),
		"payment_status": "failed",
	}, uuid.Nil)

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

	_ = s.repo.UpdateBookingFields(ctx, bookingID, map[string]any{
		"status": string(domain.BookingScheduled),
	}, auth.SubjectID)

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

	return s.repo.CreateSlot(ctx, domain.Slot{
		InstituteID: instituteID,
		StartsAt:    req.StartsAt,
		EndsAt:      req.EndsAt,
		Capacity:    req.Capacity,
		BookedCount: 0,
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

package booking

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"adlts/internal/domain"
	"adlts/internal/platform/httpx"
	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc    *Service
	tokens *security.Manager
}

func NewHandler(svc *Service, tokens *security.Manager) *Handler {
	return &Handler{svc: svc, tokens: tokens}
}

// -------------------------------------------------------
// Booking handlers
// -------------------------------------------------------

func (h *Handler) createBooking(w http.ResponseWriter, r *http.Request) {
	auth, ok := security.CurrentAuth(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil)
		return
	}

	var req CreateBookingRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}

	b, err := h.svc.CreateBooking(r.Context(), auth.SubjectID, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusCreated, toBookingResponse(b), nil)
}

func (h *Handler) getBooking(w http.ResponseWriter, r *http.Request) {
	auth, ok := security.CurrentAuth(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil)
		return
	}
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}

	b, err := h.svc.GetBooking(r.Context(), auth, id)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusOK, toBookingResponse(b), nil)
}

func (h *Handler) listBookings(w http.ResponseWriter, r *http.Request) {
	auth, ok := security.CurrentAuth(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil)
		return
	}
	page := pageParam(r.URL.Query().Get("page"))

	var statusFilter *domain.BookingStatus
	if raw := r.URL.Query().Get("status"); raw != "" {
		bs := domain.BookingStatus(raw)
		statusFilter = &bs
	}

	bookings, total, err := h.svc.ListBookings(r.Context(), auth, statusFilter, page)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	resp := make([]BookingResponse, 0, len(bookings))
	for _, b := range bookings {
		resp = append(resp, toBookingResponse(b))
	}
	httpx.Success(w, http.StatusOK, resp, &httpx.Meta{Page: page, Total: total, Limit: 20})
}

func (h *Handler) verifyBooking(w http.ResponseWriter, r *http.Request) {
	auth, ok := security.CurrentAuth(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil)
		return
	}
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}

	var req VerifyBookingRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}

	b, err := h.svc.VerifyBooking(r.Context(), auth, id, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusOK, toBookingResponse(b), nil)
}

func (h *Handler) scheduleBooking(w http.ResponseWriter, r *http.Request) {
	auth, ok := security.CurrentAuth(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil)
		return
	}
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}

	var req ScheduleBookingRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}

	b, err := h.svc.ScheduleBooking(r.Context(), auth, id, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusOK, toBookingResponse(b), nil)
}

func (h *Handler) rescheduleBooking(w http.ResponseWriter, r *http.Request) {
	auth, ok := security.CurrentAuth(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil)
		return
	}
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}

	var req RescheduleBookingRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}

	b, err := h.svc.RescheduleBooking(r.Context(), auth, id, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusOK, toBookingResponse(b), nil)
}

func (h *Handler) deleteBooking(w http.ResponseWriter, r *http.Request) {
	auth, ok := security.CurrentAuth(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil)
		return
	}
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}

	if err := h.svc.DeleteBooking(r.Context(), auth, id); err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "booking deleted"}, nil)
}

// -------------------------------------------------------
// Payment handlers
// -------------------------------------------------------

func (h *Handler) initiatePayment(w http.ResponseWriter, r *http.Request) {
	auth, ok := security.CurrentAuth(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil)
		return
	}
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}

	var req InitiatePaymentRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}

	resp, err := h.svc.InitiatePayment(r.Context(), auth, id, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusCreated, resp, nil)
}

func (h *Handler) handleChapaWebhook(w http.ResponseWriter, r *http.Request) {
	_, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}

	rawBody, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "could not read request body", nil)
		return
	}

	sig := r.Header.Get("x-chapa-signature")
	if sig == "" {
		sig = r.Header.Get("chapa-signature")
	}
	if !h.svc.provider.ValidateWebhookSignature(rawBody, sig) {
		httpx.Failure(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "invalid webhook signature", nil)
		return
	}

	var event struct {
		TxRef  string `json:"tx_ref"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rawBody, &event); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "invalid webhook body", nil)
		return
	}

	_ = event.Status
	if err := h.svc.HandleChapaWebhook(r.Context(), event.TxRef); err != nil {
		// Return 200 to avoid provider retries; errors are handled internally.
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) retryPayment(w http.ResponseWriter, r *http.Request) {
	auth, ok := security.CurrentAuth(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil)
		return
	}
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}

	var req InitiatePaymentRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}

	resp, err := h.svc.RetryPayment(r.Context(), auth, id, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusCreated, resp, nil)
}

func (h *Handler) listPayments(w http.ResponseWriter, r *http.Request) {
	auth, ok := security.CurrentAuth(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil)
		return
	}
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}

	payments, err := h.svc.ListPayments(r.Context(), auth, id)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	resp := make([]PaymentResponse, 0, len(payments))
	for _, p := range payments {
		resp = append(resp, toPaymentResponse(p))
	}
	httpx.Success(w, http.StatusOK, resp, nil)
}

// -------------------------------------------------------
// Slot handlers
// -------------------------------------------------------

func (h *Handler) createSlot(w http.ResponseWriter, r *http.Request) {
	auth, ok := security.CurrentAuth(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil)
		return
	}

	var req CreateSlotRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}

	slot, err := h.svc.CreateSlot(r.Context(), auth, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusCreated, toSlotResponse(slot), nil)
}

func (h *Handler) listSlots(w http.ResponseWriter, r *http.Request) {
	instituteIDRaw := r.URL.Query().Get("institute_id")
	instID, err := uuid.Parse(instituteIDRaw)
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_INSTITUTE", "institute_id query param is required", nil)
		return
	}
	page := pageParam(r.URL.Query().Get("page"))

	slots, total, err := h.svc.ListSlots(r.Context(), instID, page)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	resp := make([]SlotResponse, 0, len(slots))
	for _, s := range slots {
		resp = append(resp, toSlotResponse(s))
	}
	httpx.Success(w, http.StatusOK, resp, &httpx.Meta{Page: page, Total: total, Limit: 20})
}

func (h *Handler) getSlot(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}

	slot, err := h.svc.GetSlot(r.Context(), id)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusOK, toSlotResponse(slot), nil)
}

// -------------------------------------------------------
// Helpers
// -------------------------------------------------------

func handleServiceError(w http.ResponseWriter, err error) {
	switch err {
	case ErrBookingNotFound, ErrSlotNotFound, ErrPaymentNotFound, ErrInstituteNotFound, ErrCandidateNotFound:
		httpx.Failure(w, http.StatusNotFound, "NOT_FOUND", err.Error(), nil)
	case ErrForbidden:
		httpx.Failure(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
	case ErrAlreadyProcessed:
		httpx.Failure(w, http.StatusConflict, "ALREADY_PROCESSED", err.Error(), nil)
	case ErrInvalidStatusForAction, ErrSlotFull, ErrMaxPaymentAttempts,
		ErrInvalidVerifyAction, ErrMissingRejectionReason,
		ErrInvalidAmount, ErrInvalidCurrency, ErrMissingInstituteID,
		ErrMissingSlotID, ErrInvalidSlotTimes, ErrMissingSlotTimes:
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), nil)
	default:
		log.Printf("ERROR booking handler: %v", err)
		httpx.Failure(w, http.StatusInternalServerError, "SERVER_ERROR", "internal error", nil)
	}
}

func parseID(r *http.Request, param string) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, param))
}

func pageParam(raw string) int {
	p, _ := strconv.Atoi(raw)
	if p < 1 {
		return 1
	}
	return p
}

// -------------------------------------------------------
// Response mappers
// -------------------------------------------------------

func toBookingResponse(b domain.Booking) BookingResponse {
	resp := BookingResponse{
		ID:                   b.ID.String(),
		CandidateID:          b.CandidateID.String(),
		InstituteID:          b.InstituteID.String(),
		Status:               string(b.Status),
		RequiresVerification: b.RequiresVerification,
		PaymentStatus:        b.PaymentStatus,
		PaymentAmountCents:   b.PaymentAmountCents,
		PaymentAttempts:      b.PaymentAttempts,
		CreatedAt:            b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:            b.UpdatedAt.Format(time.RFC3339),
	}
	if b.TestID != nil {
		s := b.TestID.String()
		resp.TestID = &s
	}
	if b.SlotID != nil {
		s := b.SlotID.String()
		resp.SlotID = &s
	}
	if b.VerifiedBy != nil {
		s := b.VerifiedBy.String()
		resp.VerifiedBy = &s
	}
	if b.VerifiedAt != nil {
		s := b.VerifiedAt.Format(time.RFC3339)
		resp.VerifiedAt = &s
	}
	if b.RejectionReason != nil {
		resp.RejectionReason = b.RejectionReason
	}
	if b.ScheduledAt != nil {
		s := b.ScheduledAt.Format(time.RFC3339)
		resp.ScheduledAt = &s
	}
	if b.ArchivedAt != nil {
		s := b.ArchivedAt.Format(time.RFC3339)
		resp.ArchivedAt = &s
	}
	return resp
}

func toSlotResponse(s domain.Slot) SlotResponse {
	return SlotResponse{
		ID:          s.ID.String(),
		InstituteID: s.InstituteID.String(),
		StartsAt:    s.StartsAt.Format(time.RFC3339),
		EndsAt:      s.EndsAt.Format(time.RFC3339),
		Capacity:    s.Capacity,
		BookedCount: s.BookedCount,
		Available:   s.Capacity - s.BookedCount,
	}
}

func toPaymentResponse(p domain.Payment) PaymentResponse {
	resp := PaymentResponse{
		ID:            p.ID.String(),
		BookingID:     p.BookingID.String(),
		AmountCents:   p.AmountCents,
		Currency:      p.Currency,
		Status:        p.Status,
		Provider:      p.Provider,
		AttemptNumber: p.AttemptNumber,
		CreatedAt:     p.CreatedAt.Format(time.RFC3339),
	}
	if p.ProviderRef != nil {
		resp.ProviderRef = p.ProviderRef
	}
	return resp
}

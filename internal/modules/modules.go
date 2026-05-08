package modules

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"time"

	"adlts/internal/platform/domain"
	"adlts/internal/platform/httpx"
	"adlts/internal/platform/runtime"
	"adlts/internal/platform/security"
	"adlts/internal/platform/store"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

type module struct {
	deps     runtime.Dependencies
	upgrader websocket.Upgrader
}

func newModule(deps runtime.Dependencies) module {
	return module{
		deps:     deps,
		upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	}
}

func RegisterAuthRoutes(r chi.Router, deps runtime.Dependencies) {
	m := newModule(deps)
	authMW := security.Authenticate(deps.Tokens, deps.Store)
	r.Post("/register", m.handleRegisterCandidate)
	r.Post("/register/candidate", m.handleRegisterCandidate)
	r.Post("/register/institute", m.handleRegisterInstitute)
	r.Post("/login", m.handleLogin)
	r.With(authMW, security.RequireRoles(domain.RoleAuthority, domain.RoleAdmin)).Post("/invite", m.handleInvite)
	r.With(authMW).Post("/request-otp", m.handleRequestOTP)
	r.With(authMW).Post("/verify-otp", m.handleVerifyOTP)
}

func RegisterUserRoutes(r chi.Router, deps runtime.Dependencies) {
	m := newModule(deps)
	authMW := security.Authenticate(deps.Tokens, deps.Store)
	r.With(authMW).Get("/me", m.handleMe)
	r.With(authMW, security.RequireRoles(domain.RoleAuthority, domain.RoleAdmin)).Patch("/{id}", m.handlePatchUser)
}

func RegisterInstituteRoutes(r chi.Router, deps runtime.Dependencies) {
	m := newModule(deps)
	authMW := security.Authenticate(deps.Tokens, deps.Store)
	r.With(authMW).Get("/", m.handleListInstitutes)
}

func RegisterBookingRoutes(r chi.Router, deps runtime.Dependencies) {
	m := newModule(deps)
	authMW := security.Authenticate(deps.Tokens, deps.Store)
	r.With(authMW, security.RequireRoles(domain.RoleCandidate)).Post("/", m.handleCreateBooking)
	r.With(authMW, security.RequireRoles(domain.RoleInstituteAdmin)).Get("/verify", m.handleListPendingBookings)
	r.With(authMW, security.RequireRoles(domain.RoleInstituteAdmin)).Patch("/{id}/verify", m.handleVerifyBooking)
	r.With(authMW, security.RequireRoles(domain.RoleCandidate)).Get("/available-slots", m.handleAvailableSlots)
}

func RegisterScheduleRoutes(r chi.Router, deps runtime.Dependencies) {
	m := newModule(deps)
	authMW := security.Authenticate(deps.Tokens, deps.Store)
	r.With(authMW, security.RequireRoles(domain.RoleCandidate)).Get("/", m.handleAvailableSlots)
}

func RegisterDeviceRoutes(r chi.Router, deps runtime.Dependencies) {
	m := newModule(deps)
	authMW := security.Authenticate(deps.Tokens, deps.Store)
	r.With(authMW, security.RequireRoles(domain.RoleAdmin, domain.RoleAuthority)).Post("/register", m.handleRegisterDevice)
}

func RegisterAdminRoutes(r chi.Router, deps runtime.Dependencies) {
	m := newModule(deps)
	r.Post("/devices/heartbeat", m.handleDeviceHeartbeat)
	r.With(security.Authenticate(deps.Tokens, deps.Store), security.RequireRoles(domain.RoleAdmin, domain.RoleAuthority)).Get("/analytics/map", m.handleHeatmap)
}

func RegisterInternalRoutes(r chi.Router, deps runtime.Dependencies) {
	m := newModule(deps)
	r.With(security.RequireInternalToken(deps.Config.InternalAPIKey)).Post("/scoring/frame-process", m.handleFrameProcess)
}

func RegisterScoringRoutes(r chi.Router, deps runtime.Dependencies) {
	m := newModule(deps)
	r.With(security.RequireInternalToken(deps.Config.InternalAPIKey)).Post("/analyze", m.handleFrameProcess)
}

func RegisterExamRoutes(r chi.Router, deps runtime.Dependencies) {
	m := newModule(deps)
	authMW := security.Authenticate(deps.Tokens, deps.Store)
	r.With(authMW, security.RequireRoles(domain.RoleCandidate)).Post("/initiate", m.handleInitiateExam)
	r.With(authMW, security.RequireRoles(domain.RoleExaminer, domain.RoleAdmin, domain.RoleAuthority)).Get("/{id}/live", m.handleLiveExam)
	r.With(authMW, security.RequireRoles(domain.RoleExaminer, domain.RoleAdmin, domain.RoleAuthority)).Post("/{id}/stop", m.handleStopExam)
	r.With(authMW).Get("/{id}/results", m.handleExamResults)
	r.With(authMW, security.RequireRoles(domain.RoleExaminer, domain.RoleAdmin, domain.RoleAuthority)).Get("/{id}/telemetry", m.handleTelemetry)
	r.With(authMW).Get("/{id}/result-overlay", m.handleResultOverlay)
}

func RegisterAppealRoutes(r chi.Router, deps runtime.Dependencies) {
	m := newModule(deps)
	authMW := security.Authenticate(deps.Tokens, deps.Store)
	r.With(authMW, security.RequireRoles(domain.RoleCandidate)).Post("/", m.handleCreateAppeal)
	r.With(authMW, security.RequireRoles(domain.RoleAuthority)).Get("/pending", m.handlePendingAppeals)
	r.With(authMW, security.RequireRoles(domain.RoleAuthority)).Patch("/{id}", m.handleResolveAppeal)
	// alias for the authority path requested in the spec
	r.With(authMW, security.RequireRoles(domain.RoleAuthority)).Patch("/{id}/resolve", m.handleResolveAppeal)
}

func RegisterAuthorityRoutes(r chi.Router, deps runtime.Dependencies) {
	m := newModule(deps)
	authMW := security.Authenticate(deps.Tokens, deps.Store)
	r.With(authMW, security.RequireRoles(domain.RoleAuthority)).Patch("/appeals/{id}/resolve", m.handleResolveAppeal)
}

func RegisterAnalyticsRoutes(r chi.Router, deps runtime.Dependencies) {
	m := newModule(deps)
	authMW := security.Authenticate(deps.Tokens, deps.Store)
	r.With(authMW, security.RequireRoles(domain.RoleAuthority, domain.RoleAdmin)).Get("/global", m.handleGlobalAnalytics)
	r.With(authMW, security.RequireRoles(domain.RoleAuthority, domain.RoleAdmin)).Get("/map", m.handleHeatmap)
}

func RegisterWsRoutes(r chi.Router, deps runtime.Dependencies) {
	m := newModule(deps)
	r.Get("/iot/stream/{device_id}", m.handleStream)
	r.Get("/ws/iot/stream/{device_id}", m.handleStream)
}

func RegisterIoTRoutes(r chi.Router, deps runtime.Dependencies) {
	m := newModule(deps)
	r.Get("/stream/{device_id}", m.handleStream)
}

type inviteRequest struct {
	Email string      `json:"email"`
	Role  domain.Role `json:"role"`
}

type registerCandidateRequest struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	InstituteID string `json:"institute_id"`
	OTPCode     string `json:"otp_code,omitempty"`
}

type registerInstituteRequest struct {
	Token         string `json:"token"`
	InstituteName string `json:"institute_name"`
	AdminName     string `json:"admin_name"`
	AdminEmail    string `json:"admin_email"`
	Password      string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type verifyOTPRequest struct {
	Email string `json:"email,omitempty"`
	Code  string `json:"code"`
}

const otpValidityWindow = 10 * time.Minute

type userPatchRequest struct {
	Status      domain.AccountStatus `json:"status"`
	InstituteID string               `json:"institute_id,omitempty"`
}

type bookingCreateRequest struct {
	InstituteID     string `json:"institute_id"`
	RequestedSlotID string `json:"requested_slot_id,omitempty"`
	TrainingHours   int    `json:"training_hours,omitempty"`
}

type bookingVerifyRequest struct {
	Approved            bool   `json:"approved"`
	TrainingHours       int    `json:"training_hours,omitempty"`
	TrainingEvidenceURL string `json:"training_evidence_url,omitempty"`
	ScheduledSlotID     string `json:"scheduled_slot_id,omitempty"`
}

type deviceRegisterRequest struct {
	MACAddress string `json:"mac_address"`
	Name       string `json:"name"`
}

type deviceHeartbeatRequest struct {
	DeviceID string `json:"device_id"`
	Secret   string `json:"secret"`
}

type frameRequest struct {
	ExamID   string  `json:"exam_id,omitempty"`
	DeviceID string  `json:"device_id,omitempty"`
	Frame    string  `json:"frame,omitempty"`
	Speed    float64 `json:"speed,omitempty"`
	Source   string  `json:"source,omitempty"`
}

type examInitiateRequest struct {
	BookingID string `json:"booking_id"`
	DeviceID  string `json:"device_id"`
	QRCode    string `json:"qr_code,omitempty"`
}

type appealCreateRequest struct {
	ExamID string `json:"exam_id"`
	Reason string `json:"reason"`
}

type appealResolveRequest struct {
	Status     domain.AppealStatus `json:"status"`
	Resolution string              `json:"resolution"`
}

type analysisResult struct {
	FrameID         string             `json:"frame_id"`
	DeviceID        string             `json:"device_id"`
	ExamID          string             `json:"exam_id,omitempty"`
	DetectedObjects []string           `json:"detected_objects"`
	Violations      []domain.Violation `json:"violations"`
	ScoreDelta      float64            `json:"score_delta"`
	Speed           float64            `json:"speed"`
	At              time.Time          `json:"at"`
}

func (m module) handleInvite(w http.ResponseWriter, r *http.Request) {
	current, ok := security.CurrentUser(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authenticated user", nil)
		return
	}
	var req inviteRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid invite payload", err.Error())
		return
	}
	if req.Email == "" || req.Role == "" {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "email and role are required", nil)
		return
	}
	if req.Role == domain.RoleInternal || req.Role == domain.RoleCandidate {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ROLE", "this endpoint only issues institution or examiner invitations", nil)
		return
	}
	now := time.Now().UTC()
	invitation := &domain.Invitation{
		Token:     store.NewID(),
		Email:     strings.ToLower(strings.TrimSpace(req.Email)),
		Role:      req.Role,
		ExpiresAt: now.Add(72 * time.Hour),
		CreatedBy: current.ID,
		CreatedAt: now,
	}
	store.Write(m.deps.Store, func() struct{} {
		m.deps.Store.Invitations[invitation.Token] = invitation
		return struct{}{}
	})
	httpx.Success(w, http.StatusCreated, invitation, nil)
}

func (m module) handleRegisterCandidate(w http.ResponseWriter, r *http.Request) {
	var req registerCandidateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid candidate registration payload", err.Error())
		return
	}
	if req.Name == "" || req.Email == "" || req.Password == "" || req.InstituteID == "" {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "name, email, password, and institute_id are required", nil)
		return
	}
	if _, ok := m.deps.Store.FindInstitute(req.InstituteID); !ok {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_INSTITUTE", "selected institute does not exist", nil)
		return
	}
	if _, exists := m.deps.Store.FindUserByEmail(strings.ToLower(strings.TrimSpace(req.Email))); exists {
		httpx.Failure(w, http.StatusConflict, "EMAIL_EXISTS", "account already exists", nil)
		return
	}
	hash, err := security.HashPassword(req.Password)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "unable to hash password", err.Error())
		return
	}
	now := time.Now().UTC()
	user := &domain.User{
		ID:           store.NewID(),
		Name:         strings.TrimSpace(req.Name),
		Email:        strings.ToLower(strings.TrimSpace(req.Email)),
		PasswordHash: hash,
		Role:         domain.RoleCandidate,
		Status:       domain.AccountActive,
		InstituteID:  req.InstituteID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	store.Write(m.deps.Store, func() struct{} {
		m.deps.Store.Users[user.ID] = user
		return struct{}{}
	})
	token, _ := m.deps.Tokens.Sign(user, false)
	httpx.Success(w, http.StatusCreated, map[string]any{"user": user, "token": token, "role": user.Role}, nil)
}

func (m module) handleRegisterInstitute(w http.ResponseWriter, r *http.Request) {
	var req registerInstituteRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid institute registration payload", err.Error())
		return
	}
	if req.Token == "" || req.InstituteName == "" || req.AdminName == "" || req.AdminEmail == "" || req.Password == "" {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "token, institute_name, admin_name, admin_email, and password are required", nil)
		return
	}
	var invitation *domain.Invitation
	var inviteFound bool
	store.Read(m.deps.Store, func() struct{} {
		invitation, inviteFound = m.deps.Store.Invitations[req.Token]
		return struct{}{}
	})
	if !inviteFound || invitation.Used || time.Now().UTC().After(invitation.ExpiresAt) {
		httpx.Failure(w, http.StatusBadRequest, "INVITATION_INVALID", "invitation token is invalid or expired", nil)
		return
	}
	if strings.ToLower(strings.TrimSpace(req.AdminEmail)) != invitation.Email || invitation.Role != domain.RoleInstituteAdmin {
		httpx.Failure(w, http.StatusBadRequest, "INVITATION_MISMATCH", "invitation does not match the requested institute admin account", nil)
		return
	}
	if _, exists := m.deps.Store.FindUserByEmail(strings.ToLower(strings.TrimSpace(req.AdminEmail))); exists {
		httpx.Failure(w, http.StatusConflict, "EMAIL_EXISTS", "account already exists", nil)
		return
	}
	hash, err := security.HashPassword(req.Password)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "unable to hash password", err.Error())
		return
	}
	now := time.Now().UTC()
	institute := &domain.Institute{ID: store.NewID(), Name: strings.TrimSpace(req.InstituteName), Email: strings.ToLower(strings.TrimSpace(req.AdminEmail)), Verified: true, CreatedAt: now}
	admin := &domain.User{ID: store.NewID(), Name: strings.TrimSpace(req.AdminName), Email: strings.ToLower(strings.TrimSpace(req.AdminEmail)), PasswordHash: hash, Role: domain.RoleInstituteAdmin, Status: domain.AccountActive, InstituteID: institute.ID, CreatedAt: now, UpdatedAt: now}
	store.Write(m.deps.Store, func() struct{} {
		m.deps.Store.Institutes[institute.ID] = institute
		m.deps.Store.Users[admin.ID] = admin
		invitation.Used = true
		return struct{}{}
	})
	token, _ := m.deps.Tokens.Sign(admin, false)
	httpx.Success(w, http.StatusCreated, map[string]any{"institute": institute, "admin": admin, "token": token, "role": admin.Role}, nil)
}

func (m module) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid login payload", err.Error())
		return
	}
	user, ok := m.deps.Store.FindUserByEmail(strings.ToLower(strings.TrimSpace(req.Email)))
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password", nil)
		return
	}
	if err := security.CheckPassword(user.PasswordHash, req.Password); err != nil {
		httpx.Failure(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password", nil)
		return
	}
	if user.Status != domain.AccountActive {
		httpx.Failure(w, http.StatusForbidden, "ACCOUNT_INACTIVE", "account is not active", nil)
		return
	}
	otpRequired := user.OTPCode != "" && user.OTPVerifiedAt == nil
	token, err := m.deps.Tokens.Sign(user, !otpRequired)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "TOKEN_SIGN_FAILED", "unable to issue token", err.Error())
		return
	}
	httpx.Success(w, http.StatusOK, map[string]any{"token": token, "role": user.Role, "otp_required": otpRequired, "user": user}, nil)
}

func (m module) handleVerifyOTP(w http.ResponseWriter, r *http.Request) {
	user, ok := security.CurrentUser(r)
	if !ok {
		var req verifyOTPRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid otp payload", err.Error())
			return
		}
		if req.Email == "" || req.Code == "" {
			httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "email and code are required when no bearer token is provided", nil)
			return
		}
		found, exists := m.deps.Store.FindUserByEmail(strings.ToLower(strings.TrimSpace(req.Email)))
		if !exists {
			httpx.Failure(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found", nil)
			return
		}
		user = found
		if req.Code != user.OTPCode {
			httpx.Failure(w, http.StatusUnauthorized, "INVALID_OTP", "otp code does not match", nil)
			return
		}
	} else {
		var req verifyOTPRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid otp payload", err.Error())
			return
		}
		if req.Code != user.OTPCode {
			httpx.Failure(w, http.StatusUnauthorized, "INVALID_OTP", "otp code does not match", nil)
			return
		}
	}
	now := time.Now().UTC()
	issuedAt, issued := m.deps.Store.OTPHistory[user.ID]
	if !issued || now.Sub(issuedAt) > otpValidityWindow {
		httpx.Failure(w, http.StatusUnauthorized, "OTP_EXPIRED", "otp code has expired", nil)
		return
	}
	store.Write(m.deps.Store, func() struct{} {
		user.OTPVerifiedAt = &now
		user.UpdatedAt = now
		m.deps.Store.OTPHistory[user.ID] = now
		return struct{}{}
	})
	token, _ := m.deps.Tokens.Sign(user, true)
	httpx.Success(w, http.StatusOK, map[string]any{"token": token, "otp_verified": true, "user": user}, nil)
}

func (m module) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := security.CurrentUser(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authenticated user", nil)
		return
	}
	httpx.Success(w, http.StatusOK, user, nil)
}

func (m module) handlePatchUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "id is required", nil)
		return
	}
	var req userPatchRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid patch payload", err.Error())
		return
	}
	updated := store.Write(m.deps.Store, func() *domain.User {
		user, exists := m.deps.Store.Users[id]
		if !exists {
			return nil
		}
		if req.Status != "" {
			user.Status = req.Status
		}
		if req.InstituteID != "" {
			user.InstituteID = req.InstituteID
		}
		user.UpdatedAt = time.Now().UTC()
		return user
	})
	if updated == nil {
		httpx.Failure(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, updated, nil)
}

func (m module) handleListInstitutes(w http.ResponseWriter, r *http.Request) {
	institutes := store.Read(m.deps.Store, func() []*domain.Institute {
		result := make([]*domain.Institute, 0, len(m.deps.Store.Institutes))
		for _, institute := range m.deps.Store.Institutes {
			result = append(result, institute)
		}
		sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
		return result
	})
	httpx.Success(w, http.StatusOK, institutes, &httpx.Meta{Total: len(institutes)})
}

func (m module) handleCreateBooking(w http.ResponseWriter, r *http.Request) {
	current, ok := security.CurrentUser(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authenticated user", nil)
		return
	}
	var req bookingCreateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid booking payload", err.Error())
		return
	}
	if req.InstituteID == "" {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "institute_id is required", nil)
		return
	}
	if _, ok := m.deps.Store.FindInstitute(req.InstituteID); !ok {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_INSTITUTE", "institute does not exist", nil)
		return
	}
	if req.RequestedSlotID != "" {
		if slot, ok := m.deps.Store.Slots[req.RequestedSlotID]; !ok || slot.InstituteID != req.InstituteID {
			httpx.Failure(w, http.StatusBadRequest, "INVALID_SLOT", "requested slot is not available for the selected institute", nil)
			return
		}
	}
	now := time.Now().UTC()
	booking := &domain.Booking{
		ID:              store.NewID(),
		CandidateID:     current.ID,
		InstituteID:     req.InstituteID,
		RequestedSlotID: req.RequestedSlotID,
		Status:          domain.BookingPendingVerification,
		TrainingHours:   req.TrainingHours,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	store.Write(m.deps.Store, func() struct{} {
		m.deps.Store.Bookings[booking.ID] = booking
		return struct{}{}
	})
	httpx.Success(w, http.StatusCreated, booking, nil)
}

func (m module) handleListPendingBookings(w http.ResponseWriter, r *http.Request) {
	current, _ := security.CurrentUser(r)
	bookings := store.Read(m.deps.Store, func() []*domain.Booking {
		result := make([]*domain.Booking, 0)
		for _, booking := range m.deps.Store.Bookings {
			if booking.InstituteID == current.InstituteID && booking.Status == domain.BookingPendingVerification {
				result = append(result, booking)
			}
		}
		return result
	})
	httpx.Success(w, http.StatusOK, bookings, &httpx.Meta{Total: len(bookings)})
}

func (m module) handleVerifyBooking(w http.ResponseWriter, r *http.Request) {
	current, _ := security.CurrentUser(r)
	id := chi.URLParam(r, "id")
	var req bookingVerifyRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid booking verification payload", err.Error())
		return
	}
	updated := store.Write(m.deps.Store, func() *domain.Booking {
		booking, exists := m.deps.Store.Bookings[id]
		if !exists || booking.InstituteID != current.InstituteID {
			return nil
		}
		now := time.Now().UTC()
		if req.Approved {
			booking.Status = domain.BookingVerified
			booking.TrainingHours = req.TrainingHours
			booking.TrainingEvidenceURL = req.TrainingEvidenceURL
			booking.VerifiedBy = current.ID
			booking.VerifiedAt = &now
			if req.ScheduledSlotID != "" {
				booking.ScheduledSlotID = req.ScheduledSlotID
			}
		} else {
			booking.Status = domain.BookingRejected
		}
		booking.UpdatedAt = now
		return booking
	})
	if updated == nil {
		httpx.Failure(w, http.StatusNotFound, "BOOKING_NOT_FOUND", "booking not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, updated, nil)
}

func (m module) handleAvailableSlots(w http.ResponseWriter, r *http.Request) {
	current, ok := security.CurrentUser(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authenticated user", nil)
		return
	}
	hasVerifiedBooking := false
	store.Read(m.deps.Store, func() struct{} {
		for _, booking := range m.deps.Store.Bookings {
			if booking.CandidateID == current.ID && booking.Status == domain.BookingVerified {
				hasVerifiedBooking = true
				break
			}
		}
		return struct{}{}
	})
	if !hasVerifiedBooking {
		httpx.Failure(w, http.StatusForbidden, "BOOKING_NOT_VERIFIED", "verified booking required to view available slots", nil)
		return
	}
	slots := store.Read(m.deps.Store, func() []*domain.Slot {
		result := make([]*domain.Slot, 0)
		for _, slot := range m.deps.Store.Slots {
			if slot.InstituteID == current.InstituteID && slot.BookedCount < slot.Capacity {
				result = append(result, slot)
			}
		}
		sort.Slice(result, func(i, j int) bool { return result[i].StartTime.Before(result[j].StartTime) })
		return result
	})
	httpx.Success(w, http.StatusOK, slots, &httpx.Meta{Total: len(slots)})
}

func (m module) handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
	var req deviceRegisterRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid device payload", err.Error())
		return
	}
	if req.MACAddress == "" || req.Name == "" {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "mac_address and name are required", nil)
		return
	}
	now := time.Now().UTC()
	device := &domain.Device{ID: store.NewID(), MACAddress: strings.ToUpper(strings.TrimSpace(req.MACAddress)), Name: strings.TrimSpace(req.Name), Secret: store.NewID(), Status: "offline", CreatedAt: now}
	store.Write(m.deps.Store, func() struct{} {
		m.deps.Store.Devices[device.ID] = device
		return struct{}{}
	})
	httpx.Success(w, http.StatusCreated, map[string]any{
		"device":        device,
		"secret":        device.Secret,
		"stream_url":    fmt.Sprintf("/ws/iot/stream/%s", device.ID),
		"heartbeat_url": "/admin/devices/heartbeat",
	}, nil)
}

func (m module) handleRequestOTP(w http.ResponseWriter, r *http.Request) {
	current, ok := security.CurrentUser(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authenticated user", nil)
		return
	}
	code, err := generateOTPCode()
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "OTP_GENERATION_FAILED", "unable to generate otp", err.Error())
		return
	}
	now := time.Now().UTC()
	store.Write(m.deps.Store, func() struct{} {
		current.OTPCode = code
		current.OTPVerifiedAt = nil
		current.UpdatedAt = now
		m.deps.Store.OTPHistory[current.ID] = now
		return struct{}{}
	})
	httpx.Success(w, http.StatusOK, map[string]any{
		"otp_required":       true,
		"otp_code":           code,
		"expires_in_seconds": int(otpValidityWindow.Seconds()),
		"user":               current,
	}, nil)
}

func (m module) handleDeviceHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req deviceHeartbeatRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid heartbeat payload", err.Error())
		return
	}
	updated := store.Write(m.deps.Store, func() *domain.Device {
		device, exists := m.deps.Store.Devices[req.DeviceID]
		if !exists || device.Secret != req.Secret {
			return nil
		}
		now := time.Now().UTC()
		device.Status = "online"
		device.LastHeartbeat = &now
		return device
	})
	if updated == nil {
		httpx.Failure(w, http.StatusUnauthorized, "INVALID_DEVICE", "device could not be authenticated", nil)
		return
	}
	httpx.Success(w, http.StatusOK, updated, nil)
}

func (m module) handleStream(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")
	deviceSecret := r.Header.Get("X-Device-Secret")
	if deviceSecret == "" {
		deviceSecret = r.URL.Query().Get("device_secret")
	}
	device, ok := m.deps.Store.FindDevice(deviceID)
	if !ok || device.Secret != deviceSecret {
		httpx.Failure(w, http.StatusUnauthorized, "INVALID_DEVICE", "device could not be authenticated", nil)
		return
	}
	conn, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	device.Status = "online"
	now := time.Now().UTC()
	device.LastHeartbeat = &now
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}
		analysis := m.processFrame(deviceID, string(message), 0, "websocket")
		if err := conn.WriteJSON(analysis); err != nil {
			return
		}
	}
}

func (m module) handleFrameProcess(w http.ResponseWriter, r *http.Request) {
	var req frameRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid frame payload", err.Error())
		return
	}
	if req.DeviceID == "" {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "device_id is required", nil)
		return
	}
	analysis := m.processFrame(req.DeviceID, req.Frame, req.Speed, req.Source)
	httpx.Success(w, http.StatusOK, analysis, nil)
}

func (m module) processFrame(deviceID, frame string, speed float64, source string) analysisResult {
	now := time.Now().UTC()
	frameLower := strings.ToLower(frame)
	detected := make([]string, 0, 4)
	violations := make([]domain.Violation, 0, 2)
	scoreDelta := 0.0
	if strings.Contains(frameLower, "lane") || strings.Contains(frameLower, "boundary") {
		detected = append(detected, "lane_marking")
	}
	if strings.Contains(frameLower, "stop") {
		detected = append(detected, "stop_sign")
		if speed > 0 {
			violations = append(violations, domain.Violation{Code: "SIGN_DISREGARD", Message: "Stop sign detected while vehicle is moving", Severity: "high", Track: "stop-zone", CreatedAt: now})
			scoreDelta -= 12.5
		}
	}
	if strings.Contains(frameLower, "red") || strings.Contains(frameLower, "traffic_light") {
		detected = append(detected, "traffic_light")
	}
	if strings.Contains(frameLower, "outside") || strings.Contains(frameLower, "lane_violation") {
		violations = append(violations, domain.Violation{Code: "LANE_EXIT", Message: "Vehicle left the lane boundary", Severity: "high", Track: "lane-boundary", CreatedAt: now})
		scoreDelta -= 8.0
	}
	if len(detected) == 0 {
		detected = append(detected, "general_scene")
	}
	analysis := analysisResult{FrameID: store.NewID(), DeviceID: deviceID, DetectedObjects: detected, Violations: violations, ScoreDelta: scoreDelta, Speed: speed, At: now}
	store.Write(m.deps.Store, func() struct{} {
		m.deps.Store.Frames[analysis.FrameID] = &domain.FrameAnalysis{ID: analysis.FrameID, DeviceID: deviceID, Frame: frame, DetectedObjects: detected, Violations: violations, ScoreDelta: scoreDelta, Speed: speed, CreatedAt: now}
		if exam := m.latestExamForDevice(deviceID); exam != nil && (exam.Status == domain.ExamActive || exam.Status == domain.ExamInitiating) {
			exam.DeviceID = deviceID
			exam.Telemetry.LastFrameID = analysis.FrameID
			exam.Telemetry.ViolationCount += len(violations)
			exam.Score = clampScore(exam.Score + scoreDelta)
			exam.Telemetry.CurrentScore = exam.Score
			exam.Telemetry.Health = "nominal"
			exam.UpdatedAt = now
			exam.Violations = append(exam.Violations, violations...)
			if exam.Score < 70 {
				exam.Status = domain.ExamReviewRequired
				exam.Telemetry.Health = "review_required"
			}
		}
		return struct{}{}
	})
	return analysis
}

func (m module) latestExamForDevice(deviceID string) *domain.Exam {
	var matched *domain.Exam
	store.Read(m.deps.Store, func() struct{} {
		for _, exam := range m.deps.Store.Exams {
			if exam.DeviceID == deviceID && (exam.Status == domain.ExamActive || exam.Status == domain.ExamInitiating) {
				matched = exam
			}
		}
		return struct{}{}
	})
	return matched
}

func (m module) handleInitiateExam(w http.ResponseWriter, r *http.Request) {
	current, ok := security.CurrentUser(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authenticated user", nil)
		return
	}
	var req examInitiateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid exam initiation payload", err.Error())
		return
	}
	booking, exists := m.deps.Store.FindBooking(req.BookingID)
	if !exists || booking.CandidateID != current.ID || booking.Status != domain.BookingVerified {
		httpx.Failure(w, http.StatusBadRequest, "BOOKING_NOT_READY", "verified booking required for exam initiation", nil)
		return
	}
	if _, ok := m.deps.Store.FindDevice(req.DeviceID); !ok {
		httpx.Failure(w, http.StatusBadRequest, "DEVICE_NOT_FOUND", "device not found", nil)
		return
	}
	if req.QRCode != "" && req.QRCode != req.BookingID && req.QRCode != req.DeviceID {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_QR", "qr code does not match the booking or device linkage", nil)
		return
	}
	now := time.Now().UTC()
	exam := &domain.Exam{ID: store.NewID(), BookingID: booking.ID, CandidateID: current.ID, DeviceID: req.DeviceID, Status: domain.ExamInitiating, Score: 100, Telemetry: domain.ExamTelemetry{Health: "initiating", CurrentScore: 100, UpdatedAt: now}, StartedAt: &now, CreatedAt: now, UpdatedAt: now}
	store.Write(m.deps.Store, func() struct{} {
		m.deps.Store.Exams[exam.ID] = exam
		booking.Status = domain.BookingScheduled
		booking.ScheduledSlotID = req.DeviceID
		booking.UpdatedAt = now
		return struct{}{}
	})
	exam.Status = domain.ExamActive
	exam.Telemetry.Health = "active"
	httpx.Success(w, http.StatusCreated, exam, nil)
}

func (m module) handleLiveExam(w http.ResponseWriter, r *http.Request) {
	exam, ok := m.readExamForViewer(w, r, true)
	if !ok {
		return
	}
	httpx.Success(w, http.StatusOK, exam, nil)
}

func (m module) handleStopExam(w http.ResponseWriter, r *http.Request) {
	exam, ok := m.readExamForViewer(w, r, true)
	if !ok {
		return
	}
	now := time.Now().UTC()
	store.Write(m.deps.Store, func() struct{} {
		exam.Status = domain.ExamStopped
		exam.CompletedAt = &now
		exam.UpdatedAt = now
		exam.Telemetry.Health = "stopped"
		if exam.Score < 70 {
			exam.Status = domain.ExamReviewRequired
		}
		if exam.ResultOverlayURL == "" {
			exam.ResultOverlayURL = fmt.Sprintf("https://storage.local/exams/%s/overlay.mp4", exam.ID)
		}
		return struct{}{}
	})
	httpx.Success(w, http.StatusOK, exam, nil)
}

func (m module) handleExamResults(w http.ResponseWriter, r *http.Request) {
	exam, ok := m.readExamForViewer(w, r, false)
	if !ok {
		return
	}
	httpx.Success(w, http.StatusOK, exam, nil)
}

func (m module) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	exam, ok := m.readExamForViewer(w, r, true)
	if !ok {
		return
	}
	httpx.Success(w, http.StatusOK, exam.Telemetry, nil)
}

func (m module) handleResultOverlay(w http.ResponseWriter, r *http.Request) {
	exam, ok := m.readExamForViewer(w, r, false)
	if !ok {
		return
	}
	httpx.Success(w, http.StatusOK, map[string]any{"exam_id": exam.ID, "overlay_url": exam.ResultOverlayURL, "violations": exam.Violations}, nil)
}

func (m module) handleCreateAppeal(w http.ResponseWriter, r *http.Request) {
	current, ok := security.CurrentUser(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authenticated user", nil)
		return
	}
	var req appealCreateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid appeal payload", err.Error())
		return
	}
	exam, exists := m.deps.Store.FindExam(req.ExamID)
	if !exists || exam.CandidateID != current.ID {
		httpx.Failure(w, http.StatusBadRequest, "EXAM_NOT_FOUND", "exam not found for candidate", nil)
		return
	}
	now := time.Now().UTC()
	appeal := &domain.Appeal{ID: store.NewID(), ExamID: exam.ID, CandidateID: current.ID, Reason: req.Reason, Status: domain.AppealPending, CreatedAt: now, UpdatedAt: now}
	store.Write(m.deps.Store, func() struct{} {
		m.deps.Store.Appeals[appeal.ID] = appeal
		return struct{}{}
	})
	httpx.Success(w, http.StatusCreated, appeal, nil)
}

func (m module) handlePendingAppeals(w http.ResponseWriter, r *http.Request) {
	appeals := store.Read(m.deps.Store, func() []*domain.Appeal {
		result := make([]*domain.Appeal, 0)
		for _, appeal := range m.deps.Store.Appeals {
			if appeal.Status == domain.AppealPending {
				result = append(result, appeal)
			}
		}
		return result
	})
	httpx.Success(w, http.StatusOK, appeals, &httpx.Meta{Total: len(appeals)})
}

func (m module) handleResolveAppeal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "id is required", nil)
		return
	}
	var req appealResolveRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid appeal resolution payload", err.Error())
		return
	}
	current, _ := security.CurrentUser(r)
	updated := store.Write(m.deps.Store, func() *domain.Appeal {
		appeal, exists := m.deps.Store.Appeals[id]
		if !exists {
			return nil
		}
		now := time.Now().UTC()
		appeal.Status = req.Status
		appeal.Resolution = req.Resolution
		appeal.ResolvedBy = current.ID
		appeal.UpdatedAt = now
		if exam, exists := m.deps.Store.Exams[appeal.ExamID]; exists {
			exam.FinalizedAt = &now
			exam.UpdatedAt = now
			if req.Status == domain.AppealAccepted {
				exam.Status = domain.ExamFinalized
				exam.Score = maxFloat(exam.Score, 75)
				exam.Telemetry.Health = "appeal_accepted"
			} else {
				exam.Status = domain.ExamFinalized
				exam.Telemetry.Health = "appeal_rejected"
			}
			if exam.ResultOverlayURL == "" {
				exam.ResultOverlayURL = fmt.Sprintf("https://storage.local/exams/%s/overlay.mp4", exam.ID)
			}
		}
		return appeal
	})
	if updated == nil {
		httpx.Failure(w, http.StatusNotFound, "APPEAL_NOT_FOUND", "appeal not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, updated, nil)
}

func (m module) handleGlobalAnalytics(w http.ResponseWriter, r *http.Request) {
	total := 0
	passed := 0
	failurePoints := map[string]int{}
	store.Read(m.deps.Store, func() struct{} {
		for _, exam := range m.deps.Store.Exams {
			if exam.Status == domain.ExamFinalized || exam.Status == domain.ExamCompleted || exam.Status == domain.ExamStopped || exam.Status == domain.ExamReviewRequired {
				total++
				if exam.Score >= 70 {
					passed++
				}
				for _, violation := range exam.Violations {
					failurePoints[violation.Code]++
				}
			}
		}
		return struct{}{}
	})
	common := sortCounts(failurePoints)
	passRate := 0.0
	if total > 0 {
		passRate = float64(passed) / float64(total) * 100
	}
	httpx.Success(w, http.StatusOK, map[string]any{"total_exams": total, "passed": passed, "failed": total - passed, "pass_rate": passRate, "common_failure_points": common}, nil)
}

func (m module) handleHeatmap(w http.ResponseWriter, r *http.Request) {
	counts := map[string]int{}
	store.Read(m.deps.Store, func() struct{} {
		for _, exam := range m.deps.Store.Exams {
			for _, violation := range exam.Violations {
				key := violation.Track
				if key == "" {
					key = violation.Code
				}
				counts[key]++
			}
		}
		return struct{}{}
	})
	httpx.Success(w, http.StatusOK, map[string]any{"heatmap": sortCounts(counts)}, nil)
}

func (m module) readExam(r *http.Request) (*domain.Exam, *domain.User, bool) {
	current, ok := security.CurrentUser(r)
	if !ok {
		return nil, nil, false
	}
	id := chi.URLParam(r, "id")
	exam, exists := m.deps.Store.FindExam(id)
	if !exists {
		return nil, current, false
	}
	return exam, current, true
}

func (m module) readExamForViewer(w http.ResponseWriter, r *http.Request, examinerOnly bool) (*domain.Exam, bool) {
	exam, current, ok := m.readExam(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authenticated user", nil)
		return nil, false
	}
	if current.Role == domain.RoleCandidate && exam.CandidateID != current.ID {
		httpx.Failure(w, http.StatusForbidden, "FORBIDDEN", "candidates can only access their own exam data", nil)
		return nil, false
	}
	if examinerOnly && current.Role == domain.RoleCandidate {
		httpx.Failure(w, http.StatusForbidden, "FORBIDDEN", "examiner or authority access required", nil)
		return nil, false
	}
	return exam, true
}

func sortCounts(values map[string]int) []map[string]any {
	type kv struct {
		Key   string
		Value int
	}
	items := make([]kv, 0, len(values))
	for key, value := range values {
		items = append(items, kv{Key: key, Value: value})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Value == items[j].Value {
			return items[i].Key < items[j].Key
		}
		return items[i].Value > items[j].Value
	})
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{"key": item.Key, "count": item.Value})
	}
	return result
}

func generateOTPCode() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func maxFloat(current, threshold float64) float64 {
	if current > threshold {
		return current
	}
	return threshold
}

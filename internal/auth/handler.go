package auth

import (
	"net/http"
	"strings"
	"time"

	"adlts/internal/platform/domain"
	"adlts/internal/platform/httpx"
	"adlts/internal/platform/runtime"
	"adlts/internal/platform/security"
	"adlts/internal/platform/store"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	deps runtime.Dependencies
}

func New(deps runtime.Dependencies) Handler {
	return Handler{deps: deps}
}

func RegisterAuthRoutes(r chi.Router, deps runtime.Dependencies) {
	h := New(deps)
	authMW := security.Authenticate(deps.Tokens, deps.Store)
	r.Post("/register", h.handleRegisterCandidate)
	r.Post("/register/candidate", h.handleRegisterCandidate)
	r.Post("/register/institute", h.handleRegisterInstitute)
	r.Post("/login", h.handleLogin)
	r.With(authMW, security.RequireRoles(domain.RoleAuthority, domain.RoleAdmin)).Post("/invite", h.handleInvite)
	r.With(authMW).Post("/request-otp", h.handleRequestOTP)
	r.With(authMW).Post("/verify-otp", h.handleVerifyOTP)
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

func (h Handler) handleInvite(w http.ResponseWriter, r *http.Request) {
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
	store.Write(h.deps.Store, func() struct{} {
		h.deps.Store.Invitations[invitation.Token] = invitation
		return struct{}{}
	})
	httpx.Success(w, http.StatusCreated, invitation, nil)
}

func (h Handler) handleRegisterCandidate(w http.ResponseWriter, r *http.Request) {
	var req registerCandidateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid candidate registration payload", err.Error())
		return
	}
	if req.Name == "" || req.Email == "" || req.Password == "" || req.InstituteID == "" {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "name, email, password, and institute_id are required", nil)
		return
	}
	if _, ok := h.deps.Store.FindInstitute(req.InstituteID); !ok {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_INSTITUTE", "selected institute does not exist", nil)
		return
	}
	if _, exists := h.deps.Store.FindUserByEmail(strings.ToLower(strings.TrimSpace(req.Email))); exists {
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
	store.Write(h.deps.Store, func() struct{} {
		h.deps.Store.Users[user.ID] = user
		return struct{}{}
	})
	token, _ := h.deps.Tokens.Sign(user, false)
	httpx.Success(w, http.StatusCreated, map[string]any{"user": user, "token": token, "role": user.Role}, nil)
}

func (h Handler) handleRegisterInstitute(w http.ResponseWriter, r *http.Request) {
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
	store.Read(h.deps.Store, func() struct{} {
		invitation, inviteFound = h.deps.Store.Invitations[req.Token]
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
	if _, exists := h.deps.Store.FindUserByEmail(strings.ToLower(strings.TrimSpace(req.AdminEmail))); exists {
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
	store.Write(h.deps.Store, func() struct{} {
		h.deps.Store.Institutes[institute.ID] = institute
		h.deps.Store.Users[admin.ID] = admin
		invitation.Used = true
		return struct{}{}
	})
	token, _ := h.deps.Tokens.Sign(admin, false)
	httpx.Success(w, http.StatusCreated, map[string]any{"institute": institute, "admin": admin, "token": token, "role": admin.Role}, nil)
}

func (h Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid login payload", err.Error())
		return
	}
	user, ok := h.deps.Store.FindUserByEmail(strings.ToLower(strings.TrimSpace(req.Email)))
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
	token, err := h.deps.Tokens.Sign(user, !otpRequired)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "TOKEN_SIGN_FAILED", "unable to issue token", err.Error())
		return
	}
	httpx.Success(w, http.StatusOK, map[string]any{"token": token, "role": user.Role, "otp_required": otpRequired, "user": user}, nil)
}

func (h Handler) handleRequestOTP(w http.ResponseWriter, r *http.Request) {
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
	store.Write(h.deps.Store, func() struct{} {
		current.OTPCode = code
		current.OTPVerifiedAt = nil
		current.UpdatedAt = now
		h.deps.Store.OTPHistory[current.ID] = now
		return struct{}{}
	})
	httpx.Success(w, http.StatusOK, map[string]any{"otp_required": true, "otp_code": code, "expires_in_seconds": int(otpValidityWindow.Seconds()), "user": current}, nil)
}

func (h Handler) handleVerifyOTP(w http.ResponseWriter, r *http.Request) {
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
		found, exists := h.deps.Store.FindUserByEmail(strings.ToLower(strings.TrimSpace(req.Email)))
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
	issuedAt, issued := h.deps.Store.OTPHistory[user.ID]
	if !issued || now.Sub(issuedAt) > otpValidityWindow {
		httpx.Failure(w, http.StatusUnauthorized, "OTP_EXPIRED", "otp code has expired", nil)
		return
	}
	store.Write(h.deps.Store, func() struct{} {
		user.OTPVerifiedAt = &now
		user.UpdatedAt = now
		h.deps.Store.OTPHistory[user.ID] = now
		return struct{}{}
	})
	token, _ := h.deps.Tokens.Sign(user, true)
	httpx.Success(w, http.StatusOK, map[string]any{"token": token, "otp_verified": true, "user": user}, nil)
}

func generateOTPCode() (string, error) {
	// Use store.NewID to get a stable unique string and return last 6 chars as a simple OTP for now.
	id := store.NewID()
	if len(id) <= 6 {
		return id, nil
	}
	return id[len(id)-6:], nil
}

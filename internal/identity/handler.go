package identity

import (
	"errors"
	"net/http"
	"os"
	"strconv"

	"adlts/internal/domain"
	"adlts/internal/platform/httpx"
	"adlts/internal/platform/media"
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

// ─── Auth ─────────────────────────────────────────────────────────────────────

func (h *Handler) registerCandidate(w http.ResponseWriter, r *http.Request) {
	var req RegisterCandidateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed or has invalid fields", nil)
		return
	}
	if err := h.svc.RegisterCandidate(r.Context(), req); err != nil {
		switch {
		case errors.Is(err, ErrEmailTaken):
			httpx.Failure(w, http.StatusConflict, "EMAIL_TAKEN", err.Error(), nil)
		case errors.Is(err, ErrFayidaIDTaken):
			httpx.Failure(w, http.StatusConflict, "FAYIDA_ID_TAKEN", err.Error(), nil)
		case errors.Is(err, ErrPasswordTooShort):
			httpx.Failure(w, http.StatusUnprocessableEntity, "WEAK_PASSWORD", err.Error(), nil)
		default:
			httpx.Failure(w, http.StatusUnprocessableEntity, "REGISTRATION_FAILED", err.Error(), nil)
		}
		return
	}
	httpx.Success(w, http.StatusCreated, map[string]string{"message": "OTP sent to your email address"}, nil)
}

func (h *Handler) verifyOTP(w http.ResponseWriter, r *http.Request) {
	var req VerifyOTPRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	resp, err := h.svc.VerifyOTP(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidOTP):
			httpx.Failure(w, http.StatusBadRequest, "INVALID_OTP", "invalid or expired verification code", nil)
		default:
			httpx.Failure(w, http.StatusInternalServerError, "VERIFY_FAILED", "verification failed", nil)
		}
		return
	}
	httpx.Success(w, http.StatusOK, resp, nil)
}

func (h *Handler) resendOTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil || body.Email == "" {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "email is required", nil)
		return
	}
	_ = h.svc.ResendOTP(r.Context(), body.Email) // always 200 — no email enumeration
	httpx.Success(w, http.StatusOK, map[string]string{"message": "if that account exists and is unverified, a new code was sent"}, nil)
}

func (h *Handler) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	var req AcceptInvitationRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	resp, err := h.svc.AcceptInvitation(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInviteToken):
			httpx.Failure(w, http.StatusBadRequest, "INVALID_TOKEN", err.Error(), nil)
		case errors.Is(err, ErrInviteAlreadyUsed):
			httpx.Failure(w, http.StatusConflict, "TOKEN_USED", err.Error(), nil)
		case errors.Is(err, ErrInviteExpired):
			httpx.Failure(w, http.StatusGone, "TOKEN_EXPIRED", err.Error(), nil)
		case errors.Is(err, ErrPasswordTooShort):
			httpx.Failure(w, http.StatusUnprocessableEntity, "WEAK_PASSWORD", err.Error(), nil)
		default:
			httpx.Failure(w, http.StatusUnprocessableEntity, "ACCEPT_FAILED", err.Error(), nil)
		}
		return
	}
	httpx.Success(w, http.StatusCreated, resp, nil)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	resp, err := h.svc.Login(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrPendingVerification):
			httpx.Failure(w, http.StatusForbidden, "PENDING_VERIFICATION", "check your email for a verification code", nil)
		case errors.Is(err, ErrPendingApproval):
			httpx.Failure(w, http.StatusForbidden, "PENDING_APPROVAL", "your account is awaiting admin approval", nil)
		case errors.Is(err, ErrAccountSuspended):
			httpx.Failure(w, http.StatusForbidden, "ACCOUNT_SUSPENDED", "your account has been suspended", nil)
		case errors.Is(err, ErrAccountInactive):
			httpx.Failure(w, http.StatusForbidden, "ACCOUNT_INACTIVE", "your account is deactivated", nil)
		default:
			// Always return the same message for invalid credentials — no user enumeration
			httpx.Failure(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password", nil)
		}
		return
	}
	httpx.Success(w, http.StatusOK, resp, nil)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	// JWT is stateless. Client discards token. Token blacklisting can be added via Redis.
	httpx.Success(w, http.StatusOK, map[string]string{"message": "logged out successfully"}, nil)
}

func (h *Handler) refreshToken(w http.ResponseWriter, r *http.Request) {
	// Refresh token rotation is a future enhancement — return 501 until implemented.
	httpx.Failure(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "token refresh is not yet available", nil)
}

func (h *Handler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := httpx.DecodeJSON(r, &req); err != nil || req.Email == "" {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "email is required", nil)
		return
	}
	_ = h.svc.ForgotPassword(r.Context(), req.Email) // always 200 — no email enumeration
	httpx.Success(w, http.StatusOK, map[string]string{"message": "if that email is registered, a reset link has been sent"}, nil)
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	if err := h.svc.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
		switch {
		case errors.Is(err, ErrPasswordTooShort):
			httpx.Failure(w, http.StatusUnprocessableEntity, "WEAK_PASSWORD", err.Error(), nil)
		default:
			httpx.Failure(w, http.StatusBadRequest, "INVALID_RESET_TOKEN", "token is invalid or has expired", nil)
		}
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "password updated successfully"}, nil)
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	auth, ok := security.CurrentAuth(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required", nil)
		return
	}
	var req ChangePasswordRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	if err := h.svc.ChangePassword(r.Context(), auth, req); err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			httpx.Failure(w, http.StatusUnauthorized, "WRONG_PASSWORD", "current password is incorrect", nil)
		case errors.Is(err, ErrPasswordTooShort):
			httpx.Failure(w, http.StatusUnprocessableEntity, "WEAK_PASSWORD", err.Error(), nil)
		case errors.Is(err, ErrSamePassword):
			httpx.Failure(w, http.StatusUnprocessableEntity, "SAME_PASSWORD", err.Error(), nil)
		default:
			httpx.Failure(w, http.StatusInternalServerError, "CHANGE_FAILED", "could not change password", nil)
		}
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "password changed successfully"}, nil)
}

// ─── Candidates ───────────────────────────────────────────────────────────────

func (h *Handler) listCandidates(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := pageParam(q.Get("page"))
	candidates, total, err := h.svc.repo.ListCandidates(r.Context(), q.Get("search"), q.Get("status"), page)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "could not fetch candidates", nil)
		return
	}
	out := make([]CandidateResponse, len(candidates))
	for i, c := range candidates {
		out[i] = candidateToResponse(c)
	}
	httpx.Success(w, http.StatusOK, out, &httpx.Meta{Page: page, Total: total, Limit: 20})
}

func (h *Handler) candidateMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	c, err := h.svc.repo.CandidateByID(r.Context(), auth.SubjectID)
	if err != nil || c == nil {
		httpx.Failure(w, http.StatusNotFound, "NOT_FOUND", "candidate not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, candidateToResponse(c), nil)
}

func (h *Handler) updateCandidateMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	var req UpdateCandidateSelfRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	if err := h.svc.repo.UpdateCandidateFields(r.Context(), auth.SubjectID, candidateSelfFields(req), auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "could not update profile", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "profile updated"}, nil)
}

func (h *Handler) softDeleteCandidateMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	if err := h.svc.repo.UpdateCandidateStatus(r.Context(), auth.SubjectID, domain.UserStatusInactive, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DELETE_FAILED", "could not process request", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "account deactivation request submitted"}, nil)
}

func (h *Handler) getCandidate(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	c, err := h.svc.repo.CandidateByID(r.Context(), id)
	if err != nil || c == nil {
		httpx.Failure(w, http.StatusNotFound, "NOT_FOUND", "candidate not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, candidateToResponse(c), nil)
}

func (h *Handler) updateCandidateAdmin(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	var req UpdateCandidateSelfRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	if err := h.svc.repo.UpdateCandidateFields(r.Context(), id, candidateSelfFields(req), auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "could not update candidate", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "candidate updated"}, nil)
}

func (h *Handler) updateCandidateStatus(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	var req StatusRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	status, ok := parseUserStatus(req.Status)
	if !ok {
		httpx.Failure(w, http.StatusUnprocessableEntity, "INVALID_STATUS", "status must be active, inactive, or suspended", nil)
		return
	}
	if err := h.svc.repo.UpdateCandidateStatus(r.Context(), id, status, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "could not update status", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "status updated"}, nil)
}

func (h *Handler) deleteCandidate(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	if err := h.svc.repo.UpdateCandidateStatus(r.Context(), id, domain.UserStatusInactive, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DELETE_FAILED", "could not delete candidate", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "candidate deactivated"}, nil)
}

// ─── Experts ──────────────────────────────────────────────────────────────────

func (h *Handler) listExperts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := pageParam(q.Get("page"))
	experts, total, err := h.svc.repo.ListExperts(r.Context(), q.Get("search"), q.Get("status"), page)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "could not fetch experts", nil)
		return
	}
	out := make([]ExpertResponse, len(experts))
	for i, e := range experts {
		out[i] = expertToResponse(e)
	}
	httpx.Success(w, http.StatusOK, out, &httpx.Meta{Page: page, Total: total, Limit: 20})
}

func (h *Handler) expertMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	e, err := h.svc.repo.ExpertByID(r.Context(), auth.SubjectID)
	if err != nil || e == nil {
		httpx.Failure(w, http.StatusNotFound, "NOT_FOUND", "expert not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, expertToResponse(e), nil)
}

func (h *Handler) updateExpertMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	var req UpdateExpertSelfRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	if err := h.svc.repo.UpdateExpertFields(r.Context(), auth.SubjectID, expertSelfFields(req), auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "could not update profile", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "profile updated"}, nil)
}

func (h *Handler) getExpert(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	e, err := h.svc.repo.ExpertByID(r.Context(), id)
	if err != nil || e == nil {
		httpx.Failure(w, http.StatusNotFound, "NOT_FOUND", "expert not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, expertToResponse(e), nil)
}

func (h *Handler) updateExpertAdmin(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	var req UpdateExpertSelfRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	if err := h.svc.repo.UpdateExpertFields(r.Context(), id, expertSelfFields(req), auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "could not update expert", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "expert updated"}, nil)
}

func (h *Handler) updateExpertStatus(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	var req StatusRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	status, ok := parseUserStatus(req.Status)
	if !ok {
		httpx.Failure(w, http.StatusUnprocessableEntity, "INVALID_STATUS", "status must be active, inactive, or suspended", nil)
		return
	}
	if err := h.svc.repo.UpdateExpertStatus(r.Context(), id, status, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "could not update status", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "status updated"}, nil)
}

func (h *Handler) deleteExpert(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	if err := h.svc.repo.DeleteExpert(r.Context(), id); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DELETE_FAILED", "could not delete expert", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "expert deleted"}, nil)
}

// ─── Institutes ───────────────────────────────────────────────────────────────

func (h *Handler) listInstitutes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := pageParam(q.Get("page"))
	insts, total, err := h.svc.repo.ListInstitutes(r.Context(), q.Get("search"), q.Get("status"), page)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "could not fetch institutes", nil)
		return
	}
	out := make([]InstituteResponse, len(insts))
	for i, inst := range insts {
		out[i] = instituteToResponse(inst)
	}
	httpx.Success(w, http.StatusOK, out, &httpx.Meta{Page: page, Total: total, Limit: 20})
}

func (h *Handler) listActiveInstitutesForCandidates(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := pageParam(q.Get("page"))
	limit := httpx.QueryInt(q, "limit", 20)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	insts, total, err := h.svc.repo.ListActiveInstitutes(r.Context(), page, limit)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "could not fetch active institutes", nil)
		return
	}

	out := make([]ActiveInstituteResponse, 0, len(insts))
	for _, inst := range insts {
		out = append(out, ActiveInstituteResponse{
			ID:     inst.ID,
			Name:   inst.Name,
			Status: string(inst.Status),
			City:   inst.City,
			Region: inst.Region,
		})
	}
	httpx.Success(w, http.StatusOK, out, &httpx.Meta{Page: page, Limit: limit, Total: total})
}

func (h *Handler) instituteMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	inst, err := h.svc.repo.InstituteByID(r.Context(), auth.SubjectID)
	if err != nil || inst == nil {
		httpx.Failure(w, http.StatusNotFound, "NOT_FOUND", "institute not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, instituteToResponse(inst), nil)
}

func (h *Handler) updateInstituteMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	var req UpdateInstituteSelfRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	if err := h.svc.repo.UpdateInstituteFields(r.Context(), auth.SubjectID, instituteSelfFields(req), auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "could not update profile", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "profile updated"}, nil)
}

func (h *Handler) softDeleteInstituteMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	if err := h.svc.repo.UpdateInstituteStatus(r.Context(), auth.SubjectID, domain.OrgStatusInactive, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DELETE_FAILED", "could not process request", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "account deactivation request submitted"}, nil)
}

func (h *Handler) getInstitute(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	inst, err := h.svc.repo.InstituteByID(r.Context(), id)
	if err != nil || inst == nil {
		httpx.Failure(w, http.StatusNotFound, "NOT_FOUND", "institute not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, instituteToResponse(inst), nil)
}

func (h *Handler) updateInstituteAdmin(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	var req UpdateInstituteSelfRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	if err := h.svc.repo.UpdateInstituteFields(r.Context(), id, instituteSelfFields(req), auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "could not update institute", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "institute updated"}, nil)
}

func (h *Handler) updateInstituteStatus(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	var req StatusRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	status, ok := parseOrgStatus(req.Status)
	if !ok {
		httpx.Failure(w, http.StatusUnprocessableEntity, "INVALID_STATUS", "status must be active, inactive, suspended, or pending_approval", nil)
		return
	}
	if err := h.svc.repo.UpdateInstituteStatus(r.Context(), id, status, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "could not update status", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "status updated"}, nil)
}

func (h *Handler) deleteInstitute(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	if err := h.svc.repo.UpdateInstituteStatus(r.Context(), id, domain.OrgStatusInactive, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DELETE_FAILED", "could not delete institute", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "institute deactivated"}, nil)
}

// ─── Transport Authorities ────────────────────────────────────────────────────

func (h *Handler) listAuthorities(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := pageParam(q.Get("page"))
	auths, total, err := h.svc.repo.ListAuthorities(r.Context(), q.Get("search"), q.Get("status"), page)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "could not fetch authorities", nil)
		return
	}
	out := make([]AuthorityResponse, len(auths))
	for i, a := range auths {
		out[i] = authorityToResponse(a)
	}
	httpx.Success(w, http.StatusOK, out, &httpx.Meta{Page: page, Total: total, Limit: 20})
}

func (h *Handler) authorityMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	a, err := h.svc.repo.AuthorityByID(r.Context(), auth.SubjectID)
	if err != nil || a == nil {
		httpx.Failure(w, http.StatusNotFound, "NOT_FOUND", "transport authority not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, authorityToResponse(a), nil)
}

func (h *Handler) updateAuthorityMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	var req UpdateAuthoritySelfRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	if err := h.svc.repo.UpdateAuthorityFields(r.Context(), auth.SubjectID, authoritySelfFields(req), auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "could not update profile", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "profile updated"}, nil)
}

func (h *Handler) getAuthority(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	a, err := h.svc.repo.AuthorityByID(r.Context(), id)
	if err != nil || a == nil {
		httpx.Failure(w, http.StatusNotFound, "NOT_FOUND", "transport authority not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, authorityToResponse(a), nil)
}

func (h *Handler) updateAuthorityAdmin(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	var req UpdateAuthoritySelfRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	if err := h.svc.repo.UpdateAuthorityFields(r.Context(), id, authoritySelfFields(req), auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "could not update authority", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "authority updated"}, nil)
}

func (h *Handler) updateAuthorityStatus(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	var req StatusRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	status, ok := parseOrgStatus(req.Status)
	if !ok {
		httpx.Failure(w, http.StatusUnprocessableEntity, "INVALID_STATUS", "invalid status value", nil)
		return
	}
	if err := h.svc.repo.UpdateAuthorityFields(r.Context(), id, map[string]any{"status": status}, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "could not update status", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "status updated"}, nil)
}

func (h *Handler) deleteAuthority(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	if err := h.svc.repo.UpdateAuthorityFields(r.Context(), id, map[string]any{"status": domain.OrgStatusInactive}, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DELETE_FAILED", "could not delete authority", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "authority deactivated"}, nil)
}

// ─── Admins ───────────────────────────────────────────────────────────────────

func (h *Handler) listAdmins(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := pageParam(q.Get("page"))
	var centerID *uuid.UUID
	if raw := q.Get("test_center_id"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			centerID = &id
		}
	}
	admins, total, err := h.svc.repo.ListAdmins(r.Context(), q.Get("search"), q.Get("status"), centerID, page)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "could not fetch admins", nil)
		return
	}
	out := make([]AdminResponse, len(admins))
	for i, a := range admins {
		out[i] = adminToResponse(a)
	}
	httpx.Success(w, http.StatusOK, out, &httpx.Meta{Page: page, Total: total, Limit: 20})
}

func (h *Handler) adminMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	a, err := h.svc.repo.AdminByID(r.Context(), auth.SubjectID)
	if err != nil || a == nil {
		httpx.Failure(w, http.StatusNotFound, "NOT_FOUND", "admin not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, adminToResponse(a), nil)
}

func (h *Handler) updateAdminMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	var req UpdateAdminSelfRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	if err := h.svc.repo.UpdateAdminFields(r.Context(), auth.SubjectID, adminSelfFields(req), auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "could not update profile", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "profile updated"}, nil)
}

func (h *Handler) getAdmin(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	a, err := h.svc.repo.AdminByID(r.Context(), id)
	if err != nil || a == nil {
		httpx.Failure(w, http.StatusNotFound, "NOT_FOUND", "admin not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, adminToResponse(a), nil)
}

func (h *Handler) updateAdminAdmin(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	var req UpdateAdminSelfRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	if err := h.svc.repo.UpdateAdminFields(r.Context(), id, adminSelfFields(req), auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "could not update admin", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "admin updated"}, nil)
}

func (h *Handler) updateAdminStatus(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	var req StatusRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	status, ok := parseUserStatus(req.Status)
	if !ok {
		httpx.Failure(w, http.StatusUnprocessableEntity, "INVALID_STATUS", "status must be active, inactive, or suspended", nil)
		return
	}
	if err := h.svc.repo.UpdateAdminStatus(r.Context(), id, status, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "could not update status", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "status updated"}, nil)
}

func (h *Handler) deleteAdmin(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	if err := h.svc.repo.DeleteAdmin(r.Context(), id); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DELETE_FAILED", "could not delete admin", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "admin deleted"}, nil)
}

// ─── SuperAdmins ──────────────────────────────────────────────────────────────

func (h *Handler) listSuperAdmins(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := pageParam(q.Get("page"))
	admins, total, err := h.svc.repo.ListSuperAdmins(r.Context(), q.Get("search"), page)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "could not fetch super admins", nil)
		return
	}
	out := make([]SuperAdminResponse, len(admins))
	for i, sa := range admins {
		out[i] = superAdminToResponse(sa)
	}
	httpx.Success(w, http.StatusOK, out, &httpx.Meta{Page: page, Total: total, Limit: 20})
}

func (h *Handler) superAdminMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	sa, err := h.svc.repo.SuperAdminByID(r.Context(), auth.SubjectID)
	if err != nil || sa == nil {
		httpx.Failure(w, http.StatusNotFound, "NOT_FOUND", "super admin not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, superAdminToResponse(sa), nil)
}

func (h *Handler) updateSuperAdminMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	var req UpdateSuperAdminSelfRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	fields := map[string]any{}
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if err := h.svc.repo.UpdateSuperAdminFields(r.Context(), auth.SubjectID, fields, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "could not update profile", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "profile updated"}, nil)
}

func (h *Handler) getSuperAdmin(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	sa, err := h.svc.repo.SuperAdminByID(r.Context(), id)
	if err != nil || sa == nil {
		httpx.Failure(w, http.StatusNotFound, "NOT_FOUND", "super admin not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, superAdminToResponse(sa), nil)
}

func (h *Handler) updateSuperAdminAdmin(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	var req UpdateSuperAdminSelfRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	fields := map[string]any{}
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if err := h.svc.repo.UpdateSuperAdminFields(r.Context(), id, fields, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "could not update super admin", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "super admin updated"}, nil)
}

func (h *Handler) deleteSuperAdmin(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	if id == auth.SubjectID {
		httpx.Failure(w, http.StatusForbidden, "SELF_DELETE", "you cannot delete your own super admin account", nil)
		return
	}

	target, preErr := h.svc.repo.SuperAdminByID(r.Context(), id)
	if preErr == nil && target != nil {
		rootEmail := os.Getenv("SUPER_ADMIN_EMAIL")
		if rootEmail == "" {
			rootEmail = "root@adlts.et" // Fallback matching config.go
		}
		if target.Email == rootEmail {
			httpx.Failure(w, http.StatusForbidden, "ROOT_PROTECTED", "the system root super admin cannot be deleted", nil)
			return
		}
	}
	if err := h.svc.repo.DeleteSuperAdmin(r.Context(), id); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DELETE_FAILED", "could not delete super admin", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "super admin deleted"}, nil)
}

// ─── Invitations ──────────────────────────────────────────────────────────────

func (h *Handler) createInvitation(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	var req CreateInvitationRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body is malformed", nil)
		return
	}
	inv, err := h.svc.CreateInvitation(r.Context(), auth, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrForbiddenInviteRole):
			httpx.Failure(w, http.StatusForbidden, "FORBIDDEN_ROLE", err.Error(), nil)
		default:
			httpx.Failure(w, http.StatusUnprocessableEntity, "INVITATION_FAILED", err.Error(), nil)
		}
		return
	}
	httpx.Success(w, http.StatusCreated, invitationToResponse(inv), nil)
}

func (h *Handler) listInvitations(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	q := r.URL.Query()
	page := pageParam(q.Get("page"))
	var centerID *uuid.UUID
	if auth.EntityType == security.EntityAdmin && auth.TestCenterID != nil {
		centerID = auth.TestCenterID
	}
	invitations, total, err := h.svc.repo.ListInvitations(r.Context(), q.Get("status"), centerID, page)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "could not fetch invitations", nil)
		return
	}
	out := make([]InvitationResponse, len(invitations))
	for i, inv := range invitations {
		out[i] = invitationToResponse(inv)
	}
	httpx.Success(w, http.StatusOK, out, &httpx.Meta{Page: page, Total: total, Limit: 20})
}

func (h *Handler) getInvitation(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	inv, err := h.svc.GetInvitation(r.Context(), auth, id)
	if err != nil {
		httpx.Failure(w, http.StatusNotFound, "NOT_FOUND", "invitation not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, invitationToResponse(inv), nil)
}

func (h *Handler) resendInvitation(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	if err := h.svc.ResendInvitation(r.Context(), auth, id); err != nil {
		switch {
		case errors.Is(err, ErrInviteAlreadyUsed):
			httpx.Failure(w, http.StatusConflict, "ALREADY_USED", err.Error(), nil)
		case errors.Is(err, ErrNotFound):
			httpx.Failure(w, http.StatusNotFound, "NOT_FOUND", "invitation not found", nil)
		default:
			httpx.Failure(w, http.StatusInternalServerError, "RESEND_FAILED", err.Error(), nil)
		}
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "invitation resent"}, nil)
}

func (h *Handler) cancelInvitation(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	if err := h.svc.CancelInvitation(r.Context(), auth, id); err != nil {
		switch {
		case errors.Is(err, ErrInviteAlreadyUsed):
			httpx.Failure(w, http.StatusConflict, "ALREADY_USED", "cannot cancel used invitation", nil)
		case errors.Is(err, ErrNotFound):
			httpx.Failure(w, http.StatusNotFound, "NOT_FOUND", "invitation not found", nil)
		default:
			httpx.Failure(w, http.StatusInternalServerError, "CANCEL_FAILED", "could not cancel invitation", nil)
		}
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "invitation cancelled"}, nil)
}

// ─── Converters ───────────────────────────────────────────────────────────────

func candidateToResponse(c *domain.Candidate) CandidateResponse {
	return CandidateResponse{
		ID: c.ID, FirstName: c.FirstName, MiddleName: c.MiddleName, LastName: c.LastName,
		Email: c.Email, Phone: c.Phone, FayidaID: c.FayidaID, BirthDate: c.BirthDate,
		Gender: string(c.Gender), PhotoURL: c.PhotoURL, Status: string(c.Status),
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

func expertToResponse(e *domain.Expert) ExpertResponse {
	return ExpertResponse{
		ID: e.ID, FirstName: e.FirstName, MiddleName: e.MiddleName, LastName: e.LastName,
		Email: e.Email, Phone: e.Phone, EmployeeID: e.EmployeeID,
		Status: string(e.Status), CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func instituteToResponse(inst *domain.Institute) InstituteResponse {
	return InstituteResponse{
		ID: inst.ID, Name: inst.Name, Email: inst.Email, Phone: inst.Phone,
		LogoURL: inst.LogoURL, Status: string(inst.Status),
		Address: AddressDTO{Street: inst.Address.Street, City: inst.Address.City,
			Region: inst.Address.Region, Country: inst.Address.Country},
		CreatedAt: inst.CreatedAt, UpdatedAt: inst.UpdatedAt,
	}
}

func authorityToResponse(a *domain.TransportAuthority) AuthorityResponse {
	return AuthorityResponse{
		ID: a.ID, Name: a.Name, Email: a.Email, Phone: a.Phone, Status: string(a.Status),
		Address: AddressDTO{Street: a.Address.Street, City: a.Address.City,
			Region: a.Address.Region, Country: a.Address.Country},
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

func adminToResponse(a *domain.Admin) AdminResponse {
	return AdminResponse{
		ID: a.ID, FirstName: a.FirstName, MiddleName: a.MiddleName, LastName: a.LastName,
		Email: a.Email, TestCenterID: a.TestCenterID, Status: string(a.Status),
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

func superAdminToResponse(sa *domain.SuperAdmin) SuperAdminResponse {
	return SuperAdminResponse{ID: sa.ID, Name: sa.Name, Email: sa.Email,
		CreatedAt: sa.CreatedAt, UpdatedAt: sa.UpdatedAt}
}

func invitationToResponse(inv *domain.Invitation) InvitationResponse {
	return InvitationResponse{ID: inv.ID, Email: inv.Email, EntityType: inv.EntityType,
		ExpiresAt: inv.ExpiresAt, UsedAt: inv.UsedAt, CreatedAt: inv.CreatedAt}
}

// ─── Field pickers ────────────────────────────────────────────────────────────

func candidateSelfFields(req UpdateCandidateSelfRequest) map[string]any {
	f := map[string]any{}
	if req.FirstName != nil {
		f["first_name"] = *req.FirstName
	}
	if req.MiddleName != nil {
		f["middle_name"] = *req.MiddleName
	}
	if req.LastName != nil {
		f["last_name"] = *req.LastName
	}
	if req.Phone != nil {
		f["phone"] = *req.Phone
	}
	if req.PhotoURL != nil {
		f["photo_url"] = *req.PhotoURL
	}
	return f
}

func expertSelfFields(req UpdateExpertSelfRequest) map[string]any {
	f := map[string]any{}
	if req.FirstName != nil {
		f["first_name"] = *req.FirstName
	}
	if req.MiddleName != nil {
		f["middle_name"] = *req.MiddleName
	}
	if req.LastName != nil {
		f["last_name"] = *req.LastName
	}
	if req.Phone != nil {
		f["phone"] = *req.Phone
	}
	if req.PhotoURL != nil {
		f["photo_url"] = *req.PhotoURL
	}
	return f
}

func instituteSelfFields(req UpdateInstituteSelfRequest) map[string]any {
	f := map[string]any{}
	if req.Name != nil {
		f["name"] = *req.Name
	}
	if req.Phone != nil {
		f["phone"] = *req.Phone
	}
	if req.LogoURL != nil {
		f["logo_url"] = *req.LogoURL
	}
	if req.Address != nil {
		f["street"] = req.Address.Street
		f["city"] = req.Address.City
		f["region"] = req.Address.Region
		f["country"] = req.Address.Country
	}
	return f
}

func authoritySelfFields(req UpdateAuthoritySelfRequest) map[string]any {
	f := map[string]any{}
	if req.Name != nil {
		f["name"] = *req.Name
	}
	if req.Phone != nil {
		f["phone"] = *req.Phone
	}
	if req.Address != nil {
		f["street"] = req.Address.Street
		f["city"] = req.Address.City
		f["region"] = req.Address.Region
		f["country"] = req.Address.Country
	}
	return f
}

func adminSelfFields(req UpdateAdminSelfRequest) map[string]any {
	f := map[string]any{}
	if req.FirstName != nil {
		f["first_name"] = *req.FirstName
	}
	if req.MiddleName != nil {
		f["middle_name"] = *req.MiddleName
	}
	if req.LastName != nil {
		f["last_name"] = *req.LastName
	}
	return f
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

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

func parseUserStatus(s string) (domain.UserStatus, bool) {
	switch s {
	case "active":
		return domain.UserStatusActive, true
	case "inactive":
		return domain.UserStatusInactive, true
	case "suspended":
		return domain.UserStatusSuspended, true
	}
	return "", false
}

func parseOrgStatus(s string) (domain.OrgStatus, bool) {
	switch s {
	case "active":
		return domain.OrgStatusActive, true
	case "inactive":
		return domain.OrgStatusInactive, true
	case "suspended":
		return domain.OrgStatusSuspended, true
	case "pending_approval":
		return domain.OrgStatusPendingApproval, true
	}
	return "", false
}

// ─── Media upload helpers ─────────────────────────────────────────────────────

const maxUploadBytes = 10 << 20 // 10 MB multipart parse limit

// uploadPhoto is the shared kernel: validate + save + update DB field.
// category is the subfolder name ("candidates", "experts", …).
// table is the SQL table name for UpdateXFields calls.
func (h *Handler) uploadPhoto(
	w http.ResponseWriter, r *http.Request,
	category, field string,
	updateFn func(path string) error,
	existingPath func() string,
) {
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_FORM", "could not parse multipart form (max 10 MB)", nil)
		return
	}
	_, header, err := r.FormFile("file")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "MISSING_FILE", "form field 'file' is required", nil)
		return
	}

	old := ""
	if existingPath != nil {
		old = existingPath()
	}

	var relPath string
	if old != "" {
		relPath, err = media.Replace(category, header, old)
	} else {
		relPath, err = media.Save(category, header)
	}
	if err != nil {
		switch err.(type) {
		case media.ErrInvalidMIME:
			httpx.Failure(w, http.StatusUnprocessableEntity, "INVALID_FILE_TYPE", err.Error(), nil)
		case media.ErrFileTooLarge:
			httpx.Failure(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", err.Error(), nil)
		default:
			httpx.Failure(w, http.StatusInternalServerError, "UPLOAD_FAILED", "could not save file", nil)
		}
		return
	}

	if err := updateFn(relPath); err != nil {
		// Best-effort rollback
		_ = media.Delete(relPath)
		httpx.Failure(w, http.StatusInternalServerError, "UPDATE_FAILED", "file saved but profile not updated", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{field: relPath}, nil)
}

// ── Candidate photo ───────────────────────────────────────────────────────────

func (h *Handler) uploadCandidatePhotoMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	c, _ := h.svc.repo.CandidateByID(r.Context(), auth.SubjectID)
	old := ""
	if c != nil {
		old = c.PhotoURL
	}
	h.uploadPhoto(w, r, "candidates", "photo_url",
		func(p string) error {
			return h.svc.repo.UpdateCandidateFields(r.Context(), auth.SubjectID, map[string]any{"photo_url": p}, auth.SubjectID)
		},
		func() string { return old },
	)
}

func (h *Handler) uploadCandidatePhotoAdmin(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	c, _ := h.svc.repo.CandidateByID(r.Context(), id)
	old := ""
	if c != nil {
		old = c.PhotoURL
	}
	h.uploadPhoto(w, r, "candidates", "photo_url",
		func(p string) error {
			return h.svc.repo.UpdateCandidateFields(r.Context(), id, map[string]any{"photo_url": p}, auth.SubjectID)
		},
		func() string { return old },
	)
}

// ── Expert photo ──────────────────────────────────────────────────────────────

func (h *Handler) uploadExpertPhotoMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	e, _ := h.svc.repo.ExpertByID(r.Context(), auth.SubjectID)
	old := ""
	if e != nil {
		old = e.PhotoURL
	}
	h.uploadPhoto(w, r, "experts", "photo_url",
		func(p string) error {
			return h.svc.repo.UpdateExpertFields(r.Context(), auth.SubjectID, map[string]any{"photo_url": p}, auth.SubjectID)
		},
		func() string { return old },
	)
}

func (h *Handler) uploadExpertPhotoAdmin(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	e, _ := h.svc.repo.ExpertByID(r.Context(), id)
	old := ""
	if e != nil {
		old = e.PhotoURL
	}
	h.uploadPhoto(w, r, "experts", "photo_url",
		func(p string) error {
			return h.svc.repo.UpdateExpertFields(r.Context(), id, map[string]any{"photo_url": p}, auth.SubjectID)
		},
		func() string { return old },
	)
}

// ── Institute logo ────────────────────────────────────────────────────────────

func (h *Handler) uploadInstituteLogoMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	inst, _ := h.svc.repo.InstituteByID(r.Context(), auth.SubjectID)
	old := ""
	if inst != nil {
		old = inst.LogoURL
	}
	h.uploadPhoto(w, r, "institutes", "logo_url",
		func(p string) error {
			return h.svc.repo.UpdateInstituteFields(r.Context(), auth.SubjectID, map[string]any{"logo_url": p}, auth.SubjectID)
		},
		func() string { return old },
	)
}

func (h *Handler) uploadInstituteLogoAdmin(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	inst, _ := h.svc.repo.InstituteByID(r.Context(), id)
	old := ""
	if inst != nil {
		old = inst.LogoURL
	}
	h.uploadPhoto(w, r, "institutes", "logo_url",
		func(p string) error {
			return h.svc.repo.UpdateInstituteFields(r.Context(), id, map[string]any{"logo_url": p}, auth.SubjectID)
		},
		func() string { return old },
	)
}

// ── Transport Authority logo ──────────────────────────────────────────────────

func (h *Handler) uploadAuthorityLogoMe(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	a, _ := h.svc.repo.AuthorityByID(r.Context(), auth.SubjectID)
	old := ""
	if a != nil {
		old = a.LogoURL
	}
	h.uploadPhoto(w, r, "transport-authorities", "logo_url",
		func(p string) error {
			return h.svc.repo.UpdateAuthorityFields(r.Context(), auth.SubjectID, map[string]any{"logo_url": p}, auth.SubjectID)
		},
		func() string { return old },
	)
}

func (h *Handler) uploadAuthorityLogoAdmin(w http.ResponseWriter, r *http.Request) {
	auth, _ := security.CurrentAuth(r)
	id, err := parseID(r, "id")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	a, _ := h.svc.repo.AuthorityByID(r.Context(), id)
	old := ""
	if a != nil {
		old = a.LogoURL
	}
	h.uploadPhoto(w, r, "transport-authorities", "logo_url",
		func(p string) error {
			return h.svc.repo.UpdateAuthorityFields(r.Context(), id, map[string]any{"logo_url": p}, auth.SubjectID)
		},
		func() string { return old },
	)
}

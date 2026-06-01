package testing

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"adlts/internal/domain"
	"adlts/internal/platform/config"
	"adlts/internal/platform/httpx"
	minioclient "adlts/internal/platform/minio"
	"adlts/internal/platform/security"
	"adlts/internal/testing/dto"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"
)

// Handler holds all dependencies for the testing module HTTP layer.
type Handler struct {
	repo           *Repository
	minio          *minioclient.Client
	cfg            config.Config
	tokens         *security.Manager
	internalAPIKey string
	orchestrator   *Orchestrator // set after startup via SetOrchestrator
}

func NewHandler(repo *Repository, minio *minioclient.Client, cfg config.Config, tokens *security.Manager) *Handler {
	return &Handler{
		repo:           repo,
		minio:          minio,
		cfg:            cfg,
		tokens:         tokens,
		internalAPIKey: cfg.InternalAPIKey,
	}
}

// SetOrchestrator injects the orchestrator after construction (called from app.go).
func (h *Handler) SetOrchestrator(o *Orchestrator) { h.orchestrator = o }

// adminAbortTest lets an admin abort a running test.
func (h *Handler) adminAbortTest(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test ID must be a UUID", nil)
		return
	}
	test, err := h.repo.TestByID(r.Context(), id)
	if err != nil {
		httpx.Failure(w, http.StatusNotFound, "TEST_NOT_FOUND", "test not found", nil)
		return
	}
	if h.orchestrator != nil {
		h.orchestrator.AbortTest(r.Context(), id, test.DeviceID, domain.AbortAdminIntervention)
	} else {
		_ = h.repo.AbortTest(r.Context(), id, domain.AbortAdminIntervention, systemActorID)
		if test.DeviceID != nil {
			_ = h.repo.ReleaseDevice(r.Context(), *test.DeviceID, systemActorID)
		}
	}
	httpx.Success(w, http.StatusOK, map[string]string{"status": "aborted"}, nil)
}

// mustAuth extracts the auth context; writes 401 and returns nil if missing.
func mustAuth(w http.ResponseWriter, r *http.Request) *security.AuthContext {
	auth, ok := security.CurrentAuth(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil)
		return nil
	}
	return auth
}

// tcID safely dereferences the (possibly nil) TestCenterID from the auth context.
func tcID(auth *security.AuthContext) uuid.UUID {
	if auth.TestCenterID == nil {
		return uuid.Nil
	}
	return *auth.TestCenterID
}

// ── Device handlers ───────────────────────────────────────────────────────────

func (h *Handler) registerDevice(w http.ResponseWriter, r *http.Request) {
	var req RegisterDeviceRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "DECODE_ERROR", err.Error(), nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	centerID, err := uuid.Parse(req.TestCenterID)
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_FIELD", "test_center_id must be a valid UUID", nil)
		return
	}
	levelsJSON, _ := json.Marshal(req.AllowedLevels)
	hash, err := security.HashPassword(req.Password)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "HASH_ERROR", "failed to hash password", nil)
		return
	}
	d := &domain.Device{
		ID:            uuid.New(),
		DeviceCode:    req.DeviceCode,
		PasswordHash:  hash,
		TestCenterID:  centerID,
		AllowedLevels: string(levelsJSON),
		StreamURL:     req.StreamURL,
		Status:        domain.DeviceStatusActive,
		Audit: domain.Audit{
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
			CreatedBy: auth.SubjectID, UpdatedBy: auth.SubjectID,
		},
	}
	if err := h.repo.CreateDevice(r.Context(), d); err != nil {
		httpx.Failure(w, http.StatusConflict, "DEVICE_CODE_TAKEN", "device code is already registered", nil)
		return
	}
	resp, _ := toDeviceResponse(d)
	httpx.Success(w, http.StatusCreated, resp, nil)
}

func (h *Handler) listDevices(w http.ResponseWriter, r *http.Request) {
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	page := httpx.QueryInt(r.URL.Query(), "page", 1)
	if page < 1 {
		page = 1
	}
	var filterID *uuid.UUID
	if auth.EntityType != security.EntitySuperAdmin {
		id := tcID(auth)
		if id != uuid.Nil {
			filterID = &id
		}
	}
	devices, total, err := h.repo.ListDevices(r.Context(), filterID, page)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "failed to list devices", nil)
		return
	}
	var out []DeviceResponse
	for _, d := range devices {
		resp, _ := toDeviceResponse(d)
		out = append(out, resp)
	}
	httpx.Success(w, http.StatusOK, out, &httpx.Meta{Total: total, Page: page})
}

func (h *Handler) getDevice(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "device ID must be a UUID", nil)
		return
	}
	d, err := h.repo.DeviceByID(r.Context(), id)
	if err == ErrDeviceNotFound {
		httpx.Failure(w, http.StatusNotFound, "DEVICE_NOT_FOUND", "device not found", nil)
		return
	} else if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	resp, _ := toDeviceResponse(d)
	httpx.Success(w, http.StatusOK, resp, nil)
}

func (h *Handler) updateDevice(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "device ID must be a UUID", nil)
		return
	}
	var req UpdateDeviceRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "DECODE_ERROR", err.Error(), nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	fields := map[string]any{}
	if req.StreamURL != nil {
		fields["stream_url"] = *req.StreamURL
	}
	if len(req.AllowedLevels) > 0 {
		b, _ := json.Marshal(req.AllowedLevels)
		fields["allowed_levels"] = string(b)
	}
	if err := h.repo.UpdateDeviceFields(r.Context(), id, fields, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "device updated"}, nil)
}

func (h *Handler) updateDeviceStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "device ID must be a UUID", nil)
		return
	}
	var req UpdateDeviceStatusRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "DECODE_ERROR", err.Error(), nil)
		return
	}
	allowed := map[string]bool{"active": true, "inactive": true, "maintenance": true}
	if !allowed[req.Status] {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_STATUS", "status must be active, inactive, or maintenance", nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	if err := h.repo.UpdateDeviceFields(r.Context(), id, map[string]any{"status": req.Status}, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"status": req.Status}, nil)
}

func (h *Handler) deleteDevice(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "device ID must be a UUID", nil)
		return
	}
	d, err := h.repo.DeviceByID(r.Context(), id)
	if err == ErrDeviceNotFound {
		httpx.Failure(w, http.StatusNotFound, "DEVICE_NOT_FOUND", "device not found", nil)
		return
	} else if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	if d.Status == domain.DeviceStatusInUse {
		httpx.Failure(w, http.StatusConflict, "DEVICE_IN_USE", "cannot delete a device that is currently running a test", nil)
		return
	}
	if err := h.repo.DeleteDevice(r.Context(), id); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "device deleted"}, nil)
}

func (h *Handler) downloadDeviceQR(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "device ID must be a UUID", nil)
		return
	}
	password := r.URL.Query().Get("password")
	if password == "" {
		httpx.Failure(w, http.StatusBadRequest, "MISSING_PASSWORD", "?password= is required to generate QR", nil)
		return
	}
	d, err := h.repo.DeviceByID(r.Context(), id)
	if err == ErrDeviceNotFound {
		httpx.Failure(w, http.StatusNotFound, "DEVICE_NOT_FOUND", "device not found", nil)
		return
	} else if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	payload := fmt.Sprintf("DLS:DEVICE:%s:%s:%s", d.DeviceCode, password, d.AllowedLevels)
	png, err := qrcode.Encode(payload, qrcode.Medium, 256)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "QR_ERROR", "failed to generate QR code", nil)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="device-%s-qr.png"`, d.DeviceCode))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

// ── TestLevelType handler (public) ────────────────────────────────────────────

func (h *Handler) listTestLevelTypes(w http.ResponseWriter, r *http.Request) {
	types, err := h.repo.ListTestLevelTypes(r.Context())
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "failed to list test level types", nil)
		return
	}
	var out []TestLevelTypeResponse
	for _, t := range types {
		out = append(out, TestLevelTypeResponse{
			Code: t.Code, Name: t.Name, Description: t.Description, SortOrder: t.SortOrder,
		})
	}
	httpx.Success(w, http.StatusOK, out, nil)
}

// ── TestPlan handlers ─────────────────────────────────────────────────────────

func (h *Handler) createTestPlan(w http.ResponseWriter, r *http.Request) {
	var req CreateTestPlanRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "DECODE_ERROR", err.Error(), nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	p := &domain.TestPlan{
		ID:            uuid.New(),
		TestCenterID:  tcID(auth),
		Name:          req.Name,
		Description:   req.Description,
		PassThreshold: req.PassThreshold,
		Status:        domain.TestPlanDraft,
		Audit: domain.Audit{
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
			CreatedBy: auth.SubjectID, UpdatedBy: auth.SubjectID,
		},
	}
	if err := h.repo.CreateTestPlan(r.Context(), p); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusCreated, toPlanResponse(p, nil), nil)
}

func (h *Handler) listTestPlans(w http.ResponseWriter, r *http.Request) {
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	page := httpx.QueryInt(r.URL.Query(), "page", 1)
	if page < 1 {
		page = 1
	}
	var filterID *uuid.UUID
	if auth.EntityType != security.EntitySuperAdmin {
		id := tcID(auth)
		if id != uuid.Nil {
			filterID = &id
		}
	}
	plans, total, err := h.repo.ListTestPlans(r.Context(), filterID, page)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	var out []TestPlanResponse
	for _, p := range plans {
		out = append(out, toPlanResponse(p, nil))
	}
	httpx.Success(w, http.StatusOK, out, &httpx.Meta{Total: total, Page: page})
}

func (h *Handler) getTestPlan(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "plan ID must be a UUID", nil)
		return
	}
	p, err := h.repo.TestPlanByID(r.Context(), id)
	if err == ErrTestPlanNotFound {
		httpx.Failure(w, http.StatusNotFound, "PLAN_NOT_FOUND", "test plan not found", nil)
		return
	} else if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	maneuvers, _ := h.repo.ManeuversByPlanID(r.Context(), id)
	httpx.Success(w, http.StatusOK, toPlanResponse(p, maneuvers), nil)
}

func (h *Handler) publishTestPlan(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "plan ID must be a UUID", nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	if err := h.repo.PublishTestPlan(r.Context(), id, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"status": "active"}, nil)
}

// ── Maneuver handlers ─────────────────────────────────────────────────────────

func (h *Handler) createManeuver(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "plan ID must be a UUID", nil)
		return
	}
	var req CreateManeuverRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "DECODE_ERROR", err.Error(), nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	m := &domain.ManeuverConfig{
		ID: uuid.New(), TestPlanID: planID,
		Name: req.Name, Description: req.Description,
		SequenceNumber:    req.SequenceNumber,
		QRCodeValue:       req.QRCodeValue,
		TolerancePx:       req.TolerancePx,
		Weight:            req.Weight,
		MinFramesRequired: req.MinFramesRequired,
		Audit: domain.Audit{
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
			CreatedBy: auth.SubjectID, UpdatedBy: auth.SubjectID,
		},
	}
	if err := h.repo.CreateManeuver(r.Context(), m); err != nil {
		httpx.Failure(w, http.StatusConflict, "MANEUVER_CONFLICT", "sequence number or QR code already used in this plan", nil)
		return
	}
	httpx.Success(w, http.StatusCreated, toManeuverResponse(m), nil)
}

func (h *Handler) uploadReferenceMask(w http.ResponseWriter, r *http.Request) {
	maneuverID, err := uuid.Parse(chi.URLParam(r, "maneuverID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "maneuver ID must be a UUID", nil)
		return
	}
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "PARSE_ERROR", "failed to parse multipart form", nil)
		return
	}
	file, hdr, err := r.FormFile("mask")
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "MISSING_FILE", "mask file is required", nil)
		return
	}
	defer file.Close()
	buf := make([]byte, hdr.Size)
	if _, err := file.Read(buf); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "READ_ERROR", "failed to read file", nil)
		return
	}
	key := fmt.Sprintf("masks/%s.png", maneuverID)
	if err := h.minio.PutObject(r.Context(), key, buf, "image/png"); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "STORAGE_ERROR", "failed to upload mask", nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	if err := h.repo.UpdateManeuverMaskURL(r.Context(), maneuverID, key, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"mask_key": key}, nil)
}

func (h *Handler) downloadReferenceMask(w http.ResponseWriter, r *http.Request) {
	maneuverID, err := uuid.Parse(chi.URLParam(r, "maneuverID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "maneuver ID must be a UUID", nil)
		return
	}
	m, err := h.repo.ManeuverByID(r.Context(), maneuverID)
	if err == ErrManeuverNotFound {
		httpx.Failure(w, http.StatusNotFound, "MANEUVER_NOT_FOUND", "maneuver not found", nil)
		return
	} else if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	if m.ReferenceMaskURL == "" {
		httpx.Failure(w, http.StatusNotFound, "NO_MASK", "no reference mask uploaded for this maneuver", nil)
		return
	}
	data, err := h.minio.GetObject(r.Context(), m.ReferenceMaskURL)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "STORAGE_ERROR", "failed to retrieve mask", nil)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) downloadManeuverQR(w http.ResponseWriter, r *http.Request) {
	maneuverID, err := uuid.Parse(chi.URLParam(r, "maneuverID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "maneuver ID must be a UUID", nil)
		return
	}
	m, err := h.repo.ManeuverByID(r.Context(), maneuverID)
	if err == ErrManeuverNotFound {
		httpx.Failure(w, http.StatusNotFound, "MANEUVER_NOT_FOUND", "maneuver not found", nil)
		return
	} else if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	payload := fmt.Sprintf("DLS:SESSION:%d:%s", m.SequenceNumber, m.ID)
	png, err := qrcode.Encode(payload, qrcode.Medium, 256)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "QR_ERROR", "failed to generate QR", nil)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="maneuver-%d-qr.png"`, m.SequenceNumber))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

// ── TestLevelMapping handler ───────────────────────────────────────────────────

func (h *Handler) upsertLevelMapping(w http.ResponseWriter, r *http.Request) {
	var req UpsertLevelMappingRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "DECODE_ERROR", err.Error(), nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	exists, err := h.repo.TestLevelTypeExists(r.Context(), req.TestLevelCode)
	if err != nil || !exists {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_LEVEL_CODE", "test level code does not exist", nil)
		return
	}
	if err := h.repo.UpsertTestLevelMapping(r.Context(), tcID(auth), req.TestLevelCode, req.TestPlanID, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "level mapping updated"}, nil)
}

func (h *Handler) listLevelMappings(w http.ResponseWriter, r *http.Request) {
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	ms, err := h.repo.ListTestLevelMappings(r.Context(), tcID(auth))
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	var out []LevelMappingResponse
	for _, m := range ms {
		out = append(out, LevelMappingResponse{
			ID:            m.ID,
			TestCenterID:  m.TestCenterID,
			TestLevelCode: m.TestLevelCode,
			TestPlanID:    m.TestPlanID,
			UpdatedAt:     m.Audit.UpdatedAt,
		})
	}
	httpx.Success(w, http.StatusOK, out, nil)
}

// ── Internal test creation ────────────────────────────────────────────────────

func (h *Handler) createTestInternal(w http.ResponseWriter, r *http.Request) {
	var req CreateTestInternalRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "DECODE_ERROR", err.Error(), nil)
		return
	}
	ctx := r.Context()

	exists, err := h.repo.TestLevelTypeExists(ctx, req.TestLevelCode)
	if err != nil || !exists {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_LEVEL_CODE", "test level code does not exist", nil)
		return
	}
	plan, err := h.repo.ActiveTestPlanForLevel(ctx, req.TestCenterID, req.TestLevelCode)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	if plan == nil {
		httpx.Failure(w, http.StatusUnprocessableEntity, "NO_ACTIVE_PLAN",
			"no active test plan configured for this level at this test center", nil)
		return
	}
	test := &domain.Test{
		ID:            uuid.New(),
		BookingID:     req.BookingID,
		CandidateID:   req.CandidateID,
		TestCenterID:  req.TestCenterID,
		TestPlanID:    plan.ID,
		TestLevelCode: req.TestLevelCode,
		Status:        domain.TestStatusPending,
		Audit: domain.Audit{
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
			CreatedBy: systemActorID, UpdatedBy: systemActorID,
		},
	}
	if err := h.repo.CreateTest(ctx, test); err != nil {
		httpx.Failure(w, http.StatusConflict, "DUPLICATE_PENDING",
			"candidate already has a pending test at this test center", nil)
		return
	}
	httpx.Success(w, http.StatusCreated, toTestResponse(test), nil)
}

// ── Candidate flow handlers ───────────────────────────────────────────────────

func (h *Handler) deviceCheckin(w http.ResponseWriter, r *http.Request) {
	var req DeviceCheckinRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "DECODE_ERROR", err.Error(), nil)
		return
	}
	ctx := r.Context()
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}

	device, err := h.repo.DeviceByCode(ctx, req.DeviceCode)
	if err == ErrDeviceNotFound {
		httpx.Failure(w, http.StatusNotFound, "DEVICE_NOT_FOUND", "device not found", nil)
		return
	} else if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}

	// security.CheckPassword returns an error (nil == match)
	if err := security.CheckPassword(device.PasswordHash, req.Password); err != nil {
		httpx.Failure(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "incorrect device password", nil)
		return
	}
	if device.Status != domain.DeviceStatusActive {
		httpx.Failure(w, http.StatusConflict, "DEVICE_IN_USE", "device is currently in use by another test", nil)
		return
	}

	test, err := h.repo.PendingTestForCandidate(ctx, auth.SubjectID, req.TestCenterID)
	if err == ErrNoPendingTest {
		httpx.Failure(w, http.StatusNotFound, "NO_PENDING_TEST", "no pending test found — complete booking first", nil)
		return
	} else if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}

	// Check level allowed on device
	var allowed []string
	_ = json.Unmarshal([]byte(device.AllowedLevels), &allowed)
	levelOK := false
	for _, l := range allowed {
		if l == test.TestLevelCode {
			levelOK = true
			break
		}
	}
	if !levelOK {
		httpx.Failure(w, http.StatusForbidden, "LEVEL_NOT_ALLOWED", "this device does not support this test level", nil)
		return
	}

	// Validate booking window
	scheduledAt, err := h.repo.BookingScheduledAt(ctx, test.BookingID)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "failed to verify booking window", nil)
		return
	}
	window := time.Duration(h.cfg.BookingWindowHours) * time.Hour
	diff := time.Since(scheduledAt)
	if diff < 0 {
		diff = -diff
	}
	if diff > window {
		httpx.Failure(w, http.StatusGone, "BOOKING_WINDOW_CLOSED", "check-in is outside the allowed booking window", nil)
		return
	}

	// Atomic transaction: mark test ready + device in_use
	tx, err := h.repo.db.Begin(ctx)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "failed to start transaction", nil)
		return
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE tests SET device_id=$2, device_scanned_at=NOW(), status='ready',
			updated_at=NOW(), updated_by=$3 WHERE id=$1 AND status='pending'`,
		test.ID, device.ID, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "failed to checkin test", nil)
		return
	}
	tag, err := tx.Exec(ctx,
		`UPDATE devices SET status='in_use', current_test_id=$2, last_seen_at=NOW(),
			updated_at=NOW(), updated_by=$3 WHERE id=$1 AND status='active'`,
		device.ID, test.ID, auth.SubjectID)
	if err != nil || tag.RowsAffected() == 0 {
		httpx.Failure(w, http.StatusConflict, "DEVICE_IN_USE", "device was taken by another test", nil)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "transaction commit failed", nil)
		return
	}

	_ = h.repo.AdvanceToGuidelines(ctx, test.ID, auth.SubjectID)
	test.Status = domain.TestStatusGuidelines
	httpx.Success(w, http.StatusOK, toTestResponse(test), nil)
}

func (h *Handler) getMyPendingTest(w http.ResponseWriter, r *http.Request) {
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	test, err := h.repo.MyPendingTest(r.Context(), auth.SubjectID)
	if err == ErrNoPendingTest {
		httpx.Failure(w, http.StatusNotFound, "NO_PENDING_TEST", "no pending test found", nil)
		return
	} else if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, toTestResponse(test), nil)
}

func (h *Handler) getTestStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test ID must be a UUID", nil)
		return
	}
	test, err := h.repo.TestByID(r.Context(), id)
	if err != nil {
		httpx.Failure(w, http.StatusNotFound, "TEST_NOT_FOUND", "test not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"id": id.String(), "status": string(test.Status)}, nil)
}

func (h *Handler) acknowledgeGuidelines(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test ID must be a UUID", nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	if err := h.repo.AcknowledgeGuidelines(r.Context(), id, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"status": "acknowledged"}, nil)
}

// ── Monitor handlers (REST polling) ──────────────────────────────────────────

func (h *Handler) monitorStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test ID must be a UUID", nil)
		return
	}
	test, err := h.repo.TestByID(r.Context(), id)
	if err != nil {
		httpx.Failure(w, http.StatusNotFound, "TEST_NOT_FOUND", "test not found", nil)
		return
	}
	var abortStr *string
	if test.AbortReason != nil {
		s := string(*test.AbortReason)
		abortStr = &s
	}
	var devStr *string
	if test.DeviceID != nil {
		s := test.DeviceID.String()
		devStr = &s
	}
	httpx.Success(w, http.StatusOK, MonitorStatusResponse{
		TestID:      test.ID.String(),
		Status:      string(test.Status),
		DeviceID:    devStr,
		StartedAt:   test.StartedAt,
		CompletedAt: test.CompletedAt,
		AbortReason: abortStr,
	}, nil)
}

func (h *Handler) monitorLive(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test ID must be a UUID", nil)
		return
	}
	test, err := h.repo.TestByID(r.Context(), id)
	if err != nil {
		httpx.Failure(w, http.StatusNotFound, "TEST_NOT_FOUND", "test not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, MonitorLiveResponse{
		TestID:         test.ID.String(),
		Status:         string(test.Status),
		FrameCount:     0,
		RunningAvgIoU:  0.0,
		DeviceHealthOK: test.IoTHealthPassedAt != nil,
	}, nil)
}

// ── Guidelines stubs (public) ─────────────────────────────────────────────────

func (h *Handler) listGuidelines(w http.ResponseWriter, r *http.Request) {
	httpx.Success(w, http.StatusOK, map[string]any{
		"title": "ADLTS Testing Guidelines",
		"sections": []map[string]string{
			{"heading": "Before the Test", "body": "Arrive 10 minutes before your scheduled slot. Bring your booking confirmation QR code."},
			{"heading": "During the Test", "body": "Follow the on-screen instructions. Scan each maneuver QR code when prompted."},
			{"heading": "Results", "body": "Your result will be available within the platform after the test completes."},
		},
	}, nil)
}

func (h *Handler) listGuidelinesFAQ(w http.ResponseWriter, r *http.Request) {
	httpx.Success(w, http.StatusOK, []map[string]string{
		{"q": "What happens if I fail?", "a": "You may re-book a test after 7 days."},
		{"q": "Can I appeal the result?", "a": "Yes, you have 72 hours from test completion to submit an appeal."},
		{"q": "What if the device fails mid-test?", "a": "The test will be aborted and you will be re-scheduled at no extra cost."},
	}, nil)
}

// ── Converters ────────────────────────────────────────────────────────────────

func toPlanResponse(p *domain.TestPlan, maneuvers []*domain.ManeuverConfig) TestPlanResponse {
	resp := TestPlanResponse{
		ID: p.ID, TestCenterID: p.TestCenterID,
		Name: p.Name, Description: p.Description,
		PassThreshold: p.PassThreshold,
		Status:        string(p.Status),
		PublishedAt:   p.PublishedAt,
		CreatedAt:     p.Audit.CreatedAt,
	}
	for _, m := range maneuvers {
		resp.Maneuvers = append(resp.Maneuvers, toManeuverResponse(m))
	}
	return resp
}

func toManeuverResponse(m *domain.ManeuverConfig) ManeuverResponse {
	return ManeuverResponse{
		ID: m.ID, TestPlanID: m.TestPlanID,
		Name: m.Name, Description: m.Description,
		SequenceNumber:    m.SequenceNumber,
		QRCodeValue:       m.QRCodeValue,
		ReferenceMaskURL:  m.ReferenceMaskURL,
		TolerancePx:       m.TolerancePx,
		Weight:            m.Weight,
		MinFramesRequired: m.MinFramesRequired,
		CreatedAt:         m.Audit.CreatedAt,
	}
}

// strconv import kept for query param parsing.
var _ = strconv.Atoi

// toManeuverConfigResponse converts a ManeuverConfig domain struct to the
// migration-003 response DTO.
func toManeuverConfigResponse(m *domain.ManeuverConfig) ManeuverConfigResponse {
	return ManeuverConfigResponse{
		ID:                m.ID,
		TestPlanID:        m.TestPlanID,
		ManeuverType:      string(m.ManeuverType),
		DisplayName:       m.DisplayName,
		SequenceNumber:    m.SequenceNumber,
		Weight:            m.Weight,
		PassThreshold:     m.PassThreshold,
		TolerancePx:       m.TolerancePx,
		MinFramesRequired: m.MinFramesRequired,
		QRStartValue:      m.QRStartValue,
		QREndValue:        m.QREndValue,
		CreatedAt:         m.Audit.CreatedAt,
		UpdatedAt:         m.Audit.UpdatedAt,
	}
}

// ── Result Handlers ──────────────────────────────────────────────

// canViewResult is kept for backward compat by getFrameAnalyses.
// New result visibility logic is inlined in getTestResult per §13.
func (h *Handler) canViewResult(test *domain.Test, auth *security.AuthContext) bool {
	if auth.EntityType == security.EntityAdmin || auth.EntityType == security.EntitySuperAdmin {
		return true
	}
	if test.CompletedAt != nil && time.Since(*test.CompletedAt) > 72*time.Hour {
		return true
	}
	return false
}

func (h *Handler) getTestResult(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test ID must be a UUID", nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}

	test, err := h.repo.TestByID(r.Context(), id)
	if err != nil {
		httpx.Failure(w, http.StatusNotFound, "TEST_NOT_FOUND", "test not found", nil)
		return
	}

	// Role-based visibility guard (§13)
	switch auth.EntityType {
	case security.EntityCandidate:
		if test.ResultVisibleToCandidateAt != nil && time.Now().Before(*test.ResultVisibleToCandidateAt) {
			httpx.Failure(w, http.StatusForbidden, "RESULT_NOT_AVAILABLE",
				"result not yet available — check result_available_at", nil)
			return
		}
	case security.EntityInstitute:
		// institutes: delayed by result_visible_to_institute_at
		if test.ResultVisibleToInstituteAt != nil && time.Now().Before(*test.ResultVisibleToInstituteAt) {
			httpx.Failure(w, http.StatusForbidden, "RESULT_NOT_AVAILABLE",
				"result not yet available for your institute", nil)
			return
		}
		// institutes: fall through — they always get a restricted view
	default:
		// Admin, SuperAdmin, Expert: no time restriction
	}

	tr, srs, err := h.repo.GetTestResultWithBreakdown(r.Context(), id)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}

	switch auth.EntityType {
	case security.EntityCandidate:
		httpx.Success(w, http.StatusOK, toCandidateResultView(test, tr, srs), nil)
	case security.EntityInstitute:
		httpx.Success(w, http.StatusOK, toInstituteResultView(test, tr), nil)
	default: // Admin, SuperAdmin, Expert
		// Load events for each session
		eventsBySession := map[uuid.UUID][]domain.ManeuverEvent{}
		for _, sr := range srs {
			evs, _ := h.repo.ListManeuverEvents(r.Context(), sr.SessionID)
			eventsBySession[sr.SessionID] = evs
		}
		httpx.Success(w, http.StatusOK, toFullResultView(test, tr, srs, eventsBySession), nil)
	}
}

// ── Result view converters (§13) ─────────────────────────────────────────────

func toCandidateResultView(test *domain.Test, tr *domain.TestResult, srs []domain.SessionResult) dto.CandidateResultView {
	v := dto.CandidateResultView{
		TestID:            test.ID,
		ResultAvailableAt: test.ResultVisibleToCandidateAt,
	}
	if tr != nil {
		v.WeightedTotalScore = tr.WeightedTotalScore
		v.Passed = tr.Passed
		v.PassThreshold = tr.PassThreshold
		v.AnyCriticalFail = tr.AnyCriticalFail
		v.WeakestManeuver = tr.WeakestManeuver
		v.OverallNarrative = tr.OverallNarrative
		v.StrengthsNarrative = tr.StrengthsNarrative
		v.WeaknessesNarrative = tr.WeaknessesNarrative
		v.RecommendedFocus = tr.RecommendedFocus
	}
	for _, sr := range srs {
		display := sr.WeakestPhase // reuse as label fallback
		if display == "" {
			display = string(sr.ManeuverType)
		}
		v.ManeuverScores = append(v.ManeuverScores, dto.ManeuverScoreSummary{
			ManeuverType: string(sr.ManeuverType),
			DisplayName:  display,
			Score:        sr.Score,
			Passed:       sr.Passed,
			Weight:       sr.Weight,
			CriticalFail: sr.CriticalFail,
		})
	}
	return v
}

func toInstituteResultView(test *domain.Test, tr *domain.TestResult) dto.InstituteResultView {
	v := dto.InstituteResultView{
		TestID:        test.ID,
		CandidateID:   test.CandidateID,
		TestLevelCode: test.TestLevelCode,
		CompletedAt:   test.CompletedAt,
	}
	if tr != nil {
		v.WeightedTotalScore = tr.WeightedTotalScore
		v.Passed = tr.Passed
		v.PassThreshold = tr.PassThreshold
		v.WeakestManeuver = tr.WeakestManeuver
		v.StrengthsNarrative = tr.StrengthsNarrative
		v.WeaknessesNarrative = tr.WeaknessesNarrative
	}
	return v
}

func toFullResultView(
	test *domain.Test,
	tr *domain.TestResult,
	srs []domain.SessionResult,
	eventsBySession map[uuid.UUID][]domain.ManeuverEvent,
) dto.FullResultView {
	v := dto.FullResultView{
		TestID:        test.ID,
		CandidateID:   test.CandidateID,
		TestLevelCode: test.TestLevelCode,
		StartedAt:     test.StartedAt,
		CompletedAt:   test.CompletedAt,
	}
	if tr != nil {
		v.WeightedTotalScore = tr.WeightedTotalScore
		v.Passed = tr.Passed
		v.PassThreshold = tr.PassThreshold
		v.AnyCriticalFail = tr.AnyCriticalFail
		v.WeakestManeuver = tr.WeakestManeuver
		v.ScoreBreakdown = tr.ScoreBreakdown
		v.OverallNarrative = tr.OverallNarrative
		v.StrengthsNarrative = tr.StrengthsNarrative
		v.WeaknessesNarrative = tr.WeaknessesNarrative
		v.RecommendedFocus = tr.RecommendedFocus
		v.NarrativeModel = tr.NarrativeModel
	}
	for _, sr := range srs {
		evs := eventsBySession[sr.SessionID]
		var evViews []dto.ManeuverEventView
		for _, ev := range evs {
			evViews = append(evViews, dto.ManeuverEventView{
				EventType:  ev.EventType,
				Severity:   ev.Severity,
				StartFrame: ev.StartFrame,
				EndFrame:   ev.EndFrame,
				Detail:     ev.Detail,
			})
		}
		v.Sessions = append(v.Sessions, dto.SessionResultView{
			SessionID:          sr.SessionID,
			ManeuverType:       string(sr.ManeuverType),
			DisplayName:        string(sr.ManeuverType),
			SequenceNumber:     sr.SequenceNumber,
			Score:              sr.Score,
			Passed:             sr.Passed,
			CriticalFail:       sr.CriticalFail,
			Weight:             sr.Weight,
			FrameCount:         sr.FrameCount,
			LaneDetectedPct:    sr.LaneDetectedPct,
			AvgIoU:             sr.AvgIoU,
			MeanCenterOffsetPx: sr.MeanCenterOffset,
			OffsetVariancePx:   sr.OffsetVariance,
			WeakestPhase:       sr.WeakestPhase,
			Events:             evViews,
		})
	}
	return v
}

func (h *Handler) getFrameAnalyses(w http.ResponseWriter, r *http.Request) {
	testID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test ID must be a UUID", nil)
		return
	}
	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "session ID must be a UUID", nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}

	test, err := h.repo.TestByID(r.Context(), testID)
	if err != nil {
		httpx.Failure(w, http.StatusNotFound, "TEST_NOT_FOUND", "test not found", nil)
		return
	}

	if !h.canViewResult(test, auth) {
		httpx.Failure(w, http.StatusForbidden, "RESULT_NOT_AVAILABLE", "test results are available 72 hours after completion", nil)
		return
	}

	frames, err := h.repo.FrameAnalysesBySessionID(r.Context(), sessionID)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}

	var out []FrameAnalysisResponse
	for _, f := range frames {
		out = append(out, FrameAnalysisResponse{
			FrameSeqNo:   f.FrameSeqNo,
			CapturedAt:   f.CapturedAt,
			LaneDetected: f.LaneDetected,
			CurvatureDir: string(f.CurvatureDir),
			IoUScore:     f.IoUScore,
			FrameScore:   f.FrameScore,
			IsMocked:     f.IsMocked,
		})
	}
	httpx.Success(w, http.StatusOK, out, nil)
}

// ── Phase 4: Maneuver Type handler ──────────────────────────────────────────────

// listManeuverTypes serves GET /maneuver-types — returns the 8 static maneuver
// type definitions. No DB query: the list is compile-time constant.
func (h *Handler) listManeuverTypes(w http.ResponseWriter, r *http.Request) {
	types, _ := h.repo.GetManeuverTypes(r.Context())
	httpx.Success(w, http.StatusOK, types, nil)
}

// ── Phase 4: Test Plan CRUD additions ──────────────────────────────────────────

func (h *Handler) updateTestPlan(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "plan ID must be a UUID", nil)
		return
	}
	var req UpdateTestPlanRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "DECODE_ERROR", err.Error(), nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	plan, err := h.repo.TestPlanByID(r.Context(), planID)
	if err != nil {
		httpx.Failure(w, http.StatusNotFound, "PLAN_NOT_FOUND", "test plan not found", nil)
		return
	}
	if plan.Status != domain.TestPlanDraft {
		httpx.Failure(w, http.StatusConflict, "PLAN_NOT_DRAFT", "only draft plans can be updated", nil)
		return
	}
	fields := map[string]any{}
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if req.Description != nil {
		fields["description"] = *req.Description
	}
	if req.PassThreshold != nil {
		fields["pass_threshold"] = *req.PassThreshold
	}
	if len(fields) == 0 {
		httpx.Failure(w, http.StatusBadRequest, "EMPTY_UPDATE", "no fields to update", nil)
		return
	}
	if err := h.repo.UpdateTestPlanFields(r.Context(), planID, fields, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"status": "updated"}, nil)
}

func (h *Handler) deleteTestPlan(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "plan ID must be a UUID", nil)
		return
	}
	plan, err := h.repo.TestPlanByID(r.Context(), planID)
	if err != nil {
		httpx.Failure(w, http.StatusNotFound, "PLAN_NOT_FOUND", "test plan not found", nil)
		return
	}
	if plan.Status != domain.TestPlanDraft {
		httpx.Failure(w, http.StatusConflict, "PLAN_NOT_DRAFT", "only draft plans can be deleted", nil)
		return
	}
	if err := h.repo.DeleteTestPlan(r.Context(), planID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"status": "deleted"}, nil)
}

func (h *Handler) retireTestPlan(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "plan ID must be a UUID", nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	if err := h.repo.RetireTestPlan(r.Context(), planID, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"status": "retired"}, nil)
}

// ── Phase 4: Maneuver Config handlers (migration-003 schema) ────────────────────

func (h *Handler) listManeuverConfigs(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "plan ID must be a UUID", nil)
		return
	}
	ms, err := h.repo.ListManeuverConfigs(r.Context(), planID)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	out := make([]ManeuverConfigResponse, 0, len(ms))
	for i := range ms {
		out = append(out, toManeuverConfigResponse(&ms[i]))
	}
	httpx.Success(w, http.StatusOK, out, nil)
}

// createManeuverConfig handles POST /test-plans/{planID}/maneuvers.
// Uses the migration-003 schema: maneuver_type FK + auto-generated QR values.
func (h *Handler) createManeuverConfig(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "plan ID must be a UUID", nil)
		return
	}
	var req CreateManeuverConfigRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "DECODE_ERROR", err.Error(), nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	if req.ManeuverType == "" {
		httpx.Failure(w, http.StatusBadRequest, "MISSING_FIELD", "maneuver_type is required", nil)
		return
	}
	if req.PassThreshold == 0 {
		req.PassThreshold = 70.0
	}
	if req.TolerancePx == 0 {
		req.TolerancePx = 20
	}
	if req.MinFramesRequired == 0 {
		req.MinFramesRequired = 30
	}
	m := domain.ManeuverConfig{
		ID:                uuid.New(),
		TestPlanID:        planID,
		ManeuverType:      domain.ManeuverType(req.ManeuverType),
		DisplayName:       req.DisplayName,
		SequenceNumber:    req.SequenceNumber,
		Weight:            req.Weight,
		PassThreshold:     req.PassThreshold,
		TolerancePx:       req.TolerancePx,
		MinFramesRequired: req.MinFramesRequired,
		Audit: domain.Audit{
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
			CreatedBy: auth.SubjectID, UpdatedBy: auth.SubjectID,
		},
	}
	if err := h.repo.CreateManeuverConfig(r.Context(), m); err != nil {
		httpx.Failure(w, http.StatusConflict, "MANEUVER_CONFLICT",
			"sequence number already used in this plan or invalid maneuver_type", nil)
		return
	}
	// Re-fetch to get DB-generated qr_start_value / qr_end_value
	created, err := h.repo.GetManeuverConfig(r.Context(), m.ID)
	if err != nil {
		httpx.Success(w, http.StatusCreated, toManeuverConfigResponse(&m), nil)
		return
	}
	httpx.Success(w, http.StatusCreated, toManeuverConfigResponse(created), nil)
}

func (h *Handler) updateManeuverConfig(w http.ResponseWriter, r *http.Request) {
	maneuverID, err := uuid.Parse(chi.URLParam(r, "maneuverID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "maneuver ID must be a UUID", nil)
		return
	}
	var req UpdateManeuverConfigRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "DECODE_ERROR", err.Error(), nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	m, err := h.repo.GetManeuverConfig(r.Context(), maneuverID)
	if err == ErrManeuverNotFound {
		httpx.Failure(w, http.StatusNotFound, "MANEUVER_NOT_FOUND", "maneuver not found", nil)
		return
	} else if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	if req.DisplayName != nil {
		m.DisplayName = *req.DisplayName
	}
	if req.Weight != nil {
		m.Weight = *req.Weight
	}
	if req.PassThreshold != nil {
		m.PassThreshold = *req.PassThreshold
	}
	if req.TolerancePx != nil {
		m.TolerancePx = *req.TolerancePx
	}
	if req.MinFramesRequired != nil {
		m.MinFramesRequired = *req.MinFramesRequired
	}
	m.Audit.UpdatedBy = auth.SubjectID
	if err := h.repo.UpdateManeuverConfig(r.Context(), *m); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, toManeuverConfigResponse(m), nil)
}

func (h *Handler) deleteManeuverConfig(w http.ResponseWriter, r *http.Request) {
	maneuverID, err := uuid.Parse(chi.URLParam(r, "maneuverID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "maneuver ID must be a UUID", nil)
		return
	}
	if _, err := h.repo.GetManeuverConfig(r.Context(), maneuverID); err == ErrManeuverNotFound {
		httpx.Failure(w, http.StatusNotFound, "MANEUVER_NOT_FOUND", "maneuver not found", nil)
		return
	}
	if err := h.repo.DeleteManeuverConfig(r.Context(), maneuverID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"status": "deleted"}, nil)
}

func (h *Handler) reorderManeuverConfigs(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "planID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "plan ID must be a UUID", nil)
		return
	}
	var req ReorderManeuverConfigRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "DECODE_ERROR", err.Error(), nil)
		return
	}
	if len(req.OrderedIDs) == 0 {
		httpx.Failure(w, http.StatusBadRequest, "EMPTY_LIST", "ordered_ids must not be empty", nil)
		return
	}
	if err := h.repo.ReorderManeuverConfigs(r.Context(), planID, req.OrderedIDs); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"status": "reordered"}, nil)
}

// downloadManeuverQRZip handles GET /test-plans/{planID}/maneuvers/{maneuverID}/qr.
// It generates QR PNG images for both qr_start_value and qr_end_value, bundles
// them into a ZIP archive, and returns application/zip.
func (h *Handler) downloadManeuverQRZip(w http.ResponseWriter, r *http.Request) {
	maneuverID, err := uuid.Parse(chi.URLParam(r, "maneuverID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "maneuver ID must be a UUID", nil)
		return
	}
	m, err := h.repo.GetManeuverConfig(r.Context(), maneuverID)
	if err == ErrManeuverNotFound {
		httpx.Failure(w, http.StatusNotFound, "MANEUVER_NOT_FOUND", "maneuver not found", nil)
		return
	} else if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	if m.QRStartValue == "" || m.QREndValue == "" {
		httpx.Failure(w, http.StatusUnprocessableEntity, "QR_NOT_GENERATED",
			"QR values not yet generated — ensure maneuver_type was set on insert", nil)
		return
	}

	startPNG, err := qrcode.Encode(m.QRStartValue, qrcode.Medium, 256)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "QR_ERROR", "failed to generate start QR", nil)
		return
	}
	endPNG, err := qrcode.Encode(m.QREndValue, qrcode.Medium, 256)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "QR_ERROR", "failed to generate end QR", nil)
		return
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if f, err := zw.Create(fmt.Sprintf("%s-start-qr.png", m.ManeuverType)); err == nil {
		_, _ = f.Write(startPNG)
	}
	if f, err := zw.Create(fmt.Sprintf("%s-end-qr.png", m.ManeuverType)); err == nil {
		_, _ = f.Write(endPNG)
	}
	_ = zw.Close()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="maneuver-%s-%d-qr.zip"`, m.ManeuverType, m.SequenceNumber))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

// ── Phase 4: Tests admin CRUD ────────────────────────────────────────────────

func (h *Handler) listTests(w http.ResponseWriter, r *http.Request) {
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	q := r.URL.Query()
	statusFilter := q.Get("status")
	page := httpx.QueryInt(q, "page", 1)
	limit := httpx.QueryInt(q, "limit", 20)
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	var candidateID uuid.UUID
	if raw := q.Get("candidate_id"); raw != "" {
		candidateID, _ = uuid.Parse(raw)
	}
	tests, total, err := h.repo.ListTests(r.Context(), tcID(auth), statusFilter, candidateID, page, limit)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	out := make([]TestResponse, 0, len(tests))
	for _, t := range tests {
		out = append(out, toTestResponse(t))
	}
	httpx.Success(w, http.StatusOK, out, &httpx.Meta{Page: page, Limit: limit, Total: total})
}

func (h *Handler) getTest(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test ID must be a UUID", nil)
		return
	}
	test, err := h.repo.TestByID(r.Context(), id)
	if err != nil {
		httpx.Failure(w, http.StatusNotFound, "TEST_NOT_FOUND", "test not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, toTestResponse(test), nil)
}

// createTestAdmin handles POST /tests — admin manual test creation (override path).
// The normal path is POST /internal/tests called by the booking service.
func (h *Handler) createTestAdmin(w http.ResponseWriter, r *http.Request) {
	var req CreateTestAdminRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "DECODE_ERROR", err.Error(), nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	ctx := r.Context()
	exists, err := h.repo.TestLevelTypeExists(ctx, req.TestLevelCode)
	if err != nil || !exists {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_LEVEL_CODE", "test level code does not exist", nil)
		return
	}
	plan, err := h.repo.ActiveTestPlanForLevel(ctx, req.TestCenterID, req.TestLevelCode)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	if plan == nil {
		httpx.Failure(w, http.StatusUnprocessableEntity, "NO_ACTIVE_PLAN",
			"no active test plan for this level at this center", nil)
		return
	}
	test := &domain.Test{
		ID:            uuid.New(),
		BookingID:     req.BookingID,
		CandidateID:   req.CandidateID,
		TestCenterID:  req.TestCenterID,
		TestPlanID:    plan.ID,
		TestLevelCode: req.TestLevelCode,
		Status:        domain.TestStatusPending,
		Audit: domain.Audit{
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
			CreatedBy: auth.SubjectID, UpdatedBy: auth.SubjectID,
		},
	}
	if err := h.repo.CreateTest(ctx, test); err != nil {
		httpx.Failure(w, http.StatusConflict, "DUPLICATE_PENDING",
			"candidate already has a pending test at this center", nil)
		return
	}
	httpx.Success(w, http.StatusCreated, toTestResponse(test), nil)
}

func (h *Handler) updateTestAdmin(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test ID must be a UUID", nil)
		return
	}
	var req UpdateTestAdminRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "DECODE_ERROR", err.Error(), nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	fields := map[string]any{}
	if req.AppealWindowClosesAt != nil {
		fields["appeal_window_closes_at"] = req.AppealWindowClosesAt
	}
	if req.ResultVisibleToCandidateAt != nil {
		fields["result_visible_to_candidate_at"] = req.ResultVisibleToCandidateAt
	}
	if req.ResultVisibleToInstituteAt != nil {
		fields["result_visible_to_institute_at"] = req.ResultVisibleToInstituteAt
	}
	if len(fields) == 0 {
		httpx.Failure(w, http.StatusBadRequest, "EMPTY_UPDATE", "no fields to update", nil)
		return
	}
	if err := h.repo.UpdateTestFields(r.Context(), id, fields, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"status": "updated"}, nil)
}

func (h *Handler) deleteTestAdmin(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test ID must be a UUID", nil)
		return
	}
	auth := mustAuth(w, r)
	if auth == nil {
		return
	}
	test, err := h.repo.TestByID(r.Context(), id)
	if err != nil {
		httpx.Failure(w, http.StatusNotFound, "TEST_NOT_FOUND", "test not found", nil)
		return
	}
	if test.Status != domain.TestStatusPending {
		httpx.Failure(w, http.StatusConflict, "TEST_NOT_PENDING",
			"only pending tests can be cancelled", nil)
		return
	}
	if err := h.repo.AbortTest(r.Context(), id, domain.AbortAdminIntervention, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	if test.DeviceID != nil {
		_ = h.repo.ReleaseDevice(r.Context(), *test.DeviceID, auth.SubjectID)
	}
	httpx.Success(w, http.StatusOK, map[string]string{"status": "cancelled"}, nil)
}

// ── Phase 4: Sessions & Events ────────────────────────────────────────────────

func (h *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	testID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test ID must be a UUID", nil)
		return
	}
	if _, err := h.repo.TestByID(r.Context(), testID); err != nil {
		httpx.Failure(w, http.StatusNotFound, "TEST_NOT_FOUND", "test not found", nil)
		return
	}
	sessions, err := h.repo.ListTestSessions(r.Context(), testID)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, sessions, nil)
}

func (h *Handler) getSession(w http.ResponseWriter, r *http.Request) {
	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "session ID must be a UUID", nil)
		return
	}
	s, err := h.repo.GetTestSession(r.Context(), sessionID)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	if s == nil {
		httpx.Failure(w, http.StatusNotFound, "SESSION_NOT_FOUND", "session not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, s, nil)
}

func (h *Handler) listSessionEvents(w http.ResponseWriter, r *http.Request) {
	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "session ID must be a UUID", nil)
		return
	}
	events, err := h.repo.ListManeuverEvents(r.Context(), sessionID)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, events, nil)
}

// ── Phase 4: Recording presigned URLs ──────────────────────────────────────────

const presignExpiry = 15 * time.Minute

func (h *Handler) getTestRecording(w http.ResponseWriter, r *http.Request) {
	testID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test ID must be a UUID", nil)
		return
	}
	rec, err := h.repo.TestRecordingByTestID(r.Context(), testID)
	if err == ErrRecordingNotFound {
		httpx.Failure(w, http.StatusNotFound, "RECORDING_NOT_FOUND", "no recording found for this test", nil)
		return
	} else if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	if rec.VideoKey == "" {
		httpx.Success(w, http.StatusOK, RecordingURLResponse{
			URL: "", ExpiresAt: time.Time{}, Status: "not_ready",
		}, nil)
		return
	}
	presigned, err := presignObject(h.minio, r.Context(), rec.VideoKey)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "PRESIGN_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, RecordingURLResponse{
		URL:       presigned,
		ExpiresAt: time.Now().Add(presignExpiry),
		Status:    rec.Status,
	}, nil)
}

func (h *Handler) getSessionRecording(w http.ResponseWriter, r *http.Request) {
	testID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test ID must be a UUID", nil)
		return
	}
	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "session ID must be a UUID", nil)
		return
	}
	rec, err := h.repo.TestRecordingBySessionID(r.Context(), testID, sessionID)
	if err == ErrRecordingNotFound {
		httpx.Failure(w, http.StatusNotFound, "RECORDING_NOT_FOUND", "no recording found for this session", nil)
		return
	} else if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}
	if rec.VideoKey == "" {
		httpx.Success(w, http.StatusOK, RecordingURLResponse{
			URL: "", ExpiresAt: time.Time{}, Status: "not_ready",
		}, nil)
		return
	}
	presigned, err := presignObject(h.minio, r.Context(), rec.VideoKey)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "PRESIGN_ERROR", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, RecordingURLResponse{
		URL:       presigned,
		ExpiresAt: time.Now().Add(presignExpiry),
		Status:    rec.Status,
	}, nil)
}

// presignObject generates a 15-minute presigned GET URL for a MinIO object key.
func presignObject(m *minioclient.Client, ctx context.Context, key string) (string, error) {
	reqParams := make(url.Values)
	presignedURL, err := m.Inner().PresignedGetObject(ctx, m.Bucket(), key, presignExpiry, reqParams)
	if err != nil {
		return "", err
	}
	return presignedURL.String(), nil
}

// ── Phase 4: Internal booking service stubs (§12) ────────────────────────────────

// rescheduleTestByBooking is for PATCH /internal/tests/by-booking/{bookingID}.
// Called by the booking service when a booking is rescheduled.
type RescheduleBookingRequest struct {
	NewScheduledStart  time.Time `json:"new_scheduled_start"`
	NewScheduledEnd    time.Time `json:"new_scheduled_end"`
	BookingWindowHours *int      `json:"booking_window_hours"`
}

func (h *Handler) rescheduleTestByBooking(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "bookingID")
	id, err := uuid.Parse(raw)
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "booking ID must be a UUID", nil)
		return
	}

	var req RescheduleBookingRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "DECODE_ERROR", err.Error(), nil)
		return
	}

	// Find associated test
	t, err := h.repo.GetTestByBookingID(r.Context(), id)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "failed to fetch test", nil)
		return
	}
	if t == nil {
		httpx.Failure(w, http.StatusNotFound, "TEST_NOT_FOUND", "no test associated with booking", nil)
		return
	}

	if t.Status != domain.TestStatusPending && t.Status != domain.TestStatusReady {
		httpx.Failure(w, http.StatusConflict, "TEST_BEYOND_READY", "test is beyond ready status", nil)
		return
	}

	fields := map[string]any{
		"scheduled_start_at": req.NewScheduledStart,
		"scheduled_end_at":   req.NewScheduledEnd,
	}
	if req.BookingWindowHours != nil {
		fields["booking_window_hours"] = *req.BookingWindowHours
	}

	if err := h.repo.UpdateTestFields(r.Context(), t.ID, fields, systemActorID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", err.Error(), nil)
		return
	}

	// Return updated test
	t.ScheduledStartAt = &req.NewScheduledStart
	t.ScheduledEndAt = &req.NewScheduledEnd
	if req.BookingWindowHours != nil {
		t.BookingWindowHours = req.BookingWindowHours
	}

	httpx.Success(w, http.StatusOK, toTestResponse(t), nil)
}

// cancelTestByBooking is a STUB for DELETE /internal/tests/by-booking/{bookingID}.
// Called by the booking service when a booking is cancelled.
// TODO: implement when booking service integration is active.
func (h *Handler) cancelTestByBooking(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "bookingID")
	id, err := uuid.Parse(raw)
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "booking ID must be a UUID", nil)
		return
	}
	t, err := h.repo.GetTestByBookingID(r.Context(), id)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "failed to fetch test", nil)
		return
	}
	if t == nil {
		httpx.Success(w, http.StatusOK, map[string]string{"status": "no_test_found"}, nil)
		return
	}
	// Only cancel pending tests; other states should be left untouched
	if t.Status != domain.TestStatusPending {
		httpx.Success(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "test_not_pending"}, nil)
		return
	}
	if err := h.repo.AbortTest(r.Context(), t.ID, domain.AbortAdminIntervention, systemActorID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "failed to abort test", nil)
		return
	}
	if t.DeviceID != nil {
		_ = h.repo.ReleaseDevice(r.Context(), *t.DeviceID, systemActorID)
	}
	httpx.Success(w, http.StatusOK, map[string]string{"status": "cancelled"}, nil)
}

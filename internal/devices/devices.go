package devices

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"adlts/internal/platform/domain"
	"adlts/internal/platform/httpx"
	"adlts/internal/platform/runtime"
	"adlts/internal/platform/security"
	"adlts/internal/platform/store"
	"adlts/internal/scoring"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

type Handler struct {
	deps     runtime.Dependencies
	upgrader websocket.Upgrader
}

func New(deps runtime.Dependencies) Handler {
	return Handler{deps: deps, upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}}
}

func RegisterDeviceRoutes(r chi.Router, deps runtime.Dependencies) {
	h := New(deps)
	authMW := security.Authenticate(deps.Tokens, deps.Store)
	r.With(authMW, security.RequireRoles(domain.RoleAdmin, domain.RoleAuthority)).Post("/register", h.handleRegisterDevice)
}

type deviceRegisterRequest struct {
	MACAddress string `json:"mac_address"`
	Name       string `json:"name"`
}

type deviceHeartbeatRequest struct {
	DeviceID string `json:"device_id"`
	Secret   string `json:"secret"`
}

func (h Handler) handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
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
	store.Write(h.deps.Store, func() struct{} {
		h.deps.Store.Devices[device.ID] = device
		return struct{}{}
	})
	httpx.Success(w, http.StatusCreated, map[string]any{
		"device":        device,
		"secret":        device.Secret,
		"stream_url":    fmt.Sprintf("/ws/iot/stream/%s", device.ID),
		"heartbeat_url": "/admin/devices/heartbeat",
	}, nil)
}

func (h Handler) handleDeviceHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req deviceHeartbeatRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid heartbeat payload", err.Error())
		return
	}
	updated := store.Write(h.deps.Store, func() *domain.Device {
		device, exists := h.deps.Store.Devices[req.DeviceID]
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

func (h Handler) handleStream(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "device_id")
	deviceSecret := r.Header.Get("X-Device-Secret")
	if deviceSecret == "" {
		deviceSecret = r.URL.Query().Get("device_secret")
	}
	device, ok := h.deps.Store.FindDevice(deviceID)
	if !ok || device.Secret != deviceSecret {
		httpx.Failure(w, http.StatusUnauthorized, "INVALID_DEVICE", "device could not be authenticated", nil)
		return
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
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
		analysis := scoring.ProcessFrame(h.deps, deviceID, string(message), 0, "websocket")
		if err := conn.WriteJSON(analysis); err != nil {
			return
		}
	}
}

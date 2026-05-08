package ws

import (
	"net/http"
	"time"

	"adlts/internal/platform/httpx"
	"adlts/internal/platform/runtime"
	"adlts/internal/scoring"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

type Handler struct {
	deps     runtime.Dependencies
	upgrader websocket.Upgrader
}

func New(deps runtime.Dependencies) *Handler {
	return &Handler{
		deps:     deps,
		upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	}
}

func (h *Handler) handleStream(w http.ResponseWriter, r *http.Request) {
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

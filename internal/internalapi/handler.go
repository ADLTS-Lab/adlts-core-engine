package internalapi

import (
	"net/http"

	"adlts/internal/platform/httpx"
	"adlts/internal/platform/runtime"
	"adlts/internal/scoring"
)

type Handler struct {
	deps runtime.Dependencies
}

func New(deps runtime.Dependencies) *Handler {
	return &Handler{deps: deps}
}

type frameRequest struct {
	ExamID   string  `json:"exam_id,omitempty"`
	DeviceID string  `json:"device_id,omitempty"`
	Frame    string  `json:"frame,omitempty"`
	Speed    float64 `json:"speed,omitempty"`
	Source   string  `json:"source,omitempty"`
}

func (h *Handler) handleFrameProcess(w http.ResponseWriter, r *http.Request) {
	var req frameRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid frame payload", err.Error())
		return
	}
	if req.DeviceID == "" {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "device_id is required", nil)
		return
	}
	analysis := scoring.ProcessFrame(h.deps, req.DeviceID, req.Frame, req.Speed, req.Source)
	httpx.Success(w, http.StatusOK, analysis, nil)
}

package admin

import (
	"net/http"
	"sort"
	"time"

	"adlts/internal/platform/domain"
	"adlts/internal/platform/httpx"
	"adlts/internal/platform/runtime"
	"adlts/internal/platform/store"
)

type Handler struct {
	deps runtime.Dependencies
}

func New(deps runtime.Dependencies) *Handler {
	return &Handler{deps: deps}
}

type deviceHeartbeatRequest struct {
	DeviceID string `json:"device_id"`
	Secret   string `json:"secret"`
}

func (h *Handler) handleDeviceHeartbeat(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleHeatmap(w http.ResponseWriter, r *http.Request) {
	counts := map[string]int{}
	store.Read(h.deps.Store, func() struct{} {
		for _, exam := range h.deps.Store.Exams {
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

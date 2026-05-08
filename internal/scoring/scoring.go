package scoring

import (
	"net/http"
	"strings"
	"time"

	"adlts/internal/platform/domain"
	"adlts/internal/platform/httpx"
	"adlts/internal/platform/runtime"
	"adlts/internal/platform/store"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	deps runtime.Dependencies
}

func New(deps runtime.Dependencies) Handler { return Handler{deps: deps} }

func RegisterScoringRoutes(r chi.Router, deps runtime.Dependencies) {
	h := New(deps)
	r.With().Post("/analyze", func(w http.ResponseWriter, r *http.Request) {})
	// keep the public route; the app wiring uses this registration
	r.Post("/analyze", h.handleFrameProcess)
}

type frameRequest struct {
	ExamID   string  `json:"exam_id,omitempty"`
	DeviceID string  `json:"device_id,omitempty"`
	Frame    string  `json:"frame,omitempty"`
	Speed    float64 `json:"speed,omitempty"`
	Source   string  `json:"source,omitempty"`
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

func (h Handler) handleFrameProcess(w http.ResponseWriter, r *http.Request) {
	var req frameRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, 400, "INVALID_REQUEST", "invalid frame payload", err.Error())
		return
	}
	if req.DeviceID == "" {
		httpx.Failure(w, 400, "INVALID_REQUEST", "device_id is required", nil)
		return
	}
	analysis := ProcessFrame(h.deps, req.DeviceID, req.Frame, req.Speed, req.Source)
	httpx.Success(w, 200, analysis, nil)
}

func ProcessFrame(deps runtime.Dependencies, deviceID, frame string, speed float64, source string) analysisResult {
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
	store.Write(deps.Store, func() struct{} {
		deps.Store.Frames[analysis.FrameID] = &domain.FrameAnalysis{ID: analysis.FrameID, DeviceID: deviceID, Frame: frame, DetectedObjects: detected, Violations: violations, ScoreDelta: scoreDelta, Speed: speed, CreatedAt: now}
		if exam := latestExamForDevice(deps.Store, deviceID); exam != nil && (exam.Status == domain.ExamActive || exam.Status == domain.ExamInitiating) {
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

func latestExamForDevice(s *store.Store, deviceID string) *domain.Exam {
	var matched *domain.Exam
	store.Read(s, func() struct{} {
		for _, exam := range s.Exams {
			if exam.DeviceID == deviceID && (exam.Status == domain.ExamActive || exam.Status == domain.ExamInitiating) {
				matched = exam
			}
		}
		return struct{}{}
	})
	return matched
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

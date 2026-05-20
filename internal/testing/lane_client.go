package testing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// LaneDetectorClient calls the Python FastAPI lane-detector microservice.
// Endpoint: POST {baseURL}/detect
type LaneDetectorClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewLaneDetectorClient(baseURL string) *LaneDetectorClient {
	return &LaneDetectorClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 2 * time.Second},
	}
}

// ── §6.1 Request / Response types ────────────────────────────────────────────

// DetectRequest is the payload sent to POST /detect.
type DetectRequest struct {
	FrameB64     string `json:"frame_b64"`
	FrameSeqNo   int64  `json:"frame_seq_no"`
	TestID       string `json:"test_id"`
	SessionID    string `json:"session_id,omitempty"`
	ManeuverType string `json:"maneuver_type,omitempty"`
	PrevFrameB64 string `json:"prev_frame_b64,omitempty"`
	TimestampMs  int64  `json:"timestamp_ms"`
}

// RawLanes holds the raw polynomial-fit lane coordinates from the detector.
type RawLanes struct {
	LeftXs  []float64 `json:"left_xs"`
	LeftYs  []float64 `json:"left_ys"`
	RightXs []float64 `json:"right_xs"`
	RightYs []float64 `json:"right_ys"`
}

// DetectionResult is the full set of fields returned (or mocked) by the lane
// detector. QREvent is populated in-process by parsing the raw qr_event string.
type DetectionResult struct {
	FrameSeqNo       int64     `json:"frame_seq_no"`
	TimestampMs      int64     `json:"timestamp_ms"`
	LaneDetected     bool      `json:"lane_detected"`
	LaneDetectorMode string    `json:"lane_detector_mode"`
	CenterOffsetPx   float64   `json:"center_offset_px"`
	CurvatureR       float64   `json:"curvature_r"`
	CurvatureDir     string    `json:"curvature_dir"`
	LaneSymmetry     float64   `json:"lane_symmetry"`
	MotionDir        string    `json:"motion_dir"`
	IoUScore         float64   `json:"iou_score"`
	QREvent          *QREvent  `json:"-"` // parsed from qrEventRaw by Detect()
	ManeuverPhase    string    `json:"maneuver_phase"`
	RawLanes         *RawLanes `json:"raw_lanes,omitempty"`
	IsMocked         bool      `json:"is_mocked"`
}

// DetectResponse is a backward-compatibility type alias for DetectionResult.
// Existing code that references DetectResponse keeps compiling unchanged.
type DetectResponse = DetectionResult

// detectRawResponse is the internal JSON decode target for the Python detector
// response. It carries all DetectionResult fields plus the raw QR event string.
type detectRawResponse struct {
	FrameSeqNo       int64     `json:"frame_seq_no"`
	TimestampMs      int64     `json:"timestamp_ms"`
	LaneDetected     bool      `json:"lane_detected"`
	LaneDetectorMode string    `json:"lane_detector_mode"`
	CenterOffsetPx   float64   `json:"center_offset_px"`
	CurvatureR       float64   `json:"curvature_r"`
	CurvatureDir     string    `json:"curvature_dir"`
	LaneSymmetry     float64   `json:"lane_symmetry"`
	MotionDir        string    `json:"motion_dir"`
	IoUScore         float64   `json:"iou_score"`
	QREventRaw       *string   `json:"qr_event"` // "ADLTS:S:type:id" | "ADLTS:E:..." | null
	ManeuverPhase    string    `json:"maneuver_phase"`
	RawLanes         *RawLanes `json:"raw_lanes,omitempty"`
	IsMocked         bool      `json:"is_mocked"`
}

// Detect sends a DetectRequest to the lane detector service.
// If the service is unreachable it returns a mocked zero-result (IsMocked=true)
// so the test pipeline can continue without crashing.
func (c *LaneDetectorClient) Detect(ctx context.Context, req DetectRequest) (*DetectionResult, error) {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/detect", bytes.NewReader(body))
	if err != nil {
		return mockedDetect(), nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return mockedDetect(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return mockedDetect(), fmt.Errorf("lane-detector returned HTTP %d", resp.StatusCode)
	}

	var raw detectRawResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return mockedDetect(), nil
	}

	out := &DetectionResult{
		FrameSeqNo:       raw.FrameSeqNo,
		TimestampMs:      raw.TimestampMs,
		LaneDetected:     raw.LaneDetected,
		LaneDetectorMode: raw.LaneDetectorMode,
		CenterOffsetPx:   raw.CenterOffsetPx,
		CurvatureR:       raw.CurvatureR,
		CurvatureDir:     raw.CurvatureDir,
		LaneSymmetry:     raw.LaneSymmetry,
		MotionDir:        raw.MotionDir,
		IoUScore:         raw.IoUScore,
		QREvent:          parseQREvent(raw.QREventRaw),
		ManeuverPhase:    raw.ManeuverPhase,
		RawLanes:         raw.RawLanes,
		IsMocked:         raw.IsMocked,
	}
	return out, nil
}

// mockedDetect returns a safe zero-value DetectionResult with IsMocked=true.
func mockedDetect() *DetectionResult {
	return &DetectionResult{
		LaneDetected: false,
		CurvatureDir: "none",
		IoUScore:     0.0,
		IsMocked:     true,
	}
}

// parseQREvent parses a raw QR event string of the form
// "ADLTS:S:{maneuver_type}:{config_id}" or "ADLTS:E:{maneuver_type}:{config_id}".
// Returns nil for nil, empty, or unparseable input.
func parseQREvent(raw *string) *QREvent {
	if raw == nil || *raw == "" {
		return nil
	}
	parts := strings.SplitN(*raw, ":", 4)
	if len(parts) != 4 || parts[0] != "ADLTS" {
		return nil
	}
	var action string
	switch parts[1] {
	case "S":
		action = "start"
	case "E":
		action = "end"
	default:
		return nil
	}
	return &QREvent{
		Action:       action,
		ManeuverType: parts[2],
		ConfigID:     parts[3],
		Raw:          *raw,
	}
}

// ── §6.2 Score-maneuver client ────────────────────────────────────────────────

// ScoreManeuverClient calls the Python scorer microservice.
// Endpoint: POST {baseURL}/score_maneuver
type ScoreManeuverClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewScoreManeuverClient(baseURL string) *ScoreManeuverClient {
	return &ScoreManeuverClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ScoreManeuverRequest is the payload for POST /score_maneuver.
type ScoreManeuverRequest struct {
	ManeuverType     string         `json:"maneuver_type"`
	ManeuverConfigID string         `json:"maneuver_config_id"`
	SessionID        string         `json:"session_id"`
	TolerancePx      int            `json:"tolerance_px"`
	PassThreshold    float64        `json:"pass_threshold"`
	Frames           []ScoringFrame `json:"frames"`
}

// ScoringFrame is one frame entry in a ScoreManeuverRequest.
type ScoringFrame struct {
	FrameSeqNo     int64   `json:"frame_seq_no"`
	LaneDetected   bool    `json:"lane_detected"`
	CenterOffsetPx float64 `json:"center_offset_px"`
	CurvatureR     float64 `json:"curvature_r"`
	CurvatureDir   string  `json:"curvature_dir"`
	LaneSymmetry   float64 `json:"lane_symmetry"`
	MotionDir      string  `json:"motion_dir"`
	IoUScore       float64 `json:"iou_score"`
	ManeuverPhase  string  `json:"maneuver_phase"`
}

// ManeuverScoreResult is the response from POST /score_maneuver.
type ManeuverScoreResult struct {
	ManeuverType         string           `json:"maneuver_type"`
	SessionID            string           `json:"session_id"`
	Score                float64          `json:"score"`
	Passed               bool             `json:"passed"`
	CriticalFail         bool             `json:"critical_fail"`
	DimensionScores      *ScoreDimensions `json:"dimension_scores"`
	PhaseScores          ScorePhases      `json:"phase_scores"`
	WeakestPhase         string           `json:"weakest_phase"`
	MeanCenterOffsetPx   float64          `json:"mean_center_offset_px"`
	OffsetVariancePx     float64          `json:"offset_variance_px"`
	DirectionAccuracy    float64          `json:"direction_accuracy"`
	Events               []ScoringEvent   `json:"events"`
	EventCountBySeverity map[string]int   `json:"event_count_by_severity"`
}

// ScoreDimensions holds the per-dimension breakdown of a maneuver score.
type ScoreDimensions struct {
	Centering         float64  `json:"centering"`
	Direction         float64  `json:"direction"`
	Smoothness        float64  `json:"smoothness"`
	LaneQuality       float64  `json:"lane_quality"`
	ReverseCompliance *float64 `json:"reverse_compliance"` // null for non-reverse maneuvers
}

// ScorePhases holds the entry/body/exit breakdown of a maneuver score.
type ScorePhases struct {
	Entry float64 `json:"entry"`
	Body  float64 `json:"body"`
	Exit  float64 `json:"exit"`
}

// ScoringEvent is one discrete event reported by the scorer.
type ScoringEvent struct {
	EventType  string          `json:"event_type"`
	Severity   string          `json:"severity"`
	StartFrame int64           `json:"start_frame"`
	EndFrame   *int64          `json:"end_frame,omitempty"`
	Detail     json.RawMessage `json:"detail,omitempty"`
}

// ScoreManeuver posts the collected frames to /score_maneuver and returns the
// full scoring result. Returns an error on any HTTP != 200 or decode failure.
func (c *ScoreManeuverClient) ScoreManeuver(ctx context.Context, req ScoreManeuverRequest) (*ManeuverScoreResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("ScoreManeuver: marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/score_maneuver", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ScoreManeuver: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ScoreManeuver: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ScoreManeuver: scorer returned HTTP %d", resp.StatusCode)
	}

	var out ManeuverScoreResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ScoreManeuver: decode: %w", err)
	}
	return &out, nil
}

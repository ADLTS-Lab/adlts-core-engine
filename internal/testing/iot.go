package testing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"adlts/internal/domain"

	"github.com/google/uuid"
)

// HealthChecker probes the device stream before a test starts.
// It attempts up to maxRetries pings of the device's /health endpoint.
type HealthChecker struct {
	repo       *Repository
	httpClient *http.Client
	maxRetries int
}

func NewHealthChecker(repo *Repository) *HealthChecker {
	return &HealthChecker{
		repo:       repo,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		maxRetries: 3,
	}
}

// Check performs an IoT health check for the test, saves the result, and
// returns (passed bool, error).
//
// It calls GET {streamURL_base}/health and measures round-trip latency.
// The /health endpoint is expected to return HTTP 200.
func (h *HealthChecker) Check(ctx context.Context, test *domain.Test, streamURL string) (bool, error) {
	healthURL := streamURL + "/health"
	if streamURL == "" {
		return false, fmt.Errorf("stream URL is empty")
	}
	// Strip /stream suffix if someone passed the full stream URL
	if len(streamURL) > 7 && streamURL[len(streamURL)-7:] == "/stream" {
		healthURL = streamURL[:len(streamURL)-7] + "/health"
	}

	var (
		passed          = false
		streamReachable = false
		latencyMs       = 0
		camStatus       = domain.HealthFailed
		netStatus       = domain.HealthFailed
		lastErr         = ""
		attempts        = 0
	)

	for attempts < h.maxRetries {
		attempts++
		start := time.Now()
		resp, err := h.httpClient.Get(healthURL)
		latencyMs = int(time.Since(start).Milliseconds())

		if err != nil {
			lastErr = err.Error()
			netStatus = domain.HealthFailed
			time.Sleep(500 * time.Millisecond)
			continue
		}
		resp.Body.Close()

		streamReachable = resp.StatusCode == http.StatusOK
		if streamReachable {
			netStatus = domain.HealthOk
			camStatus = domain.HealthOk
			passed = true
			break
		}
		lastErr = fmt.Sprintf("HTTP %d from /health", resp.StatusCode)
		time.Sleep(500 * time.Millisecond)
	}

	check := &domain.IoTHealthCheck{
		ID:               uuid.New(),
		TestID:           test.ID,
		Passed:           passed,
		StreamReachable:  streamReachable,
		NetworkLatencyMs: latencyMs,
		CameraStatus:     camStatus,
		NetworkStatus:    netStatus,
		ErrorMessage:     lastErr,
		Attempts:         attempts,
		CheckedAt:        time.Now(),
	}
	if err := h.repo.InsertIoTHealthCheck(ctx, check); err != nil {
		return passed, fmt.Errorf("failed to persist health check: %w", err)
	}
	return passed, nil
}

// ---------------------------------------------------------------------------
// HealthDetail — full device health snapshot returned by GET /health/detail
// (§8.2)
// ---------------------------------------------------------------------------

type HealthDetail struct {
	DeviceCode      string `json:"device_code"`
	UptimeS         int    `json:"uptime_s"`
	WifiRSSI        int    `json:"wifi_rssi"`
	CameraOK        bool   `json:"camera_ok"`
	SDPresent       bool   `json:"sd_present"`
	SDFreeMB        int    `json:"sd_free_mb"`
	FlashPinOK      bool   `json:"flash_pin_ok"`
	LEDGreenOK      bool   `json:"led_green_ok"`
	LEDBlueOK       bool   `json:"led_blue_ok"`
	LEDRedOK        bool   `json:"led_red_ok"`
	StreamURL       string `json:"stream_url"`
	FirmwareVersion string `json:"firmware_version"`
}

// ---------------------------------------------------------------------------
// IoTClient — fire-and-forget command sender + detailed health poller
// (§8.3)
// ---------------------------------------------------------------------------

type IoTClient struct {
	streamURL  string // base URL, e.g. "http://192.168.1.42"
	httpClient *http.Client
}

// NewIoTClient creates an IoTClient targeting the given base URL.
func NewIoTClient(streamURL string) *IoTClient {
	return &IoTClient{
		streamURL:  streamURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// post marshals body to JSON and POSTs it to streamURL+path.
// Returns an error for any network failure or HTTP status >= 400.
func (c *IoTClient) post(path string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("iot post marshal: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.streamURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("iot post request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("iot post %s: %w", path, err)
	}
	defer resp.Body.Close()
	// Drain body so the connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("iot post %s: unexpected status %d", path, resp.StatusCode)
	}
	return nil
}

// SendStart notifies the device that a test run is beginning.
// POST /command/start  {"test_id": testID, "plan_name": planName}
func (c *IoTClient) SendStart(testID, planName string) error {
	return c.post("/command/start", map[string]interface{}{
		"test_id":   testID,
		"plan_name": planName,
	})
}

// SendEnd notifies the device that the test run has finished.
// POST /command/end  {"test_id": testID, "result": "pass"|"fail"}
func (c *IoTClient) SendEnd(testID string, passed bool) error {
	result := "fail"
	if passed {
		result = "pass"
	}
	return c.post("/command/end", map[string]interface{}{
		"test_id": testID,
		"result":  result,
	})
}

// SendAbort notifies the device that the test run was aborted.
// POST /command/abort  {"test_id": testID, "reason": reason}
func (c *IoTClient) SendAbort(testID, reason string) error {
	return c.post("/command/abort", map[string]interface{}{
		"test_id": testID,
		"reason":  reason,
	})
}

// SetFlash controls the device's flash LED.
// POST /command/flash  {"on": on, "brightness": brightness}
func (c *IoTClient) SetFlash(on bool, brightness int) error {
	return c.post("/command/flash", map[string]interface{}{
		"on":         on,
		"brightness": brightness,
	})
}

// HealthCheck fetches a full device health snapshot from GET /health/detail.
// It retries up to 3 times with a 500 ms pause between attempts, using a
// 2-second per-request timeout. Returns the decoded *HealthDetail or an error.
func (c *IoTClient) HealthCheck() (*HealthDetail, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	url := c.streamURL + "/health/detail"

	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("iot health/detail request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("iot health/detail attempt %d: %w", attempt, err)
			if attempt < maxRetries {
				time.Sleep(500 * time.Millisecond)
			}
			continue
		}

		if resp.StatusCode >= 400 {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("iot health/detail attempt %d: status %d", attempt, resp.StatusCode)
			if attempt < maxRetries {
				time.Sleep(500 * time.Millisecond)
			}
			continue
		}

		var detail HealthDetail
		if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("iot health/detail decode: %w", err)
		}
		_ = resp.Body.Close()
		return &detail, nil
	}

	return nil, lastErr
}

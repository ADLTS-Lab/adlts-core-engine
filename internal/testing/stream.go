package testing

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"adlts/internal/domain"
	minioclient "adlts/internal/platform/minio"

	"github.com/google/uuid"
	miniosdk "github.com/minio/minio-go/v7"
)

// Frame is a single captured frame from the ESP32-CAM MJPEG stream.
type Frame struct {
	SeqNo      int64
	Data       []byte // raw JPEG bytes
	CapturedAt time.Time
}

// ── QR / maneuver segmentation types ─────────────────────────────────────────

// QREvent is parsed from the raw "ADLTS:S/E:..." string returned by the Python
// detector. It signals the start or end of a named maneuver segment.
type QREvent struct {
	Action       string // "start" | "end"
	ManeuverType string
	ConfigID     string
	Raw          string
}

// ActiveManeuver tracks the currently open maneuver segment.
type ActiveManeuver struct {
	ManeuverType string
	ConfigID     string
	StartFrame   int64
	StartedAt    time.Time
}

// ManeuverSegmenter is driven by DetectionResult.QREvent events. It opens and
// closes maneuver sessions, collecting frames in between, and fires callbacks
// so the orchestrator can trigger scoring (wired in Phase 7).
type ManeuverSegmenter struct {
	current     *ActiveManeuver
	frameBuffer []DetectionResult
	onOpen      func(maneuverType, configID string, startFrame int64, startedAt time.Time)
	onClose     func(maneuverType, configID string, frames []DetectionResult, endedAt time.Time)
}

func NewManeuverSegmenter(
	onOpen func(maneuverType, configID string, startFrame int64, startedAt time.Time),
	onClose func(maneuverType, configID string, frames []DetectionResult, endedAt time.Time),
) *ManeuverSegmenter {
	return &ManeuverSegmenter{onOpen: onOpen, onClose: onClose}
}

// Feed processes one detection result. If a QR start/end event is present it
// opens or closes the active maneuver segment accordingly.
func (ms *ManeuverSegmenter) Feed(dr DetectionResult) {
	if dr.QREvent != nil {
		switch dr.QREvent.Action {
		case "start":
			if ms.current != nil && ms.onClose != nil {
				ms.onClose(ms.current.ManeuverType, ms.current.ConfigID, ms.frameBuffer, time.Now())
			}
			ms.current = &ActiveManeuver{
				ManeuverType: dr.QREvent.ManeuverType,
				ConfigID:     dr.QREvent.ConfigID,
				StartFrame:   dr.FrameSeqNo,
				StartedAt:    time.Now(),
			}
			ms.frameBuffer = nil
			if ms.onOpen != nil {
				ms.onOpen(dr.QREvent.ManeuverType, dr.QREvent.ConfigID, dr.FrameSeqNo, ms.current.StartedAt)
			}
		case "end":
			if ms.current != nil && ms.current.ConfigID == dr.QREvent.ConfigID {
				if ms.onClose != nil {
					ms.onClose(ms.current.ManeuverType, ms.current.ConfigID, ms.frameBuffer, time.Now())
				}
				ms.current = nil
				ms.frameBuffer = nil
			}
		}
	}
	if ms.current != nil {
		ms.frameBuffer = append(ms.frameBuffer, dr)
	}
}

// Flush force-closes any open maneuver at end of stream.
func (ms *ManeuverSegmenter) Flush() {
	if ms.current != nil && ms.onClose != nil {
		ms.onClose(ms.current.ManeuverType, ms.current.ConfigID, ms.frameBuffer, time.Now())
		ms.current = nil
		ms.frameBuffer = nil
	}
}

// ── MJPEG stream ingestor ─────────────────────────────────────────────────────

// StreamIngestor reads an MJPEG stream, decodes individual JPEG frames, and
// fans them out to detect and record channels. It runs until the stream is
// closed or the context is cancelled.
type StreamIngestor struct {
	streamURL  string
	detectCh   chan Frame // consumed by detection worker pool
	recordCh   chan Frame // consumed by RecordingEngine
	httpClient *http.Client
}

func NewStreamIngestor(streamURL string, detectBuf, recordBuf int) *StreamIngestor {
	return &StreamIngestor{
		streamURL:  streamURL,
		detectCh:   make(chan Frame, detectBuf),
		recordCh:   make(chan Frame, recordBuf),
		httpClient: &http.Client{Timeout: 0}, // no global timeout — streaming
	}
}

// Run ingests the MJPEG stream.
// Returns when the context is cancelled or the stream ends.
func (s *StreamIngestor) Run(ctx context.Context) {
	defer close(s.detectCh)
	defer close(s.recordCh)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.streamURL, nil)
	if err != nil {
		return
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// Parse the multipart boundary from Content-Type
	contentType := resp.Header.Get("Content-Type")
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return
	}
	boundary := params["boundary"]
	if boundary == "" {
		return
	}

	mr := multipart.NewReader(resp.Body, boundary)
	var seq int64

	for {
		part, err := mr.NextPart()
		if err == io.EOF || ctx.Err() != nil {
			return
		}
		if err != nil {
			time.Sleep(50 * time.Millisecond) // transient error — retry
			continue
		}

		data, err := io.ReadAll(part)
		part.Close()
		if err != nil || !bytes.HasPrefix(data, []byte{0xFF, 0xD8}) {
			continue // not a valid JPEG — skip
		}

		seq++
		f := Frame{SeqNo: seq, Data: data, CapturedAt: time.Now()}

		// Non-blocking fan-out to avoid back-pressure deadlock
		select {
		case s.detectCh <- f:
		default:
		}
		select {
		case s.recordCh <- f:
		default:
		}
	}
}

// ── Detection worker pool ────────────────────────────────────────────────────

// DetectionWorker drains detectCh, sends each frame to the lane detector,
// and accumulates FrameAnalysis rows in a buffer. Every flushInterval it
// batch-inserts the buffer into the DB.
type DetectionWorker struct {
	sessionID   uuid.UUID
	testID      uuid.UUID
	maneuver    *domain.ManeuverConfig
	detectCh    <-chan Frame
	laneClient  *LaneDetectorClient
	repo        *Repository
	segmenter   *ManeuverSegmenter // optional; wired in Phase 7
	mu          sync.Mutex
	analyses    []*domain.FrameAnalysis
	flushTicker *time.Ticker
}

// NewDetectionWorker creates a DetectionWorker. The optional trailing segmenter
// argument wires in a ManeuverSegmenter (Phase 7); pass nothing or nil for
// backward compatibility with callers that predate Phase 6.
func NewDetectionWorker(
	testID, sessionID uuid.UUID,
	maneuver *domain.ManeuverConfig,
	detectCh <-chan Frame,
	laneClient *LaneDetectorClient,
	repo *Repository,
	segmenter ...*ManeuverSegmenter,
) *DetectionWorker {
	var seg *ManeuverSegmenter
	if len(segmenter) > 0 {
		seg = segmenter[0]
	}
	return &DetectionWorker{
		sessionID:   sessionID,
		testID:      testID,
		maneuver:    maneuver,
		detectCh:    detectCh,
		laneClient:  laneClient,
		repo:        repo,
		segmenter:   seg,
		flushTicker: time.NewTicker(2 * time.Second),
	}
}

// Run processes frames until detectCh is closed or ctx is cancelled.
// Returns total frame count, lane-detected count, and avg IoU.
func (w *DetectionWorker) Run(ctx context.Context) (frameCount, laneCount int, avgIoU float64) {
	defer w.flushTicker.Stop()
	var totalIoU float64

	flush := func() {
		w.mu.Lock()
		batch := w.analyses
		w.analyses = nil
		w.mu.Unlock()
		if len(batch) > 0 {
			_ = w.repo.BatchInsertFrameAnalyses(ctx, batch)
		}
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case <-w.flushTicker.C:
			flush()
		case frame, ok := <-w.detectCh:
			if !ok {
				flush()
				return
			}

			// Build the full §6.1 request
			req := DetectRequest{
				FrameB64:     base64.StdEncoding.EncodeToString(frame.Data),
				FrameSeqNo:   frame.SeqNo,
				TestID:       w.testID.String(),
				SessionID:    w.sessionID.String(),
				ManeuverType: string(w.maneuver.ManeuverType),
				TimestampMs:  frame.CapturedAt.UnixMilli(),
			}
			result, _ := w.laneClient.Detect(ctx, req)
			if result == nil {
				result = mockedDetect()
			}

			curv := domain.CurvatureDir(result.CurvatureDir)
			if curv == "" {
				curv = domain.CurvatureNone
			}

			// Per-frame score: IoU ÷ 1.0 × 100 (simple linear mapping)
			frameScore := result.IoUScore * 100.0

			// Carry the raw QR event string into the DB row if present.
			var qrEventRaw *string
			if result.QREvent != nil {
				s := result.QREvent.Raw
				qrEventRaw = &s
			}

			fa := &domain.FrameAnalysis{
				ID:               uuid.New(),
				TestID:           w.testID,
				SessionID:        w.sessionID,
				FrameSeqNo:       frame.SeqNo,
				CapturedAt:       frame.CapturedAt,
				LaneDetected:     result.LaneDetected,
				CurvatureDir:     curv,
				IoUScore:         result.IoUScore,
				FrameScore:       frameScore,
				IsMocked:         result.IsMocked,
				LaneDetectorMode: result.LaneDetectorMode,
				CenterOffsetPx:   result.CenterOffsetPx,
				CurvatureR:       result.CurvatureR,
				LaneSymmetry:     result.LaneSymmetry,
				MotionDir:        result.MotionDir,
				QREvent:          qrEventRaw,
				ManeuverPhase:    result.ManeuverPhase,
				CreatedAt:        time.Now(),
			}

			frameCount++
			if result.LaneDetected {
				laneCount++
			}
			totalIoU += result.IoUScore

			w.mu.Lock()
			w.analyses = append(w.analyses, fa)
			w.mu.Unlock()

			// Drive the maneuver segmenter if one is wired in.
			if w.segmenter != nil {
				w.segmenter.Feed(*result)
			}
		}
	}
}

// ── Recording engine ──────────────────────────────────────────────────────────

// storageClient is the minimal interface RecordingEngine needs from the MinIO wrapper.
type storageClient interface {
	PutObject(ctx context.Context, key string, data []byte, ct string) error
}

// RecordingEngine consumes the recordCh channel from the StreamIngestor and
// writes each JPEG frame to MinIO under two paths:
//
//   - Full path (always):   recordings/{testID}/full/frame_{seqNo:08d}.jpg
//   - Session path (when activeManeuver != nil):
//     recordings/{testID}/{maneuverType}_{configID[:8]}/frame_{sessionSeq:08d}.jpg
//
// On completion it updates the test_recordings row to status='saved'.
type RecordingEngine struct {
	testID uuid.UUID
	prefix string
	ch     <-chan Frame
	minio  storageClient
	repo   *Repository

	mu             sync.Mutex
	activeManeuver *ActiveManeuver  // nil when not in a session
	sessionSeqs    map[string]int64 // configID -> per-session frame counter
}

func NewRecordingEngine(testID uuid.UUID, ch <-chan Frame, m storageClient, repo *Repository) *RecordingEngine {
	prefix := fmt.Sprintf("recordings/%s/full/", testID)
	return &RecordingEngine{
		testID:      testID,
		prefix:      prefix,
		ch:          ch,
		minio:       m,
		repo:        repo,
		sessionSeqs: make(map[string]int64),
	}
}

// SetActiveManeuver updates the currently-active maneuver segment (thread-safe).
// Pass nil to clear the active maneuver.
func (e *RecordingEngine) SetActiveManeuver(m *ActiveManeuver) {
	e.mu.Lock()
	e.activeManeuver = m
	e.mu.Unlock()
}

// Run drains the channel writing frames to MinIO, then finalizes the recording.
func (e *RecordingEngine) Run(ctx context.Context) {
	var frameCount int
	var totalBytes int64
	status := "saved"

	_ = e.repo.CreateTestRecording(ctx, e.testID, e.prefix)

	for frame := range e.ch {
		if ctx.Err() != nil {
			status = "failed"
			break
		}

		// ── Full path (always written) ─────────────────────────────────
		fullKey := fmt.Sprintf("%sframe_%08d.jpg", e.prefix, frame.SeqNo)
		if err := e.minio.PutObject(ctx, fullKey, frame.Data, "image/jpeg"); err != nil {
			status = "failed"
			continue
		}
		frameCount++
		totalBytes += int64(len(frame.Data))

		// ── Session path (only when inside a maneuver segment) ─────────
		e.mu.Lock()
		am := e.activeManeuver
		e.mu.Unlock()

		if am != nil {
			configIDShort := am.ConfigID
			if len(configIDShort) > 8 {
				configIDShort = configIDShort[:8]
			}
			e.mu.Lock()
			sessionSeq := e.sessionSeqs[am.ConfigID]
			e.sessionSeqs[am.ConfigID] = sessionSeq + 1
			e.mu.Unlock()

			sessionKey := fmt.Sprintf("recordings/%s/%s_%s/frame_%08d.jpg",
				e.testID, am.ManeuverType, configIDShort, sessionSeq)
			_ = e.minio.PutObject(ctx, sessionKey, frame.Data, "image/jpeg")
		}
	}

	_ = e.repo.FinalizeTestRecording(ctx, e.testID, frameCount, totalBytes, status)
}

// StitchVideo downloads JPEG frames from a MinIO prefix, stitches them into an
// MP4 with ffmpeg, uploads the result, and records the video_key.
// It runs in a goroutine launched by the orchestrator after test/session
// completion. Requires ffmpeg to be installed on the server.
func (e *RecordingEngine) StitchVideo(ctx context.Context, prefix string, m *minioclient.Client) {
	testIDStr := e.testID.String()
	tmpDir := fmt.Sprintf("/tmp/stitch_%s_%d", testIDStr, time.Now().UnixNano())
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return
	}
	defer os.RemoveAll(tmpDir)

	// ── List + download frames ────────────────────────────────────────
	objCh := m.Inner().ListObjects(ctx, m.Bucket(), miniosdk.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	for obj := range objCh {
		if obj.Err != nil {
			continue
		}
		data, err := m.GetObject(ctx, obj.Key)
		if err != nil {
			continue
		}
		base := path.Base(obj.Key)
		localPath := fmt.Sprintf("%s/%s", tmpDir, base)
		if err := os.WriteFile(localPath, data, 0o644); err != nil {
			continue
		}
	}

	// ── Run ffmpeg ────────────────────────────────────────────────────
	mp4Path := fmt.Sprintf("%s/out.mp4", tmpDir)
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-framerate", "15",
		"-pattern_type", "glob", "-i", fmt.Sprintf("%s/frame_*.jpg", tmpDir),
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-y", mp4Path)
	if err := cmd.Run(); err != nil {
		return // ffmpeg not installed or no frames — silent fail
	}

	// ── Upload MP4 ────────────────────────────────────────────────────
	data, err := os.ReadFile(mp4Path)
	if err != nil {
		return
	}
	mp4Key := strings.TrimSuffix(prefix, "/") + ".mp4"
	if err := e.minio.PutObject(ctx, mp4Key, data, "video/mp4"); err != nil {
		return
	}

	// ── Record video key ──────────────────────────────────────────────
	_ = e.repo.SetVideoKey(ctx, e.testID, mp4Key)
}

// ── QR detection helper ───────────────────────────────────────────────────────

// containsQRValue checks whether the raw JPEG bytes contain the expected QR
// code string using a simple byte-level substring search on the JPEG comment
// block. This is a lightweight heuristic; a full decode is done by the scoring
// engine only when the frame count reaches min_frames_required.
func containsQRValue(data []byte, qrValue string) bool {
	return strings.Contains(string(data), qrValue)
}

package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"adlts/internal/domain"
	minioclient "adlts/internal/platform/minio"

	"github.com/google/uuid"
)

// ScoringEngine computes the weighted score for a completed test.
type ScoringEngine struct {
	repo        *Repository
	scoreClient *ScoreManeuverClient // nil → fall back to legacy IoU-based scoring
}

// NewScoringEngine creates a ScoringEngine.
// Pass nil for scoreClient to fall back to the legacy IoU-based scoring method.
func NewScoringEngine(repo *Repository, scoreClient *ScoreManeuverClient) *ScoringEngine {
	return &ScoringEngine{repo: repo, scoreClient: scoreClient}
}

// ScoreSession computes the score for a single test session from its raw
// DetectionWorker metrics and persists a SessionResult row.
//
// Scoring formula:
//   - laneDetectedPct = laneCount / frameCount × 100
//   - avgIoU = totalIoU / frameCount
//   - sessionScore = laneDetectedPct × 0.5 + avgIoU × 100 × 0.5
//   - passed = sessionScore >= plan.PassThreshold
func (e *ScoringEngine) ScoreSession(
	ctx context.Context,
	testID uuid.UUID,
	session *domain.TestSession,
	maneuver *domain.ManeuverConfig,
	frameCount, laneCount int,
	totalIoU float64,
	passThreshold float64,
) (*domain.SessionResult, error) {
	if frameCount == 0 {
		frameCount = 1 // avoid division by zero
	}
	laneDetectedPct := float64(laneCount) / float64(frameCount) * 100.0
	avgIoU := totalIoU / float64(frameCount)
	sessionScore := laneDetectedPct*0.5 + avgIoU*100.0*0.5
	passed := sessionScore >= passThreshold

	sr := &domain.SessionResult{
		ID:              uuid.New(),
		TestID:          testID,
		SessionID:       session.ID,
		ManeuverID:      maneuver.ID,
		SequenceNumber:  session.SequenceNumber,
		Score:           sessionScore,
		Weight:          maneuver.Weight,
		Passed:          passed,
		FrameCount:      frameCount,
		LaneDetectedPct: laneDetectedPct,
		AvgIoU:          avgIoU,
	}

	if err := e.repo.InsertSessionResult(ctx, sr); err != nil {
		return nil, err
	}

	// Update the test_sessions row with scored values
	_ = e.repo.UpdateTestSessionScored(ctx, session.ID, sessionScore, passed, avgIoU, frameCount, laneCount)

	return sr, nil
}

// ComputeWeightedTotal computes the weighted average score from all session
// results for a test and persists the final score back to the tests table.
//
// Weighted formula:
//
//	weighted_total = Σ(session.score × session.weight) / Σ(session.weight)
func (e *ScoringEngine) ComputeWeightedTotal(
	ctx context.Context,
	testID uuid.UUID,
	sessionResults []*domain.SessionResult,
	passThreshold float64,
	actorID uuid.UUID,
) (score float64, passed bool, err error) {
	var weightedSum, weightSum float64
	for _, sr := range sessionResults {
		weightedSum += sr.Score * sr.Weight
		weightSum += sr.Weight
	}
	if weightSum > 0 {
		score = weightedSum / weightSum
	}
	passed = score >= passThreshold

	err = e.repo.UpdateTestFields(ctx, testID, map[string]any{
		"weighted_total_score": score,
		"passed":               passed,
	}, actorID)
	return score, passed, err
}

// ScoreManeuver calls the Python /score_maneuver endpoint, persists all events
// and the full SessionResult row.
// Falls back to IoU-based legacy scoring if Python is unreachable or scoreClient is nil.
func (e *ScoringEngine) ScoreManeuver(
	ctx context.Context,
	testID, sessionID uuid.UUID,
	maneuver *domain.ManeuverConfig,
	frames []DetectionResult,
) error {
	if e.scoreClient == nil || len(frames) == 0 {
		return e.legacyScoreFromFrames(ctx, testID, sessionID, maneuver, frames)
	}

	// Build the ScoringFrame slice
	scoringFrames := make([]ScoringFrame, len(frames))
	for i, f := range frames {
		scoringFrames[i] = ScoringFrame{
			FrameSeqNo:     f.FrameSeqNo,
			LaneDetected:   f.LaneDetected,
			CenterOffsetPx: f.CenterOffsetPx,
			CurvatureR:     f.CurvatureR,
			CurvatureDir:   f.CurvatureDir,
			LaneSymmetry:   f.LaneSymmetry,
			MotionDir:      f.MotionDir,
			IoUScore:       f.IoUScore,
			ManeuverPhase:  f.ManeuverPhase,
		}
	}

	req := ScoreManeuverRequest{
		ManeuverType:     string(maneuver.ManeuverType),
		ManeuverConfigID: maneuver.ID.String(),
		SessionID:        sessionID.String(),
		TolerancePx:      maneuver.TolerancePx,
		PassThreshold:    maneuver.PassThreshold,
		Frames:           scoringFrames,
	}

	result, err := e.scoreClient.ScoreManeuver(ctx, req)
	if err != nil {
		// Python scorer unreachable — fall back to legacy scoring
		return e.legacyScoreFromFrames(ctx, testID, sessionID, maneuver, frames)
	}

	// Persist each event returned by the scorer
	for _, ev := range result.Events {
		me := domain.ManeuverEvent{
			ID:           uuid.New(),
			TestID:       testID,
			SessionID:    sessionID,
			ManeuverType: string(maneuver.ManeuverType),
			EventType:    ev.EventType,
			Severity:     ev.Severity,
			StartFrame:   ev.StartFrame,
			EndFrame:     ev.EndFrame,
			Detail:       ev.Detail,
			CreatedAt:    time.Now(),
		}
		_ = e.repo.InsertManeuverEvent(ctx, me)
	}

	// Marshal DimensionScores → domain.JSONB
	var dimScoresJSON domain.JSONB
	if result.DimensionScores != nil {
		dimScoresJSON, _ = json.Marshal(result.DimensionScores)
	}

	// Marshal EventCountBySeverity → domain.JSONB
	var evCountJSON domain.JSONB
	if result.EventCountBySeverity != nil {
		evCountJSON, _ = json.Marshal(result.EventCountBySeverity)
	}

	// Compute frame stats from the frames slice
	var frameCount, laneDetectedCount int
	var totalIoU float64
	for _, f := range frames {
		frameCount++
		if f.LaneDetected {
			laneDetectedCount++
		}
		totalIoU += f.IoUScore
	}
	var laneDetectedPct, avgIoU float64
	if frameCount > 0 {
		laneDetectedPct = float64(laneDetectedCount) / float64(frameCount) * 100.0
		avgIoU = totalIoU / float64(frameCount)
	}

	sr := domain.SessionResult{
		ID:                   uuid.New(),
		TestID:               testID,
		SessionID:            sessionID,
		ManeuverID:           maneuver.ID,
		ManeuverType:         maneuver.ManeuverType,
		SequenceNumber:       maneuver.SequenceNumber,
		Score:                result.Score,
		Weight:               maneuver.Weight,
		Passed:               result.Passed,
		CriticalFail:         result.CriticalFail,
		FrameCount:           frameCount,
		LaneDetectedPct:      laneDetectedPct,
		AvgIoU:               avgIoU,
		MeanCenterOffset:     result.MeanCenterOffsetPx,
		OffsetVariance:       result.OffsetVariancePx,
		DimensionScores:      dimScoresJSON,
		EventCountBySeverity: evCountJSON,
		WeakestPhase:         result.WeakestPhase,
	}

	if err := e.repo.UpsertSessionResult(ctx, sr); err != nil {
		return err
	}

	return e.repo.UpdateTestSessionScored(ctx, sessionID, result.Score, result.Passed, avgIoU, frameCount, laneDetectedCount)
}

// legacyScoreFromFrames computes a simple IoU-based score when the Python scorer
// is unavailable or scoreClient is nil.
func (e *ScoringEngine) legacyScoreFromFrames(
	ctx context.Context,
	testID, sessionID uuid.UUID,
	maneuver *domain.ManeuverConfig,
	frames []DetectionResult,
) error {
	var frameCount, laneDetectedCount int
	var totalIoU float64
	for _, f := range frames {
		frameCount++
		if f.LaneDetected {
			laneDetectedCount++
		}
		totalIoU += f.IoUScore
	}
	if frameCount == 0 {
		frameCount = 1 // avoid division by zero
	}
	laneDetectedPct := float64(laneDetectedCount) / float64(frameCount) * 100.0
	avgIoU := totalIoU / float64(frameCount)
	score := laneDetectedPct*0.5 + avgIoU*100.0*0.5
	passed := score >= maneuver.PassThreshold

	emptyJSON := domain.JSONB("{}")
	sr := domain.SessionResult{
		ID:                   uuid.New(),
		TestID:               testID,
		SessionID:            sessionID,
		ManeuverID:           maneuver.ID,
		ManeuverType:         maneuver.ManeuverType,
		SequenceNumber:       maneuver.SequenceNumber,
		Score:                score,
		Weight:               maneuver.Weight,
		Passed:               passed,
		CriticalFail:         false,
		FrameCount:           frameCount,
		LaneDetectedPct:      laneDetectedPct,
		AvgIoU:               avgIoU,
		DimensionScores:      emptyJSON,
		EventCountBySeverity: emptyJSON,
	}

	if err := e.repo.UpsertSessionResult(ctx, sr); err != nil {
		return err
	}

	return e.repo.UpdateTestSessionScored(ctx, sessionID, score, passed, avgIoU, frameCount, laneDetectedCount)
}

// FinalizeTest computes the weighted total score, commits test_results, fires
// narrative generation asynchronously (must not block or fail the test).
func (e *ScoringEngine) FinalizeTest(
	ctx context.Context,
	testID uuid.UUID,
	test *domain.Test,
	plan *domain.TestPlan,
	narrative *NarrativeGenerator,
	actorID uuid.UUID,
) error {
	srs, err := e.repo.GetAllSessionResults(ctx, testID)
	if err != nil {
		return err
	}

	var weightedSum, weightSum float64
	var anyCriticalFail bool
	weakestManeuver := ""
	var weakestScore float64 = 101.0
	var weakestSet bool

	for _, sr := range srs {
		weightedSum += sr.Score * sr.Weight
		weightSum += sr.Weight
		if sr.CriticalFail {
			anyCriticalFail = true
		}
		if !weakestSet || sr.Score < weakestScore {
			weakestScore = sr.Score
			weakestManeuver = string(sr.ManeuverType)
			weakestSet = true
		}
	}

	var finalScore float64
	if weightSum > 0 {
		finalScore = weightedSum / weightSum
	}
	passed := finalScore >= plan.PassThreshold && !anyCriticalFail

	// Build score breakdown JSON
	type breakdownEntry struct {
		ManeuverType string  `json:"maneuver_type"`
		Score        float64 `json:"score"`
		Passed       bool    `json:"passed"`
		Weight       float64 `json:"weight"`
		CriticalFail bool    `json:"critical_fail"`
	}
	entries := make([]breakdownEntry, 0, len(srs))
	for _, sr := range srs {
		entries = append(entries, breakdownEntry{
			ManeuverType: string(sr.ManeuverType),
			Score:        sr.Score,
			Passed:       sr.Passed,
			Weight:       sr.Weight,
			CriticalFail: sr.CriticalFail,
		})
	}
	breakdownJSON, _ := json.Marshal(entries)

	tr := &domain.TestResult{
		ID:                 uuid.New(),
		TestID:             testID,
		WeightedTotalScore: finalScore,
		Passed:             passed,
		PassThreshold:      plan.PassThreshold,
		AnyCriticalFail:    anyCriticalFail,
		WeakestManeuver:    weakestManeuver,
		ScoreBreakdown:     domain.JSONB(breakdownJSON),
		NarrativeModel:     "pending", // filled in asynchronously below
	}
	_ = e.repo.InsertTestResult(ctx, tr)
	_ = e.repo.CompleteTest(ctx, testID, finalScore, passed, actorID)

	// Fire narrative generation asynchronously — must not block or fail the test
	if narrative != nil {
		go func() {
			narr, _ := narrative.Generate(context.Background(), &NarrativeInput{
				TestID:         testID.String(),
				LevelCode:      test.TestLevelCode,
				TotalScore:     finalScore,
				Passed:         passed,
				PassThreshold:  plan.PassThreshold,
				SessionResults: srs,
			})
			if narr == nil {
				narr = &NarrativeOutput{Overall: "Test completed.", ModelUsed: "fallback"}
			}
			_ = e.repo.UpdateTestResultNarrative(context.Background(), testID,
				narr.Overall, narr.Strengths, narr.Weaknesses, narr.RecommendedFocus, narr.ModelUsed)
		}()
	}

	return nil
}

// ── IoU calculation (bounding box overlap) ───────────────────────────────────

// IoU computes Intersection over Union for two bounding boxes.
// Each box is [x1, y1, x2, y2] in pixel coordinates.
func IoU(boxA, boxB [4]float64) float64 {
	interX1 := max64(boxA[0], boxB[0])
	interY1 := max64(boxA[1], boxB[1])
	interX2 := min64(boxA[2], boxB[2])
	interY2 := min64(boxA[3], boxB[3])

	interW := interX2 - interX1
	interH := interY2 - interY1
	if interW <= 0 || interH <= 0 {
		return 0.0
	}
	intersection := interW * interH

	areaA := (boxA[2] - boxA[0]) * (boxA[3] - boxA[1])
	areaB := (boxB[2] - boxB[0]) * (boxB[3] - boxB[1])
	union := areaA + areaB - intersection
	if union <= 0 {
		return 0.0
	}
	return intersection / union
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// ── Test orchestrator ─────────────────────────────────────────────────────────

// Orchestrator wires together the IoT check, stream ingestor, detection worker,
// recording engine, and scoring engine for a single test lifecycle.
type Orchestrator struct {
	repo         *Repository
	healthCheck  *HealthChecker
	laneClient   *LaneDetectorClient
	scoring      *ScoringEngine
	narrative    *NarrativeGenerator
	minioStorage storageClient
	iotClient    *IoTClient          // nil when not configured
	minioFull    *minioclient.Client // for StitchVideo; may be nil
}

// NewOrchestrator creates an Orchestrator.
//   - scoreClient: pass nil to disable Python scoring (uses legacy IoU fallback)
//   - iotClient:   pass nil to skip IoT commands
//   - minioFull:   pass nil to skip async video stitching
func NewOrchestrator(
	repo *Repository,
	laneClient *LaneDetectorClient,
	scoreClient *ScoreManeuverClient,
	narrative *NarrativeGenerator,
	minioStorage storageClient,
	iotClient *IoTClient,
	minioFull *minioclient.Client,
) *Orchestrator {
	return &Orchestrator{
		repo:         repo,
		healthCheck:  NewHealthChecker(repo),
		laneClient:   laneClient,
		scoring:      NewScoringEngine(repo, scoreClient),
		narrative:    narrative,
		minioStorage: minioStorage,
		iotClient:    iotClient,
		minioFull:    minioFull,
	}
}

// Run executes the full test lifecycle in a background goroutine.
// It is started by the admin's "end IoT check + start" trigger.
func (o *Orchestrator) Run(ctx context.Context, test *domain.Test, plan *domain.TestPlan, streamURL string) {
	go o.runLifecycle(ctx, test, plan, streamURL)
}

func (o *Orchestrator) runLifecycle(ctx context.Context, test *domain.Test, plan *domain.TestPlan, streamURL string) {
	actorID := systemActorID

	// ── 1. IoT health check ────────────────────────────────────────────────────
	_ = o.repo.UpdateTestStatus(ctx, test.ID, domain.TestStatusIoTHealth, "iot_health_start_at", actorID)
	passed, err := o.healthCheck.Check(ctx, test, streamURL)
	if err != nil || !passed {
		_ = o.repo.AbortTest(ctx, test.ID, domain.AbortHealthCheckFailed, actorID)
		if test.DeviceID != nil {
			_ = o.repo.ReleaseDevice(ctx, *test.DeviceID, actorID)
		}
		return
	}

	// ── 2. Load maneuvers ──────────────────────────────────────────────────────
	maneuvers, err := o.repo.ManeuversByPlanID(ctx, plan.ID)
	if err != nil || len(maneuvers) == 0 {
		_ = o.repo.AbortTest(ctx, test.ID, domain.AbortSystemError, actorID)
		return
	}

	// ── 3. Send IoT Start ─────────────────────────────────────────────────────
	// Construct per device when test starts, using streamURL
	iotClient := NewIoTClient(streamURL)
	if iotClient != nil {
		_ = iotClient.SendStart(test.ID.String(), plan.Name)
	}

	// ── 4. Start stream + recording ───────────────────────────────────────────
	_ = o.repo.StartTest(ctx, test.ID, actorID)
	ingestor := NewStreamIngestor(streamURL, 30, 30)
	recEngine := NewRecordingEngine(test.ID, ingestor.recordCh, o.minioStorage, o.repo)

	streamCtx, cancelStream := context.WithCancel(ctx)
	defer cancelStream()

	go ingestor.Run(streamCtx)
	go recEngine.Run(ctx)

	// ── 5. Event-driven maneuver loop ─────────────────────────────────────────
	for _, maneuver := range maneuvers {
		session := &domain.TestSession{
			ID:             uuid.New(),
			TestID:         test.ID,
			ManeuverID:     maneuver.ID,
			ManeuverType:   maneuver.ManeuverType,
			SequenceNumber: maneuver.SequenceNumber,
			StartedAt:      time.Now(),
			QRStartData:    "ADLTS:S:" + string(maneuver.ManeuverType) + ":" + maneuver.ID.String(),
		}
		_ = o.repo.InsertTestSession(ctx, session)

		// Track whether ScoreManeuver was called (via segmenter onClose)
		var scoringCalled sync.Once

		segmenter := NewManeuverSegmenter(
			func(mType, configID string, startFrame int64, startedAt time.Time) {
				// QR start confirmed; update recording engine
				recEngine.SetActiveManeuver(&ActiveManeuver{
					ManeuverType: mType,
					ConfigID:     configID,
					StartFrame:   startFrame,
					StartedAt:    startedAt,
				})
			},
			func(mType, configID string, frames []DetectionResult, endedAt time.Time) {
				scoringCalled.Do(func() {
					recEngine.SetActiveManeuver(nil)
					_ = o.scoring.ScoreManeuver(ctx, test.ID, session.ID, maneuver, frames)
				})
			},
		)

		// Inject a synthetic QR-start so the segmenter begins buffering immediately
		// even if the physical QR card isn't scanned. When the QR card IS scanned,
		// the segmenter will fire onClose and re-open with the real configID.
		segmenter.Feed(DetectionResult{
			FrameSeqNo: 0,
			QREvent: &QREvent{
				Action:       "start",
				ManeuverType: string(maneuver.ManeuverType),
				ConfigID:     maneuver.ID.String(),
			},
		})

		sessionCtx, cancelSession := context.WithCancel(streamCtx)
		sessionDetectCh := make(chan Frame, 30)
		worker := NewDetectionWorker(test.ID, session.ID, maneuver, sessionDetectCh, o.laneClient, o.repo, segmenter)

		go func() {
			defer close(sessionDetectCh)
			for f := range ingestor.detectCh {
				select {
				case sessionDetectCh <- f:
				case <-sessionCtx.Done():
					return
				}
			}
		}()

		worker.Run(sessionCtx)
		cancelSession()

		// Flush triggers onClose with buffered frames if QR end wasn't seen
		segmenter.Flush()
	}

	// ── 6. Stop stream ─────────────────────────────────────────────────────────
	_ = o.repo.SetFinishing(ctx, test.ID, actorID)
	cancelStream()

	// ── 7. FinalizeTest (weighted score + narrative async) ────────────────────
	if err := o.scoring.FinalizeTest(ctx, test.ID, test, plan, o.narrative, actorID); err != nil {
		// Even on error, continue the lifecycle to release the device
	}

	// ── 8. Send IoT End ────────────────────────────────────────────────────────
	if iotClient != nil {
		tr, _ := o.repo.TestResultByTestID(ctx, test.ID)
		passed := tr != nil && tr.Passed
		_ = iotClient.SendEnd(test.ID.String(), passed)
	}

	// ── 9. Start async video stitching ─────────────────────────────────────────
	if o.minioFull != nil {
		fullPrefix := fmt.Sprintf("recordings/%s/full", test.ID)
		go recEngine.StitchVideo(context.Background(), fullPrefix, o.minioFull)
	}

	// ── 10. Release device ─────────────────────────────────────────────────────
	if test.DeviceID != nil {
		_ = o.repo.ReleaseDevice(ctx, *test.DeviceID, actorID)
	}
}

// AbortTest aborts a running test externally (admin or stream-loss event).
func (o *Orchestrator) AbortTest(ctx context.Context, testID uuid.UUID, deviceID *uuid.UUID, reason domain.AbortReason) {
	_ = o.repo.AbortTest(ctx, testID, reason, systemActorID)
	if deviceID != nil {
		// Release the device back to active state
		_ = o.repo.ReleaseDevice(ctx, *deviceID, systemActorID)

		// Try to construct a per-device IoT client using the device's StreamURL
		if d, err := o.repo.DeviceByID(ctx, *deviceID); err == nil && d != nil {
			iot := NewIoTClient(d.StreamURL)
			if iot != nil {
				_ = iot.SendAbort(testID.String(), string(reason))
			}
		}
		return
	}

	// Fallback: if no deviceID provided, use the orchestrator-level client if configured
	if o.iotClient != nil {
		_ = o.iotClient.SendAbort(testID.String(), string(reason))
	}
}

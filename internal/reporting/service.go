package reporting

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sync/singleflight"
)
type Service struct {
	testingCore TestingCoreClient
	identity    IdentityClient
	geminiLLM   GeminiClient
	analytics   *AnalyticsEngine
	renderer    *Renderer
	outputDir   string
	group       singleflight.Group
	logger      Logger
}

type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

func NewService(testingCore TestingCoreClient, identity IdentityClient, geminiLLM GeminiClient, renderer *Renderer, outputDir string, logger Logger) *Service {
	if renderer == nil {
		renderer, _ = NewRenderer()
	}
	if outputDir == "" {
		outputDir = "../generated-reports"
	}
	return &Service{
		testingCore: testingCore,
		identity:    identity,
		geminiLLM:   geminiLLM,
		analytics:   NewAnalyticsEngine(),
		renderer:    renderer,
		outputDir:   outputDir,
		logger:      logger,
	}
}

func (s *Service) CachedPDFPath(testID string) string {
	return filepath.Join(s.outputDir, testID, "report.pdf")
}

func (s *Service) GenerateReport(ctx context.Context, testID string) ([]byte, error) {
	value, err, _ := s.group.Do(testID, func() (any, error) {
		return s.generate(ctx, testID)
	})
	if err != nil {
		return nil, err
	}
	return value.([]byte), nil
}

func (s *Service) generate(ctx context.Context, testID string) ([]byte, error) {
	cacheDir := filepath.Join(s.outputDir, testID)
	pdfPath := filepath.Join(cacheDir, "report.pdf")
	if cached, err := os.ReadFile(pdfPath); err == nil && len(cached) > 0 {
		return cached, nil
	}

	test, err := s.testingCore.GetTest(ctx, testID)
	if err != nil {
		return nil, err
	}
	if test.Status != "completed" && test.CompletedAt == nil {
		return nil, fmt.Errorf("test %s is not completed", testID)
	}
	result, err := s.testingCore.GetResult(ctx, testID)
	if err != nil {
		return nil, err
	}
	sessions, err := s.testingCore.ListSessions(ctx, testID)
	if err != nil {
		return nil, err
	}
	events := make(map[string][]ManeuverEvent, len(sessions))
	for _, session := range sessions {
		items, err := s.testingCore.ListEvents(ctx, testID, session.ID.String())
		if err != nil {
			return nil, err
		}
		events[session.ID.String()] = items
	}
	candidate, err := s.identity.GetCandidate(ctx, test.CandidateID)
	if err != nil {
		return nil, err
	}

	ctxModel := ReportContext{
		Test:      test,
		Result:    result,
		Sessions:  sessions,
		Events:    events,
		Candidate: candidate,
	}
	analytics := s.analytics.Analyze(ctxModel)
	analyticsJSON, err := AnalyticsPayload(analytics)
	if err != nil {
		return nil, err
	}
	narrative, err := s.geminiLLM.GenerateNarrative(ctx, analytics)
	if err != nil {
		return nil, err
	}

	data := ReportData{
		GeneratedAt: time.Now().UTC(),
		Test:        test,
		Result:      result,
		Candidate:   candidate,
		Analytics:   analytics,
		Narrative:   narrative,
	}

	html, err := s.renderer.RenderHTML(data)
	if err != nil {
		return nil, err
	}
	pdf, err := RenderPDF(ctx, html)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "analytics.json"), analyticsJSON, 0o644); err != nil {
		return nil, err
	}
	if narrativeJSON, err := json.MarshalIndent(narrative, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(cacheDir, "narrative.json"), narrativeJSON, 0o644)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "report.html"), html, 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(pdfPath, pdf, 0o644); err != nil {
		return nil, err
	}
	return pdf, nil
}

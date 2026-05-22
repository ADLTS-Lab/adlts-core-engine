package appeal_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"adlts/internal/appeal"
	"adlts/internal/platform/db"
	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AppealTestSuite struct {
	suite.Suite
	db     *pgxpool.Pool
	router chi.Router
	tokens *security.Manager

	testCenterID uuid.UUID
	candidateID  uuid.UUID
	candidateT   string
	expertID     uuid.UUID
	expertT      string

	sessionID uuid.UUID
	testID    uuid.UUID
}

func (s *AppealTestSuite) SetupSuite() {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:password@localhost:5433/adlts_test?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		s.T().Skip("skipping appeal integration tests: Postgres not reachable; set TEST_DATABASE_URL to run")
		return
	}
	s.db = pool

	schema001, err := os.ReadFile("../../migrations/001_schema.sql")
	require.NoError(s.T(), err)
	schema002, err := os.ReadFile("../../migrations/002_add_testid_to_appeals.sql")
	require.NoError(s.T(), err)
	schema003, err := os.ReadFile("../../migrations/003_create_appeal_evidence.sql")
	require.NoError(s.T(), err)

	_, _ = s.db.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
	_, err = s.db.Exec(ctx, string(schema001))
	require.NoError(s.T(), err)
	_, err = s.db.Exec(ctx, string(schema002))
	require.NoError(s.T(), err)
	_, err = s.db.Exec(ctx, string(schema003))
	require.NoError(s.T(), err)

	s.tokens = security.NewManager("test-secret-min-32-chars-long-12345678")

	repo := appeal.NewRepository(s.db)
	svc := appeal.NewService(repo, s.db)
	h := appeal.NewHandler(svc)

	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		api.Group(func(prv chi.Router) {
			prv.Use(security.Authenticate(s.tokens))
			h.Mount(prv)
		})
	})
	s.router = r
}

func (s *AppealTestSuite) TearDownSuite() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *AppealTestSuite) SetupTest() {
	if s.db == nil {
		return
	}
	s.cleanDB()
	s.seedCore()
}

func (s *AppealTestSuite) cleanDB() {
	ctx := context.Background()
	tables := []string{
		"appeal_evidence",
		"appeals",
		"test_results",
		"tests",
		"session_telemetry",
		"violations",
		"detection_events",
		"recordings",
		"sessions",
		"bookings",
		"slots",
		"institute_verifications",
		"devices",
		"candidates",
		"experts",
		"institutes",
		"admins",
		"test_centers",
	}
	for _, t := range tables {
		_, _ = s.db.Exec(ctx, "TRUNCATE TABLE "+t+" CASCADE")
	}
}

func (s *AppealTestSuite) seedCore() {
	ctx := context.Background()
	hash, _ := security.HashPassword("Password123")

	s.testCenterID = uuid.New()
	_, err := s.db.Exec(ctx, `INSERT INTO test_centers (id, name, email, password_hash, phone, status, created_at, updated_at, created_by, updated_by)
		VALUES ($1, 'Test Center', 'center@adlts.et', $2, 'phone', 'active', NOW(), NOW(), $3, $3)`, s.testCenterID, hash, s.testCenterID)
	require.NoError(s.T(), err)

	// Institute needed for institute_verifications
	instID := uuid.New()
	_, err = s.db.Exec(ctx, `INSERT INTO institutes (id, name, email, password_hash, phone, status, created_at, updated_at, created_by, updated_by)
		VALUES ($1, 'Institute', 'inst@adlts.et', $2, '911', 'active', NOW(), NOW(), $3, $3)`, instID, hash, s.testCenterID)
	require.NoError(s.T(), err)

	s.candidateID = uuid.New()
	_, err = s.db.Exec(ctx, `INSERT INTO candidates (id, first_name, last_name, email, password_hash, status, phone, fayida_id, birth_date, gender, created_at, updated_at)
		VALUES ($1, 'John', 'Doe', 'john@adlts.et', $2, 'active', '911000111', 'FAYIDA-APPEAL-1', $3, 'male', NOW(), NOW())`, s.candidateID, hash, time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(s.T(), err)
	s.candidateT, _ = s.tokens.Sign(s.candidateID, security.EntityCandidate, "john@adlts.et", nil)

	s.expertID = uuid.New()
	_, err = s.db.Exec(ctx, `INSERT INTO experts (id, first_name, last_name, email, password_hash, status, fayida_id, employee_id, created_at, updated_at, created_by, updated_by)
		VALUES ($1, 'Exp', 'Ert', 'expert@adlts.et', $2, 'active', 'FAYIDA-EXPERT-APPEAL-1', 'EMP-APPEAL-1', NOW(), NOW(), $3, $3)`, s.expertID, hash, s.expertID)
	require.NoError(s.T(), err)
	s.expertT, _ = s.tokens.Sign(s.expertID, security.EntityExpert, "expert@adlts.et", nil)

	invID := uuid.New()
	_, err = s.db.Exec(ctx, `INSERT INTO institute_verifications (id, candidate_id, institute_id, issued_at, expires_at, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, NOW(), NOW() + INTERVAL '30 days', NOW(), NOW(), $4, $4)`, invID, s.candidateID, instID, s.candidateID)
	require.NoError(s.T(), err)

	bookingID := uuid.New()
	_, err = s.db.Exec(ctx, `INSERT INTO bookings (id, candidate_id, test_center_id, institute_verification_id, status, training_hours, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, 'scheduled', 0, NOW(), NOW(), $2, $2)`, bookingID, s.candidateID, s.testCenterID, invID)
	require.NoError(s.T(), err)

	s.sessionID = uuid.New()
	_, err = s.db.Exec(ctx, `INSERT INTO sessions (id, booking_id, candidate_id, test_center_id, status, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, 'completed', NOW(), NOW(), $3, $3)`, s.sessionID, bookingID, s.candidateID, s.testCenterID)
	require.NoError(s.T(), err)

	s.testID = uuid.New()
	_, err = s.db.Exec(ctx, `INSERT INTO tests (id, session_id, appeal_window_closes_at, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, NOW(), NOW(), $4, $4)`, s.testID, s.sessionID, time.Now().Add(1*time.Hour), s.candidateID)
	require.NoError(s.T(), err)

	_, err = s.db.Exec(ctx, `INSERT INTO test_results (test_id, passed) VALUES ($1, false)`, s.testID)
	require.NoError(s.T(), err)
}

func (s *AppealTestSuite) request(method, path, token string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, "/api/v1"+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

func (s *AppealTestSuite) Test_CreateGetResolve_AppealAccepted() {
	createBody := map[string]any{
		"test_id":    s.testID.String(),
		"session_id": s.sessionID.String(),
		"reason":     "I disagree with the evaluation",
	}
	w := s.request("POST", "/appeals", s.candidateT, createBody)
	require.Equal(s.T(), http.StatusCreated, w.Code)

	var created map[string]any
	_ = json.NewDecoder(w.Body).Decode(&created)
	data := created["data"].(map[string]any)
	appealID := data["id"].(string)
	require.NotEmpty(s.T(), appealID)

	w = s.request("GET", "/appeals/"+appealID, s.candidateT, nil)
	require.Equal(s.T(), http.StatusOK, w.Code)

	w = s.request("PATCH", "/appeals/"+appealID+"/resolve", s.expertT, map[string]any{
		"decision":   "accepted",
		"resolution": "Accepted after review",
	})
	require.Equal(s.T(), http.StatusOK, w.Code)

	w = s.request("GET", "/appeals/"+appealID, s.candidateT, nil)
	require.Equal(s.T(), http.StatusOK, w.Code)

	var got map[string]any
	_ = json.NewDecoder(w.Body).Decode(&got)
	appealObj := got["data"].(map[string]any)
	require.Equal(s.T(), "accepted", appealObj["status"])
	require.Equal(s.T(), "Accepted after review", appealObj["resolution"])

	var passed bool
	err := s.db.QueryRow(context.Background(), `SELECT passed FROM test_results WHERE test_id=$1`, s.testID).Scan(&passed)
	require.NoError(s.T(), err)
	require.True(s.T(), passed)
}

func (s *AppealTestSuite) Test_CreateAppeal_WindowClosed() {
	_, err := s.db.Exec(context.Background(), `UPDATE tests SET appeal_window_closes_at=$2 WHERE id=$1`, s.testID, time.Now().Add(-1*time.Minute))
	require.NoError(s.T(), err)

	w := s.request("POST", "/appeals", s.candidateT, map[string]any{
		"test_id":    s.testID.String(),
		"session_id": s.sessionID.String(),
		"reason":     "too late",
	})
	require.Equal(s.T(), http.StatusForbidden, w.Code)
}

func TestAppealSuite(t *testing.T) {
	suite.Run(t, new(AppealTestSuite))
}

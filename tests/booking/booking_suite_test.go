package booking_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"adlts/internal/booking"
	"adlts/internal/platform/config"
	"adlts/internal/platform/db"
	"adlts/internal/platform/mailer"
	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type fakeProvider struct {
	verifyStatus string
}

func (f *fakeProvider) InitiatePayment(ctx context.Context, req booking.PaymentInitRequest) (booking.PaymentInitResult, error) {
	return booking.PaymentInitResult{CheckoutURL: "https://pay.test/checkout", TxRef: req.TxRef}, nil
}

func (f *fakeProvider) VerifyTransaction(ctx context.Context, txRef string) (booking.PaymentVerifyResult, error) {
	return booking.PaymentVerifyResult{TxRef: txRef, Status: f.verifyStatus, AmountCents: 10000}, nil
}

func (f *fakeProvider) ValidateWebhookSignature(payload []byte, signature string) bool {
	return signature == "valid"
}

type BookingTestSuite struct {
	suite.Suite
	db     *pgxpool.Pool
	router chi.Router
	tokens *security.Manager

	candidateID uuid.UUID
	candidateT  string
	adminID     uuid.UUID
	adminT      string
	instituteID uuid.UUID
}

func (s *BookingTestSuite) SetupSuite() {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:password@localhost:5433/adlts_test?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		s.T().Skip("skipping booking integration tests: Postgres not reachable; set TEST_DATABASE_URL to run")
		return
	}
	s.db = pool

	schema001, err := os.ReadFile("../../migrations/001_schema.sql")
	require.NoError(s.T(), err)
	schema002, err := os.ReadFile("../../migrations/002_booking_scheduling_payment.sql")
	require.NoError(s.T(), err)
	_, _ = s.db.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
	_, err = s.db.Exec(ctx, string(schema001))
	require.NoError(s.T(), err)
	_, err = s.db.Exec(ctx, string(schema002))
	require.NoError(s.T(), err)

	cfg := config.Config{JWTSecret: "test-secret-min-32-chars-long-12345678"}
	s.tokens = security.NewManager(cfg.JWTSecret)
	mail := mailer.New("localhost", "1025", "", "", "test@adlts.et", "Test")
	provider := &fakeProvider{verifyStatus: "success"}

	repo := booking.NewRepository(s.db)
	svc := booking.NewService(repo, provider, mail, "http://localhost:8080", "http://localhost:3000")
	h := booking.NewHandler(svc, s.tokens)

	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		h.Mount(api)
	})
	s.router = r
}

func (s *BookingTestSuite) TearDownSuite() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *BookingTestSuite) SetupTest() {
	s.cleanDB()
	s.seedUsers()
}

func (s *BookingTestSuite) cleanDB() {
	ctx := context.Background()
	tables := []string{
		"payments", "bookings", "slots",
		"candidates", "institutes", "admins", "test_centers",
	}
	for _, t := range tables {
		_, _ = s.db.Exec(ctx, "TRUNCATE TABLE "+t+" CASCADE")
	}
}

func (s *BookingTestSuite) seedUsers() {
	ctx := context.Background()
	hash, _ := security.HashPassword("Password123")

	centerID := uuid.New()
	_, err := s.db.Exec(ctx, `INSERT INTO test_centers (id, name, email, password_hash, phone, status, created_at, updated_at, created_by, updated_by) VALUES ($1, 'Test Center', 'center@adlts.et', $2, 'phone', 'active', NOW(), NOW(), $3, $3)`, centerID, hash, centerID)
	require.NoError(s.T(), err)

	s.adminID = uuid.New()
	_, err = s.db.Exec(ctx, `INSERT INTO admins (id, first_name, last_name, email, password_hash, status, test_center_id, created_at, updated_at, created_by, updated_by) VALUES ($1, 'Local', 'Admin', 'admin@adlts.et', $2, 'active', $3, NOW(), NOW(), $4, $4)`, s.adminID, hash, centerID, s.adminID)
	require.NoError(s.T(), err)
	s.adminT, _ = s.tokens.Sign(s.adminID, security.EntityAdmin, "admin@adlts.et", &centerID)

	s.instituteID = uuid.New()
	_, err = s.db.Exec(ctx, `INSERT INTO institutes (id, name, email, password_hash, phone, status, created_at, updated_at, created_by, updated_by) VALUES ($1, 'Institute', 'inst@adlts.et', $2, '911', 'active', NOW(), NOW(), $3, $3)`, s.instituteID, hash, s.adminID)
	require.NoError(s.T(), err)

	s.candidateID = uuid.New()
	_, err = s.db.Exec(ctx, `INSERT INTO candidates (id, first_name, last_name, email, password_hash, status, phone, fayida_id, birth_date, gender, created_at, updated_at) VALUES ($1, 'John', 'Doe', 'john@adlts.et', $2, 'active', '911000111', 'FAYIDA-1', $3, 'male', NOW(), NOW())`, s.candidateID, hash, time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(s.T(), err)
	s.candidateT, _ = s.tokens.Sign(s.candidateID, security.EntityCandidate, "john@adlts.et", nil)
}

func (s *BookingTestSuite) request(method, path, token string, body any) *httptest.ResponseRecorder {
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

func (s *BookingTestSuite) TestCreateBookingAndList() {
	body := map[string]any{"institute_id": s.instituteID.String()}
	w := s.request("POST", "/bookings", s.candidateT, body)
	require.Equal(s.T(), http.StatusCreated, w.Code)

	w = s.request("GET", "/bookings", s.candidateT, nil)
	require.Equal(s.T(), http.StatusOK, w.Code)
}

func (s *BookingTestSuite) TestScheduleBookingAndPaymentFlow() {
	body := map[string]any{"institute_id": s.instituteID.String()}
	w := s.request("POST", "/bookings", s.candidateT, body)
	require.Equal(s.T(), http.StatusCreated, w.Code)
	var created map[string]any
	_ = json.NewDecoder(w.Body).Decode(&created)
	data := created["data"].(map[string]any)
	bookingID := data["id"].(string)

	slotBody := map[string]any{
		"institute_id": s.instituteID.String(),
		"starts_at":    time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"ends_at":      time.Now().Add(25 * time.Hour).Format(time.RFC3339),
		"capacity":     1,
	}
	w = s.request("POST", "/slots", s.adminT, slotBody)
	require.Equal(s.T(), http.StatusCreated, w.Code)
	var slotCreated map[string]any
	_ = json.NewDecoder(w.Body).Decode(&slotCreated)
	slotData := slotCreated["data"].(map[string]any)
	slotID := slotData["id"].(string)

	w = s.request("PATCH", "/bookings/"+bookingID+"/schedule", s.adminT, map[string]any{"slot_id": slotID})
	require.Equal(s.T(), http.StatusOK, w.Code)

	payBody := map[string]any{"amount_cents": 10000, "currency": "ETB"}
	w = s.request("POST", "/bookings/"+bookingID+"/payments", s.candidateT, payBody)
	require.Equal(s.T(), http.StatusCreated, w.Code)
	var payResp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&payResp)
	payData := payResp["data"].(map[string]any)
	txRef := payData["tx_ref"].(string)

	payload, _ := json.Marshal(map[string]any{"tx_ref": txRef, "status": "success"})
	req, _ := http.NewRequest("POST", "/api/v1/bookings/"+bookingID+"/payments/callback", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-chapa-signature", "valid")
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	require.Equal(s.T(), http.StatusOK, w.Code)
}

func TestBookingSuite(t *testing.T) {
	suite.Run(t, new(BookingTestSuite))
}

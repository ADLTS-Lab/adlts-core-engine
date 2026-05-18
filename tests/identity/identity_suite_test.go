package identity_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"adlts/internal/domain"
	"adlts/internal/identity"
	"adlts/internal/platform/config"
	"adlts/internal/platform/db"
	"adlts/internal/platform/mailer"
	"adlts/internal/platform/security"
)

type IdentityTestSuite struct {
	suite.Suite
	db     *pgxpool.Pool
	router chi.Router
	tokens *security.Manager
	admin  *domain.Admin
	sa     *domain.SuperAdmin
	adminT string
	saT    string
}

func (s *IdentityTestSuite) SetupSuite() {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:password@localhost:5433/adlts_test?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	require.NoError(s.T(), err, "failed to connect to local test postgres. ensure one is running on 5432 with user/pass postgres/postgres or set TEST_DATABASE_URL")

	s.db = pool

	// Execute migrations
	schemaBytes, err := os.ReadFile("../../migrations/001_schema.sql")
	if err == nil {
		_, _ = s.db.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
		_, err = s.db.Exec(ctx, string(schemaBytes))
		require.NoError(s.T(), err)
	}

	cfg := config.Config{JWTSecret: "test-secret-min-32-chars-long-12345678"}
	s.tokens = security.NewManager(cfg.JWTSecret)

	// Mock Mailer
	mail := mailer.New("localhost", "1025", "", "", "test@adlts.et", "Test")

	repo := identity.NewRepository(s.db)
	svc := identity.NewService(repo, s.tokens, mail)
	handler := identity.NewHandler(svc, s.tokens)

	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		handler.Mount(api)
	})
	s.router = r
}

func (s *IdentityTestSuite) TearDownSuite() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *IdentityTestSuite) SetupTest() {
	s.cleanDB()
	s.seedRootUsers()
}

func (s *IdentityTestSuite) cleanDB() {
	ctx := context.Background()
	tables := []string{
		"invitations", "password_reset_tokens", "otp_codes",
		"candidates", "experts", "institutes", "transport_authorities", "admins", "super_admins", "test_centers",
	}
	for _, t := range tables {
		_, _ = s.db.Exec(ctx, "TRUNCATE TABLE "+t+" CASCADE")
	}
}

func (s *IdentityTestSuite) seedRootUsers() {
	ctx := context.Background()
	hash, _ := security.HashPassword("Password123")

	// Create test center
	centerID := uuid.New()
	_, err := s.db.Exec(ctx, `INSERT INTO test_centers (id, name, email, password_hash, phone, status, created_at, updated_at, created_by, updated_by) VALUES ($1, 'Test Center', 'center@adlts.et', $2, 'phone', 'active', NOW(), NOW(), $3, $3)`, centerID, hash, centerID)
	require.NoError(s.T(), err)

	// Create SuperAdmin
	s.sa = &domain.SuperAdmin{
		ID:           uuid.New(),
		Name:         "Root Super Admin",
		Email:        "root@adlts.et",
		PasswordHash: hash,
	}
	require.NoError(s.T(), identity.NewRepository(s.db).CreateSuperAdmin(ctx, s.sa))
	s.saT, _ = s.tokens.Sign(s.sa.ID, security.EntitySuperAdmin, s.sa.Email, nil)

	// Create Admin
	s.admin = &domain.Admin{
		BasePerson: domain.BasePerson{
			ID:           uuid.New(),
			FirstName:    "Local",
			LastName:     "Admin",
			Email:        "admin@adlts.et",
			PasswordHash: hash,
			Status:       domain.UserStatusActive,
		},
		TestCenterID: centerID,
	}
	require.NoError(s.T(), identity.NewRepository(s.db).CreateAdmin(ctx, s.admin))
	s.adminT, _ = s.tokens.Sign(s.admin.ID, security.EntityAdmin, s.admin.Email, &centerID)
}

func (s *IdentityTestSuite) request(method, path string, token string, body interface{}) *httptest.ResponseRecorder {
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

func (s *IdentityTestSuite) Test_CandidateRegistrationAndOTP() {
	// Register Candidate
	regBody := map[string]interface{}{
		"first_name": "John",
		"last_name":  "Doe",
		"email":      "john.doe@example.com",
		"password":   "SecretPass123",
		"fayida_id":  "FAYIDA-JOHN-1",
		"phone":      "911001122",
		"birth_date": "1990-01-01T00:00:00Z",
		"gender":     "male",
	}
	w := s.request("POST", "/auth/candidates/register", "", regBody)
	require.Equal(s.T(), http.StatusCreated, w.Code, "Expected created status")

	// Get the OTP token generated since it's a test db
	var otp string
	err := s.db.QueryRow(context.Background(), "SELECT code FROM otp_codes WHERE email='john.doe@example.com'").Scan(&otp)
	require.NoError(s.T(), err, "Failed to retrieve OTP code from DB")

	// Verify OTP
	verifyBody := map[string]interface{}{
		"email": "john.doe@example.com",
		"code":  otp,
	}
	w = s.request("POST", "/auth/candidates/verify-otp", "", verifyBody)
	require.Equal(s.T(), http.StatusOK, w.Code, "Expected OTP verification OK")

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)
	data := response["data"].(map[string]interface{})
	require.NotEmpty(s.T(), data["access_token"])
}

func (s *IdentityTestSuite) Test_InvitationsFlow() {
	// Create Invitation
	inviteBody := map[string]interface{}{
		"email":       "expert@adlts.et",
		"entity_type": "expert",
	}
	w := s.request("POST", "/invitations", s.adminT, inviteBody)
	require.Equal(s.T(), http.StatusCreated, w.Code)
	var created map[string]interface{}
	json.NewDecoder(w.Body).Decode(&created)
	data := created["data"].(map[string]interface{})
	invID := data["id"].(string)

	// Fetch token from db
	var token string
	err := s.db.QueryRow(context.Background(), "SELECT token FROM invitations WHERE email='expert@adlts.et'").Scan(&token)
	require.NoError(s.T(), err)

	// List Invitations
	w = s.request("GET", "/invitations?status=pending", s.adminT, nil)
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Get Single Invitation
	w = s.request("GET", "/invitations/"+invID, s.adminT, nil)
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Resend Invitation
	w = s.request("POST", "/invitations/"+invID+"/resend", s.adminT, nil)
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Accept Invitation
	var newToken string
	err = s.db.QueryRow(context.Background(), "SELECT token FROM invitations WHERE email='expert@adlts.et'").Scan(&newToken)
	require.NoError(s.T(), err)
	require.NotEqual(s.T(), token, newToken)

	acceptBody := map[string]interface{}{
		"token":       newToken,
		"password":    "NewStrongPassword123",
		"first_name":  "New",
		"last_name":   "Expert",
		"fayida_id":   "FAYIDA-EXPERT-999",
		"employee_id": "EMP-999",
	}
	w = s.request("POST", "/auth/invitations/accept", "", acceptBody)
	s.T().Logf("Accept Invitation Response: %s", w.Body.String())
	require.Equal(s.T(), http.StatusCreated, w.Code)

	// Verify Cancel fails now that it's used
	w = s.request("DELETE", "/invitations/"+invID, s.adminT, nil)
	require.Equal(s.T(), http.StatusConflict, w.Code)
}

func (s *IdentityTestSuite) Test_CancelInvitation() {
	// Create, then cancel
	inviteBody := map[string]interface{}{
		"email":       "institute@adlts.et",
		"entity_type": "institute",
	}
	w := s.request("POST", "/invitations", s.adminT, inviteBody)
	require.Equal(s.T(), http.StatusCreated, w.Code)
	var created map[string]interface{}
	json.NewDecoder(w.Body).Decode(&created)
	data := created["data"].(map[string]interface{})
	invID := data["id"].(string)

	w = s.request("DELETE", "/invitations/"+invID, s.adminT, nil)
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Attempt to get should still work since it's just soft-cancelled via UsedAt? No, DELETE physically removes it.
	// Oh wait, Delete removes it.
	w = s.request("GET", "/invitations/"+invID, s.adminT, nil)
	require.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *IdentityTestSuite) Test_LoginAndMe() {
	loginBody := map[string]interface{}{
		"email":    s.admin.Email,
		"password": "Password123", // Correct password
	}
	w := s.request("POST", "/auth/login", "", loginBody)
	require.Equal(s.T(), http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})
	token := data["access_token"].(string)

	w = s.request("GET", "/admins/me", token, nil)
	require.Equal(s.T(), http.StatusOK, w.Code)

	var me map[string]interface{}
	json.NewDecoder(w.Body).Decode(&me)
	meData := me["data"].(map[string]interface{})
	require.Equal(s.T(), "Local", meData["first_name"])
}

func (s *IdentityTestSuite) Test_FailInvalidLogin() {
	loginBody := map[string]interface{}{
		"email":    "nonexistent@example.com",
		"password": "Password123",
	}
	w := s.request("POST", "/auth/login", "", loginBody)
	require.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func TestIdentitySuite(t *testing.T) {
	suite.Run(t, new(IdentityTestSuite))
}

package identity

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"adlts/internal/domain"
	"adlts/internal/platform/mailer"
	"adlts/internal/platform/security"

	"github.com/google/uuid"
)

// BaseURL is the public-facing URL used to build links in emails.
// Set via APP_URL env var; falls back to localhost for dev.
var BaseURL = "http://localhost:8080"

type Service struct {
	repo   *Repository
	tokens *security.Manager
	mail   *mailer.Mailer
}

func NewService(repo *Repository, tokens *security.Manager, mail *mailer.Mailer) *Service {
	return &Service{repo: repo, tokens: tokens, mail: mail}
}

// SeedSuperAdmin ensures that at least the configured root super admin exists.
func (s *Service) SeedSuperAdmin(ctx context.Context, name, email, password string) error {
	sa, _ := s.repo.SuperAdminByEmail(ctx, email)
	if sa != nil {
		return nil // Already seeded
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash super admin password: %w", err)
	}
	now := time.Now().UTC()
	id := uuid.New()
	admin := &domain.SuperAdmin{
		ID:           id,
		Name:         name,
		Email:        email,
		PasswordHash: hash,
		Audit: domain.Audit{
			CreatedAt: now, UpdatedAt: now, CreatedBy: id, UpdatedBy: id,
		},
	}
	return s.repo.CreateSuperAdmin(ctx, admin)
}

// ── Candidate registration ────────────────────────────────────────────────────

func (s *Service) RegisterCandidate(ctx context.Context, req RegisterCandidateRequest) error {
	req.NormalizeFayidaID()
	if err := req.validate(); err != nil {
		return err
	}
	existing, _ := s.repo.CandidateByEmail(ctx, req.Email)
	if existing != nil {
		return ErrEmailTaken
	}
	fayidaCheck, _ := s.repo.CandidateByFayidaID(ctx, req.FayidaID)
	if fayidaCheck != nil {
		return ErrFayidaIDTaken
	}
	hash, err := security.HashPassword(req.Password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	now := time.Now().UTC()
	c := &domain.Candidate{
		BasePerson: domain.BasePerson{
			ID:           uuid.New(),
			FirstName:    req.FirstName,
			MiddleName:   req.MiddleName,
			LastName:     req.LastName,
			Email:        req.Email,
			PasswordHash: hash,
			Status:       domain.UserStatusPendingVerification,
			Audit:        domain.Audit{CreatedAt: now, UpdatedAt: now},
		},
		Phone:     req.Phone,
		FayidaID:  req.FayidaID,
		BirthDate: req.BirthDate,
		Gender:    domain.Gender(req.Gender),
	}
	if err := s.repo.CreateCandidate(ctx, c); err != nil {
		return fmt.Errorf("create candidate: %w", err)
	}
	return s.sendOTP(ctx, req.Email)
}

func (s *Service) VerifyOTP(ctx context.Context, req VerifyOTPRequest) (LoginResponse, error) {
	cand, err := s.repo.CandidateByEmail(ctx, req.Email)
	if err != nil || cand == nil {
		return LoginResponse{}, ErrInvalidOTP
	}
	ok, err := s.repo.VerifyOTP(ctx, req.Email, req.Code)
	if err != nil {
		return LoginResponse{}, fmt.Errorf("verify otp: %w", err)
	}
	if !ok {
		return LoginResponse{}, ErrInvalidOTP
	}
	if err := s.repo.UpdateCandidateStatus(ctx, cand.ID, domain.UserStatusActive, cand.ID); err != nil {
		return LoginResponse{}, fmt.Errorf("activate candidate: %w", err)
	}
	return s.issueToken(cand.ID, security.EntityCandidate, cand.Email, nil)
}

// ResendOTP re-sends a verification code. Rate limiting is enforced by UpsertOTP
// (replaces previous code with 0 attempts — server-side rate limit via DB).
func (s *Service) ResendOTP(ctx context.Context, email string) error {
	cand, _ := s.repo.CandidateByEmail(ctx, email)
	if cand == nil {
		return nil // Never reveal whether email exists
	}
	if cand.Status != domain.UserStatusPendingVerification {
		return ErrAlreadyVerified
	}
	return s.sendOTP(ctx, email)
}

// ── Universal login ───────────────────────────────────────────────────────────

func (s *Service) Login(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	if req.Email == "" || req.Password == "" {
		return LoginResponse{}, ErrInvalidCredentials
	}

	type tryFn func() (LoginResponse, error)
	tries := []tryFn{
		func() (LoginResponse, error) { return s.loginCandidate(ctx, req) },
		func() (LoginResponse, error) { return s.loginExpert(ctx, req) },
		func() (LoginResponse, error) { return s.loginAdmin(ctx, req) },
		func() (LoginResponse, error) { return s.loginSuperAdmin(ctx, req) },
		func() (LoginResponse, error) { return s.loginInstitute(ctx, req) },
		func() (LoginResponse, error) { return s.loginAuthority(ctx, req) },
	}
	for _, try := range tries {
		resp, err := try()
		if err == nil {
			return resp, nil
		}
	}
	return LoginResponse{}, ErrInvalidCredentials
}

func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (LoginResponse, error) {
	_ = ctx
	if strings.TrimSpace(refreshToken) == "" {
		return LoginResponse{}, ErrInvalidRefreshToken
	}

	claims, err := s.tokens.Parse(refreshToken)
	if err != nil {
		return LoginResponse{}, ErrInvalidRefreshToken
	}
	if claims.TokenType != security.TokenTypeRefresh {
		return LoginResponse{}, ErrInvalidRefreshToken
	}
	if claims.SubjectID == uuid.Nil || claims.EntityType == "" || claims.Email == "" {
		return LoginResponse{}, ErrInvalidRefreshToken
	}
	if entityTable(claims.EntityType) == "" {
		return LoginResponse{}, ErrInvalidRefreshToken
	}

	return s.issueToken(claims.SubjectID, claims.EntityType, claims.Email, claims.TestCenterID)
}

func (s *Service) loginCandidate(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	c, err := s.repo.CandidateByEmail(ctx, req.Email)
	if err != nil || c == nil {
		return LoginResponse{}, ErrInvalidCredentials
	}
	if err := security.CheckPassword(c.PasswordHash, req.Password); err != nil {
		return LoginResponse{}, ErrInvalidCredentials
	}
	switch c.Status {
	case domain.UserStatusPendingVerification:
		return LoginResponse{}, ErrPendingVerification
	case domain.UserStatusSuspended:
		return LoginResponse{}, ErrAccountSuspended
	case domain.UserStatusInactive:
		return LoginResponse{}, ErrAccountInactive
	}
	return s.issueToken(c.ID, security.EntityCandidate, c.Email, nil)
}

func (s *Service) loginExpert(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	e, err := s.repo.ExpertByEmail(ctx, req.Email)
	if err != nil || e == nil {
		return LoginResponse{}, ErrInvalidCredentials
	}
	if err := security.CheckPassword(e.PasswordHash, req.Password); err != nil {
		return LoginResponse{}, ErrInvalidCredentials
	}
	switch e.Status {
	case domain.UserStatusSuspended:
		return LoginResponse{}, ErrAccountSuspended
	case domain.UserStatusInactive:
		return LoginResponse{}, ErrAccountInactive
	}
	return s.issueToken(e.ID, security.EntityExpert, e.Email, nil)
}

func (s *Service) loginAdmin(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	a, err := s.repo.AdminByEmail(ctx, req.Email)
	if err != nil || a == nil {
		return LoginResponse{}, ErrInvalidCredentials
	}
	if err := security.CheckPassword(a.PasswordHash, req.Password); err != nil {
		return LoginResponse{}, ErrInvalidCredentials
	}
	switch a.Status {
	case domain.UserStatusSuspended:
		return LoginResponse{}, ErrAccountSuspended
	case domain.UserStatusInactive:
		return LoginResponse{}, ErrAccountInactive
	}
	return s.issueToken(a.ID, security.EntityAdmin, a.Email, &a.TestCenterID)
}

func (s *Service) loginSuperAdmin(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	sa, err := s.repo.SuperAdminByEmail(ctx, req.Email)
	if err != nil || sa == nil {
		return LoginResponse{}, ErrInvalidCredentials
	}
	if err := security.CheckPassword(sa.PasswordHash, req.Password); err != nil {
		return LoginResponse{}, ErrInvalidCredentials
	}
	return s.issueToken(sa.ID, security.EntitySuperAdmin, sa.Email, nil)
}

func (s *Service) loginInstitute(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	inst, err := s.repo.InstituteByEmail(ctx, req.Email)
	if err != nil || inst == nil {
		return LoginResponse{}, ErrInvalidCredentials
	}
	if err := security.CheckPassword(inst.PasswordHash, req.Password); err != nil {
		return LoginResponse{}, ErrInvalidCredentials
	}
	switch inst.Status {
	case domain.OrgStatusSuspended:
		return LoginResponse{}, ErrAccountSuspended
	case domain.OrgStatusInactive:
		return LoginResponse{}, ErrAccountInactive
	case domain.OrgStatusPendingApproval:
		return LoginResponse{}, ErrPendingApproval
	}
	return s.issueToken(inst.ID, security.EntityInstitute, inst.Email, nil)
}

func (s *Service) loginAuthority(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	a, err := s.repo.AuthorityByEmail(ctx, req.Email)
	if err != nil || a == nil {
		return LoginResponse{}, ErrInvalidCredentials
	}
	if err := security.CheckPassword(a.PasswordHash, req.Password); err != nil {
		return LoginResponse{}, ErrInvalidCredentials
	}
	switch a.Status {
	case domain.OrgStatusSuspended:
		return LoginResponse{}, ErrAccountSuspended
	case domain.OrgStatusInactive:
		return LoginResponse{}, ErrAccountInactive
	}
	return s.issueToken(a.ID, security.EntityTransportAuthority, a.Email, nil)
}

func (s *Service) issueToken(id uuid.UUID, et security.EntityType, email string, centerID *uuid.UUID) (LoginResponse, error) {
	accessToken, err := s.tokens.SignAccessToken(id, et, email, centerID)
	if err != nil {
		return LoginResponse{}, fmt.Errorf("sign access token: %w", err)
	}
	refreshToken, err := s.tokens.SignRefreshToken(id, et, email, centerID)
	if err != nil {
		return LoginResponse{}, fmt.Errorf("sign refresh token: %w", err)
	}
	return LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		EntityType:   string(et),
	}, nil
}

// ── Password flows ────────────────────────────────────────────────────────────

func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	token := uuid.NewString()
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	if err := s.repo.UpsertPasswordResetToken(ctx, email, token, expiresAt); err != nil {
		return err
	}
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", BaseURL, token)
	return s.mail.SendPasswordReset(email, resetLink)
}

func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if len(newPassword) < 8 {
		return ErrPasswordTooShort
	}
	email, err := s.repo.ConsumePasswordResetToken(ctx, token)
	if err != nil {
		return ErrInvalidResetToken
	}
	hash, err := security.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash: %w", err)
	}
	return s.updatePasswordByEmail(ctx, email, hash)
}

func (s *Service) ChangePassword(ctx context.Context, auth *security.AuthContext, req ChangePasswordRequest) error {
	if len(req.NewPassword) < 8 {
		return ErrPasswordTooShort
	}
	if req.CurrentPassword == req.NewPassword {
		return ErrSamePassword
	}
	table := entityTable(auth.EntityType)
	if table == "" {
		return errors.New("unknown entity type")
	}
	// Verify current password first
	currentHash, err := s.repo.GetPasswordHash(ctx, table, auth.SubjectID)
	if err != nil {
		return fmt.Errorf("fetch hash: %w", err)
	}
	if err := security.CheckPassword(currentHash, req.CurrentPassword); err != nil {
		return ErrInvalidCredentials
	}
	hash, err := security.HashPassword(req.NewPassword)
	if err != nil {
		return fmt.Errorf("hash: %w", err)
	}
	return s.repo.UpdatePassword(ctx, table, auth.SubjectID, hash)
}

func (s *Service) updatePasswordByEmail(ctx context.Context, email, hash string) error {
	type finder func() error
	finders := []finder{
		func() error {
			c, err := s.repo.CandidateByEmail(ctx, email)
			if err != nil || c == nil {
				return ErrNotFound
			}
			return s.repo.UpdatePassword(ctx, "candidates", c.ID, hash)
		},
		func() error {
			e, err := s.repo.ExpertByEmail(ctx, email)
			if err != nil || e == nil {
				return ErrNotFound
			}
			return s.repo.UpdatePassword(ctx, "experts", e.ID, hash)
		},
		func() error {
			a, err := s.repo.AdminByEmail(ctx, email)
			if err != nil || a == nil {
				return ErrNotFound
			}
			return s.repo.UpdatePassword(ctx, "admins", a.ID, hash)
		},
		func() error {
			sa, err := s.repo.SuperAdminByEmail(ctx, email)
			if err != nil || sa == nil {
				return ErrNotFound
			}
			return s.repo.UpdatePassword(ctx, "super_admins", sa.ID, hash)
		},
		func() error {
			inst, err := s.repo.InstituteByEmail(ctx, email)
			if err != nil || inst == nil {
				return ErrNotFound
			}
			return s.repo.UpdatePassword(ctx, "institutes", inst.ID, hash)
		},
		func() error {
			auth, err := s.repo.AuthorityByEmail(ctx, email)
			if err != nil || auth == nil {
				return ErrNotFound
			}
			return s.repo.UpdatePassword(ctx, "transport_authorities", auth.ID, hash)
		},
	}
	for _, fn := range finders {
		if err := fn(); err == nil {
			return nil
		}
	}
	return ErrNotFound
}

// ── Invitation flow ───────────────────────────────────────────────────────────

func (s *Service) CreateInvitation(ctx context.Context, auth *security.AuthContext, req CreateInvitationRequest) (*domain.Invitation, error) {
	if req.Email == "" || req.EntityType == "" {
		return nil, errors.New("email and entity_type are required")
	}

	// Admin can only invite: expert, institute
	if auth.EntityType == security.EntityAdmin {
		allowed := map[string]bool{"expert": true, "institute": true}
		if !allowed[req.EntityType] {
			return nil, ErrForbiddenInviteRole
		}
		req.TestCenterID = auth.TestCenterID // Admin invitations are always scoped to their own center
	}

	// Admin invitations for admin type require a test_center_id
	if req.EntityType == "admin" && req.TestCenterID == nil {
		return nil, errors.New("test_center_id is required when inviting an admin")
	}

	now := time.Now().UTC()
	inv := &domain.Invitation{
		ID:           uuid.New(),
		Token:        uuid.NewString(),
		Email:        req.Email,
		EntityType:   req.EntityType,
		TestCenterID: req.TestCenterID,
		ExpiresAt:    now.Add(72 * time.Hour),
		CreatedBy:    auth.SubjectID,
		CreatedAt:    now,
	}
	if err := s.repo.CreateInvitation(ctx, inv); err != nil {
		return nil, fmt.Errorf("create invitation: %w", err)
	}

	inviteLink := fmt.Sprintf("%s/accept-invitation?token=%s", BaseURL, inv.Token)
	_ = s.mail.SendInvitation(inv.Email, inv.EntityType, inviteLink)

	return inv, nil
}

func (s *Service) AcceptInvitation(ctx context.Context, req AcceptInvitationRequest) (LoginResponse, error) {
	req.NormalizeFayidaID()
	if req.Token == "" || req.Password == "" {
		return LoginResponse{}, errors.New("token and password are required")
	}
	if len(req.Password) < 8 {
		return LoginResponse{}, ErrPasswordTooShort
	}

	inv, err := s.repo.InvitationByToken(ctx, req.Token)
	if err != nil || inv == nil {
		return LoginResponse{}, ErrInvalidInviteToken
	}
	if inv.UsedAt != nil {
		return LoginResponse{}, ErrInviteAlreadyUsed
	}
	if time.Now().After(inv.ExpiresAt) {
		return LoginResponse{}, ErrInviteExpired
	}

	hash, err := security.HashPassword(req.Password)
	if err != nil {
		return LoginResponse{}, fmt.Errorf("hash: %w", err)
	}
	now := time.Now().UTC()

	var entityID uuid.UUID
	var entityType security.EntityType
	var testCenterID *uuid.UUID

	switch inv.EntityType {
	case "expert":
		e := &domain.Expert{
			BasePerson: domain.BasePerson{
				ID: uuid.New(), FirstName: req.FirstName, MiddleName: req.MiddleName,
				LastName: req.LastName, Email: inv.Email, PasswordHash: hash,
				Status: domain.UserStatusActive,
				Audit:  domain.Audit{CreatedAt: now, UpdatedAt: now, CreatedBy: inv.CreatedBy, UpdatedBy: inv.CreatedBy},
			},
			Phone:      req.Phone,
			FayidaID:   req.FayidaID,
			EmployeeID: req.EmployeeID,
		}
		if err := s.repo.CreateExpert(ctx, e); err != nil {
			return LoginResponse{}, fmt.Errorf("create expert: %w", err)
		}
		entityID, entityType = e.ID, security.EntityExpert

	case "institute":
		inst := &domain.Institute{BaseOrg: domain.BaseOrg{
			ID: uuid.New(), Name: req.Name, Email: inv.Email,
			PasswordHash: hash, Status: domain.OrgStatusPendingApproval,
			Audit: domain.Audit{CreatedAt: now, UpdatedAt: now, CreatedBy: inv.CreatedBy, UpdatedBy: inv.CreatedBy},
		}}
		if err := s.repo.CreateInstitute(ctx, inst); err != nil {
			return LoginResponse{}, fmt.Errorf("create institute: %w", err)
		}
		entityID, entityType = inst.ID, security.EntityInstitute

	case "admin":
		if inv.TestCenterID == nil {
			return LoginResponse{}, errors.New("invitation missing test_center_id")
		}
		a := &domain.Admin{
			BasePerson: domain.BasePerson{
				ID: uuid.New(), FirstName: req.FirstName, MiddleName: req.MiddleName,
				LastName: req.LastName, Email: inv.Email, PasswordHash: hash,
				Status: domain.UserStatusActive,
				Audit:  domain.Audit{CreatedAt: now, UpdatedAt: now, CreatedBy: inv.CreatedBy, UpdatedBy: inv.CreatedBy},
			},
			TestCenterID: *inv.TestCenterID,
		}
		if err := s.repo.CreateAdmin(ctx, a); err != nil {
			return LoginResponse{}, fmt.Errorf("create admin: %w", err)
		}
		entityID, entityType = a.ID, security.EntityAdmin
		testCenterID = inv.TestCenterID

	case "transport_authority":
		a := &domain.TransportAuthority{BaseOrg: domain.BaseOrg{
			ID: uuid.New(), Name: req.Name, Email: inv.Email,
			PasswordHash: hash, Status: domain.OrgStatusActive,
			Audit: domain.Audit{CreatedAt: now, UpdatedAt: now, CreatedBy: inv.CreatedBy, UpdatedBy: inv.CreatedBy},
		}}
		if err := s.repo.CreateAuthority(ctx, a); err != nil {
			return LoginResponse{}, fmt.Errorf("create authority: %w", err)
		}
		entityID, entityType = a.ID, security.EntityTransportAuthority

	case "super_admin":
		sa := &domain.SuperAdmin{
			ID: uuid.New(), Name: req.Name, Email: inv.Email, PasswordHash: hash,
			Audit: domain.Audit{CreatedAt: now, UpdatedAt: now, CreatedBy: inv.CreatedBy, UpdatedBy: inv.CreatedBy},
		}
		if err := s.repo.CreateSuperAdmin(ctx, sa); err != nil {
			return LoginResponse{}, fmt.Errorf("create super_admin: %w", err)
		}
		entityID, entityType = sa.ID, security.EntitySuperAdmin

	default:
		return LoginResponse{}, fmt.Errorf("unknown entity_type: %s", inv.EntityType)
	}

	_ = s.repo.MarkInvitationUsed(ctx, inv.ID)
	return s.issueToken(entityID, entityType, inv.Email, testCenterID)
}

func (s *Service) GetInvitation(ctx context.Context, auth *security.AuthContext, id uuid.UUID) (*domain.Invitation, error) {
	inv, err := s.repo.InvitationByID(ctx, id)
	if err != nil || inv == nil {
		return nil, ErrNotFound
	}
	// Admins can only view invitations under their test center
	if auth.EntityType == security.EntityAdmin {
		if inv.TestCenterID == nil || *inv.TestCenterID != *auth.TestCenterID {
			return nil, ErrNotFound
		}
	}
	return inv, nil
}

func (s *Service) ResendInvitation(ctx context.Context, auth *security.AuthContext, id uuid.UUID) error {
	inv, err := s.GetInvitation(ctx, auth, id)
	if err != nil {
		return err
	}
	if inv.UsedAt != nil {
		return ErrInviteAlreadyUsed
	}

	newToken := uuid.NewString()
	newExpiry := time.Now().UTC().Add(72 * time.Hour)
	if err := s.repo.UpdateInvitationToken(ctx, id, newToken, newExpiry); err != nil {
		return fmt.Errorf("update invitation token: %w", err)
	}

	inviteLink := fmt.Sprintf("%s/accept-invitation?token=%s", BaseURL, newToken)
	_ = s.mail.SendInvitation(inv.Email, inv.EntityType, inviteLink)

	return nil
}

func (s *Service) CancelInvitation(ctx context.Context, auth *security.AuthContext, id uuid.UUID) error {
	inv, err := s.GetInvitation(ctx, auth, id)
	if err != nil {
		return err
	}
	if inv.UsedAt != nil {
		return ErrInviteAlreadyUsed
	}
	if err := s.repo.CancelInvitation(ctx, id); err != nil {
		return fmt.Errorf("cancel invitation: %w", err)
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (s *Service) sendOTP(ctx context.Context, email string) error {
	code, err := randomDigits(6)
	if err != nil {
		return fmt.Errorf("generate otp: %w", err)
	}
	expiresAt := time.Now().UTC().Add(5 * time.Minute)
	if err := s.repo.UpsertOTP(ctx, email, code, expiresAt); err != nil {
		return fmt.Errorf("store otp: %w", err)
	}
	return s.mail.SendOTP(email, code)
}

func randomDigits(n int) (string, error) {
	const digits = "0123456789"
	result := make([]byte, n)
	for i := range result {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		result[i] = digits[idx.Int64()]
	}
	return string(result), nil
}

func entityTable(et security.EntityType) string {
	switch et {
	case security.EntityCandidate:
		return "candidates"
	case security.EntityExpert:
		return "experts"
	case security.EntityAdmin:
		return "admins"
	case security.EntitySuperAdmin:
		return "super_admins"
	case security.EntityInstitute:
		return "institutes"
	case security.EntityTransportAuthority:
		return "transport_authorities"
	default:
		return ""
	}
}

// ── Domain errors exposed to handler ─────────────────────────────────────────

var (
	ErrEmailTaken          = errors.New("email already registered")
	ErrFayidaIDTaken       = errors.New("fayida ID already registered")
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrPendingVerification = errors.New("account pending email verification")
	ErrPendingApproval     = errors.New("account pending admin approval")
	ErrAccountSuspended    = errors.New("account is suspended")
	ErrAccountInactive     = errors.New("account is deactivated")
	ErrAlreadyVerified     = errors.New("account already verified")
	ErrInvalidOTP          = errors.New("invalid or expired verification code")
	ErrInvalidResetToken   = errors.New("invalid or expired reset token")
	ErrInvalidInviteToken  = errors.New("invalid invitation token")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrInviteAlreadyUsed   = errors.New("invitation already used")
	ErrInviteExpired       = errors.New("invitation has expired")
	ErrForbiddenInviteRole = errors.New("admin can only invite expert or institute")
	ErrPasswordTooShort    = errors.New("password must be at least 8 characters")
	ErrSamePassword        = errors.New("new password must differ from current password")
	ErrNotFound            = errors.New("not found")
)

// validate performs basic input validation on RegisterCandidateRequest.
func (r RegisterCandidateRequest) validate() error {
	if r.FirstName == "" || r.LastName == "" {
		return errors.New("first_name and last_name are required")
	}
	if r.Email == "" {
		return errors.New("email is required")
	}
	if len(r.Password) < 8 {
		return ErrPasswordTooShort
	}
	if r.FayidaID == "" {
		return errors.New("fayida_id is required")
	}
	if r.BirthDate.IsZero() {
		return errors.New("birth_date is required")
	}
	if r.Gender != "male" && r.Gender != "female" {
		return errors.New("gender must be male or female")
	}
	return nil
}

package identity

import (
	"time"

	"github.com/google/uuid"
)

// ── Auth ──────────────────────────────────────────────────────────────────────

type RegisterCandidateRequest struct {
	FirstName  string    `json:"first_name"`
	MiddleName string    `json:"middle_name"`
	LastName   string    `json:"last_name"`
	Email      string    `json:"email"`
	Password   string    `json:"password"`
	Phone      string    `json:"phone"`
	FayidaID   string    `json:"fayida_id"`
	BirthDate  time.Time `json:"birth_date"`
	Gender     string    `json:"gender"`
}

type VerifyOTPRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	EntityType   string `json:"entity_type"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type AcceptInvitationRequest struct {
	Token      string `json:"token"`
	Password   string `json:"password"`
	FirstName  string `json:"first_name,omitempty"` 
	MiddleName string `json:"middle_name,omitempty"`
	LastName   string `json:"last_name,omitempty"`
	Name       string `json:"name,omitempty"`
	Phone      string `json:"phone,omitempty"`
	FayidaID   string `json:"fayida_id,omitempty"`
	EmployeeID string `json:"employee_id,omitempty"`
}

// ── Candidates ────────────────────────────────────────────────────────────────

type CandidateResponse struct {
	ID         uuid.UUID  `json:"id"`
	FirstName  string     `json:"first_name"`
	MiddleName string     `json:"middle_name"`
	LastName   string     `json:"last_name"`
	Email      string     `json:"email"`
	Phone      string     `json:"phone"`
	FayidaID   string     `json:"fayida_id"`
	BirthDate  time.Time  `json:"birth_date"`
	Gender     string     `json:"gender"`
	PhotoURL   string     `json:"photo_url,omitempty"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type UpdateCandidateSelfRequest struct {
	FirstName  *string `json:"first_name,omitempty"`
	MiddleName *string `json:"middle_name,omitempty"`
	LastName   *string `json:"last_name,omitempty"`
	Phone      *string `json:"phone,omitempty"`
	PhotoURL   *string `json:"photo_url,omitempty"`
	// email and fayida_id are IMMUTABLE — rejected if present
}

type UpdateCandidateAdminRequest struct {
	UpdateCandidateSelfRequest
}

type StatusRequest struct {
	Status string `json:"status"` 
}

// ── Experts ───────────────────────────────────────────────────────────────────

type ExpertResponse struct {
	ID         uuid.UUID `json:"id"`
	FirstName  string    `json:"first_name"`
	MiddleName string    `json:"middle_name"`
	LastName   string    `json:"last_name"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
	EmployeeID string    `json:"employee_id"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type UpdateExpertSelfRequest struct {
	FirstName  *string `json:"first_name,omitempty"`
	MiddleName *string `json:"middle_name,omitempty"`
	LastName   *string `json:"last_name,omitempty"`
	Phone      *string `json:"phone,omitempty"`
	PhotoURL   *string `json:"photo_url,omitempty"`
	// email, fayida_id, employee_id are IMMUTABLE
}

// ── Institutes ────────────────────────────────────────────────────────────────

type InstituteResponse struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Email   string    `json:"email"`
	Phone   string    `json:"phone"`
	LogoURL string    `json:"logo_url,omitempty"`
	Status  string    `json:"status"`
	Address AddressDTO `json:"address"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpdateInstituteSelfRequest struct {
	Name    *string     `json:"name,omitempty"`
	Phone   *string     `json:"phone,omitempty"`
	LogoURL *string     `json:"logo_url,omitempty"`
	Address *AddressDTO `json:"address,omitempty"`
	// email is IMMUTABLE
}

// ── Transport Authorities ─────────────────────────────────────────────────────

type AuthorityResponse struct {
	ID      uuid.UUID  `json:"id"`
	Name    string     `json:"name"`
	Email   string     `json:"email"`
	Phone   string     `json:"phone"`
	Status  string     `json:"status"`
	Address AddressDTO `json:"address"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpdateAuthoritySelfRequest struct {
	Name    *string     `json:"name,omitempty"`
	Phone   *string     `json:"phone,omitempty"`
	Address *AddressDTO `json:"address,omitempty"`
	// email is IMMUTABLE
}

// ── Admins ────────────────────────────────────────────────────────────────────

type AdminResponse struct {
	ID           uuid.UUID `json:"id"`
	FirstName    string    `json:"first_name"`
	MiddleName   string    `json:"middle_name"`
	LastName     string    `json:"last_name"`
	Email        string    `json:"email"`
	TestCenterID uuid.UUID `json:"test_center_id"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type UpdateAdminSelfRequest struct {
	FirstName  *string `json:"first_name,omitempty"`
	MiddleName *string `json:"middle_name,omitempty"`
	LastName   *string `json:"last_name,omitempty"`
	// email and test_center_id are IMMUTABLE
}

// ── SuperAdmins ───────────────────────────────────────────────────────────────

type SuperAdminResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpdateSuperAdminSelfRequest struct {
	Name *string `json:"name,omitempty"`
	// email is IMMUTABLE
}

// ── Invitations ───────────────────────────────────────────────────────────────

type CreateInvitationRequest struct {
	Email        string     `json:"email"`
	EntityType   string     `json:"entity_type"` // "expert"|"institute"|"admin"|"transport_authority"|"super_admin"
	TestCenterID *uuid.UUID `json:"test_center_id,omitempty"` // required for admin
}

type InvitationResponse struct {
	ID         uuid.UUID  `json:"id"`
	Email      string     `json:"email"`
	EntityType string     `json:"entity_type"`
	ExpiresAt  time.Time  `json:"expires_at"`
	UsedAt     *time.Time `json:"used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ── Shared ────────────────────────────────────────────────────────────────────

type AddressDTO struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country"`
}

type ListQuery struct {
	Page   int    `json:"page"`
	Search string `json:"search"`
	Status string `json:"status"`
}

type PagedResponse[T any] struct {
	Data  []T `json:"data"`
	Total int `json:"total"`
	Page  int `json:"page"`
}

package identity

import (
	"context"
	"fmt"
	"strings"
	"time"

	"adlts/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── Candidates ────────────────────────────────────────────────────────────────

func (r *Repository) CreateCandidate(ctx context.Context, c *domain.Candidate) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO candidates
			(id, first_name, middle_name, last_name, email, password_hash, status,
			 phone, fayida_id, birth_date, gender, photo_url,
			 street, city, region, country,
			 created_at, updated_at, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
		c.ID, c.FirstName, c.MiddleName, c.LastName, c.Email, c.PasswordHash, c.Status,
		c.Phone, c.FayidaID, c.BirthDate, c.Gender, c.PhotoURL,
		c.Address.Street, c.Address.City, c.Address.Region, c.Address.Country,
		c.CreatedAt, c.UpdatedAt, c.Audit.CreatedBy, c.Audit.UpdatedBy,
	)
	return err
}

func (r *Repository) CandidateByID(ctx context.Context, id uuid.UUID) (*domain.Candidate, error) {
	return scanCandidate(r.db.QueryRow(ctx, `SELECT `+candidateCols+` FROM candidates WHERE id=$1`, id))
}

func (r *Repository) CandidateByEmail(ctx context.Context, email string) (*domain.Candidate, error) {
	return scanCandidate(r.db.QueryRow(ctx, `SELECT `+candidateCols+` FROM candidates WHERE email=$1`, email))
}

func (r *Repository) CandidateByFayidaID(ctx context.Context, fayidaID string) (*domain.Candidate, error) {
	return scanCandidate(r.db.QueryRow(ctx, `SELECT `+candidateCols+` FROM candidates WHERE fayida_id=$1`, fayidaID))
}

func (r *Repository) ListCandidates(ctx context.Context, search, status string, page int) ([]*domain.Candidate, int, error) {
	where, args := buildFilter(search, status, "user_status", 1)
	rows, err := r.db.Query(ctx,
		`SELECT `+candidateCols+` FROM candidates `+where+` ORDER BY created_at DESC LIMIT 20 OFFSET $`+fmt.Sprintf("%d", len(args)+1),
		append(args, (page-1)*20)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*domain.Candidate
	for rows.Next() {
		c, err := scanCandidateRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	var total int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM candidates `+where, args...).Scan(&total)
	return out, total, nil
}

func (r *Repository) UpdateCandidateFields(ctx context.Context, id uuid.UUID, fields map[string]any, updatedBy uuid.UUID) error {
	return updateFields(ctx, r.db, "candidates", id, fields, updatedBy)
}

func (r *Repository) UpdateCandidateStatus(ctx context.Context, id uuid.UUID, status domain.UserStatus, updatedBy uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE candidates SET status=$2, updated_at=NOW(), updated_by=$3 WHERE id=$1`,
		id, status, updatedBy)
	return err
}

// ── Experts ───────────────────────────────────────────────────────────────────

func (r *Repository) CreateExpert(ctx context.Context, e *domain.Expert) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO experts
			(id, first_name, middle_name, last_name, email, password_hash, status,
			 phone, fayida_id, employee_id, birth_date, gender, photo_url,
			 created_at, updated_at, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		e.ID, e.FirstName, e.MiddleName, e.LastName, e.Email, e.PasswordHash, e.Status,
		e.Phone, e.FayidaID, e.EmployeeID, nullIfZeroTime(e.BirthDate), nullIfEmpty(string(e.Gender)), e.PhotoURL,
		e.CreatedAt, e.UpdatedAt, e.Audit.CreatedBy, e.Audit.UpdatedBy,
	)
	return err
}

func (r *Repository) ExpertByID(ctx context.Context, id uuid.UUID) (*domain.Expert, error) {
	return scanExpert(r.db.QueryRow(ctx, `SELECT `+expertCols+` FROM experts WHERE id=$1`, id))
}

func (r *Repository) ExpertByEmail(ctx context.Context, email string) (*domain.Expert, error) {
	return scanExpert(r.db.QueryRow(ctx, `SELECT `+expertCols+` FROM experts WHERE email=$1`, email))
}

func (r *Repository) ListExperts(ctx context.Context, search, status string, page int) ([]*domain.Expert, int, error) {
	where, args := buildFilter(search, status, "user_status", 1)
	rows, err := r.db.Query(ctx,
		`SELECT `+expertCols+` FROM experts `+where+` ORDER BY created_at DESC LIMIT 20 OFFSET $`+argN(len(args)+1),
		append(args, (page-1)*20)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*domain.Expert
	for rows.Next() {
		e, err := scanExpertRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	var total int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM experts `+where, args...).Scan(&total)
	return out, total, nil
}

func (r *Repository) UpdateExpertFields(ctx context.Context, id uuid.UUID, fields map[string]any, updatedBy uuid.UUID) error {
	return updateFields(ctx, r.db, "experts", id, fields, updatedBy)
}

func (r *Repository) UpdateExpertStatus(ctx context.Context, id uuid.UUID, status domain.UserStatus, updatedBy uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE experts SET status=$2, updated_at=NOW(), updated_by=$3 WHERE id=$1`,
		id, status, updatedBy)
	return err
}

func (r *Repository) DeleteExpert(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM experts WHERE id=$1`, id)
	return err
}

// ── Institutes ───────────────────────────────────────────────────────────────

func (r *Repository) CreateInstitute(ctx context.Context, inst *domain.Institute) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO institutes
			(id, name, name_am, email, password_hash, phone, logo_url, status,
			 street, city, region, country,
			 created_at, updated_at, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		inst.ID, inst.Name, inst.NameAm, inst.Email, inst.PasswordHash,
		inst.Phone, inst.LogoURL, inst.Status,
		inst.Address.Street, inst.Address.City, inst.Address.Region, inst.Address.Country,
		inst.CreatedAt, inst.UpdatedAt, inst.Audit.CreatedBy, inst.Audit.UpdatedBy,
	)
	return err
}

func (r *Repository) InstituteByID(ctx context.Context, id uuid.UUID) (*domain.Institute, error) {
	return scanInstitute(r.db.QueryRow(ctx, `SELECT `+instituteCols+` FROM institutes WHERE id=$1`, id))
}

func (r *Repository) InstituteByEmail(ctx context.Context, email string) (*domain.Institute, error) {
	return scanInstitute(r.db.QueryRow(ctx, `SELECT `+instituteCols+` FROM institutes WHERE email=$1`, email))
}

func (r *Repository) ListInstitutes(ctx context.Context, search, status string, page int) ([]*domain.Institute, int, error) {
	where, args := buildFilter(search, status, "org_status", 1)
	rows, err := r.db.Query(ctx,
		`SELECT `+instituteCols+` FROM institutes `+where+` ORDER BY created_at DESC LIMIT 20 OFFSET $`+argN(len(args)+1),
		append(args, (page-1)*20)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*domain.Institute
	for rows.Next() {
		inst, err := scanInstituteRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, inst)
	}
	var total int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM institutes `+where, args...).Scan(&total)
	return out, total, nil
}

type ActiveInstitute struct {
	ID     uuid.UUID
	Name   string
	Status domain.OrgStatus
	City   string
	Region string
}

func (r *Repository) ListActiveInstitutes(ctx context.Context, page, limit int) ([]ActiveInstitute, int, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM institutes WHERE status='active'`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, name, status, COALESCE(city,''), COALESCE(region,'')
		FROM institutes
		WHERE status='active'
		ORDER BY name ASC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]ActiveInstitute, 0, limit)
	for rows.Next() {
		var item ActiveInstitute
		if err := rows.Scan(&item.ID, &item.Name, &item.Status, &item.City, &item.Region); err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *Repository) UpdateInstituteFields(ctx context.Context, id uuid.UUID, fields map[string]any, updatedBy uuid.UUID) error {
	return updateFields(ctx, r.db, "institutes", id, fields, updatedBy)
}

func (r *Repository) UpdateInstituteStatus(ctx context.Context, id uuid.UUID, status domain.OrgStatus, updatedBy uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE institutes SET status=$2, updated_at=NOW(), updated_by=$3 WHERE id=$1`,
		id, status, updatedBy)
	return err
}

// ── Transport Authorities ─────────────────────────────────────────────────────

func (r *Repository) CreateAuthority(ctx context.Context, a *domain.TransportAuthority) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO transport_authorities
			(id, name, name_am, email, password_hash, phone, logo_url, status,
			 street, city, region, country,
			 created_at, updated_at, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		a.ID, a.Name, a.NameAm, a.Email, a.PasswordHash,
		a.Phone, a.LogoURL, a.Status,
		a.Address.Street, a.Address.City, a.Address.Region, a.Address.Country,
		a.CreatedAt, a.UpdatedAt, a.Audit.CreatedBy, a.Audit.UpdatedBy,
	)
	return err
}

func (r *Repository) AuthorityByID(ctx context.Context, id uuid.UUID) (*domain.TransportAuthority, error) {
	return scanAuthority(r.db.QueryRow(ctx, `SELECT `+authorityCols+` FROM transport_authorities WHERE id=$1`, id))
}

func (r *Repository) AuthorityByEmail(ctx context.Context, email string) (*domain.TransportAuthority, error) {
	return scanAuthority(r.db.QueryRow(ctx, `SELECT `+authorityCols+` FROM transport_authorities WHERE email=$1`, email))
}

func (r *Repository) ListAuthorities(ctx context.Context, search, status string, page int) ([]*domain.TransportAuthority, int, error) {
	where, args := buildFilter(search, status, "org_status", 1)
	rows, err := r.db.Query(ctx,
		`SELECT `+authorityCols+` FROM transport_authorities `+where+` ORDER BY created_at DESC LIMIT 20 OFFSET $`+argN(len(args)+1),
		append(args, (page-1)*20)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*domain.TransportAuthority
	for rows.Next() {
		a, err := scanAuthorityRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	var total int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM transport_authorities `+where, args...).Scan(&total)
	return out, total, nil
}

func (r *Repository) UpdateAuthorityFields(ctx context.Context, id uuid.UUID, fields map[string]any, updatedBy uuid.UUID) error {
	return updateFields(ctx, r.db, "transport_authorities", id, fields, updatedBy)
}

// ── Admins ───────────────────────────────────────────────────────────────────

func (r *Repository) CreateAdmin(ctx context.Context, a *domain.Admin) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO admins
			(id, first_name, middle_name, last_name, email, password_hash, status,
			 test_center_id, created_at, updated_at, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		a.ID, a.FirstName, a.MiddleName, a.LastName, a.Email, a.PasswordHash, a.Status,
		a.TestCenterID, a.CreatedAt, a.UpdatedAt, a.Audit.CreatedBy, a.Audit.UpdatedBy,
	)
	return err
}

func (r *Repository) AdminByID(ctx context.Context, id uuid.UUID) (*domain.Admin, error) {
	return scanAdmin(r.db.QueryRow(ctx, `SELECT `+adminCols+` FROM admins WHERE id=$1`, id))
}

func (r *Repository) AdminByEmail(ctx context.Context, email string) (*domain.Admin, error) {
	return scanAdmin(r.db.QueryRow(ctx, `SELECT `+adminCols+` FROM admins WHERE email=$1`, email))
}

func (r *Repository) ListAdmins(ctx context.Context, search, status string, centerID *uuid.UUID, page int) ([]*domain.Admin, int, error) {
	where, args := buildFilter(search, status, "user_status", 1)
	if centerID != nil {
		if len(args) == 0 {
			where = "WHERE test_center_id=$1"
			args = append(args, *centerID)
		} else {
			where += fmt.Sprintf(" AND test_center_id=$%d", len(args)+1)
			args = append(args, *centerID)
		}
	}
	rows, err := r.db.Query(ctx,
		`SELECT `+adminCols+` FROM admins `+where+` ORDER BY created_at DESC LIMIT 20 OFFSET $`+argN(len(args)+1),
		append(args, (page-1)*20)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*domain.Admin
	for rows.Next() {
		a, err := scanAdminRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	var total int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM admins `+where, args...).Scan(&total)
	return out, total, nil
}

func (r *Repository) UpdateAdminFields(ctx context.Context, id uuid.UUID, fields map[string]any, updatedBy uuid.UUID) error {
	return updateFields(ctx, r.db, "admins", id, fields, updatedBy)
}

func (r *Repository) UpdateAdminStatus(ctx context.Context, id uuid.UUID, status domain.UserStatus, updatedBy uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE admins SET status=$2, updated_at=NOW(), updated_by=$3 WHERE id=$1`,
		id, status, updatedBy)
	return err
}

func (r *Repository) DeleteAdmin(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM admins WHERE id=$1`, id)
	return err
}

// ── SuperAdmins ───────────────────────────────────────────────────────────────

func (r *Repository) CreateSuperAdmin(ctx context.Context, sa *domain.SuperAdmin) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO super_admins (id, name, email, password_hash, created_at, updated_at, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		sa.ID, sa.Name, sa.Email, sa.PasswordHash,
		sa.CreatedAt, sa.UpdatedAt, sa.Audit.CreatedBy, sa.Audit.UpdatedBy,
	)
	return err
}

func (r *Repository) SuperAdminByID(ctx context.Context, id uuid.UUID) (*domain.SuperAdmin, error) {
	return scanSuperAdmin(r.db.QueryRow(ctx, `SELECT `+superAdminCols+` FROM super_admins WHERE id=$1`, id))
}

func (r *Repository) SuperAdminByEmail(ctx context.Context, email string) (*domain.SuperAdmin, error) {
	return scanSuperAdmin(r.db.QueryRow(ctx, `SELECT `+superAdminCols+` FROM super_admins WHERE email=$1`, email))
}

func (r *Repository) ListSuperAdmins(ctx context.Context, search string, page int) ([]*domain.SuperAdmin, int, error) {
	where, args := "", []any{}
	if search != "" {
		where = "WHERE name ILIKE $1"
		args = append(args, "%"+search+"%")
	}
	rows, err := r.db.Query(ctx,
		`SELECT `+superAdminCols+` FROM super_admins `+where+` ORDER BY created_at DESC LIMIT 20 OFFSET $`+argN(len(args)+1),
		append(args, (page-1)*20)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*domain.SuperAdmin
	for rows.Next() {
		sa, err := scanSuperAdminRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, sa)
	}
	var total int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM super_admins `+where, args...).Scan(&total)
	return out, total, nil
}

func (r *Repository) UpdateSuperAdminFields(ctx context.Context, id uuid.UUID, fields map[string]any, updatedBy uuid.UUID) error {
	return updateFields(ctx, r.db, "super_admins", id, fields, updatedBy)
}

func (r *Repository) DeleteSuperAdmin(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM super_admins WHERE id=$1`, id)
	return err
}

// ── Invitations ───────────────────────────────────────────────────────────────

func (r *Repository) CreateInvitation(ctx context.Context, inv *domain.Invitation) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO invitations (id, token, email, entity_type, test_center_id, expires_at, created_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		inv.ID, inv.Token, inv.Email, inv.EntityType, inv.TestCenterID,
		inv.ExpiresAt, inv.CreatedBy, inv.CreatedAt,
	)
	return err
}

func (r *Repository) InvitationByToken(ctx context.Context, token string) (*domain.Invitation, error) {
	var inv domain.Invitation
	err := r.db.QueryRow(ctx,
		`SELECT id, token, email, entity_type, test_center_id, expires_at, used_at, created_by, created_at
		 FROM invitations WHERE token=$1`, token,
	).Scan(&inv.ID, &inv.Token, &inv.Email, &inv.EntityType, &inv.TestCenterID,
		&inv.ExpiresAt, &inv.UsedAt, &inv.CreatedBy, &inv.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &inv, nil
}

func (r *Repository) InvitationByID(ctx context.Context, id uuid.UUID) (*domain.Invitation, error) {
	var inv domain.Invitation
	err := r.db.QueryRow(ctx,
		`SELECT id, token, email, entity_type, test_center_id, expires_at, used_at, created_by, created_at
		 FROM invitations WHERE id=$1`, id,
	).Scan(&inv.ID, &inv.Token, &inv.Email, &inv.EntityType, &inv.TestCenterID,
		&inv.ExpiresAt, &inv.UsedAt, &inv.CreatedBy, &inv.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &inv, nil
}

func (r *Repository) UpdateInvitationToken(ctx context.Context, id uuid.UUID, token string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE invitations SET token=$2, expires_at=$3 WHERE id=$1 AND used_at IS NULL`, id, token, expiresAt)
	return err
}

func (r *Repository) MarkInvitationUsed(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE invitations SET used_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *Repository) ListInvitations(ctx context.Context, status string, centerID *uuid.UUID, page int) ([]*domain.Invitation, int, error) {
	conds := []string{}
	args := []any{}
	if status != "" {
		switch status {
		case "pending":
			conds = append(conds, "used_at IS NULL AND expires_at > NOW()")
		case "used":
			conds = append(conds, "used_at IS NOT NULL")
		case "expired":
			conds = append(conds, "used_at IS NULL AND expires_at <= NOW()")
		}
	}
	if centerID != nil {
		args = append(args, *centerID)
		conds = append(conds, fmt.Sprintf("test_center_id=$%d", len(args)))
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, (page-1)*20)
	q := fmt.Sprintf("SELECT id, token, email, entity_type, test_center_id, expires_at, used_at, created_by, created_at FROM invitations %s ORDER BY created_at DESC LIMIT 20 OFFSET $%d", where, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*domain.Invitation
	for rows.Next() {
		var inv domain.Invitation
		if err := rows.Scan(&inv.ID, &inv.Token, &inv.Email, &inv.EntityType, &inv.TestCenterID,
			&inv.ExpiresAt, &inv.UsedAt, &inv.CreatedBy, &inv.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, &inv)
	}
	countArgs := args[:len(args)-1]
	var total int
	_ = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM invitations "+where, countArgs...).Scan(&total)
	return out, total, nil
}

func (r *Repository) CancelInvitation(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM invitations WHERE id=$1 AND used_at IS NULL`, id)
	return err
}

// ── OTP ───────────────────────────────────────────────────────────────────────

func (r *Repository) UpsertOTP(ctx context.Context, email, code string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO otp_codes (email, code, expires_at, attempts, created_at)
		VALUES ($1, $2, $3, 0, NOW())
		ON CONFLICT (email) DO UPDATE SET code=$2, expires_at=$3, attempts=0, created_at=NOW()`,
		email, code, expiresAt,
	)
	return err
}

func (r *Repository) VerifyOTP(ctx context.Context, email, code string) (bool, error) {
	var stored string
	var expiresAt time.Time
	var attempts int
	err := r.db.QueryRow(ctx,
		`UPDATE otp_codes SET attempts=attempts+1 WHERE email=$1 RETURNING code, expires_at, attempts`,
		email,
	).Scan(&stored, &expiresAt, &attempts)
	if err != nil {
		return false, nil // no OTP found
	}
	if attempts > 5 {
		_, _ = r.db.Exec(ctx, `DELETE FROM otp_codes WHERE email=$1`, email)
		return false, fmt.Errorf("too many attempts")
	}
	if time.Now().After(expiresAt) {
		_, _ = r.db.Exec(ctx, `DELETE FROM otp_codes WHERE email=$1`, email)
		return false, fmt.Errorf("OTP expired")
	}
	if stored != code {
		return false, nil
	}
	_, _ = r.db.Exec(ctx, `DELETE FROM otp_codes WHERE email=$1`, email)
	return true, nil
}

// ── Password reset ────────────────────────────────────────────────────────────

func (r *Repository) UpsertPasswordResetToken(ctx context.Context, email, token string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO password_reset_tokens (email, token, expires_at, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (email) DO UPDATE SET token=$2, expires_at=$3, created_at=NOW()`,
		email, token, expiresAt,
	)
	return err
}

func (r *Repository) ConsumePasswordResetToken(ctx context.Context, token string) (string, error) {
	var email string
	var expiresAt time.Time
	err := r.db.QueryRow(ctx,
		`DELETE FROM password_reset_tokens WHERE token=$1 RETURNING email, expires_at`,
		token,
	).Scan(&email, &expiresAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", ErrInvalidResetToken
		}
		return "", err
	}
	if time.Now().After(expiresAt) {
		return "", ErrInvalidResetToken
	}
	return email, nil
}

// ── Password change ───────────────────────────────────────────────────────────

// GetPasswordHash returns the bcrypt hash stored for the given entity.
func (r *Repository) GetPasswordHash(ctx context.Context, table string, id uuid.UUID) (string, error) {
	var hash string
	err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT password_hash FROM %s WHERE id=$1`, table), id,
	).Scan(&hash)
	if err != nil {
		return "", err
	}
	return hash, nil
}

// UpdatePassword sets a new bcrypt hash for the given entity.
func (r *Repository) UpdatePassword(ctx context.Context, table string, id uuid.UUID, hash string) error {
	_, err := r.db.Exec(ctx,
		fmt.Sprintf(`UPDATE %s SET password_hash=$2, updated_at=NOW() WHERE id=$1`, table),
		id, hash,
	)
	return err
}

// ── Shared helpers ────────────────────────────────────────────────────────────

// updateFields builds a dynamic UPDATE SET from the provided map.
// Only safe because callers construct the map — never derived from raw user input.
func updateFields(ctx context.Context, db *pgxpool.Pool, table string, id uuid.UUID, fields map[string]any, updatedBy uuid.UUID) error {
	if len(fields) == 0 {
		return nil
	}
	setClauses := make([]string, 0, len(fields)+2)
	args := make([]any, 0, len(fields)+2)
	i := 1
	for col, val := range fields {
		setClauses = append(setClauses, fmt.Sprintf("%s=$%d", col, i))
		args = append(args, val)
		i++
	}
	setClauses = append(setClauses, fmt.Sprintf("updated_at=NOW()"))
	setClauses = append(setClauses, fmt.Sprintf("updated_by=$%d", i))
	args = append(args, updatedBy)
	i++
	args = append(args, id)
	q := fmt.Sprintf("UPDATE %s SET %s WHERE id=$%d", table, strings.Join(setClauses, ","), i)
	_, err := db.Exec(ctx, q, args...)
	return err
}

func buildFilter(search, status, statusType string, startIdx int) (string, []any) {
	conds := []string{}
	args := []any{}
	if status != "" {
		args = append(args, status)
		conds = append(conds, fmt.Sprintf("status=$%d", len(args)))
	}
	if search != "" {
		args = append(args, "%"+search+"%")
		conds = append(conds, fmt.Sprintf("(first_name ILIKE $%d OR last_name ILIKE $%d OR email ILIKE $%d OR name ILIKE $%d)",
			len(args), len(args), len(args), len(args)))
	}
	if len(conds) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

func argN(n int) string { return fmt.Sprintf("%d", n) }

// ── Column lists ──────────────────────────────────────────────────────────────

const candidateCols = `id, first_name, middle_name, last_name, email, password_hash, status,
	phone, fayida_id, birth_date, gender, photo_url,
	street, city, region, country,
	created_at, updated_at, created_by, updated_by`

const expertCols = `id, first_name, middle_name, last_name, email, password_hash, status,
	phone, fayida_id, employee_id, birth_date, gender, photo_url,
	created_at, updated_at, created_by, updated_by`

const instituteCols = `id, name, name_am, email, password_hash, phone, logo_url, status,
	street, city, region, country,
	created_at, updated_at, created_by, updated_by`

const authorityCols = `id, name, name_am, email, password_hash, phone, logo_url, status,
	street, city, region, country,
	created_at, updated_at, created_by, updated_by`

const adminCols = `id, first_name, middle_name, last_name, email, password_hash, status,
	test_center_id, created_at, updated_at, created_by, updated_by`

const superAdminCols = `id, name, email, password_hash, created_at, updated_at, created_by, updated_by`

// ── Row scanners ──────────────────────────────────────────────────────────────

type scannable interface {
	Scan(dest ...any) error
}

func scanCandidate(row scannable) (*domain.Candidate, error) {
	var c domain.Candidate
	err := row.Scan(
		&c.ID, &c.FirstName, &c.MiddleName, &c.LastName, &c.Email, &c.PasswordHash, &c.Status,
		&c.Phone, &c.FayidaID, &c.BirthDate, &c.Gender, &c.PhotoURL,
		&c.Address.Street, &c.Address.City, &c.Address.Region, &c.Address.Country,
		&c.CreatedAt, &c.UpdatedAt, &c.Audit.CreatedBy, &c.Audit.UpdatedBy,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func scanCandidateRow(rows pgx.Rows) (*domain.Candidate, error) {
	return scanCandidate(rows)
}

func scanExpert(row scannable) (*domain.Expert, error) {
	var e domain.Expert
	err := row.Scan(
		&e.ID, &e.FirstName, &e.MiddleName, &e.LastName, &e.Email, &e.PasswordHash, &e.Status,
		&e.Phone, &e.FayidaID, &e.EmployeeID, &e.BirthDate, &e.Gender, &e.PhotoURL,
		&e.CreatedAt, &e.UpdatedAt, &e.Audit.CreatedBy, &e.Audit.UpdatedBy,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

func scanExpertRow(rows pgx.Rows) (*domain.Expert, error) { return scanExpert(rows) }

func scanInstitute(row scannable) (*domain.Institute, error) {
	var inst domain.Institute
	err := row.Scan(
		&inst.ID, &inst.Name, &inst.NameAm, &inst.Email, &inst.PasswordHash,
		&inst.Phone, &inst.LogoURL, &inst.Status,
		&inst.Address.Street, &inst.Address.City, &inst.Address.Region, &inst.Address.Country,
		&inst.CreatedAt, &inst.UpdatedAt, &inst.Audit.CreatedBy, &inst.Audit.UpdatedBy,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &inst, nil
}

func scanInstituteRow(rows pgx.Rows) (*domain.Institute, error) { return scanInstitute(rows) }

func scanAuthority(row scannable) (*domain.TransportAuthority, error) {
	var a domain.TransportAuthority
	err := row.Scan(
		&a.ID, &a.Name, &a.NameAm, &a.Email, &a.PasswordHash,
		&a.Phone, &a.LogoURL, &a.Status,
		&a.Address.Street, &a.Address.City, &a.Address.Region, &a.Address.Country,
		&a.CreatedAt, &a.UpdatedAt, &a.Audit.CreatedBy, &a.Audit.UpdatedBy,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

func scanAuthorityRow(rows pgx.Rows) (*domain.TransportAuthority, error) { return scanAuthority(rows) }

func scanAdmin(row scannable) (*domain.Admin, error) {
	var a domain.Admin
	err := row.Scan(
		&a.ID, &a.FirstName, &a.MiddleName, &a.LastName, &a.Email, &a.PasswordHash, &a.Status,
		&a.TestCenterID, &a.CreatedAt, &a.UpdatedAt, &a.Audit.CreatedBy, &a.Audit.UpdatedBy,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

func scanAdminRow(rows pgx.Rows) (*domain.Admin, error) { return scanAdmin(rows) }

func scanSuperAdmin(row scannable) (*domain.SuperAdmin, error) {
	var sa domain.SuperAdmin
	err := row.Scan(
		&sa.ID, &sa.Name, &sa.Email, &sa.PasswordHash,
		&sa.CreatedAt, &sa.UpdatedAt, &sa.Audit.CreatedBy, &sa.Audit.UpdatedBy,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &sa, nil
}

func scanSuperAdminRow(rows pgx.Rows) (*domain.SuperAdmin, error) { return scanSuperAdmin(rows) }

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullIfZeroTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

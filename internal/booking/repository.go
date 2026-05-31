package booking

import (
	"context"
	"errors"
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

// -------------------------------------------------------
// Booking queries
// -------------------------------------------------------

func (r *Repository) CreateBooking(ctx context.Context, b domain.Booking, createdBy uuid.UUID) (domain.Booking, error) {
	b.ID = uuid.New()
	now := time.Now().UTC()
	b.CreatedAt = now
	b.UpdatedAt = now

	const q = `
		INSERT INTO bookings
			(id, candidate_id, institute_id, test_id, slot_id, status,
			 requires_verification, verified_by, verified_at, rejection_reason,
			 scheduled_at, payment_ref, payment_status, payment_amount_cents,
			 payment_attempts, archived_at, created_at, updated_at, created_by, updated_by)
		VALUES
			($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`

	if _, err := r.db.Exec(ctx, q,
		b.ID, b.CandidateID, b.InstituteID, b.TestID, b.SlotID, b.Status,
		b.RequiresVerification, b.VerifiedBy, b.VerifiedAt, b.RejectionReason,
		b.ScheduledAt, b.PaymentRef, b.PaymentStatus, b.PaymentAmountCents,
		b.PaymentAttempts, b.ArchivedAt, b.CreatedAt, b.UpdatedAt, createdBy, createdBy,
	); err != nil {
		return domain.Booking{}, err
	}
	return r.BookingByID(ctx, b.ID)
}

func (r *Repository) BookingByID(ctx context.Context, id uuid.UUID) (domain.Booking, error) {
	row := r.db.QueryRow(ctx, bookingDetailQuery(`WHERE b.id=$1`), id)
	return scanBooking(row)
}

type BookingFilter struct {
	CandidateID *uuid.UUID
	InstituteID *uuid.UUID
	Status      *domain.BookingStatus
}

func (r *Repository) ListBookings(ctx context.Context, f BookingFilter, page int) ([]domain.Booking, int, error) {
	if page < 1 {
		page = 1
	}
	where, args := buildBookingFilter(f)
	args = append(args, (page-1)*20)
	q := fmt.Sprintf(`%s ORDER BY b.created_at DESC LIMIT 20 OFFSET $%d`, bookingDetailQuery(where), len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []domain.Booking
	for rows.Next() {
		b, err := scanBookingRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, b)
	}

	countArgs := args[:len(args)-1]
	var total int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM bookings b `+where, countArgs...).Scan(&total)
	return out, total, nil
}

func (r *Repository) UpdateBookingFields(ctx context.Context, id uuid.UUID, fields map[string]any, updatedBy uuid.UUID) error {
	return updateFields(ctx, r.db, "bookings", id, fields, updatedBy)
}

func (r *Repository) DeleteBooking(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM bookings WHERE id=$1`, id)
	return err
}

// -------------------------------------------------------
// Slot queries
// -------------------------------------------------------

func (r *Repository) CreateSlot(ctx context.Context, s domain.Slot, createdBy uuid.UUID) (domain.Slot, error) {
	s.ID = uuid.New()
	now := time.Now().UTC()
	s.CreatedAt = now
	s.UpdatedAt = now

	const q = `
		INSERT INTO slots
			(id, institute_id, test_center_id, test_id, starts_at, ends_at, capacity, booked_count,
			 start_time, end_time, created_at, updated_at, created_by, updated_by)
		VALUES
			($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING ` + slotCols

	row := r.db.QueryRow(ctx, q,
		s.ID, s.InstituteID, s.TestCenterID, s.TestID, s.StartsAt, s.EndsAt, s.Capacity, s.BookedCount,
		s.StartsAt, s.EndsAt, s.CreatedAt, s.UpdatedAt, createdBy, createdBy,
	)
	return scanSlot(row)
}

func (r *Repository) SlotByID(ctx context.Context, id uuid.UUID) (domain.Slot, error) {
	row := r.db.QueryRow(ctx, `SELECT `+slotCols+` FROM slots WHERE id=$1`, id)
	return scanSlot(row)
}

func (r *Repository) ListSlots(ctx context.Context, instituteID uuid.UUID, page int) ([]domain.Slot, int, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * 20
	rows, err := r.db.Query(ctx,
		`SELECT `+slotCols+` FROM slots WHERE institute_id=$1 ORDER BY starts_at ASC LIMIT 20 OFFSET $2`,
		instituteID, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []domain.Slot
	for rows.Next() {
		s, err := scanSlotRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, s)
	}
	var total int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM slots WHERE institute_id=$1`, instituteID).Scan(&total)
	return out, total, nil
}

func (r *Repository) IncrementSlotBookedCount(ctx context.Context, slotID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE slots SET booked_count=booked_count+1, updated_at=NOW() WHERE id=$1`,
		slotID)
	return err
}

func (r *Repository) DecrementSlotBookedCount(ctx context.Context, slotID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE slots SET booked_count=GREATEST(booked_count-1,0), updated_at=NOW() WHERE id=$1`,
		slotID)
	return err
}

// -------------------------------------------------------
// Payment queries
// -------------------------------------------------------

func (r *Repository) CreatePayment(ctx context.Context, p domain.Payment) (domain.Payment, error) {
	p.ID = uuid.New()
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now

	const q = `
		INSERT INTO payments
			(id, booking_id, amount_cents, currency, status, provider,
			 provider_ref, attempt_number, created_at, updated_at)
		VALUES
			($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING ` + paymentCols

	row := r.db.QueryRow(ctx, q,
		p.ID, p.BookingID, p.AmountCents, p.Currency, p.Status, p.Provider,
		p.ProviderRef, p.AttemptNumber, p.CreatedAt, p.UpdatedAt,
	)
	return scanPayment(row)
}

func (r *Repository) PaymentByID(ctx context.Context, id uuid.UUID) (domain.Payment, error) {
	row := r.db.QueryRow(ctx, `SELECT `+paymentCols+` FROM payments WHERE id=$1`, id)
	return scanPayment(row)
}

func (r *Repository) PaymentByProviderRef(ctx context.Context, providerRef string) (domain.Payment, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+paymentCols+` FROM payments WHERE provider_ref=$1 ORDER BY created_at DESC LIMIT 1`,
		providerRef)
	return scanPayment(row)
}

func (r *Repository) ListPaymentsByBooking(ctx context.Context, bookingID uuid.UUID) ([]domain.Payment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+paymentCols+` FROM payments WHERE booking_id=$1 ORDER BY attempt_number ASC`,
		bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Payment
	for rows.Next() {
		p, err := scanPaymentRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *Repository) UpdatePaymentFields(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now().UTC()
	setClauses := make([]string, 0, len(fields))
	args := make([]any, 0, len(fields)+1)
	i := 1
	for col, val := range fields {
		setClauses = append(setClauses, fmt.Sprintf("%s=$%d", col, i))
		args = append(args, val)
		i++
	}
	args = append(args, id)
	q := fmt.Sprintf("UPDATE payments SET %s WHERE id=$%d", strings.Join(setClauses, ","), i)
	_, err := r.db.Exec(ctx, q, args...)
	return err
}

func (r *Repository) LatestPaymentAttemptNumber(ctx context.Context, bookingID uuid.UUID) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(attempt_number), 0) FROM payments WHERE booking_id=$1`,
		bookingID).Scan(&n)
	return n, err
}

type TestCreationDetails struct {
	BookingID     uuid.UUID
	CandidateID   uuid.UUID
	TestCenterID  uuid.UUID
	TestLevelCode string
}

func (r *Repository) TestCreationDetails(ctx context.Context, bookingID uuid.UUID) (TestCreationDetails, error) {
	var d TestCreationDetails
	var testCenterID uuid.NullUUID
	err := r.db.QueryRow(ctx, `
		SELECT
			b.id,
			b.candidate_id,
			COALESCE(b.test_center_id, s.test_center_id),
			COALESCE(NULLIF(b.test_level_code, ''), 'class_b')
		FROM bookings b
		LEFT JOIN slots s ON s.id = b.slot_id
		WHERE b.id = $1
	`, bookingID).Scan(&d.BookingID, &d.CandidateID, &testCenterID, &d.TestLevelCode)
	if err != nil {
		if err == pgx.ErrNoRows {
			return TestCreationDetails{}, ErrBookingNotFound
		}
		return TestCreationDetails{}, err
	}
	if !testCenterID.Valid {
		return TestCreationDetails{}, errors.New("booking test center is not set")
	}
	d.TestCenterID = testCenterID.UUID
	return d, nil
}

// -------------------------------------------------------
// Lookup helpers
// -------------------------------------------------------

type CandidateContact struct {
	Email     string
	FirstName string
	LastName  string
	Phone     string
}

func (r *Repository) CandidateContactByID(ctx context.Context, id uuid.UUID) (CandidateContact, error) {
	var c CandidateContact
	err := r.db.QueryRow(ctx,
		`SELECT email, first_name, last_name, phone FROM candidates WHERE id=$1`,
		id,
	).Scan(&c.Email, &c.FirstName, &c.LastName, &c.Phone)
	if err != nil {
		if err == pgx.ErrNoRows {
			return CandidateContact{}, ErrCandidateNotFound
		}
		return CandidateContact{}, err
	}
	return c, nil
}

func (r *Repository) InstituteExists(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM institutes WHERE id=$1)`,
		id,
	).Scan(&exists)
	return exists, err
}

func (r *Repository) CandidateStatusByID(ctx context.Context, id uuid.UUID) (string, error) {
	var status string
	err := r.db.QueryRow(ctx, `SELECT status FROM candidates WHERE id=$1`, id).Scan(&status)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", ErrCandidateNotFound
		}
		return "", err
	}
	return status, nil
}

// -------------------------------------------------------
// Shared helpers
// -------------------------------------------------------

type scannable interface {
	Scan(dest ...any) error
}

func scanBooking(row scannable) (domain.Booking, error) {
	var b domain.Booking
	err := row.Scan(
		&b.ID, &b.CandidateID, &b.InstituteID, &b.TestID, &b.SlotID,
		&b.Status, &b.RequiresVerification, &b.VerifiedBy, &b.VerifiedAt,
		&b.RejectionReason, &b.ScheduledAt, &b.PaymentRef, &b.PaymentStatus,
		&b.PaymentAmountCents, &b.PaymentAttempts, &b.ArchivedAt,
		&b.CreatedAt, &b.UpdatedAt, &b.CandidateName, &b.CandidateEmail,
		&b.CandidatePhone, &b.CandidateFayidaID, &b.InstituteName,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.Booking{}, ErrBookingNotFound
		}
		return domain.Booking{}, err
	}
	return b, nil
}

func scanBookingRow(rows pgx.Rows) (domain.Booking, error) {
	return scanBooking(rows)
}

func scanSlot(row scannable) (domain.Slot, error) {
	var s domain.Slot
	err := row.Scan(
		&s.ID, &s.InstituteID, &s.TestCenterID, &s.TestID, &s.StartsAt, &s.EndsAt,
		&s.Capacity, &s.BookedCount, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.Slot{}, ErrSlotNotFound
		}
		return domain.Slot{}, err
	}
	return s, nil
}

func scanSlotRow(rows pgx.Rows) (domain.Slot, error) {
	return scanSlot(rows)
}

func scanPayment(row scannable) (domain.Payment, error) {
	var p domain.Payment
	err := row.Scan(
		&p.ID, &p.BookingID, &p.AmountCents, &p.Currency, &p.Status,
		&p.Provider, &p.ProviderRef, &p.AttemptNumber, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.Payment{}, ErrPaymentNotFound
		}
		return domain.Payment{}, err
	}
	return p, nil
}

func scanPaymentRow(rows pgx.Rows) (domain.Payment, error) {
	return scanPayment(rows)
}

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
	setClauses = append(setClauses, "updated_at=NOW()")
	setClauses = append(setClauses, fmt.Sprintf("updated_by=$%d", i))
	args = append(args, updatedBy)
	i++
	args = append(args, id)
	q := fmt.Sprintf("UPDATE %s SET %s WHERE id=$%d", table, strings.Join(setClauses, ","), i)
	_, err := db.Exec(ctx, q, args...)
	return err
}

func buildBookingFilter(f BookingFilter) (string, []any) {
	conds := []string{}
	args := []any{}
	if f.CandidateID != nil {
		args = append(args, *f.CandidateID)
		conds = append(conds, fmt.Sprintf("b.candidate_id=$%d", len(args)))
	}
	if f.InstituteID != nil {
		args = append(args, *f.InstituteID)
		conds = append(conds, fmt.Sprintf("b.institute_id=$%d", len(args)))
	}
	if f.Status != nil {
		args = append(args, string(*f.Status))
		conds = append(conds, fmt.Sprintf("b.status=$%d", len(args)))
	}
	if len(conds) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

const bookingCols = `id, candidate_id, institute_id, test_id, slot_id, status,
	requires_verification, verified_by, verified_at, rejection_reason, scheduled_at,
	payment_ref, payment_status, payment_amount_cents, payment_attempts, archived_at,
	created_at, updated_at`

const bookingDetailCols = `b.id, b.candidate_id, b.institute_id, b.test_id, b.slot_id, b.status,
	b.requires_verification, b.verified_by, b.verified_at, b.rejection_reason, b.scheduled_at,
	b.payment_ref, b.payment_status, b.payment_amount_cents, b.payment_attempts, b.archived_at,
	b.created_at, b.updated_at,
	concat_ws(' ', c.first_name, NULLIF(c.middle_name, ''), c.last_name) AS candidate_name,
	c.email AS candidate_email,
	c.phone AS candidate_phone,
	c.fayida_id AS candidate_fayida_id,
	i.name AS institute_name`

func bookingDetailQuery(where string) string {
	return fmt.Sprintf(`SELECT %s
		FROM bookings b
		LEFT JOIN candidates c ON c.id = b.candidate_id
		LEFT JOIN institutes i ON i.id = b.institute_id
		%s`, bookingDetailCols, where)
}

const slotCols = `id, institute_id, test_center_id, test_id, starts_at, ends_at, capacity,
	booked_count, created_at, updated_at`

const paymentCols = `id, booking_id, amount_cents, currency, status, provider,
	provider_ref, attempt_number, created_at, updated_at`

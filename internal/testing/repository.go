package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"adlts/internal/domain"
	"adlts/internal/platform/security"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the shared data access layer for the testing module.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── updateFields helper (mirrors identity pattern) ────────────────────────────

func (r *Repository) updateFields(
	ctx context.Context,
	table string,
	id uuid.UUID,
	fields map[string]any,
	updatedBy uuid.UUID,
) error {
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now()
	fields["updated_by"] = updatedBy

	setClauses := make([]string, 0, len(fields))
	args := make([]any, 0, len(fields)+1)
	i := 1
	for col, val := range fields {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	args = append(args, id)
	q := fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d", table, strings.Join(setClauses, ", "), i)
	_, err := r.db.Exec(ctx, q, args...)
	return err
}

// ── Device methods ────────────────────────────────────────────────────────────

const deviceCols = `id, device_code, password_hash, test_center_id,
	allowed_levels, stream_url, status, current_test_id, last_seen_at,
	created_at, updated_at, created_by, updated_by`

func scanDevice(row pgx.Row) (*domain.Device, error) {
	d := &domain.Device{}
	var levelsJSON string
	err := row.Scan(
		&d.ID, &d.DeviceCode, &d.PasswordHash, &d.TestCenterID,
		&levelsJSON, &d.StreamURL, &d.Status, &d.CurrentTestID, &d.LastSeenAt,
		&d.Audit.CreatedAt, &d.Audit.UpdatedAt, &d.Audit.CreatedBy, &d.Audit.UpdatedBy,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(levelsJSON), &d.AllowedLevels)
	return d, nil
}

func (r *Repository) CreateDevice(ctx context.Context, d *domain.Device) error {
	levelsJSON, _ := json.Marshal(d.AllowedLevels)
	_, err := r.db.Exec(ctx,
		`INSERT INTO devices (id, device_code, password_hash, test_center_id,
			allowed_levels, stream_url, status, created_at, updated_at, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		d.ID, d.DeviceCode, d.PasswordHash, d.TestCenterID,
		string(levelsJSON), d.StreamURL, d.Status,
		d.Audit.CreatedAt, d.Audit.UpdatedAt, d.Audit.CreatedBy, d.Audit.UpdatedBy,
	)
	return err
}

func (r *Repository) DeviceByID(ctx context.Context, id uuid.UUID) (*domain.Device, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+deviceCols+` FROM devices WHERE id = $1`, id)
	d, err := scanDevice(row)
	if err == pgx.ErrNoRows {
		return nil, ErrDeviceNotFound
	}
	return d, err
}

func (r *Repository) DeviceByCode(ctx context.Context, code string) (*domain.Device, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+deviceCols+` FROM devices WHERE device_code = $1`, code)
	d, err := scanDevice(row)
	if err == pgx.ErrNoRows {
		return nil, ErrDeviceNotFound
	}
	return d, err
}

func (r *Repository) ListDevices(ctx context.Context, testCenterID *uuid.UUID, page int) ([]*domain.Device, int, error) {
	const pageSize = 20
	offset := (page - 1) * pageSize

	var totalRow pgx.Row
	var rows pgx.Rows
	var err error

	if testCenterID != nil {
		totalRow = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM devices WHERE test_center_id=$1`, *testCenterID)
		rows, err = r.db.Query(ctx,
			`SELECT `+deviceCols+` FROM devices WHERE test_center_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			*testCenterID, pageSize, offset)
	} else {
		totalRow = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM devices`)
		rows, err = r.db.Query(ctx,
			`SELECT `+deviceCols+` FROM devices ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
			pageSize, offset)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var total int
	if err := totalRow.Scan(&total); err != nil {
		return nil, 0, err
	}

	var devices []*domain.Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, 0, err
		}
		devices = append(devices, d)
	}
	return devices, total, rows.Err()
}

func (r *Repository) UpdateDeviceFields(ctx context.Context, id uuid.UUID, fields map[string]any, updatedBy uuid.UUID) error {
	return r.updateFields(ctx, "devices", id, fields, updatedBy)
}

func (r *Repository) DeleteDevice(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM devices WHERE id=$1`, id)
	return err
}

// DeviceCheckin atomically marks device as in_use and links to current test.
// Returns ErrDeviceInUse if device is not in 'active' status.
func (r *Repository) DeviceCheckin(ctx context.Context, deviceID, testID, actorID uuid.UUID) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE devices SET status='in_use', current_test_id=$2, last_seen_at=NOW(),
			updated_at=NOW(), updated_by=$3
		WHERE id=$1 AND status='active'`,
		deviceID, testID, actorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrDeviceInUse
	}
	return nil
}

func (r *Repository) FrameAnalysesBySessionID(ctx context.Context, sessionID uuid.UUID) ([]*domain.FrameAnalysis, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, test_id, session_id, frame_seq_no, captured_at, lane_detected, curvature_dir, iou_score, frame_score, is_mocked, created_at
		 FROM frame_analyses WHERE session_id=$1 ORDER BY frame_seq_no ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var analyses []*domain.FrameAnalysis
	for rows.Next() {
		fa := &domain.FrameAnalysis{}
		if err := rows.Scan(
			&fa.ID, &fa.TestID, &fa.SessionID, &fa.FrameSeqNo, &fa.CapturedAt,
			&fa.LaneDetected, &fa.CurvatureDir, &fa.IoUScore, &fa.FrameScore, &fa.IsMocked, &fa.CreatedAt,
		); err != nil {
			return nil, err
		}
		analyses = append(analyses, fa)
	}
	return analyses, nil
}

// ReleaseDevice returns the device to 'active' state after a test ends.
func (r *Repository) ReleaseDevice(ctx context.Context, deviceID, actorID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE devices SET status='active', current_test_id=NULL,
			updated_at=NOW(), updated_by=$2
		WHERE id=$1`,
		deviceID, actorID)
	return err
}

// ── TestLevelType methods ─────────────────────────────────────────────────────

func (r *Repository) ListTestLevelTypes(ctx context.Context) ([]*domain.TestLevelType, error) {
	rows, err := r.db.Query(ctx,
		`SELECT code, name, description, sort_order FROM test_level_types ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var types []*domain.TestLevelType
	for rows.Next() {
		t := &domain.TestLevelType{}
		if err := rows.Scan(&t.Code, &t.Name, &t.Description, &t.SortOrder); err != nil {
			return nil, err
		}
		types = append(types, t)
	}
	return types, rows.Err()
}

func (r *Repository) TestLevelTypeExists(ctx context.Context, code string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM test_level_types WHERE code=$1)`, code).Scan(&exists)
	return exists, err
}

// ── TestPlan methods ──────────────────────────────────────────────────────────

const planCols = `id, test_center_id, name, description, pass_threshold,
	status, published_at, created_at, updated_at, created_by, updated_by`

func scanPlan(row pgx.Row) (*domain.TestPlan, error) {
	p := &domain.TestPlan{}
	err := row.Scan(
		&p.ID, &p.TestCenterID, &p.Name, &p.Description, &p.PassThreshold,
		&p.Status, &p.PublishedAt,
		&p.Audit.CreatedAt, &p.Audit.UpdatedAt, &p.Audit.CreatedBy, &p.Audit.UpdatedBy,
	)
	return p, err
}

func (r *Repository) CreateTestPlan(ctx context.Context, p *domain.TestPlan) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO test_plans (id, test_center_id, name, description, pass_threshold,
			status, created_at, updated_at, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		p.ID, p.TestCenterID, p.Name, p.Description, p.PassThreshold,
		p.Status, p.Audit.CreatedAt, p.Audit.UpdatedAt, p.Audit.CreatedBy, p.Audit.UpdatedBy,
	)
	return err
}

func (r *Repository) TestPlanByID(ctx context.Context, id uuid.UUID) (*domain.TestPlan, error) {
	row := r.db.QueryRow(ctx, `SELECT `+planCols+` FROM test_plans WHERE id=$1`, id)
	p, err := scanPlan(row)
	if err == pgx.ErrNoRows {
		return nil, ErrTestPlanNotFound
	}
	return p, err
}

func (r *Repository) ListTestPlans(ctx context.Context, testCenterID *uuid.UUID, page int) ([]*domain.TestPlan, int, error) {
	const pageSize = 20
	offset := (page - 1) * pageSize
	var rows pgx.Rows
	var err error
	var total int

	if testCenterID != nil {
		if e := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM test_plans WHERE test_center_id=$1`, *testCenterID).Scan(&total); e != nil {
			return nil, 0, e
		}
		rows, err = r.db.Query(ctx,
			`SELECT `+planCols+` FROM test_plans WHERE test_center_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			*testCenterID, pageSize, offset)
	} else {
		if e := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM test_plans`).Scan(&total); e != nil {
			return nil, 0, e
		}
		rows, err = r.db.Query(ctx,
			`SELECT `+planCols+` FROM test_plans ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
			pageSize, offset)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var plans []*domain.TestPlan
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, 0, err
		}
		plans = append(plans, p)
	}
	return plans, total, rows.Err()
}

func (r *Repository) UpdateTestPlanFields(ctx context.Context, id uuid.UUID, fields map[string]any, updatedBy uuid.UUID) error {
	return r.updateFields(ctx, "test_plans", id, fields, updatedBy)
}

func (r *Repository) PublishTestPlan(ctx context.Context, id, actorID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE test_plans SET status='active', published_at=NOW(), updated_at=NOW(), updated_by=$2 WHERE id=$1`,
		id, actorID)
	return err
}

func (r *Repository) RetireTestPlan(ctx context.Context, id, actorID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE test_plans SET status='retired', updated_at=NOW(), updated_by=$2 WHERE id=$1`,
		id, actorID)
	return err
}

// ── Maneuver methods ──────────────────────────────────────────────────────────

const maneuverCols = `id, test_plan_id, name, description, sequence_number, qr_code_value,
	COALESCE(reference_mask_url,''), tolerance_px, weight, min_frames_required,
	created_at, updated_at, created_by, updated_by`

func scanManeuver(row pgx.Row) (*domain.ManeuverConfig, error) {
	m := &domain.ManeuverConfig{}
	err := row.Scan(
		&m.ID, &m.TestPlanID, &m.Name, &m.Description, &m.SequenceNumber, &m.QRCodeValue,
		&m.ReferenceMaskURL, &m.TolerancePx, &m.Weight, &m.MinFramesRequired,
		&m.Audit.CreatedAt, &m.Audit.UpdatedAt, &m.Audit.CreatedBy, &m.Audit.UpdatedBy,
	)
	return m, err
}

func (r *Repository) CreateManeuver(ctx context.Context, m *domain.ManeuverConfig) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO maneuver_configs (id, test_plan_id, name, description, sequence_number,
			qr_code_value, tolerance_px, weight, min_frames_required,
			created_at, updated_at, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		m.ID, m.TestPlanID, m.Name, m.Description, m.SequenceNumber,
		m.QRCodeValue, m.TolerancePx, m.Weight, m.MinFramesRequired,
		m.Audit.CreatedAt, m.Audit.UpdatedAt, m.Audit.CreatedBy, m.Audit.UpdatedBy,
	)
	return err
}

func (r *Repository) ManeuverByID(ctx context.Context, id uuid.UUID) (*domain.ManeuverConfig, error) {
	row := r.db.QueryRow(ctx, `SELECT `+maneuverCols+` FROM maneuver_configs WHERE id=$1`, id)
	m, err := scanManeuver(row)
	if err == pgx.ErrNoRows {
		return nil, ErrManeuverNotFound
	}
	return m, err
}

func (r *Repository) ManeuversByPlanID(ctx context.Context, planID uuid.UUID) ([]*domain.ManeuverConfig, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+maneuverCols+` FROM maneuver_configs WHERE test_plan_id=$1 ORDER BY sequence_number`,
		planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ms []*domain.ManeuverConfig
	for rows.Next() {
		m, err := scanManeuver(rows)
		if err != nil {
			return nil, err
		}
		ms = append(ms, m)
	}
	return ms, rows.Err()
}

func (r *Repository) UpdateManeuverFields(ctx context.Context, id uuid.UUID, fields map[string]any, updatedBy uuid.UUID) error {
	return r.updateFields(ctx, "maneuver_configs", id, fields, updatedBy)
}

func (r *Repository) UpdateManeuverMaskURL(ctx context.Context, maneuverID uuid.UUID, url string, updatedBy uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE maneuver_configs SET reference_mask_url=$2, updated_at=NOW(), updated_by=$3 WHERE id=$1`,
		maneuverID, url, updatedBy)
	return err
}

func (r *Repository) DeleteManeuver(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM maneuver_configs WHERE id=$1`, id)
	return err
}

// ── TestLevelMapping methods ──────────────────────────────────────────────────

func (r *Repository) UpsertTestLevelMapping(ctx context.Context, testCenterID uuid.UUID, levelCode string, planID, actorID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO test_level_mappings (id, test_center_id, test_level_code, test_plan_id,
			created_at, updated_at, created_by, updated_by)
		VALUES ($1,$2,$3,$4,NOW(),NOW(),$5,$5)
		ON CONFLICT (test_center_id, test_level_code) DO UPDATE
			SET test_plan_id=EXCLUDED.test_plan_id, updated_at=NOW(), updated_by=$5`,
		uuid.New(), testCenterID, levelCode, planID, actorID,
	)
	return err
}

func (r *Repository) ListTestLevelMappings(ctx context.Context, testCenterID uuid.UUID) ([]*domain.TestLevelMapping, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, test_center_id, test_level_code, test_plan_id, created_at, updated_at, created_by, updated_by
		FROM test_level_mappings WHERE test_center_id=$1`, testCenterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ms []*domain.TestLevelMapping
	for rows.Next() {
		m := &domain.TestLevelMapping{}
		if err := rows.Scan(&m.ID, &m.TestCenterID, &m.TestLevelCode, &m.TestPlanID,
			&m.Audit.CreatedAt, &m.Audit.UpdatedAt, &m.Audit.CreatedBy, &m.Audit.UpdatedBy); err != nil {
			return nil, err
		}
		ms = append(ms, m)
	}
	return ms, rows.Err()
}

// ActiveTestPlanForLevel returns the active test plan mapped to a level code at a center.
func (r *Repository) ActiveTestPlanForLevel(ctx context.Context, testCenterID uuid.UUID, levelCode string) (*domain.TestPlan, error) {
	row := r.db.QueryRow(ctx,
		`SELECT tp.id, tp.test_center_id, tp.name, tp.description, tp.pass_threshold,
			tp.status, tp.published_at, tp.created_at, tp.updated_at, tp.created_by, tp.updated_by
		FROM test_plans tp
		JOIN test_level_mappings tlm ON tlm.test_plan_id = tp.id
		WHERE tlm.test_center_id=$1 AND tlm.test_level_code=$2 AND tp.status='active'
		LIMIT 1`,
		testCenterID, levelCode)
	p, err := scanPlan(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return p, err
}

// ── Test methods ──────────────────────────────────────────────────────────────

const testCols = `id, booking_id, candidate_id, test_center_id, test_plan_id, device_id,
	test_level_code, status, abort_reason,
	scheduled_start_at, scheduled_end_at, booking_window_hours,
	device_scanned_at, guidelines_start_at, acknowledged_at,
	iot_health_start_at, iot_health_passed_at, started_at, finishing_at, completed_at, aborted_at,
	weighted_total_score, passed, appeal_window_closes_at,
	result_visible_to_candidate_at, result_visible_to_institute_at,
	created_at, updated_at, created_by, updated_by`

func scanTest(row pgx.Row) (*domain.Test, error) {
	t := &domain.Test{}
	err := row.Scan(
		&t.ID, &t.BookingID, &t.CandidateID, &t.TestCenterID, &t.TestPlanID, &t.DeviceID,
		&t.TestLevelCode, &t.Status, &t.AbortReason,
		&t.ScheduledStartAt, &t.ScheduledEndAt, &t.BookingWindowHours,
		&t.DeviceScannedAt, &t.GuidelinesStartAt, &t.AcknowledgedAt,
		&t.IoTHealthStartAt, &t.IoTHealthPassedAt, &t.StartedAt, &t.FinishingAt, &t.CompletedAt, &t.AbortedAt,
		&t.WeightedTotalScore, &t.Passed, &t.AppealWindowClosesAt,
		&t.ResultVisibleToCandidateAt, &t.ResultVisibleToInstituteAt,
		&t.Audit.CreatedAt, &t.Audit.UpdatedAt, &t.Audit.CreatedBy, &t.Audit.UpdatedBy,
	)
	return t, err
}

func (r *Repository) CreateTest(ctx context.Context, t *domain.Test) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO tests (id, booking_id, candidate_id, test_center_id, test_plan_id,
			test_level_code, status, created_at, updated_at, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		t.ID, t.BookingID, t.CandidateID, t.TestCenterID, t.TestPlanID,
		t.TestLevelCode, t.Status, t.Audit.CreatedAt, t.Audit.UpdatedAt, t.Audit.CreatedBy, t.Audit.UpdatedBy,
	)
	return err
}

func (r *Repository) TestByID(ctx context.Context, id uuid.UUID) (*domain.Test, error) {
	row := r.db.QueryRow(ctx, `SELECT `+testCols+` FROM tests WHERE id=$1`, id)
	t, err := scanTest(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNoPendingTest
	}
	return t, err
}

func (r *Repository) PendingTestForCandidate(ctx context.Context, candidateID, testCenterID uuid.UUID) (*domain.Test, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+testCols+` FROM tests WHERE candidate_id=$1 AND test_center_id=$2 AND status='pending' LIMIT 1`,
		candidateID, testCenterID)
	t, err := scanTest(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNoPendingTest
	}
	return t, err
}

func (r *Repository) MyPendingTest(ctx context.Context, candidateID uuid.UUID) (*domain.Test, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+testCols+` FROM tests WHERE candidate_id=$1 AND status='pending' LIMIT 1`,
		candidateID)
	t, err := scanTest(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNoPendingTest
	}
	return t, err
}

// UpdateTestStatus sets a single status + timestamp column atomically.
func (r *Repository) UpdateTestStatus(ctx context.Context, id uuid.UUID, status domain.TestStatus, tsField string, actorID uuid.UUID) error {
	q := fmt.Sprintf(
		`UPDATE tests SET status=$2, %s=NOW(), updated_at=NOW(), updated_by=$3 WHERE id=$1`,
		tsField)
	_, err := r.db.Exec(ctx, q, id, status, actorID)
	return err
}

func (r *Repository) UpdateTestFields(ctx context.Context, id uuid.UUID, fields map[string]any, updatedBy uuid.UUID) error {
	return r.updateFields(ctx, "tests", id, fields, updatedBy)
}

// CheckinTest atomically records device_scanned_at and advances to 'ready'.
func (r *Repository) CheckinTest(ctx context.Context, id, deviceID uuid.UUID, actorID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE tests SET device_id=$2, device_scanned_at=NOW(), status='ready',
			updated_at=NOW(), updated_by=$3
		WHERE id=$1 AND status='pending'`,
		id, deviceID, actorID)
	return err
}

func (r *Repository) AdvanceToGuidelines(ctx context.Context, id, actorID uuid.UUID) error {
	return r.UpdateTestStatus(ctx, id, domain.TestStatusGuidelines, "guidelines_start_at", actorID)
}

func (r *Repository) AcknowledgeGuidelines(ctx context.Context, id, actorID uuid.UUID) error {
	return r.UpdateTestStatus(ctx, id, domain.TestStatusAcknowledged, "acknowledged_at", actorID)
}

func (r *Repository) StartTest(ctx context.Context, id, actorID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE tests SET status='running', started_at=NOW(), iot_health_passed_at=NOW(),
			updated_at=NOW(), updated_by=$2
		WHERE id=$1`,
		id, actorID)
	return err
}

func (r *Repository) SetFinishing(ctx context.Context, id, actorID uuid.UUID) error {
	return r.UpdateTestStatus(ctx, id, domain.TestStatusFinishing, "finishing_at", actorID)
}

func (r *Repository) CompleteTest(ctx context.Context, id uuid.UUID, score float64, passed bool, actorID uuid.UUID) error {
	appealClose := time.Now().Add(72 * time.Hour)
	_, err := r.db.Exec(ctx,
		`UPDATE tests SET status='completed', completed_at=NOW(), weighted_total_score=$2,
			passed=$3, appeal_window_closes_at=$4, result_visible_to_candidate_at=NOW(),
			updated_at=NOW(), updated_by=$5
		WHERE id=$1`,
		id, score, passed, appealClose, actorID)
	return err
}

func (r *Repository) AbortTest(ctx context.Context, id uuid.UUID, reason domain.AbortReason, actorID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE tests SET status='aborted', abort_reason=$2, aborted_at=NOW(),
			updated_at=NOW(), updated_by=$3
		WHERE id=$1`,
		id, reason, actorID)
	return err
}

func (r *Repository) ExpireStaleTests(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE tests SET status='expired', updated_at=NOW()
		WHERE status='pending' AND created_at < NOW() - INTERVAL '24 hours'`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ── IoT health check ──────────────────────────────────────────────────────────

func (r *Repository) InsertIoTHealthCheck(ctx context.Context, h *domain.IoTHealthCheck) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO iot_health_checks
			(id, test_id, passed, stream_reachable, network_latency_ms,
			camera_status, network_status, error_message, attempts, checked_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		h.ID, h.TestID, h.Passed, h.StreamReachable, h.NetworkLatencyMs,
		h.CameraStatus, h.NetworkStatus, h.ErrorMessage, h.Attempts, h.CheckedAt,
	)
	return err
}

// ── Recording ─────────────────────────────────────────────────────────────────

func (r *Repository) CreateTestRecording(ctx context.Context, testID uuid.UUID, prefix string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO test_recordings (id, test_id, minio_prefix, started_at, status, created_at)
		VALUES ($1,$2,$3,NOW(),'recording',NOW())`,
		uuid.New(), testID, prefix,
	)
	return err
}

func (r *Repository) FinalizeTestRecording(ctx context.Context, testID uuid.UUID, frameCount int, sizeBytes int64, status string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE test_recordings SET frame_count=$2, size_bytes=$3, ended_at=NOW(), status=$4
		WHERE test_id=$1`,
		testID, frameCount, sizeBytes, status,
	)
	return err
}

// ── Booking window check ──────────────────────────────────────────────────────

// BookingScheduledAt fetches the scheduled_at timestamp from the bookings table
// used to validate the ±N hour checkin window.
func (r *Repository) BookingScheduledAt(ctx context.Context, bookingID uuid.UUID) (time.Time, error) {
	var scheduledAt time.Time
	err := r.db.QueryRow(ctx,
		`SELECT scheduled_at FROM bookings WHERE id=$1`, bookingID).Scan(&scheduledAt)
	return scheduledAt, err
}

// ── Session result & test result ──────────────────────────────────────────────

func (r *Repository) InsertSessionResult(ctx context.Context, sr *domain.SessionResult) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO session_results
			(id, test_id, session_id, maneuver_id, sequence_number, score, weight,
			passed, frame_count, lane_detected_pct, avg_iou, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW())`,
		sr.ID, sr.TestID, sr.SessionID, sr.ManeuverID, sr.SequenceNumber,
		sr.Score, sr.Weight, sr.Passed, sr.FrameCount, sr.LaneDetectedPct, sr.AvgIoU,
	)
	return err
}

func (r *Repository) GetAllSessionResults(ctx context.Context, testID uuid.UUID) ([]*domain.SessionResult, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, test_id, session_id, maneuver_id, sequence_number, score, weight,
			passed, frame_count, lane_detected_pct, avg_iou,
			COALESCE(maneuver_type, ''), COALESCE(critical_fail, false),
			created_at
		FROM session_results WHERE test_id=$1 ORDER BY sequence_number`,
		testID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*domain.SessionResult
	for rows.Next() {
		sr := &domain.SessionResult{}
		var maneuverType string
		if err := rows.Scan(
			&sr.ID, &sr.TestID, &sr.SessionID, &sr.ManeuverID, &sr.SequenceNumber,
			&sr.Score, &sr.Weight, &sr.Passed, &sr.FrameCount, &sr.LaneDetectedPct, &sr.AvgIoU,
			&maneuverType, &sr.CriticalFail,
			&sr.CreatedAt,
		); err != nil {
			return nil, err
		}
		sr.ManeuverType = domain.ManeuverType(maneuverType)
		results = append(results, sr)
	}
	return results, rows.Err()
}

func (r *Repository) InsertTestResult(ctx context.Context, tr *domain.TestResult) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO test_results
			(id, test_id, weighted_total_score, passed, pass_threshold,
			 any_critical_fail, weakest_maneuver, score_breakdown,
			 overall_narrative, strengths_narrative, weaknesses_narrative,
			 recommended_focus, narrative_model, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NOW())`,
		tr.ID, tr.TestID, tr.WeightedTotalScore, tr.Passed, tr.PassThreshold,
		tr.AnyCriticalFail, tr.WeakestManeuver, tr.ScoreBreakdown,
		tr.OverallNarrative, tr.StrengthsNarrative, tr.WeaknessesNarrative,
		tr.RecommendedFocus, tr.NarrativeModel,
	)
	return err
}

func (r *Repository) TestResultByTestID(ctx context.Context, testID uuid.UUID) (*domain.TestResult, error) {
	tr := &domain.TestResult{}
	err := r.db.QueryRow(ctx,
		`SELECT id, test_id, weighted_total_score, passed, pass_threshold,
			 any_critical_fail, weakest_maneuver, score_breakdown,
			 overall_narrative, strengths_narrative, weaknesses_narrative,
			 recommended_focus, narrative_model, created_at
		FROM test_results WHERE test_id=$1`, testID).
		Scan(&tr.ID, &tr.TestID, &tr.WeightedTotalScore, &tr.Passed, &tr.PassThreshold,
			&tr.AnyCriticalFail, &tr.WeakestManeuver, &tr.ScoreBreakdown,
			&tr.OverallNarrative, &tr.StrengthsNarrative, &tr.WeaknessesNarrative,
			&tr.RecommendedFocus, &tr.NarrativeModel, &tr.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return tr, err
}

// UpdateTestResultNarrative updates the LLM-generated narrative fields on an
// existing test_results row. Called asynchronously after the row is committed.
func (r *Repository) UpdateTestResultNarrative(
	ctx context.Context,
	testID uuid.UUID,
	overall, strengths, weaknesses, focus, model string,
) error {
	_, err := r.db.Exec(ctx,
		`UPDATE test_results
		 SET overall_narrative=$2, strengths_narrative=$3,
		     weaknesses_narrative=$4, recommended_focus=$5, narrative_model=$6
		 WHERE test_id=$1`,
		testID, overall, strengths, weaknesses, focus, model)
	return err
}

// ── Test session & frame analyses ─────────────────────────────────────────────

func (r *Repository) InsertTestSession(ctx context.Context, s *domain.TestSession) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO test_sessions (id, test_id, maneuver_id, sequence_number,
			start_frame_seq, started_at, frame_count)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		s.ID, s.TestID, s.ManeuverID, s.SequenceNumber,
		s.StartFrameSeq, s.StartedAt, s.FrameCount,
	)
	return err
}

func (r *Repository) CloseTestSession(ctx context.Context, sessionID uuid.UUID, endSeq int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE test_sessions SET end_frame_seq=$2, ended_at=NOW() WHERE id=$1`,
		sessionID, endSeq)
	return err
}

func (r *Repository) UpdateTestSessionScored(ctx context.Context, sessionID uuid.UUID, score float64, passed bool, avgIoU float64, frameCount, laneCount int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE test_sessions SET score=$2, passed=$3, avg_iou_score=$4,
			frame_count=$5, lane_detected_count=$6, scored_at=NOW()
		WHERE id=$1`,
		sessionID, score, passed, avgIoU, frameCount, laneCount)
	return err
}

func (r *Repository) BatchInsertFrameAnalyses(ctx context.Context, frames []*domain.FrameAnalysis) error {
	if len(frames) == 0 {
		return nil
	}
	// Build multi-row INSERT
	cols := "(id,test_id,session_id,frame_seq_no,captured_at,lane_detected,curvature_dir," +
		"iou_score,frame_score,speed_kmh,heading_deg,is_mocked,created_at)"
	values := make([]string, 0, len(frames))
	args := make([]any, 0, len(frames)*13)
	for i, f := range frames {
		base := i * 13
		values = append(values, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7,
			base+8, base+9, base+10, base+11, base+12, base+13,
		))
		args = append(args,
			f.ID, f.TestID, f.SessionID, f.FrameSeqNo, f.CapturedAt, f.LaneDetected, f.CurvatureDir,
			f.IoUScore, f.FrameScore, f.SpeedKmh, f.HeadingDeg, f.IsMocked, f.CreatedAt,
		)
	}
	_, err := r.db.Exec(ctx,
		fmt.Sprintf("INSERT INTO frame_analyses %s VALUES %s", cols, strings.Join(values, ",")),
		args...)
	return err
}

// ── ManeuverTypes (Phase 3 — static enum served from Go, no DB query needed) ─────

// GetManeuverTypes returns the full static list of supported maneuver types.
// The list mirrors the maneuver_types seed rows in migration 003.
func (r *Repository) GetManeuverTypes(_ context.Context) ([]domain.ManeuverTypeInfo, error) {
	return domain.AllManeuverTypes, nil
}

// ── ManeuverConfig — full-schema helpers (migration 003 columns) ─────────────

// maneuverConfigCols lists all columns for the updated maneuver_configs table.
// Legacy columns (name, description, qr_code_value) are still included so the
// old scan path continues to work alongside the new one.
const maneuverConfigCols = `id, test_plan_id,
	COALESCE(maneuver_type::text,''), COALESCE(display_name,''),
	name, description, sequence_number,
	COALESCE(qr_code_value,''), COALESCE(reference_mask_url,''),
	COALESCE(qr_start_value,''), COALESCE(qr_end_value,''),
	tolerance_px, weight, COALESCE(pass_threshold,70.0), min_frames_required,
	created_at, updated_at, created_by, updated_by`

func scanManeuverConfig(row pgx.Row) (*domain.ManeuverConfig, error) {
	m := &domain.ManeuverConfig{}
	err := row.Scan(
		&m.ID, &m.TestPlanID,
		&m.ManeuverType, &m.DisplayName,
		&m.Name, &m.Description, &m.SequenceNumber,
		&m.QRCodeValue, &m.ReferenceMaskURL,
		&m.QRStartValue, &m.QREndValue,
		&m.TolerancePx, &m.Weight, &m.PassThreshold, &m.MinFramesRequired,
		&m.Audit.CreatedAt, &m.Audit.UpdatedAt, &m.Audit.CreatedBy, &m.Audit.UpdatedBy,
	)
	return m, err
}

// GetManeuverConfig returns a single ManeuverConfig by ID, reading all migration-003 columns.
func (r *Repository) GetManeuverConfig(ctx context.Context, maneuverID uuid.UUID) (*domain.ManeuverConfig, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+maneuverConfigCols+` FROM maneuver_configs WHERE id=$1`, maneuverID)
	m, err := scanManeuverConfig(row)
	if err == pgx.ErrNoRows {
		return nil, ErrManeuverNotFound
	}
	return m, err
}

// ListManeuverConfigs returns all maneuvers for a plan ordered by sequence_number,
// reading all migration-003 columns.
func (r *Repository) ListManeuverConfigs(ctx context.Context, planID uuid.UUID) ([]domain.ManeuverConfig, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+maneuverConfigCols+` FROM maneuver_configs WHERE test_plan_id=$1 ORDER BY sequence_number`,
		planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ms []domain.ManeuverConfig
	for rows.Next() {
		m, err := scanManeuverConfig(rows)
		if err != nil {
			return nil, err
		}
		ms = append(ms, *m)
	}
	return ms, rows.Err()
}

// CreateManeuverConfig inserts a new maneuver using the migration-003 schema.
// qr_start_value and qr_end_value are set automatically by the DB trigger when
// maneuver_type is provided.
func (r *Repository) CreateManeuverConfig(ctx context.Context, m domain.ManeuverConfig) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO maneuver_configs
			(id, test_plan_id, maneuver_type, display_name, name, description,
			 sequence_number, tolerance_px, weight, pass_threshold,
			 min_frames_required, created_at, updated_at, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		m.ID, m.TestPlanID, string(m.ManeuverType), m.DisplayName,
		m.Name, m.Description, m.SequenceNumber,
		m.TolerancePx, m.Weight, m.PassThreshold, m.MinFramesRequired,
		m.Audit.CreatedAt, m.Audit.UpdatedAt, m.Audit.CreatedBy, m.Audit.UpdatedBy,
	)
	return err
}

// UpdateManeuverConfig updates mutable fields on a maneuver config.
func (r *Repository) UpdateManeuverConfig(ctx context.Context, m domain.ManeuverConfig) error {
	return r.updateFields(ctx, "maneuver_configs", m.ID, map[string]any{
		"maneuver_type":       string(m.ManeuverType),
		"display_name":        m.DisplayName,
		"name":                m.Name,
		"description":         m.Description,
		"weight":              m.Weight,
		"pass_threshold":      m.PassThreshold,
		"tolerance_px":        m.TolerancePx,
		"min_frames_required": m.MinFramesRequired,
	}, m.Audit.UpdatedBy)
}

// DeleteManeuverConfig removes a maneuver config and renumbers siblings.
func (r *Repository) DeleteManeuverConfig(ctx context.Context, maneuverID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM maneuver_configs WHERE id=$1`, maneuverID)
	return err
}

// ReorderManeuverConfigs reassigns sequence_number in the given order within a plan.
// orderedIDs must contain all maneuver IDs for the plan — gaps will cause mismatches.
func (r *Repository) ReorderManeuverConfigs(ctx context.Context, planID uuid.UUID, orderedIDs []uuid.UUID) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for i, id := range orderedIDs {
		_, err = tx.Exec(ctx,
			`UPDATE maneuver_configs SET sequence_number=$1, updated_at=NOW()
			 WHERE id=$2 AND test_plan_id=$3`,
			i+1, id, planID)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ── ManeuverEvent methods (Phase 3) ──────────────────────────────────────────

// InsertManeuverEvent persists a single ManeuverEvent row.
func (r *Repository) InsertManeuverEvent(ctx context.Context, ev domain.ManeuverEvent) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO maneuver_events
			(id, test_id, session_id, maneuver_type, event_type, severity,
			 start_frame, end_frame, detail, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		ev.ID, ev.TestID, ev.SessionID, ev.ManeuverType,
		ev.EventType, ev.Severity, ev.StartFrame, ev.EndFrame,
		ev.Detail, ev.CreatedAt,
	)
	return err
}

// ListManeuverEvents returns all events for a session ordered by start_frame.
func (r *Repository) ListManeuverEvents(ctx context.Context, sessionID uuid.UUID) ([]domain.ManeuverEvent, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, test_id, session_id, maneuver_type, event_type, severity,
			start_frame, end_frame, detail, created_at
		FROM maneuver_events WHERE session_id=$1 ORDER BY start_frame`,
		sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var evs []domain.ManeuverEvent
	for rows.Next() {
		var ev domain.ManeuverEvent
		if err := rows.Scan(
			&ev.ID, &ev.TestID, &ev.SessionID, &ev.ManeuverType,
			&ev.EventType, &ev.Severity, &ev.StartFrame, &ev.EndFrame,
			&ev.Detail, &ev.CreatedAt,
		); err != nil {
			return nil, err
		}
		evs = append(evs, ev)
	}
	return evs, rows.Err()
}

// ── UpsertSessionResult (Phase 3 — handles migration-003 columns) ─────────────

// UpsertSessionResult inserts or updates a session_results row with the full
// migration-003 column set.  Use this instead of InsertSessionResult when the
// Python scorer has returned detailed scoring data.
func (r *Repository) UpsertSessionResult(ctx context.Context, sr domain.SessionResult) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO session_results
			(id, test_id, session_id, maneuver_id, maneuver_type,
			 sequence_number, score, weight, passed, critical_fail,
			 frame_count, lane_detected_pct, avg_iou,
			 mean_center_offset_px, offset_variance_px,
			 dimension_scores, event_count_by_severity, weakest_phase,
			 created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,NOW())
		ON CONFLICT (session_id) DO UPDATE SET
			score                  = EXCLUDED.score,
			passed                 = EXCLUDED.passed,
			critical_fail          = EXCLUDED.critical_fail,
			mean_center_offset_px  = EXCLUDED.mean_center_offset_px,
			offset_variance_px     = EXCLUDED.offset_variance_px,
			dimension_scores       = EXCLUDED.dimension_scores,
			event_count_by_severity= EXCLUDED.event_count_by_severity,
			weakest_phase          = EXCLUDED.weakest_phase`,
		sr.ID, sr.TestID, sr.SessionID, sr.ManeuverID, string(sr.ManeuverType),
		sr.SequenceNumber, sr.Score, sr.Weight, sr.Passed, sr.CriticalFail,
		sr.FrameCount, sr.LaneDetectedPct, sr.AvgIoU,
		sr.MeanCenterOffset, sr.OffsetVariance,
		sr.DimensionScores, sr.EventCountBySeverity, sr.WeakestPhase,
	)
	return err
}

// ── TestSession read methods (Phase 3) ────────────────────────────────────────

// testSessionFullCols selects all test_session columns including migration-003 additions.
// COALESCE guards are used on nullable new columns to avoid scan errors on older rows.
const testSessionFullCols = `id, test_id, maneuver_id,
	COALESCE(maneuver_type::text,''), sequence_number,
	start_frame_seq, end_frame_seq, started_at, ended_at,
	COALESCE(qr_start_data,''), qr_end_data,
	score, passed, COALESCE(critical_fail,false),
	scored_at, frame_count, lane_detected_count, avg_iou_score,
	mean_center_offset_px, offset_variance_px,
	dimension_scores, event_count_by_severity,
	COALESCE(weakest_phase,'')`

func scanTestSessionFull(row pgx.Row) (*domain.TestSession, error) {
	s := &domain.TestSession{}
	err := row.Scan(
		&s.ID, &s.TestID, &s.ManeuverID,
		&s.ManeuverType, &s.SequenceNumber,
		&s.StartFrameSeq, &s.EndFrameSeq, &s.StartedAt, &s.EndedAt,
		&s.QRStartData, &s.QREndData,
		&s.Score, &s.SessionPassed, &s.CriticalFail,
		&s.ScoredAt, &s.FrameCount, &s.LaneDetectedCount, &s.AvgIoUScore,
		&s.MeanCenterOffset, &s.OffsetVariance,
		&s.DimensionScores, &s.EventCountBySeverity,
		&s.WeakestPhase,
	)
	return s, err
}

// ListTestSessions returns all sessions for a test, ordered by sequence_number.
func (r *Repository) ListTestSessions(ctx context.Context, testID uuid.UUID) ([]domain.TestSession, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+testSessionFullCols+` FROM test_sessions WHERE test_id=$1 ORDER BY sequence_number`,
		testID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []domain.TestSession
	for rows.Next() {
		s, err := scanTestSessionFull(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *s)
	}
	return sessions, rows.Err()
}

// GetTestSession returns a single session by ID.
func (r *Repository) GetTestSession(ctx context.Context, sessionID uuid.UUID) (*domain.TestSession, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+testSessionFullCols+` FROM test_sessions WHERE id=$1`, sessionID)
	s, err := scanTestSessionFull(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return s, err
}

// ── TestResult with full breakdown (Phase 3) ──────────────────────────────────

// GetTestResultWithBreakdown fetches the test_results row with all migration-003
// columns, plus all associated session_results.
func (r *Repository) GetTestResultWithBreakdown(ctx context.Context, testID uuid.UUID) (*domain.TestResult, []domain.SessionResult, error) {
	// Load test result
	tr := &domain.TestResult{}
	err := r.db.QueryRow(ctx,
		`SELECT id, test_id, weighted_total_score, passed, pass_threshold,
			COALESCE(any_critical_fail,false),
			COALESCE(weakest_maneuver,''),
			score_breakdown,
			overall_narrative, strengths_narrative, weaknesses_narrative,
			recommended_focus, narrative_model, created_at
		FROM test_results WHERE test_id=$1`, testID).
		Scan(&tr.ID, &tr.TestID, &tr.WeightedTotalScore, &tr.Passed, &tr.PassThreshold,
			&tr.AnyCriticalFail, &tr.WeakestManeuver,
			&tr.ScoreBreakdown,
			&tr.OverallNarrative, &tr.StrengthsNarrative, &tr.WeaknessesNarrative,
			&tr.RecommendedFocus, &tr.NarrativeModel, &tr.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	// Load session results
	rows, err := r.db.Query(ctx,
		`SELECT id, test_id, session_id, maneuver_id,
			COALESCE(maneuver_type::text,''),
			sequence_number, score, weight, passed,
			COALESCE(critical_fail,false),
			frame_count, lane_detected_pct, avg_iou,
			COALESCE(mean_center_offset_px,0), COALESCE(offset_variance_px,0),
			dimension_scores, event_count_by_severity,
			COALESCE(weakest_phase,''),
			created_at
		FROM session_results WHERE test_id=$1 ORDER BY sequence_number`,
		testID)
	if err != nil {
		return tr, nil, err
	}
	defer rows.Close()

	var srs []domain.SessionResult
	for rows.Next() {
		var sr domain.SessionResult
		if err := rows.Scan(
			&sr.ID, &sr.TestID, &sr.SessionID, &sr.ManeuverID,
			&sr.ManeuverType,
			&sr.SequenceNumber, &sr.Score, &sr.Weight, &sr.Passed,
			&sr.CriticalFail,
			&sr.FrameCount, &sr.LaneDetectedPct, &sr.AvgIoU,
			&sr.MeanCenterOffset, &sr.OffsetVariance,
			&sr.DimensionScores, &sr.EventCountBySeverity,
			&sr.WeakestPhase,
			&sr.CreatedAt,
		); err != nil {
			return tr, nil, err
		}
		srs = append(srs, sr)
	}
	return tr, srs, rows.Err()
}

// ── Booking-service integration helpers (Phase 3) ─────────────────────────────

// GetTestByBookingID returns the test associated with a given booking ID.
// Used by the internal reschedule / cancel stubs (§12).
func (r *Repository) GetTestByBookingID(ctx context.Context, bookingID uuid.UUID) (*domain.Test, error) {
	row := r.db.QueryRow(ctx, `SELECT `+testCols+` FROM tests WHERE booking_id=$1`, bookingID)
	t, err := scanTest(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return t, err
}

// SetVideoKey stores the MinIO object key for the stitched full-test MP4.
func (r *Repository) SetVideoKey(ctx context.Context, testID uuid.UUID, key string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE test_recordings SET video_key=$2 WHERE test_id=$1`,
		testID, key)
	return err
}

// ── Tests — admin listing (Phase 4) ────────────────────────────────────────────

// ListTests returns paginated tests for a test center with optional filters.
// statusFilter = "" disables status filtering.
// candidateID = uuid.Nil disables candidate filtering.
func (r *Repository) ListTests(ctx context.Context, centerID uuid.UUID, statusFilter string, candidateID uuid.UUID, page int) ([]*domain.Test, error) {
	const pageSize = 20
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	args := []any{centerID}
	where := `WHERE test_center_id=$1`
	i := 2
	if statusFilter != "" {
		where += fmt.Sprintf(" AND status=$%d", i)
		args = append(args, statusFilter)
		i++
	}
	if candidateID != uuid.Nil {
		where += fmt.Sprintf(" AND candidate_id=$%d", i)
		args = append(args, candidateID)
		i++
	}
	args = append(args, pageSize, offset)
	q := fmt.Sprintf(
		`SELECT `+testCols+` FROM tests %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, i, i+1)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tests []*domain.Test
	for rows.Next() {
		t, err := scanTest(rows)
		if err != nil {
			return nil, err
		}
		tests = append(tests, t)
	}
	return tests, rows.Err()
}

// DeleteTestPlan hard-deletes a draft test plan. The handler must verify the
// plan is in draft status before calling this.
func (r *Repository) DeleteTestPlan(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM test_plans WHERE id=$1`, id)
	return err
}

// ── TestRecording lookups (Phase 4) ──────────────────────────────────────────

const recordingCols = `id, test_id, minio_prefix, frame_count, size_bytes,
	started_at, ended_at, status, created_at,
	COALESCE(maneuver_type,''), session_id, COALESCE(video_key,'')`

func scanRecording(row pgx.Row) (*domain.TestRecording, error) {
	rec := &domain.TestRecording{}
	err := row.Scan(
		&rec.ID, &rec.TestID, &rec.MinioPrefix, &rec.FrameCount, &rec.SizeBytes,
		&rec.StartedAt, &rec.EndedAt, &rec.Status, &rec.CreatedAt,
		&rec.ManeuverType, &rec.SessionID, &rec.VideoKey,
	)
	return rec, err
}

// TestRecordingByTestID returns the full-test recording row (session_id IS NULL).
func (r *Repository) TestRecordingByTestID(ctx context.Context, testID uuid.UUID) (*domain.TestRecording, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+recordingCols+` FROM test_recordings
		WHERE test_id=$1 AND session_id IS NULL LIMIT 1`, testID)
	rec, err := scanRecording(row)
	if err == pgx.ErrNoRows {
		return nil, ErrRecordingNotFound
	}
	return rec, err
}

// TestRecordingBySessionID returns the per-maneuver recording row for a session.
func (r *Repository) TestRecordingBySessionID(ctx context.Context, testID, sessionID uuid.UUID) (*domain.TestRecording, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+recordingCols+` FROM test_recordings
		WHERE test_id=$1 AND session_id=$2 LIMIT 1`, testID, sessionID)
	rec, err := scanRecording(row)
	if err == pgx.ErrNoRows {
		return nil, ErrRecordingNotFound
	}
	return rec, err
}

// ── Helpers to build TestResponse from domain ─────────────────────────────────

func toTestResponse(t *domain.Test) TestResponse {
	var abortStr *string
	if t.AbortReason != nil {
		s := string(*t.AbortReason)
		abortStr = &s
	}
	return TestResponse{
		ID:                 t.ID,
		BookingID:          t.BookingID,
		CandidateID:        t.CandidateID,
		TestCenterID:       t.TestCenterID,
		TestPlanID:         t.TestPlanID,
		DeviceID:           t.DeviceID,
		TestLevelCode:      t.TestLevelCode,
		Status:             string(t.Status),
		AbortReason:        abortStr,
		ScheduledStartAt:   t.ScheduledStartAt,
		ScheduledEndAt:     t.ScheduledEndAt,
		BookingWindowHours: t.BookingWindowHours,
		StartedAt:          t.StartedAt,
		CompletedAt:        t.CompletedAt,
		CreatedAt:          t.Audit.CreatedAt,
	}
}

func toDeviceResponse(d *domain.Device) (DeviceResponse, error) {
	var levels []string
	if err := json.Unmarshal([]byte(d.AllowedLevels), &levels); err != nil {
		levels = []string{}
	}
	return DeviceResponse{
		ID:            d.ID,
		DeviceCode:    d.DeviceCode,
		TestCenterID:  d.TestCenterID,
		AllowedLevels: levels,
		StreamURL:     d.StreamURL,
		Status:        string(d.Status),
		CurrentTestID: d.CurrentTestID,
		LastSeenAt:    d.LastSeenAt,
		CreatedAt:     d.Audit.CreatedAt,
		UpdatedAt:     d.Audit.UpdatedAt,
	}, nil
}

// systemActorID is used as the created_by / updated_by for system-initiated
// operations (internal test creation, IoT health check, scoring engine writes).
// It is a sentinel UUID not associated with any real user.
var systemActorID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// Ensure security package is imported (used by checkin handler for password verify).
var _ = security.CheckPassword

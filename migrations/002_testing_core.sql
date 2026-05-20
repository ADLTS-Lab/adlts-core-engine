-- ============================================================
-- ADLTS Testing Core Schema (Combined Migration)
-- Replaces: 002_testing_core.sql + 003_testing_core_update.sql
-- Applied after 001_schema.sql
-- ============================================================

-- ── Test level types (seeded reference table) ────────────────────────────────

CREATE TABLE IF NOT EXISTS test_level_types (
    code        TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    sort_order  INT  NOT NULL DEFAULT 0
);

INSERT INTO test_level_types (code, name, description, sort_order) VALUES
    ('class_a',  'Class A',  'Motorcycles',               1),
    ('class_a1', 'Class A1', 'Light motorcycles',          2),
    ('class_b',  'Class B',  'Passenger cars',             3),
    ('class_b1', 'Class B1', 'Light vehicles / tricycles', 4),
    ('class_be', 'Class BE', 'Car with trailer',           5),
    ('class_c',  'Class C',  'Truck / lorry',              6),
    ('class_c1', 'Class C1', 'Medium goods vehicle',       7),
    ('class_ce', 'Class CE', 'Truck with trailer',         8),
    ('class_d',  'Class D',  'Bus / coach',                9),
    ('class_d1', 'Class D1', 'Minibus',                   10)
ON CONFLICT (code) DO NOTHING;

-- ── Enums ────────────────────────────────────────────────────────────────────

DO $$ BEGIN
    CREATE TYPE device_status_testing AS ENUM (
        'active', 'inactive', 'in_use', 'maintenance'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE test_status AS ENUM (
        'pending', 'ready', 'guidelines', 'acknowledged',
        'iot_health', 'running', 'finishing', 'completed',
        'aborted', 'expired'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE abort_reason AS ENUM (
        'health_check_failed', 'stream_lost', 'admin_intervention',
        'candidate_no_show', 'device_failure', 'system_error'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE test_plan_status AS ENUM ('draft', 'active', 'retired');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE health_status AS ENUM ('ok', 'degraded', 'failed');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE verdict_type AS ENUM ('pass', 'fail', 'pending');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE curvature_dir AS ENUM ('straight', 'left', 'right', 'none');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ── Maneuver types reference table (seeded, admin READ-ONLY) ─────────────────

CREATE TABLE IF NOT EXISTS maneuver_types (
    code              VARCHAR(64)  PRIMARY KEY,
    name              VARCHAR(128) NOT NULL,
    description       TEXT,
    requires_reverse  BOOLEAN      DEFAULT FALSE,
    sort_order        INT          DEFAULT 0
);

INSERT INTO maneuver_types (code, name, description, requires_reverse, sort_order) VALUES
    ('straight_line',       'Straight Line',          'Drive straight maintaining lane center',   false, 1),
    ('figure_8',            'Figure 8',               'Complete two arcs forming figure-8',       false, 2),
    ('left_curve',          'Left Curve',             'Navigate a left-bending curve forward',    false, 3),
    ('right_curve',         'Right Curve',            'Navigate a right-bending curve forward',   false, 4),
    ('left_curve_reverse',  'Left Curve (Reverse)',   'Reverse through a left-bending curve',     true,  5),
    ('right_curve_reverse', 'Right Curve (Reverse)',  'Reverse through a right-bending curve',    true,  6),
    ('parking',             'Parking',                'Park forward within a marked bay',         false, 7),
    ('reverse_parking',     'Reverse Parking',        'Reverse park within a marked bay',         true,  8)
ON CONFLICT DO NOTHING;

-- ── Recreate devices with Testing Core schema ─────────────────────────────────
-- Drops the scaffolded table from 001_schema.sql which had wrong columns.

DROP TABLE IF EXISTS devices CASCADE;

CREATE TABLE IF NOT EXISTS devices (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_code     TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    test_center_id  UUID NOT NULL REFERENCES test_centers(id) ON DELETE RESTRICT,
    allowed_levels  TEXT NOT NULL DEFAULT '["class_b"]',
    stream_url      TEXT NOT NULL,
    status          device_status_testing NOT NULL DEFAULT 'active',
    current_test_id UUID UNIQUE,
    last_seen_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by      UUID NOT NULL,
    updated_by      UUID NOT NULL
);

-- ── Test plans ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS test_plans (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    test_center_id   UUID NOT NULL REFERENCES test_centers(id) ON DELETE RESTRICT,
    name             TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    pass_threshold   NUMERIC(5,2) NOT NULL DEFAULT 70.00,
    status           test_plan_status NOT NULL DEFAULT 'draft',
    published_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by       UUID NOT NULL,
    updated_by       UUID NOT NULL
);

-- ── Maneuver configurations ───────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS maneuver_configs (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    test_plan_id        UUID NOT NULL REFERENCES test_plans(id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    sequence_number     INT  NOT NULL,
    qr_code_value       TEXT NOT NULL,
    reference_mask_url  TEXT,
    tolerance_px        INT  NOT NULL DEFAULT 20,
    weight              NUMERIC(5,2) NOT NULL DEFAULT 1.00,
    min_frames_required INT  NOT NULL DEFAULT 30,
    maneuver_type       VARCHAR(64) REFERENCES maneuver_types(code),
    display_name        VARCHAR(128),
    pass_threshold      FLOAT DEFAULT 70.0,
    qr_start_value      VARCHAR(256),
    qr_end_value        VARCHAR(256),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by          UUID NOT NULL,
    updated_by          UUID NOT NULL,
    UNIQUE (test_plan_id, sequence_number),
    UNIQUE (test_plan_id, qr_code_value)
);

-- Auto-generate QR values on insert via trigger
CREATE OR REPLACE FUNCTION set_maneuver_qr_values()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.maneuver_type IS NOT NULL THEN
        NEW.qr_start_value := 'ADLTS:S:' || NEW.maneuver_type || ':' || NEW.id::text;
        NEW.qr_end_value   := 'ADLTS:E:' || NEW.maneuver_type || ':' || NEW.id::text;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_maneuver_qr ON maneuver_configs;
CREATE TRIGGER trg_maneuver_qr
BEFORE INSERT ON maneuver_configs
FOR EACH ROW
WHEN (NEW.maneuver_type IS NOT NULL)
EXECUTE FUNCTION set_maneuver_qr_values();

-- ── Test level → plan mappings ────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS test_level_mappings (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    test_center_id   UUID NOT NULL REFERENCES test_centers(id) ON DELETE CASCADE,
    test_level_code  TEXT NOT NULL REFERENCES test_level_types(code),
    test_plan_id     UUID NOT NULL REFERENCES test_plans(id) ON DELETE CASCADE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by       UUID NOT NULL,
    updated_by       UUID NOT NULL,
    UNIQUE (test_center_id, test_level_code)
);

-- ── Tests (one per booking attempt) ──────────────────────────────────────────

CREATE TABLE IF NOT EXISTS tests (
    id                           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    booking_id                   UUID NOT NULL REFERENCES bookings(id) ON DELETE RESTRICT,
    candidate_id                 UUID NOT NULL REFERENCES candidates(id) ON DELETE RESTRICT,
    test_center_id               UUID NOT NULL REFERENCES test_centers(id) ON DELETE RESTRICT,
    test_plan_id                 UUID NOT NULL REFERENCES test_plans(id) ON DELETE RESTRICT,
    device_id                    UUID REFERENCES devices(id),
    test_level_code              TEXT NOT NULL REFERENCES test_level_types(code),
    status                       test_status NOT NULL DEFAULT 'pending',
    abort_reason                 abort_reason,
    device_scanned_at            TIMESTAMPTZ,
    guidelines_start_at          TIMESTAMPTZ,
    acknowledged_at              TIMESTAMPTZ,
    iot_health_start_at          TIMESTAMPTZ,
    iot_health_passed_at         TIMESTAMPTZ,
    started_at                   TIMESTAMPTZ,
    finishing_at                 TIMESTAMPTZ,
    completed_at                 TIMESTAMPTZ,
    aborted_at                   TIMESTAMPTZ,
    weighted_total_score         NUMERIC(5,2),
    passed                       BOOLEAN,
    appeal_window_closes_at      TIMESTAMPTZ,
    result_visible_to_candidate_at TIMESTAMPTZ,
    result_visible_to_institute_at TIMESTAMPTZ,
    scheduled_start_at           TIMESTAMPTZ,
    scheduled_end_at             TIMESTAMPTZ,
    booking_window_hours         INT,
    created_at                   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by                   UUID NOT NULL,
    updated_by                   UUID NOT NULL
);

-- Only one pending test per candidate per center
CREATE UNIQUE INDEX IF NOT EXISTS idx_tests_one_pending
    ON tests (candidate_id, test_center_id)
    WHERE status = 'pending';

-- ── IoT health checks ─────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS iot_health_checks (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    test_id             UUID NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
    passed              BOOLEAN NOT NULL DEFAULT FALSE,
    stream_reachable    BOOLEAN NOT NULL DEFAULT FALSE,
    network_latency_ms  INT     NOT NULL DEFAULT 0,
    camera_status       health_status NOT NULL DEFAULT 'failed',
    network_status      health_status NOT NULL DEFAULT 'failed',
    error_message       TEXT,
    attempts            INT NOT NULL DEFAULT 0,
    checked_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Test sessions (one per maneuver / QR gate) ────────────────────────────────

CREATE TABLE IF NOT EXISTS test_sessions (
    id                          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    test_id                     UUID NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
    maneuver_id                 UUID NOT NULL REFERENCES maneuver_configs(id),
    sequence_number             INT  NOT NULL,
    start_frame_seq             BIGINT NOT NULL DEFAULT 0,
    end_frame_seq               BIGINT,
    started_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at                    TIMESTAMPTZ,
    score                       NUMERIC(5,2),
    passed                      BOOLEAN,
    scored_at                   TIMESTAMPTZ,
    frame_count                 INT NOT NULL DEFAULT 0,
    lane_detected_count         INT NOT NULL DEFAULT 0,
    avg_iou_score               NUMERIC(5,4),
    maneuver_type               VARCHAR(64),
    qr_start_data               VARCHAR(256),
    qr_end_data                 VARCHAR(256),
    critical_fail               BOOLEAN DEFAULT FALSE,
    mean_center_offset_px       FLOAT,
    offset_variance_px          FLOAT,
    dimension_scores            JSONB,
    event_count_by_severity     JSONB,
    weakest_phase               VARCHAR(32)
);

-- ── Frame analyses (batch-inserted every 2s) ──────────────────────────────────

CREATE TABLE IF NOT EXISTS frame_analyses (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    test_id                 UUID NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
    session_id              UUID NOT NULL REFERENCES test_sessions(id) ON DELETE CASCADE,
    frame_seq_no            BIGINT NOT NULL,
    captured_at             TIMESTAMPTZ NOT NULL,
    lane_detected           BOOLEAN NOT NULL DEFAULT FALSE,
    curvature_dir           curvature_dir NOT NULL DEFAULT 'none',
    iou_score               NUMERIC(5,4) NOT NULL DEFAULT 0,
    frame_score             NUMERIC(5,2) NOT NULL DEFAULT 0,
    speed_kmh               NUMERIC(6,2) NOT NULL DEFAULT 0,
    heading_deg             NUMERIC(6,2) NOT NULL DEFAULT 0,
    is_mocked               BOOLEAN NOT NULL DEFAULT TRUE,
    lane_detector_mode      VARCHAR(32),
    center_offset_px        FLOAT,
    curvature_r             FLOAT,
    lane_symmetry           FLOAT,
    motion_dir              VARCHAR(16),
    qr_event                VARCHAR(256),
    maneuver_phase          VARCHAR(32),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Maneuver Events ──────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS maneuver_events (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_id        UUID NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
    session_id     UUID NOT NULL REFERENCES test_sessions(id) ON DELETE CASCADE,
    maneuver_type  VARCHAR(64) NOT NULL,
    event_type     VARCHAR(64) NOT NULL,
    severity       VARCHAR(16) NOT NULL CHECK (severity IN ('minor', 'major', 'critical')),
    start_frame    BIGINT NOT NULL,
    end_frame      BIGINT,
    detail         JSONB,
    created_at     TIMESTAMPTZ DEFAULT now()
);

-- ── Session results (one per closed session) ──────────────────────────────────

CREATE TABLE IF NOT EXISTS session_results (
    id                          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    test_id                     UUID NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
    session_id                  UUID NOT NULL UNIQUE REFERENCES test_sessions(id) ON DELETE CASCADE,
    maneuver_id                 UUID NOT NULL REFERENCES maneuver_configs(id),
    sequence_number             INT  NOT NULL,
    score                       NUMERIC(5,2) NOT NULL DEFAULT 0,
    weight                      NUMERIC(5,2) NOT NULL DEFAULT 1,
    passed                      BOOLEAN NOT NULL DEFAULT FALSE,
    frame_count                 INT NOT NULL DEFAULT 0,
    lane_detected_pct           NUMERIC(5,2) NOT NULL DEFAULT 0,
    avg_iou                     NUMERIC(5,4) NOT NULL DEFAULT 0,
    maneuver_type               VARCHAR(64),
    critical_fail               BOOLEAN DEFAULT FALSE,
    mean_center_offset_px       FLOAT,
    offset_variance_px          FLOAT,
    direction_accuracy          FLOAT,
    dimension_scores            JSONB,
    event_count_by_severity     JSONB,
    weakest_phase               VARCHAR(32),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Test results (one per completed test) ─────────────────────────────────────

CREATE TABLE IF NOT EXISTS test_results (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    test_id                 UUID NOT NULL UNIQUE REFERENCES tests(id) ON DELETE CASCADE,
    weighted_total_score    NUMERIC(5,2) NOT NULL DEFAULT 0,
    passed                  BOOLEAN NOT NULL DEFAULT FALSE,
    pass_threshold          NUMERIC(5,2) NOT NULL DEFAULT 70,
    overall_narrative       TEXT NOT NULL DEFAULT '',
    strengths_narrative     TEXT NOT NULL DEFAULT '',
    weaknesses_narrative    TEXT NOT NULL DEFAULT '',
    recommended_focus       TEXT NOT NULL DEFAULT '',
    narrative_model         TEXT NOT NULL DEFAULT 'gemini-1.5-flash',
    any_critical_fail       BOOLEAN DEFAULT FALSE,
    weakest_maneuver        VARCHAR(64),
    score_breakdown         JSONB,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Recording metadata (owned by testing core) ────────────────────────────────

CREATE TABLE IF NOT EXISTS test_recordings (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    test_id      UUID NOT NULL UNIQUE REFERENCES tests(id) ON DELETE CASCADE,
    minio_prefix TEXT   NOT NULL,
    frame_count  INT    NOT NULL DEFAULT 0,
    size_bytes   BIGINT NOT NULL DEFAULT 0,
    started_at   TIMESTAMPTZ NOT NULL,
    ended_at     TIMESTAMPTZ,
    status       TEXT NOT NULL DEFAULT 'recording',
    maneuver_type VARCHAR(64),
    session_id   UUID REFERENCES test_sessions(id),
    video_key    TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Indexes ───────────────────────────────────────────────────────────────────

CREATE INDEX IF NOT EXISTS idx_devices_test_center
    ON devices (test_center_id);
CREATE INDEX IF NOT EXISTS idx_devices_status
    ON devices (status);
CREATE INDEX IF NOT EXISTS idx_test_plans_test_center
    ON test_plans (test_center_id);
CREATE INDEX IF NOT EXISTS idx_test_plans_status
    ON test_plans (status);
CREATE INDEX IF NOT EXISTS idx_maneuvers_plan_id
    ON maneuver_configs (test_plan_id);
CREATE INDEX IF NOT EXISTS idx_test_level_mappings_center
    ON test_level_mappings (test_center_id);
CREATE INDEX IF NOT EXISTS idx_tests_candidate_id
    ON tests (candidate_id);
CREATE INDEX IF NOT EXISTS idx_tests_test_center_id
    ON tests (test_center_id);
CREATE INDEX IF NOT EXISTS idx_tests_status
    ON tests (status);
CREATE INDEX IF NOT EXISTS idx_tests_device_id
    ON tests (device_id);
CREATE INDEX IF NOT EXISTS idx_test_sessions_test_id
    ON test_sessions (test_id);
CREATE INDEX IF NOT EXISTS idx_frame_analyses_test_session
    ON frame_analyses (test_id, session_id);
CREATE INDEX IF NOT EXISTS idx_frame_analyses_seq
    ON frame_analyses (test_id, frame_seq_no);
CREATE INDEX IF NOT EXISTS idx_session_results_test_id
    ON session_results (test_id);
CREATE INDEX IF NOT EXISTS idx_test_recordings_test_id
    ON test_recordings (test_id);
CREATE INDEX IF NOT EXISTS idx_iot_health_test_id
    ON iot_health_checks (test_id);
CREATE INDEX IF NOT EXISTS idx_maneuver_events_session
    ON maneuver_events (session_id);
CREATE INDEX IF NOT EXISTS idx_maneuver_events_severity
    ON maneuver_events (test_id, severity);

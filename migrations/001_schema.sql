CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ── Enums ───────────────────────────────────────────────────────────────────

CREATE TYPE user_status AS ENUM ('active','inactive','suspended','pending_verification');
CREATE TYPE org_status  AS ENUM ('active','inactive','suspended','pending_approval');
CREATE TYPE gender      AS ENUM ('male','female');

-- ── Identity tables ─────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS super_admins (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name          TEXT NOT NULL,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    UUID NOT NULL,
    updated_by    UUID NOT NULL
);

CREATE TABLE IF NOT EXISTS test_centers (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name          TEXT NOT NULL,
    name_am       TEXT,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    phone         TEXT NOT NULL,
    logo_url      TEXT,
    status        org_status NOT NULL DEFAULT 'active',
    street        TEXT,
    city          TEXT,
    region        TEXT,
    country       TEXT NOT NULL DEFAULT 'Ethiopia',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    UUID NOT NULL,
    updated_by    UUID NOT NULL
);

CREATE TABLE IF NOT EXISTS admins (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    first_name     TEXT NOT NULL,
    middle_name    TEXT NOT NULL DEFAULT '',
    last_name      TEXT NOT NULL,
    first_name_am  TEXT,
    middle_name_am TEXT,
    last_name_am   TEXT,
    email          TEXT NOT NULL UNIQUE,
    password_hash  TEXT NOT NULL,
    status         user_status NOT NULL DEFAULT 'pending_verification',
    test_center_id UUID NOT NULL REFERENCES test_centers(id) ON DELETE RESTRICT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by     UUID NOT NULL,
    updated_by     UUID NOT NULL
);

CREATE TABLE IF NOT EXISTS candidates (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    first_name    TEXT NOT NULL,
    middle_name   TEXT NOT NULL DEFAULT '',
    last_name     TEXT NOT NULL,
    first_name_am TEXT,
    middle_name_am TEXT,
    last_name_am  TEXT,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    status        user_status NOT NULL DEFAULT 'pending_verification',
    phone         TEXT NOT NULL,
    fayida_id     TEXT NOT NULL UNIQUE,
    birth_date    DATE NOT NULL,
    gender        gender NOT NULL,
    photo_url     TEXT,
    street        TEXT,
    city          TEXT,
    region        TEXT,
    country       TEXT NOT NULL DEFAULT 'Ethiopia',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    UUID,
    updated_by    UUID
);

CREATE TABLE IF NOT EXISTS experts (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    first_name     TEXT NOT NULL,
    middle_name    TEXT NOT NULL DEFAULT '',
    last_name      TEXT NOT NULL,
    first_name_am  TEXT,
    middle_name_am TEXT,
    last_name_am   TEXT,
    email          TEXT NOT NULL UNIQUE,
    password_hash  TEXT NOT NULL,
    status         user_status NOT NULL DEFAULT 'active',
    phone          TEXT,
    fayida_id      TEXT NOT NULL UNIQUE,
    employee_id    TEXT NOT NULL UNIQUE,
    birth_date     DATE,
    gender         gender,
    photo_url      TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by     UUID NOT NULL,
    updated_by     UUID NOT NULL
);

CREATE TABLE IF NOT EXISTS institutes (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name          TEXT NOT NULL,
    name_am       TEXT,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    phone         TEXT,
    logo_url      TEXT,
    status        org_status NOT NULL DEFAULT 'pending_approval',
    street        TEXT,
    city          TEXT,
    region        TEXT,
    country       TEXT NOT NULL DEFAULT 'Ethiopia',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    UUID NOT NULL,
    updated_by    UUID NOT NULL
);

CREATE TABLE IF NOT EXISTS transport_authorities (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name          TEXT NOT NULL,
    name_am       TEXT,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    phone         TEXT,
    logo_url      TEXT,
    status        org_status NOT NULL DEFAULT 'active',
    street        TEXT,
    city          TEXT,
    region        TEXT,
    country       TEXT NOT NULL DEFAULT 'Ethiopia',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    UUID NOT NULL,
    updated_by    UUID NOT NULL
);

-- ── Auth support tables ─────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS invitations (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    token          TEXT NOT NULL UNIQUE,
    email          TEXT NOT NULL,
    entity_type    TEXT NOT NULL, -- 'expert'|'institute'|'admin'|'transport_authority'|'super_admin'
    test_center_id UUID REFERENCES test_centers(id) ON DELETE SET NULL,
    expires_at     TIMESTAMPTZ NOT NULL,
    used_at        TIMESTAMPTZ,
    created_by     UUID NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS otp_codes (
    email      TEXT PRIMARY KEY,
    code       TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    attempts   INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    email      TEXT PRIMARY KEY,
    token      TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Activity tables (scaffolded — filled in by activity modules) ────────────

CREATE TABLE IF NOT EXISTS devices (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    mac_address    TEXT NOT NULL UNIQUE,
    name           TEXT NOT NULL,
    secret         TEXT NOT NULL,
    test_center_id UUID NOT NULL REFERENCES test_centers(id) ON DELETE RESTRICT,
    status         TEXT NOT NULL DEFAULT 'offline',
    last_heartbeat TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by     UUID NOT NULL,
    updated_by     UUID NOT NULL
);

CREATE TABLE IF NOT EXISTS institute_verifications (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    candidate_id UUID NOT NULL REFERENCES candidates(id),
    institute_id UUID NOT NULL REFERENCES institutes(id),
    issued_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by   UUID NOT NULL,
    updated_by   UUID NOT NULL
);

CREATE TABLE IF NOT EXISTS slots (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    test_center_id UUID NOT NULL REFERENCES test_centers(id),
    start_time     TIMESTAMPTZ NOT NULL,
    end_time       TIMESTAMPTZ NOT NULL,
    capacity       INT NOT NULL DEFAULT 1,
    booked_count   INT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by     UUID NOT NULL,
    updated_by     UUID NOT NULL
);

CREATE TABLE IF NOT EXISTS bookings (
    id                        UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    candidate_id              UUID NOT NULL REFERENCES candidates(id),
    test_center_id            UUID NOT NULL REFERENCES test_centers(id),
    institute_verification_id UUID NOT NULL REFERENCES institute_verifications(id),
    slot_id                   UUID REFERENCES slots(id),
    status                    TEXT NOT NULL DEFAULT 'pending_verification',
    training_hours            INT NOT NULL DEFAULT 0,
    training_evidence_url     TEXT,
    verified_by               UUID,
    verified_at               TIMESTAMPTZ,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by                UUID NOT NULL,
    updated_by                UUID NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    booking_id         UUID NOT NULL REFERENCES bookings(id),
    candidate_id       UUID NOT NULL REFERENCES candidates(id),
    test_center_id     UUID NOT NULL REFERENCES test_centers(id),
    device_id          UUID REFERENCES devices(id),
    status             TEXT NOT NULL DEFAULT 'scheduled',
    score              NUMERIC(5,2) NOT NULL DEFAULT 0,
    recording_url      TEXT,
    result_overlay_url TEXT,
    started_at         TIMESTAMPTZ,
    completed_at       TIMESTAMPTZ,
    finalized_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by         UUID NOT NULL,
    updated_by         UUID NOT NULL
);

CREATE TABLE IF NOT EXISTS session_telemetry (
    session_id      UUID PRIMARY KEY REFERENCES sessions(id),
    current_score   NUMERIC(5,2) NOT NULL DEFAULT 0,
    violation_count INT NOT NULL DEFAULT 0,
    last_frame_index INT NOT NULL DEFAULT 0,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS violations (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id   UUID NOT NULL REFERENCES sessions(id),
    code         TEXT NOT NULL,
    message      TEXT NOT NULL,
    severity     TEXT NOT NULL DEFAULT 'minor',
    frame_index  INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS detection_events (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id   UUID NOT NULL REFERENCES sessions(id),
    device_id    UUID NOT NULL REFERENCES devices(id),
    frame_index  INT NOT NULL,
    objects      JSONB,
    violations   JSONB,
    score_delta  NUMERIC(5,2) NOT NULL DEFAULT 0,
    speed        NUMERIC(6,2) NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS appeals (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id   UUID NOT NULL REFERENCES sessions(id),
    candidate_id UUID NOT NULL REFERENCES candidates(id),
    expert_id    UUID REFERENCES experts(id),
    reason       TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending',
    resolution   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by   UUID NOT NULL,
    updated_by   UUID NOT NULL
);

CREATE TABLE IF NOT EXISTS recordings (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES sessions(id),
    url        TEXT NOT NULL,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ
);

-- ── Indexes ──────────────────────────────────────────────────────────────────

CREATE INDEX IF NOT EXISTS idx_candidates_email        ON candidates(email);
CREATE INDEX IF NOT EXISTS idx_candidates_fayida_id    ON candidates(fayida_id);
CREATE INDEX IF NOT EXISTS idx_candidates_status       ON candidates(status);
CREATE INDEX IF NOT EXISTS idx_experts_email           ON experts(email);
CREATE INDEX IF NOT EXISTS idx_experts_employee_id     ON experts(employee_id);
CREATE INDEX IF NOT EXISTS idx_admins_email            ON admins(email);
CREATE INDEX IF NOT EXISTS idx_admins_test_center_id   ON admins(test_center_id);
CREATE INDEX IF NOT EXISTS idx_institutes_email        ON institutes(email);
CREATE INDEX IF NOT EXISTS idx_institutes_status       ON institutes(status);
CREATE INDEX IF NOT EXISTS idx_transport_auth_email    ON transport_authorities(email);
CREATE INDEX IF NOT EXISTS idx_invitations_token       ON invitations(token);
CREATE INDEX IF NOT EXISTS idx_invitations_email       ON invitations(email);
CREATE INDEX IF NOT EXISTS idx_sessions_candidate_id   ON sessions(candidate_id);
CREATE INDEX IF NOT EXISTS idx_sessions_test_center_id ON sessions(test_center_id);
CREATE INDEX IF NOT EXISTS idx_appeals_expert_id       ON appeals(expert_id);
CREATE INDEX IF NOT EXISTS idx_bookings_candidate_id   ON bookings(candidate_id);

-- ============================================================
-- 002_booking_scheduling_payment.sql
-- Adds columns and tables needed for booking, scheduling, payment.
-- Run after 001_schema.sql.
-- ============================================================

-- -------------------------------------------------------
-- Relax legacy constraints to support new flow
-- -------------------------------------------------------
ALTER TABLE bookings
  ALTER COLUMN test_center_id DROP NOT NULL,
  ALTER COLUMN institute_verification_id DROP NOT NULL;

ALTER TABLE slots
  ALTER COLUMN test_center_id DROP NOT NULL;

-- -------------------------------------------------------
-- Extend the bookings table
-- -------------------------------------------------------
ALTER TABLE bookings
  ADD COLUMN IF NOT EXISTS institute_id         UUID REFERENCES institutes(id),
  ADD COLUMN IF NOT EXISTS test_id              UUID,
  ADD COLUMN IF NOT EXISTS status               TEXT        NOT NULL DEFAULT 'drafted',
  ADD COLUMN IF NOT EXISTS requires_verification BOOLEAN    NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS rejection_reason     TEXT,
  ADD COLUMN IF NOT EXISTS scheduled_at         TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS payment_ref          TEXT,
  ADD COLUMN IF NOT EXISTS payment_status       TEXT        NOT NULL DEFAULT 'unpaid',
  ADD COLUMN IF NOT EXISTS payment_amount_cents INTEGER,
  ADD COLUMN IF NOT EXISTS payment_attempts     INTEGER     NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS archived_at          TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ADD COLUMN IF NOT EXISTS updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW();

ALTER TABLE bookings
  ALTER COLUMN status SET DEFAULT 'drafted';

-- -------------------------------------------------------
-- Extend the slots table
-- -------------------------------------------------------
ALTER TABLE slots
  ADD COLUMN IF NOT EXISTS institute_id UUID        REFERENCES institutes(id),
  ADD COLUMN IF NOT EXISTS test_id      UUID,
  ADD COLUMN IF NOT EXISTS starts_at    TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS ends_at      TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS capacity     INTEGER     NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS booked_count INTEGER     NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ADD COLUMN IF NOT EXISTS updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- -------------------------------------------------------
-- Payments table (audit log of every payment attempt)
-- -------------------------------------------------------
CREATE TABLE IF NOT EXISTS payments (
  id             UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
  booking_id     UUID        NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
  amount_cents   INTEGER     NOT NULL,
  currency       TEXT        NOT NULL DEFAULT 'ETB',
  status         TEXT        NOT NULL DEFAULT 'pending',
  provider       TEXT        NOT NULL DEFAULT 'chapa',
  provider_ref   TEXT,
  attempt_number INTEGER     NOT NULL DEFAULT 1,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- -------------------------------------------------------
-- Indexes
-- -------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_bookings_status       ON bookings(status);
CREATE INDEX IF NOT EXISTS idx_bookings_candidate    ON bookings(candidate_id);
CREATE INDEX IF NOT EXISTS idx_bookings_institute    ON bookings(institute_id);
CREATE INDEX IF NOT EXISTS idx_bookings_slot         ON bookings(slot_id);
CREATE INDEX IF NOT EXISTS idx_slots_institute       ON slots(institute_id);
CREATE INDEX IF NOT EXISTS idx_payments_booking      ON payments(booking_id);
CREATE INDEX IF NOT EXISTS idx_payments_provider_ref ON payments(provider_ref);

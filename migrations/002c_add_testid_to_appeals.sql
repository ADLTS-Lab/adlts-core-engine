-- Add test_id to appeals and ensure it references tests(id)
ALTER TABLE appeals
    ADD COLUMN IF NOT EXISTS test_id UUID REFERENCES tests(id);

-- Optional: set test_id from existing session->tests relationship if possible
-- (left to DB migration operator if data backfill required)

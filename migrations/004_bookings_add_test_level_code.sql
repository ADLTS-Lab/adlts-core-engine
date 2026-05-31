-- Add test_level_code to bookings
ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS test_level_code TEXT REFERENCES test_level_types(code);

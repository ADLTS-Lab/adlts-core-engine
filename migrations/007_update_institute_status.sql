for ALTER TABLE institutes ALTER COLUMN status SET DEFAULT 'active';
UPDATE institutes SET status = 'active', updated_at = NOW() WHERE status = 'pending_approval';

-- Existing bookings created as "verified" bypassed verification; reset so verify flow works
UPDATE bookings SET status = 'pending_verification', updated_at = NOW()
WHERE status = 'verified' AND verified_by IS NULL;

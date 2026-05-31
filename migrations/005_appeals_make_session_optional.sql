-- Make session_id nullable on appeals (legacy schema has NOT NULL)
ALTER TABLE appeals ALTER COLUMN session_id DROP NOT NULL;

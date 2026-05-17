-- Create appeal_evidence table to store snapshots supporting appeals
CREATE TABLE IF NOT EXISTS appeal_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    appeal_id UUID REFERENCES appeals(id) ON DELETE CASCADE,
    test_results_snapshot JSONB,
    frame_analyses_snapshot JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

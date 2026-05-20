## Recording / Replay Module

> Your module is responsible only for **serving playback** of the saved recording.
> You do NOT need to connect to the device stream or capture frames. (not live)

### What Testing Core does for you

During every test in `status = 'running'`, the testing core's `RecordingEngine` automatically:
1. Captures raw JPEG frames from the ESP32-CAM stream
2. Writes them to MinIO at `recordings/{test_id}/frame_{seq:08d}.jpg`
3. On test end/abort: marks the recording as `saved` and writes a `test_recordings` table row

### What your module reads

Query the `test_recordings` table after a test is completed:
```sql
SELECT id, test_id, minio_prefix, frame_count, size_bytes, started_at, ended_at, status
FROM test_recordings
WHERE test_id = $1 AND status = 'saved';
```

- `minio_prefix` = `"recordings/{test_id}/"` — list all objects under this prefix to get frames in order
- Frame naming is zero-padded: `frame_00000001.jpg`, `frame_00000002.jpg`, ...
- Sort lexicographically by object key to get correct playback order

### What your team provides

Your module exposes a **playback endpoint** (your own routes, not part of testing core):
```
GET /api/v1/recordings/:test_id/play  → serve MJPEG stream or frame list for the frontend player
```

You read frames from MinIO using the `minio_prefix` and stream them to the frontend player. The testing core does not define or own this endpoint.

---
    
## Appeals Module

### What Testing Core exposes to you

#### Result visibility
Appeals can only be filed within the appeal window. Testing Core sets:
```sql
tests.appeal_window_closes_at = NOW() + INTERVAL '72 hours'  -- set when test completes
```

Your appeal creation endpoint must check:
```sql
SELECT appeal_window_closes_at FROM tests WHERE id = $1
-- reject if NOW() > appeal_window_closes_at
```

#### Result data you can read

```sql
-- Full test result
SELECT * FROM test_results WHERE test_id = $1;

-- Per-session breakdown
SELECT * FROM session_results WHERE test_id = $1 ORDER BY sequence_number;

-- Frame-level evidence
SELECT * FROM frame_analyses WHERE test_id = $1 AND session_id = $2
ORDER BY frame_seq_no;
```

#### IoT health check evidence (dispute support)
```sql
SELECT * FROM iot_health_checks WHERE test_id = $1;
```

#### `appeals` table: update existing schema

The current `appeals` table references the old `sessions` table. **Update it:**
```sql
ALTER TABLE appeals ADD COLUMN IF NOT EXISTS test_id UUID REFERENCES tests(id);
-- Keep session_id for backward compat, but test_id is the primary link
```

#### Appeal outcome → Testing Core

When an appeal is **accepted**, your module must call:
```sql
UPDATE test_results
SET passed = true   -- or whatever the corrected verdict is
WHERE test_id = $1;

UPDATE tests
SET updated_at = NOW()
WHERE id = $1;
```

There is no Testing Core API for this, you write directly to the shared DB (same pattern as other modules).

---

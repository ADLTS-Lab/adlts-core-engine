# MinIO Migration / Data Population Steps

This document describes the manual steps to populate MinIO with recorded frames and verify the `test_recordings` metadata so the playback module can serve recordings.

Prerequisites
- MinIO server reachable at `MINIO_ENDPOINT` (e.g. http://minio.local:9000)
- Access credentials: `MINIO_ACCESS_KEY` and `MINIO_SECRET_KEY`
- Bucket created and configured: `MINIO_BUCKET`

Steps
1. Create the bucket (if missing)
   - Using `mc` (MinIO Client):
     ```bash
     mc alias set myminio $MINIO_ENDPOINT $MINIO_ACCESS_KEY $MINIO_SECRET_KEY
     mc mb myminio/$MINIO_BUCKET
     ```

2. Upload saved frames for a given `test_id` into the bucket under the prefix `recordings/{test_id}/`
   - Ensure files are named zero-padded as `frame_00000001.jpg`, `frame_00000002.jpg`, ...
   - Example (Linux/macOS):
     ```bash
     TEST_ID=your-test-uuid
     mc cp ./frames/*.jpg myminio/$MINIO_BUCKET/recordings/$TEST_ID/ --recursive
     ```

3. Verify object listing and order
   - Objects should list lexicographically in correct chronological order because of zero-padding:
     ```bash
     mc ls --recursive myminio/$MINIO_BUCKET/recordings/$TEST_ID | awk '{print $5}'
     ```

4. Insert (or update) `test_recordings` row in the database
   - Required columns: `test_id`, `minio_prefix`, `frame_count`, `size_bytes`, `started_at`, `ended_at`, `status`
   - Example SQL (adjust sizes/timestamps accordingly):
     ```sql
     INSERT INTO test_recordings (id, test_id, minio_prefix, frame_count, size_bytes, started_at, ended_at, status, created_at, updated_at)
     VALUES (gen_random_uuid(), '<TEST_ID>'::uuid, 'recordings/<TEST_ID>/', <FRAME_COUNT>, <TOTAL_BYTES>, NOW() - INTERVAL '5 minutes', NOW(), 'saved', NOW(), NOW());
     ```

5. Verify via SQL
   ```sql
   SELECT id, test_id, minio_prefix, frame_count, size_bytes, started_at, ended_at, status
   FROM test_recordings
   WHERE test_id = '<TEST_ID>'::uuid AND status = 'saved';
   ```

6. Environment variables for the playback service
   - Set these in your environment or container deployment:
     - `MINIO_ENDPOINT` (e.g. http://minio.local:9000)
     - `MINIO_ACCESS_KEY`
     - `MINIO_SECRET_KEY`
     - `MINIO_BUCKET`

7. Quick smoke-test (playback)
   - Start the API server and request the frame list:
     ```bash
     curl -H "Authorization: Bearer <token>" http://localhost:8080/api/v1/recordings/<TEST_ID>/frames
     ```
   - Stream MJPEG (browser/player):
     Open `http://localhost:8080/api/v1/recordings/<TEST_ID>/play` in a browser that can render MJPEG (or use a simple player that supports MJPEG).

Notes & Troubleshooting
- Lexicographic sorting is essential; zero-padded filenames guarantee ordering.
- The playback module is read-only: it will never modify `test_recordings` or recordings.
- If presigned URLs are required to be accessible externally, ensure MinIO is reachable and credentials are correct.
- For large recordings, consider streaming directly from object storage instead of loading full frames into memory.

If you want, I can also prepare a small shell script to bulk upload frames and insert the `test_recordings` row.

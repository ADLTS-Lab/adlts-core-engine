-- ============================================================
-- ADLTS Seed Defaults — Test Center, Device, Plans, Mappings
-- Applied after 005_appeals_make_session_optional.sql
-- ============================================================

-- Well-known UUIDs for the default seed entities.
-- These are also defined as constants in internal/domain/testing.go
-- so the Go code can reference them during auto-creation from bookings.

-- ── 1. Default Test Center ─────────────────────────────────────────────
INSERT INTO test_centers (id, name, email, password_hash, phone, street, city, region, status,
                          created_at, updated_at, created_by, updated_by)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'Addis Ababa Test Center',
    'testcenter@adlts.et',
    '$2a$10$4TlIjUBS4LD00/XZ8nNLfO/BZ4QPb5iU8N2QhlP6dAH3gAYMasJtu',  -- BCrypt of "device123"
    '+251-911-000001',
    'Bole 01', 'Addis Ababa', 'Addis Ababa',
    'active',
    NOW(), NOW(),
    '00000000-0000-0000-0000-000000000000',  -- system actor
    '00000000-0000-0000-0000-000000000000'
)
ON CONFLICT (id) DO NOTHING;

-- ── 2. Default Device ─────────────────────────────────────────────────
INSERT INTO devices (id, device_code, password_hash, test_center_id,
                     allowed_levels, stream_url, status,
                     created_at, updated_at, created_by, updated_by)
VALUES (
    '00000000-0000-0000-0000-000000000002',
    'DEV-001',
    '$2a$10$placeholder_hash_for_seed_only',
    '00000000-0000-0000-0000-000000000001',
    '["class_a","class_b"]',
    'http://192.168.68.182',
    'active',
    NOW(), NOW(),
    '00000000-0000-0000-0000-000000000000',
    '00000000-0000-0000-0000-000000000000'
)
ON CONFLICT (id) DO NOTHING;

-- ── 3. Default Test Plan ──────────────────────────────────────────────
-- Named "Standard Driving Test" with pass_threshold = 60.0
INSERT INTO test_plans (id, test_center_id, name, description, pass_threshold,
                        status, published_at,
                        created_at, updated_at, created_by, updated_by)
VALUES (
    '00000000-0000-0000-0000-000000000003',
    '00000000-0000-0000-0000-000000000001',
    'Standard Driving Test',
    'Default test plan for passenger car license (Class B). Includes straight lines, curves, and parallel parking.',
    60.00,
    'active', NOW(),
    NOW(), NOW(),
    '00000000-0000-0000-0000-000000000000',
    '00000000-0000-0000-0000-000000000000'
)
ON CONFLICT (id) DO NOTHING;

-- ── 4. Maneuver Configurations ───────────────────────────────────────
-- These match the MANEUVER_SEQUENCE and MANEUVER_WEIGHTS from adlts/backend/config.py
-- and are mapped to the seeded maneuver_types from migration 002.

-- 4a. Straight 1
INSERT INTO maneuver_configs (id, test_plan_id, name, description, sequence_number,
                              qr_code_value, tolerance_px, weight, min_frames_required,
                              maneuver_type, display_name, pass_threshold,
                              created_at, updated_at, created_by, updated_by)
SELECT
    '00000000-0000-0000-0000-000000000010',
    '00000000-0000-0000-0000-000000000003',
    'Straight Line 1', 'Drive straight maintaining lane center', 1,
    'maneuver:straight_1', 20, 1.00, 30,
    'straight_line', 'Straight 1', 70.0,
    NOW(), NOW(),
    '00000000-0000-0000-0000-000000000000',
    '00000000-0000-0000-0000-000000000000'
WHERE NOT EXISTS (SELECT 1 FROM maneuver_configs WHERE id = '00000000-0000-0000-0000-000000000010');

-- 4b. Left Curve 1
INSERT INTO maneuver_configs (id, test_plan_id, name, description, sequence_number,
                              qr_code_value, tolerance_px, weight, min_frames_required,
                              maneuver_type, display_name, pass_threshold,
                              created_at, updated_at, created_by, updated_by)
SELECT
    '00000000-0000-0000-0000-000000000011',
    '00000000-0000-0000-0000-000000000003',
    'Left Curve 1', 'Navigate a left-bending curve', 2,
    'maneuver:left_curve_1', 25, 1.50, 30,
    'left_curve', 'Left Curve 1', 70.0,
    NOW(), NOW(),
    '00000000-0000-0000-0000-000000000000',
    '00000000-0000-0000-0000-000000000000'
WHERE NOT EXISTS (SELECT 1 FROM maneuver_configs WHERE id = '00000000-0000-0000-0000-000000000011');

-- 4c. Left Curve 2 (opposite direction)
INSERT INTO maneuver_configs (id, test_plan_id, name, description, sequence_number,
                              qr_code_value, tolerance_px, weight, min_frames_required,
                              maneuver_type, display_name, pass_threshold,
                              created_at, updated_at, created_by, updated_by)
SELECT
    '00000000-0000-0000-0000-000000000012',
    '00000000-0000-0000-0000-000000000003',
    'Left Curve 2', 'Navigate a second left-bending curve', 3,
    'maneuver:left_curve_2', 25, 1.50, 30,
    'left_curve', 'Left Curve 2', 70.0,
    NOW(), NOW(),
    '00000000-0000-0000-0000-000000000000',
    '00000000-0000-0000-0000-000000000000'
WHERE NOT EXISTS (SELECT 1 FROM maneuver_configs WHERE id = '00000000-0000-0000-0000-000000000012');

-- 4d. Straight 2
INSERT INTO maneuver_configs (id, test_plan_id, name, description, sequence_number,
                              qr_code_value, tolerance_px, weight, min_frames_required,
                              maneuver_type, display_name, pass_threshold,
                              created_at, updated_at, created_by, updated_by)
SELECT
    '00000000-0000-0000-0000-000000000013',
    '00000000-0000-0000-0000-000000000003',
    'Straight Line 2', 'Drive straight to complete the course', 4,
    'maneuver:straight_2', 20, 1.00, 30,
    'straight_line', 'Straight 2', 70.0,
    NOW(), NOW(),
    '00000000-0000-0000-0000-000000000000',
    '00000000-0000-0000-0000-000000000000'
WHERE NOT EXISTS (SELECT 1 FROM maneuver_configs WHERE id = '00000000-0000-0000-0000-000000000013');

-- 4e. Parallel Parking
INSERT INTO maneuver_configs (id, test_plan_id, name, description, sequence_number,
                              qr_code_value, tolerance_px, weight, min_frames_required,
                              maneuver_type, display_name, pass_threshold,
                              created_at, updated_at, created_by, updated_by)
SELECT
    '00000000-0000-0000-0000-000000000014',
    '00000000-0000-0000-0000-000000000003',
    'Parallel Parking', 'Park within a marked bay', 5,
    'maneuver:parallel_parking', 30, 2.00, 30,
    'parking', 'Parallel Parking', 70.0,
    NOW(), NOW(),
    '00000000-0000-0000-0000-000000000000',
    '00000000-0000-0000-0000-000000000000'
WHERE NOT EXISTS (SELECT 1 FROM maneuver_configs WHERE id = '00000000-0000-0000-0000-000000000014');

-- ── 5. Test Level Mapping ─────────────────────────────────────────────
-- Maps Class B to the Standard Driving Test plan at the default test center.
INSERT INTO test_level_mappings (id, test_center_id, test_level_code, test_plan_id,
                                 created_at, updated_at, created_by, updated_by)
VALUES (
    '00000000-0000-0000-0000-000000000020',
    '00000000-0000-0000-0000-000000000001',
    'class_b',
    '00000000-0000-0000-0000-000000000003',
    NOW(), NOW(),
    '00000000-0000-0000-0000-000000000000',
    '00000000-0000-0000-0000-000000000000'
)
ON CONFLICT (test_center_id, test_level_code) DO NOTHING;
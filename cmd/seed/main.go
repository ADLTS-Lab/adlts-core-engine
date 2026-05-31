package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"adlts/internal/platform/db"
	"adlts/internal/platform/security"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	seedSuperAdminID = mustUUID("10000000-0000-4000-8000-000000000001")
	seedCenterID     = mustUUID("10000000-0000-4000-8000-000000000002")
	seedAdminID      = mustUUID("10000000-0000-4000-8000-000000000003")

	seedInstitute1ID = mustUUID("10000000-0000-4000-8000-000000000011")
	seedInstitute2ID = mustUUID("10000000-0000-4000-8000-000000000012")
	seedInstitute3ID = mustUUID("10000000-0000-4000-8000-000000000013")

	seedCandidate1ID = mustUUID("10000000-0000-4000-8000-000000000021")
	seedCandidate2ID = mustUUID("10000000-0000-4000-8000-000000000022")
	seedCandidate3ID = mustUUID("10000000-0000-4000-8000-000000000023")
	seedCandidate4ID = mustUUID("10000000-0000-4000-8000-000000000024")
	seedCandidate5ID = mustUUID("10000000-0000-4000-8000-000000000025")
	seedCandidate6ID = mustUUID("10000000-0000-4000-8000-000000000026")

	seedExpertID    = mustUUID("10000000-0000-4000-8000-000000000031")
	seedAuthorityID = mustUUID("10000000-0000-4000-8000-000000000041")

	seedInvitationID    = mustUUID("10000000-0000-4000-8000-000000000051")
	seedVerification1ID = mustUUID("10000000-0000-4000-8000-000000000061")
	seedVerification2ID = mustUUID("10000000-0000-4000-8000-000000000062")

	seedSlot1ID = mustUUID("10000000-0000-4000-8000-000000000071")
	seedSlot2ID = mustUUID("10000000-0000-4000-8000-000000000072")
	seedSlot3ID = mustUUID("10000000-0000-4000-8000-000000000073")

	seedBookingPendingID    = mustUUID("10000000-0000-4000-8000-000000000081")
	seedBookingVerifiedID   = mustUUID("10000000-0000-4000-8000-000000000082")
	seedBookingScheduledID  = mustUUID("10000000-0000-4000-8000-000000000083")
	seedBookingConfirmed1ID = mustUUID("10000000-0000-4000-8000-000000000084")
	seedBookingRejectedID   = mustUUID("10000000-0000-4000-8000-000000000085")
	seedBookingConfirmed2ID = mustUUID("10000000-0000-4000-8000-000000000086")

	seedPaymentID = mustUUID("10000000-0000-4000-8000-000000000091")

	seedDevice1ID = mustUUID("10000000-0000-4000-8000-000000000101")
	seedDevice2ID = mustUUID("10000000-0000-4000-8000-000000000102")

	seedPlanID      = mustUUID("10000000-0000-4000-8000-000000000111")
	seedManeuver1ID = mustUUID("10000000-0000-4000-8000-000000000121")
	seedManeuver2ID = mustUUID("10000000-0000-4000-8000-000000000122")
	seedManeuver3ID = mustUUID("10000000-0000-4000-8000-000000000123")
	seedMappingID   = mustUUID("10000000-0000-4000-8000-000000000131")

	seedRunningTestID    = mustUUID("10000000-0000-4000-8000-000000000141")
	seedCompletedTest1ID = mustUUID("10000000-0000-4000-8000-000000000142")
	seedCompletedTest2ID = mustUUID("10000000-0000-4000-8000-000000000143")

	seedSessionRunningID = mustUUID("10000000-0000-4000-8000-000000000151")
	seedSessionResult1ID = mustUUID("10000000-0000-4000-8000-000000000152")
	seedSessionResult2ID = mustUUID("10000000-0000-4000-8000-000000000153")
	seedTestResult1ID    = mustUUID("10000000-0000-4000-8000-000000000161")
	seedTestResult2ID    = mustUUID("10000000-0000-4000-8000-000000000162")
	seedAppealID         = mustUUID("10000000-0000-4000-8000-000000000171")
	seedLegacySessionID  = mustUUID("10000000-0000-4000-8000-000000000172")
)

func main() {
	ctx := context.Background()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("begin transaction: %v", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := seed(ctx, tx); err != nil {
		log.Fatalf("seed failed: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit seed transaction: %v", err)
	}

	printSummary()
}

func seed(ctx context.Context, tx pgx.Tx) error {
	now := time.Now().UTC()

	superHash, err := security.HashPassword("SuperAdmin123!")
	if err != nil {
		return err
	}
	centerHash, err := security.HashPassword("Center123!")
	if err != nil {
		return err
	}
	adminHash, err := security.HashPassword("Admin123!")
	if err != nil {
		return err
	}
	candidateHash, err := security.HashPassword("Candidate123!")
	if err != nil {
		return err
	}
	instituteHash, err := security.HashPassword("Institute123!")
	if err != nil {
		return err
	}
	expertHash, err := security.HashPassword("Expert123!")
	if err != nil {
		return err
	}
	authorityHash, err := security.HashPassword("Authority123!")
	if err != nil {
		return err
	}
	deviceHash, err := security.HashPassword("Device123!")
	if err != nil {
		return err
	}

	superAdminID, err := upsertSuperAdmin(ctx, tx, seedSuperAdminID, "Integration Super Admin", "super@adlts.et", superHash)
	if err != nil {
		return fmt.Errorf("upsert super admin: %w", err)
	}

	centerID, err := upsertTestCenter(ctx, tx, seedCenterID, centerHash, superAdminID)
	if err != nil {
		return fmt.Errorf("upsert test center: %w", err)
	}

	adminID, err := upsertAdmin(ctx, tx, seedAdminID, centerID, adminHash, superAdminID)
	if err != nil {
		return fmt.Errorf("upsert admin: %w", err)
	}

	institute1ID, err := upsertInstitute(ctx, tx, seedInstitute1ID, "Integration Institute One", "institute@test.et", instituteHash, superAdminID)
	if err != nil {
		return fmt.Errorf("upsert institute1: %w", err)
	}
	institute2ID, err := upsertInstitute(ctx, tx, seedInstitute2ID, "Integration Institute Two", "institute2@test.et", instituteHash, superAdminID)
	if err != nil {
		return fmt.Errorf("upsert institute2: %w", err)
	}
	institute3ID, err := upsertInstitute(ctx, tx, seedInstitute3ID, "Integration Institute Three", "institute3@test.et", instituteHash, superAdminID)
	if err != nil {
		return fmt.Errorf("upsert institute3: %w", err)
	}

	candidates := []struct {
		id        uuid.UUID
		firstName string
		lastName  string
		email     string
		fayidaID  string
		birthDate time.Time
	}{
		{seedCandidate1ID, "Primary", "Candidate", "candidate@test.et", "FAYIDA-CAND-001", time.Date(1996, 1, 12, 0, 0, 0, 0, time.UTC)},
		{seedCandidate2ID, "Hana", "Bekele", "candidate2@test.et", "FAYIDA-CAND-002", time.Date(1994, 7, 8, 0, 0, 0, 0, time.UTC)},
		{seedCandidate3ID, "Abel", "Mekonnen", "candidate3@test.et", "FAYIDA-CAND-003", time.Date(1995, 9, 3, 0, 0, 0, 0, time.UTC)},
		{seedCandidate4ID, "Liya", "Solomon", "candidate4@test.et", "FAYIDA-CAND-004", time.Date(1997, 2, 18, 0, 0, 0, 0, time.UTC)},
		{seedCandidate5ID, "Yonatan", "Tadesse", "candidate5@test.et", "FAYIDA-CAND-005", time.Date(1993, 11, 22, 0, 0, 0, 0, time.UTC)},
		{seedCandidate6ID, "Mahi", "Asfaw", "candidate6@test.et", "FAYIDA-CAND-006", time.Date(1992, 5, 16, 0, 0, 0, 0, time.UTC)},
	}
	for _, c := range candidates {
		if _, err := upsertCandidate(ctx, tx, c.id, c.firstName, c.lastName, c.email, c.fayidaID, c.birthDate, candidateHash, c.id); err != nil {
			return fmt.Errorf("upsert candidate %s: %w", c.email, err)
		}
	}

	expertID, err := upsertExpert(ctx, tx, seedExpertID, expertHash, superAdminID)
	if err != nil {
		return fmt.Errorf("upsert expert: %w", err)
	}
	if _, err := upsertAuthority(ctx, tx, seedAuthorityID, "Integration Authority", "authority@test.et", authorityHash, superAdminID); err != nil {
		return fmt.Errorf("upsert authority: %w", err)
	}

	if err := upsertInvitation(ctx, tx, adminID, now); err != nil {
		return fmt.Errorf("upsert invitation: %w", err)
	}

	if err := upsertInstituteVerification(ctx, tx, seedVerification1ID, seedCandidate1ID, institute1ID, adminID, now); err != nil {
		return fmt.Errorf("upsert institute verification 1: %w", err)
	}
	if err := upsertInstituteVerification(ctx, tx, seedVerification2ID, seedCandidate2ID, institute1ID, adminID, now); err != nil {
		return fmt.Errorf("upsert institute verification 2: %w", err)
	}

	slot1Start := now.Add(24 * time.Hour).Truncate(time.Minute)
	slot1End := slot1Start.Add(2 * time.Hour)
	slot2Start := now.Add(-48 * time.Hour).Truncate(time.Minute)
	slot2End := slot2Start.Add(2 * time.Hour)
	slot3Start := now.Add(72 * time.Hour).Truncate(time.Minute)
	slot3End := slot3Start.Add(2 * time.Hour)

	if err := upsertSlot(ctx, tx, seedSlot1ID, centerID, institute1ID, slot1Start, slot1End, adminID, 3, 1); err != nil {
		return fmt.Errorf("upsert slot1: %w", err)
	}
	if err := upsertSlot(ctx, tx, seedSlot2ID, centerID, institute1ID, slot2Start, slot2End, adminID, 3, 2); err != nil {
		return fmt.Errorf("upsert slot2: %w", err)
	}
	if err := upsertSlot(ctx, tx, seedSlot3ID, centerID, institute2ID, slot3Start, slot3End, adminID, 2, 0); err != nil {
		return fmt.Errorf("upsert slot3: %w", err)
	}

	if err := upsertBooking(ctx, tx, seedBookingPendingID, seedCandidate3ID, centerID, institute1ID, seedVerification1ID, nil, "pending_verification", true, nil, nil, nil, nil, "unpaid", nil, adminID, now); err != nil {
		return fmt.Errorf("upsert booking pending: %w", err)
	}
	verifiedAt := now.Add(-36 * time.Hour)
	if err := upsertBooking(ctx, tx, seedBookingVerifiedID, seedCandidate4ID, centerID, institute1ID, seedVerification1ID, nil, "verified", true, &adminID, &verifiedAt, nil, nil, "unpaid", nil, adminID, now); err != nil {
		return fmt.Errorf("upsert booking verified: %w", err)
	}
	if err := upsertBooking(ctx, tx, seedBookingScheduledID, seedCandidate1ID, centerID, institute1ID, seedVerification1ID, &seedSlot1ID, "scheduled", true, &adminID, &verifiedAt, nil, &slot1Start, "unpaid", intPtr(85000), adminID, now); err != nil {
		return fmt.Errorf("upsert booking scheduled: %w", err)
	}
	if err := upsertBooking(ctx, tx, seedBookingConfirmed1ID, seedCandidate2ID, centerID, institute1ID, seedVerification2ID, &seedSlot2ID, "confirmed", false, &adminID, &verifiedAt, nil, &slot2Start, "paid", intPtr(95000), adminID, now); err != nil {
		return fmt.Errorf("upsert booking confirmed1: %w", err)
	}
	rejectReason := "training evidence did not meet requirements"
	if err := upsertBooking(ctx, tx, seedBookingRejectedID, seedCandidate5ID, centerID, institute2ID, seedVerification1ID, &seedSlot3ID, "rejected", true, &adminID, &verifiedAt, &rejectReason, &slot3Start, "unpaid", intPtr(88000), adminID, now); err != nil {
		return fmt.Errorf("upsert booking rejected: %w", err)
	}
	if err := upsertBooking(ctx, tx, seedBookingConfirmed2ID, seedCandidate6ID, centerID, institute3ID, seedVerification2ID, &seedSlot2ID, "confirmed", false, &adminID, &verifiedAt, nil, &slot2Start, "paid", intPtr(92000), adminID, now); err != nil {
		return fmt.Errorf("upsert booking confirmed2: %w", err)
	}

	if err := upsertDevice(ctx, tx, seedDevice1ID, centerID, "DEVICE-SEED-001", deviceHash, `["class_b","class_c"]`, "rtsp://camera.seed.1/stream", "active", nil, adminID, now); err != nil {
		return fmt.Errorf("upsert device1: %w", err)
	}
	if err := upsertDevice(ctx, tx, seedDevice2ID, centerID, "DEVICE-SEED-002", deviceHash, `["class_b"]`, "rtsp://camera.seed.2/stream", "in_use", &seedRunningTestID, adminID, now); err != nil {
		return fmt.Errorf("upsert device2: %w", err)
	}

	if err := upsertTestPlan(ctx, tx, seedPlanID, centerID, adminID, now); err != nil {
		return fmt.Errorf("upsert test plan: %w", err)
	}

	m1ID, err := upsertManeuver(ctx, tx, seedManeuver1ID, seedPlanID, 1, "Straight Line", "Maintain lane center through straight section", "Q-SEED-001", adminID, now)
	if err != nil {
		return fmt.Errorf("upsert maneuver1: %w", err)
	}
	m2ID, err := upsertManeuver(ctx, tx, seedManeuver2ID, seedPlanID, 2, "Figure Eight", "Complete figure-8 maneuver safely", "Q-SEED-002", adminID, now)
	if err != nil {
		return fmt.Errorf("upsert maneuver2: %w", err)
	}
	m3ID, err := upsertManeuver(ctx, tx, seedManeuver3ID, seedPlanID, 3, "Parking", "Complete parking sequence within boundary", "Q-SEED-003", adminID, now)
	if err != nil {
		return fmt.Errorf("upsert maneuver3: %w", err)
	}

	if err := upsertLevelMapping(ctx, tx, seedMappingID, centerID, seedPlanID, adminID, now); err != nil {
		return fmt.Errorf("upsert test level mapping: %w", err)
	}

	runningStart := now.Add(-30 * time.Minute)
	completed1At := now.Add(-48 * time.Hour)
	completed2At := now.Add(-24 * time.Hour)

	if err := upsertTest(ctx, tx, seedRunningTestID, seedBookingScheduledID, seedCandidate1ID, centerID, seedPlanID, seedDevice2ID, "running", 0, false, &runningStart, nil, adminID, now); err != nil {
		return fmt.Errorf("upsert running test: %w", err)
	}
	if err := upsertTest(ctx, tx, seedCompletedTest1ID, seedBookingConfirmed1ID, seedCandidate2ID, centerID, seedPlanID, seedDevice1ID, "completed", 88.40, true, nil, &completed1At, adminID, now); err != nil {
		return fmt.Errorf("upsert completed test1: %w", err)
	}
	if err := upsertTest(ctx, tx, seedCompletedTest2ID, seedBookingConfirmed2ID, seedCandidate6ID, centerID, seedPlanID, seedDevice1ID, "completed", 73.10, true, nil, &completed2At, adminID, now); err != nil {
		return fmt.Errorf("upsert completed test2: %w", err)
	}

	if err := upsertRunningSession(ctx, tx, seedSessionRunningID, seedRunningTestID, m1ID, 1, now.Add(-10*time.Minute)); err != nil {
		return fmt.Errorf("upsert running session: %w", err)
	}
	if err := upsertCompletedSessionAndResult(ctx, tx, seedSessionResult1ID, seedCompletedTest1ID, m2ID, 1, 90.5, true, now.Add(-50*time.Hour)); err != nil {
		return fmt.Errorf("upsert completed session1/result: %w", err)
	}
	if err := upsertCompletedSessionAndResult(ctx, tx, seedSessionResult2ID, seedCompletedTest2ID, m3ID, 1, 74.2, true, now.Add(-30*time.Hour)); err != nil {
		return fmt.Errorf("upsert completed session2/result: %w", err)
	}

	if err := upsertTestResult(ctx, tx, seedTestResult1ID, seedCompletedTest1ID, 88.40, true, "Strong vehicle control and situational awareness."); err != nil {
		return fmt.Errorf("upsert test result1: %w", err)
	}
	if err := upsertTestResult(ctx, tx, seedTestResult2ID, seedCompletedTest2ID, 73.10, true, "Passed with minor issues in parking precision."); err != nil {
		return fmt.Errorf("upsert test result2: %w", err)
	}

	if err := upsertLegacySession(ctx, tx, seedLegacySessionID, seedBookingConfirmed1ID, seedCandidate2ID, centerID, seedDevice1ID, adminID, 88.40, now); err != nil {
		return fmt.Errorf("upsert legacy session: %w", err)
	}

	if err := upsertAppeal(ctx, tx, seedAppealID, seedCompletedTest1ID, seedLegacySessionID, seedCandidate2ID, expertID, now); err != nil {
		return fmt.Errorf("upsert appeal: %w", err)
	}

	paymentsExists, err := tableExists(ctx, tx, "payments")
	if err != nil {
		return fmt.Errorf("check payments table: %w", err)
	}
	if paymentsExists {
		if err := upsertPayment(ctx, tx, seedPaymentID, seedBookingConfirmed1ID, 95000, now); err != nil {
			return fmt.Errorf("upsert payment: %w", err)
		}
	}

	return nil
}

func upsertSuperAdmin(ctx context.Context, tx pgx.Tx, id uuid.UUID, name, email, passwordHash string) (uuid.UUID, error) {
	var out uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO super_admins (id, name, email, password_hash, created_at, updated_at, created_by, updated_by)
		VALUES ($1,$2,$3,$4,NOW(),NOW(),$1,$1)
		ON CONFLICT (email) DO UPDATE SET
			name=EXCLUDED.name,
			password_hash=EXCLUDED.password_hash,
			updated_at=NOW(),
			updated_by=super_admins.id
		RETURNING id
	`, id, name, email, passwordHash).Scan(&out)
	return out, err
}

func upsertTestCenter(ctx context.Context, tx pgx.Tx, id uuid.UUID, passwordHash string, actorID uuid.UUID) (uuid.UUID, error) {
	var out uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO test_centers (
			id, name, email, password_hash, phone, status,
			street, city, region, country, created_at, updated_at, created_by, updated_by
		)
		VALUES (
			$1,'Integration Test Center','center@adlts.et',$2,'+251911000100','active',
			'Mexico Square','Addis Ababa','Addis Ababa','Ethiopia',NOW(),NOW(),$3,$3
		)
		ON CONFLICT (email) DO UPDATE SET
			name=EXCLUDED.name,
			password_hash=EXCLUDED.password_hash,
			phone=EXCLUDED.phone,
			status=EXCLUDED.status,
			street=EXCLUDED.street,
			city=EXCLUDED.city,
			region=EXCLUDED.region,
			country=EXCLUDED.country,
			updated_at=NOW(),
			updated_by=EXCLUDED.updated_by
		RETURNING id
	`, id, passwordHash, actorID).Scan(&out)
	return out, err
}

func upsertAdmin(ctx context.Context, tx pgx.Tx, id, testCenterID uuid.UUID, passwordHash string, actorID uuid.UUID) (uuid.UUID, error) {
	var out uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO admins (
			id, first_name, middle_name, last_name, email, password_hash, status,
			test_center_id, created_at, updated_at, created_by, updated_by
		)
		VALUES (
			$1,'Integration','','Admin','admin@adlts.et',$2,'active',
			$3,NOW(),NOW(),$4,$4
		)
		ON CONFLICT (email) DO UPDATE SET
			first_name=EXCLUDED.first_name,
			middle_name=EXCLUDED.middle_name,
			last_name=EXCLUDED.last_name,
			password_hash=EXCLUDED.password_hash,
			status=EXCLUDED.status,
			test_center_id=EXCLUDED.test_center_id,
			updated_at=NOW(),
			updated_by=EXCLUDED.updated_by
		RETURNING id
	`, id, passwordHash, testCenterID, actorID).Scan(&out)
	return out, err
}

func upsertInstitute(ctx context.Context, tx pgx.Tx, id uuid.UUID, name, email, passwordHash string, actorID uuid.UUID) (uuid.UUID, error) {
	var out uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO institutes (
			id, name, name_am, email, password_hash, phone, logo_url, status,
			street, city, region, country, created_at, updated_at, created_by, updated_by
		)
		VALUES (
			$1,$2,$2,$3,$4,'+251911000200','/logos/institutes/seed.svg','active',
			'Bole Road','Addis Ababa','Addis Ababa','Ethiopia',NOW(),NOW(),$5,$5
		)
		ON CONFLICT (email) DO UPDATE SET
			name=EXCLUDED.name,
			name_am=EXCLUDED.name_am,
			password_hash=EXCLUDED.password_hash,
			logo_url=EXCLUDED.logo_url,
			phone=EXCLUDED.phone,
			status=EXCLUDED.status,
			street=EXCLUDED.street,
			city=EXCLUDED.city,
			region=EXCLUDED.region,
			country=EXCLUDED.country,
			updated_at=NOW(),
			updated_by=EXCLUDED.updated_by
		RETURNING id
	`, id, name, email, passwordHash, actorID).Scan(&out)
	return out, err
}

func upsertCandidate(ctx context.Context, tx pgx.Tx, id uuid.UUID, firstName, lastName, email, fayidaID string, birthDate time.Time, passwordHash string, actorID uuid.UUID) (uuid.UUID, error) {
	var out uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO candidates (
			id, first_name, middle_name, last_name, email, password_hash, status,
			phone, fayida_id, birth_date, gender, photo_url, street, city, region, country,
			created_at, updated_at, created_by, updated_by
		)
		VALUES (
			$1,$2,'',$3,$4,$5,'active',
			'+251900123456',$6,$7,'male','','CMC','Addis Ababa','Addis Ababa','Ethiopia',
			NOW(),NOW(),$8,$8
		)
		ON CONFLICT (email) DO UPDATE SET
			first_name=EXCLUDED.first_name,
			middle_name=EXCLUDED.middle_name,
			last_name=EXCLUDED.last_name,
			password_hash=EXCLUDED.password_hash,
			status=EXCLUDED.status,
			phone=EXCLUDED.phone,
			fayida_id=EXCLUDED.fayida_id,
			birth_date=EXCLUDED.birth_date,
			gender=EXCLUDED.gender,
			photo_url=EXCLUDED.photo_url,
			street=EXCLUDED.street,
			city=EXCLUDED.city,
			region=EXCLUDED.region,
			country=EXCLUDED.country,
			updated_at=NOW(),
			updated_by=EXCLUDED.updated_by
		RETURNING id
	`, id, firstName, lastName, email, passwordHash, fayidaID, birthDate, actorID).Scan(&out)
	return out, err
}

func upsertExpert(ctx context.Context, tx pgx.Tx, id uuid.UUID, passwordHash string, actorID uuid.UUID) (uuid.UUID, error) {
	var out uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO experts (
			id, first_name, middle_name, last_name, email, password_hash, status, phone,
			fayida_id, employee_id, birth_date, gender, created_at, updated_at, created_by, updated_by
		)
		VALUES (
			$1,'Integration','','Expert','expert@test.et',$2,'active','+251911200100',
			'FAYIDA-EXPERT-001','EMP-EXPERT-001','1988-06-12','male',NOW(),NOW(),$3,$3
		)
		ON CONFLICT (email) DO UPDATE SET
			first_name=EXCLUDED.first_name,
			middle_name=EXCLUDED.middle_name,
			last_name=EXCLUDED.last_name,
			password_hash=EXCLUDED.password_hash,
			status=EXCLUDED.status,
			phone=EXCLUDED.phone,
			fayida_id=EXCLUDED.fayida_id,
			employee_id=EXCLUDED.employee_id,
			birth_date=EXCLUDED.birth_date,
			gender=EXCLUDED.gender,
			updated_at=NOW(),
			updated_by=EXCLUDED.updated_by
		RETURNING id
	`, id, passwordHash, actorID).Scan(&out)
	return out, err
}

func upsertAuthority(ctx context.Context, tx pgx.Tx, id uuid.UUID, name, email, passwordHash string, actorID uuid.UUID) (uuid.UUID, error) {
	var out uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO transport_authorities (
			id, name, name_am, email, password_hash, phone, logo_url, status,
			street, city, region, country, created_at, updated_at, created_by, updated_by
		)
		VALUES (
			$1,$2,$2,$3,$4,'+251911300100','/logos/authorities/seed.svg','active',
			'Kazanchis','Addis Ababa','Addis Ababa','Ethiopia',NOW(),NOW(),$5,$5
		)
		ON CONFLICT (email) DO UPDATE SET
			name=EXCLUDED.name,
			name_am=EXCLUDED.name_am,
			email=EXCLUDED.email,
			password_hash=EXCLUDED.password_hash,
			logo_url=EXCLUDED.logo_url,
			phone=EXCLUDED.phone,
			status=EXCLUDED.status,
			street=EXCLUDED.street,
			city=EXCLUDED.city,
			region=EXCLUDED.region,
			country=EXCLUDED.country,
			updated_at=NOW(),
			updated_by=EXCLUDED.updated_by
		RETURNING id
	`, id, name, email, passwordHash, actorID).Scan(&out)
	return out, err
}

func upsertInvitation(ctx context.Context, tx pgx.Tx, createdBy uuid.UUID, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO invitations (
			id, token, email, entity_type, test_center_id, expires_at, used_at, created_by, created_at
		)
		VALUES (
			$1,'seed-invite-token-001','newexpert@adlts.et','expert',$2,$3,NULL,$4,$5
		)
		ON CONFLICT (id) DO UPDATE SET
			token=EXCLUDED.token,
			email=EXCLUDED.email,
			entity_type=EXCLUDED.entity_type,
			test_center_id=EXCLUDED.test_center_id,
			expires_at=EXCLUDED.expires_at,
			used_at=NULL,
			created_by=EXCLUDED.created_by,
			created_at=EXCLUDED.created_at
	`, seedInvitationID, seedCenterID, now.Add(48*time.Hour), createdBy, now)
	return err
}

func upsertInstituteVerification(ctx context.Context, tx pgx.Tx, id, candidateID, instituteID, actorID uuid.UUID, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO institute_verifications (
			id, candidate_id, institute_id, issued_at, expires_at, created_at, updated_at, created_by, updated_by
		)
		VALUES ($1,$2,$3,$4,$5,$4,$4,$6,$6)
		ON CONFLICT (id) DO UPDATE SET
			candidate_id=EXCLUDED.candidate_id,
			institute_id=EXCLUDED.institute_id,
			issued_at=EXCLUDED.issued_at,
			expires_at=EXCLUDED.expires_at,
			updated_at=NOW(),
			updated_by=EXCLUDED.updated_by
	`, id, candidateID, instituteID, now, now.Add(365*24*time.Hour), actorID)
	return err
}

func upsertSlot(ctx context.Context, tx pgx.Tx, id, centerID, instituteID uuid.UUID, startsAt, endsAt time.Time, actorID uuid.UUID, capacity, bookedCount int) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO slots (
			id, test_center_id, institute_id, test_id, starts_at, ends_at, start_time, end_time, capacity, booked_count,
			created_at, updated_at, created_by, updated_by
		)
		VALUES (
			$1,$2,$3,NULL,$4,$5,$4,$5,$6,$7,NOW(),NOW(),$8,$8
		)
		ON CONFLICT (id) DO UPDATE SET
			test_center_id=EXCLUDED.test_center_id,
			institute_id=EXCLUDED.institute_id,
			starts_at=EXCLUDED.starts_at,
			ends_at=EXCLUDED.ends_at,
			start_time=EXCLUDED.start_time,
			end_time=EXCLUDED.end_time,
			capacity=EXCLUDED.capacity,
			booked_count=EXCLUDED.booked_count,
			updated_at=NOW(),
			updated_by=EXCLUDED.updated_by
	`, id, centerID, instituteID, startsAt, endsAt, capacity, bookedCount, actorID)
	return err
}

func upsertBooking(
	ctx context.Context,
	tx pgx.Tx,
	id, candidateID, centerID, instituteID, verificationID uuid.UUID,
	slotID *uuid.UUID,
	status string,
	requiresVerification bool,
	verifiedBy *uuid.UUID,
	verifiedAt *time.Time,
	rejectionReason *string,
	scheduledAt *time.Time,
	paymentStatus string,
	paymentAmountCents *int,
	actorID uuid.UUID,
	now time.Time,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO bookings (
			id, candidate_id, test_center_id, institute_verification_id, institute_id, slot_id,
			test_id, status, requires_verification, training_hours, training_evidence_url,
			verified_by, verified_at, rejection_reason, scheduled_at, payment_ref, payment_status,
			payment_amount_cents, payment_attempts, archived_at, test_level_code,
			created_at, updated_at, created_by, updated_by
		)
		VALUES (
			$1,$2,$3,$4,$5,$6,
			NULL,$7,$8,0,NULL,
			$9,$10,$11,$12,NULL,$13,
			$14,1,NULL,'class_b',
			$16,$16,$15,$15
		)
		ON CONFLICT (id) DO UPDATE SET
			candidate_id=EXCLUDED.candidate_id,
			test_center_id=EXCLUDED.test_center_id,
			institute_verification_id=EXCLUDED.institute_verification_id,
			institute_id=EXCLUDED.institute_id,
			slot_id=EXCLUDED.slot_id,
			status=EXCLUDED.status,
			requires_verification=EXCLUDED.requires_verification,
			verified_by=EXCLUDED.verified_by,
			verified_at=EXCLUDED.verified_at,
			rejection_reason=EXCLUDED.rejection_reason,
			scheduled_at=EXCLUDED.scheduled_at,
			payment_status=EXCLUDED.payment_status,
			payment_amount_cents=EXCLUDED.payment_amount_cents,
			test_level_code=EXCLUDED.test_level_code,
			updated_at=NOW(),
			updated_by=EXCLUDED.updated_by
	`, id, candidateID, centerID, verificationID, instituteID, slotID, status, requiresVerification,
		verifiedBy, verifiedAt, rejectionReason, scheduledAt, paymentStatus, paymentAmountCents, actorID, now)
	return err
}

func upsertDevice(
	ctx context.Context,
	tx pgx.Tx,
	id, centerID uuid.UUID,
	deviceCode, passwordHash, allowedLevels, streamURL, status string,
	currentTestID *uuid.UUID,
	actorID uuid.UUID,
	now time.Time,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO devices (
			id, device_code, password_hash, test_center_id, allowed_levels, stream_url, status, current_test_id,
			last_seen_at, created_at, updated_at, created_by, updated_by
		)
		VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10,$11,$11
		)
		ON CONFLICT (device_code) DO UPDATE SET
			password_hash=EXCLUDED.password_hash,
			test_center_id=EXCLUDED.test_center_id,
			allowed_levels=EXCLUDED.allowed_levels,
			stream_url=EXCLUDED.stream_url,
			status=EXCLUDED.status,
			current_test_id=EXCLUDED.current_test_id,
			last_seen_at=EXCLUDED.last_seen_at,
			updated_at=NOW(),
			updated_by=EXCLUDED.updated_by
	`, id, deviceCode, passwordHash, centerID, allowedLevels, streamURL, status, currentTestID, now, now, actorID)
	return err
}

func upsertTestPlan(ctx context.Context, tx pgx.Tx, id, centerID, actorID uuid.UUID, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO test_plans (
			id, test_center_id, name, description, pass_threshold, status, published_at,
			created_at, updated_at, created_by, updated_by
		)
		VALUES (
			$1,$2,'Class B Integration Plan','Seeded plan for integration verification',70.00,'active',$3,
			$3,$3,$4,$4
		)
		ON CONFLICT (id) DO UPDATE SET
			test_center_id=EXCLUDED.test_center_id,
			name=EXCLUDED.name,
			description=EXCLUDED.description,
			pass_threshold=EXCLUDED.pass_threshold,
			status=EXCLUDED.status,
			published_at=EXCLUDED.published_at,
			updated_at=NOW(),
			updated_by=EXCLUDED.updated_by
	`, id, centerID, now, actorID)
	return err
}

func upsertManeuver(ctx context.Context, tx pgx.Tx, id, planID uuid.UUID, sequence int, name, description, qrCode string, actorID uuid.UUID, now time.Time) (uuid.UUID, error) {
	var out uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO maneuver_configs (
			id, test_plan_id, name, description, sequence_number, qr_code_value,
			tolerance_px, weight, min_frames_required, maneuver_type, display_name, pass_threshold,
			created_at, updated_at, created_by, updated_by
		)
		VALUES (
			$1,$2,$3,$4,$5,$6,
			20,1.00,30,NULL,$9,70.0,
			$7,$7,$8,$8
		)
		ON CONFLICT (test_plan_id, sequence_number) DO UPDATE SET
			name=EXCLUDED.name,
			description=EXCLUDED.description,
			qr_code_value=EXCLUDED.qr_code_value,
			tolerance_px=EXCLUDED.tolerance_px,
			weight=EXCLUDED.weight,
			min_frames_required=EXCLUDED.min_frames_required,
			display_name=EXCLUDED.display_name,
			pass_threshold=EXCLUDED.pass_threshold,
			updated_at=NOW(),
			updated_by=EXCLUDED.updated_by
		RETURNING id
	`, id, planID, name, description, sequence, qrCode, now, actorID, name).Scan(&out)
	return out, err
}

func upsertLevelMapping(ctx context.Context, tx pgx.Tx, id, centerID, planID, actorID uuid.UUID, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO test_level_mappings (
			id, test_center_id, test_level_code, test_plan_id, created_at, updated_at, created_by, updated_by
		)
		VALUES ($1,$2,'class_b',$3,$4,$4,$5,$5)
		ON CONFLICT (test_center_id, test_level_code) DO UPDATE SET
			test_plan_id=EXCLUDED.test_plan_id,
			updated_at=NOW(),
			updated_by=EXCLUDED.updated_by
	`, id, centerID, planID, now, actorID)
	return err
}

func upsertTest(
	ctx context.Context,
	tx pgx.Tx,
	id, bookingID, candidateID, centerID, planID, deviceID uuid.UUID,
	status string,
	score float64,
	passed bool,
	startedAt *time.Time,
	completedAt *time.Time,
	actorID uuid.UUID,
	now time.Time,
) error {
	var weightedScore *float64
	var passedPtr *bool
	var visibleAt *time.Time
	if status == "completed" {
		weightedScore = &score
		passedPtr = &passed
		t := now
		if completedAt != nil {
			t = *completedAt
		}
		visibleAt = &t
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO tests (
			id, booking_id, candidate_id, test_center_id, test_plan_id, device_id,
			test_level_code, status,
			scheduled_start_at, scheduled_end_at, booking_window_hours,
			device_scanned_at, guidelines_start_at, acknowledged_at, iot_health_start_at, iot_health_passed_at,
			started_at, finishing_at, completed_at,
			weighted_total_score, passed, appeal_window_closes_at, result_visible_to_candidate_at, result_visible_to_institute_at,
			created_at, updated_at, created_by, updated_by
		)
		VALUES (
			$1,$2,$3,$4,$5,$6,
			'class_b',$7,
			$8,$9,2,
			$10,NULL,NULL,NULL,$10,
			$10,NULL,$11,
			$12,$13,$14,$15,$15,
			$16,$16,$17,$17
		)
		ON CONFLICT (id) DO UPDATE SET
			booking_id=EXCLUDED.booking_id,
			candidate_id=EXCLUDED.candidate_id,
			test_center_id=EXCLUDED.test_center_id,
			test_plan_id=EXCLUDED.test_plan_id,
			device_id=EXCLUDED.device_id,
			status=EXCLUDED.status,
			scheduled_start_at=EXCLUDED.scheduled_start_at,
			scheduled_end_at=EXCLUDED.scheduled_end_at,
			device_scanned_at=EXCLUDED.device_scanned_at,
			iot_health_passed_at=EXCLUDED.iot_health_passed_at,
			started_at=EXCLUDED.started_at,
			completed_at=EXCLUDED.completed_at,
			weighted_total_score=EXCLUDED.weighted_total_score,
			passed=EXCLUDED.passed,
			appeal_window_closes_at=EXCLUDED.appeal_window_closes_at,
			result_visible_to_candidate_at=EXCLUDED.result_visible_to_candidate_at,
			result_visible_to_institute_at=EXCLUDED.result_visible_to_institute_at,
			updated_at=NOW(),
			updated_by=EXCLUDED.updated_by
	`, id, bookingID, candidateID, centerID, planID, deviceID, status,
		startedAt, timePtrAdd(startedAt, 90*time.Minute), startedAt, completedAt,
		weightedScore, passedPtr, timePtrAdd(completedAt, 72*time.Hour), visibleAt, now, actorID)
	return err
}

func upsertRunningSession(ctx context.Context, tx pgx.Tx, sessionID, testID, maneuverID uuid.UUID, sequence int, startedAt time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO test_sessions (
			id, test_id, maneuver_id, sequence_number, start_frame_seq, ended_at, started_at,
			frame_count, lane_detected_count, score, passed, scored_at, avg_iou_score
		)
		VALUES ($1,$2,$3,$4,0,NULL,$5,0,0,NULL,NULL,NULL,NULL)
		ON CONFLICT (id) DO UPDATE SET
			test_id=EXCLUDED.test_id,
			maneuver_id=EXCLUDED.maneuver_id,
			sequence_number=EXCLUDED.sequence_number,
			started_at=EXCLUDED.started_at
	`, sessionID, testID, maneuverID, sequence, startedAt)
	return err
}

func upsertCompletedSessionAndResult(ctx context.Context, tx pgx.Tx, sessionID, testID, maneuverID uuid.UUID, sequence int, score float64, passed bool, startedAt time.Time) error {
	endedAt := startedAt.Add(10 * time.Minute)
	_, err := tx.Exec(ctx, `
		INSERT INTO test_sessions (
			id, test_id, maneuver_id, sequence_number, start_frame_seq, end_frame_seq,
			started_at, ended_at, score, passed, scored_at, frame_count, lane_detected_count, avg_iou_score
		)
		VALUES ($1,$2,$3,$4,0,650,$5,$6,$7,$8,$6,650,620,0.93)
		ON CONFLICT (id) DO UPDATE SET
			test_id=EXCLUDED.test_id,
			maneuver_id=EXCLUDED.maneuver_id,
			sequence_number=EXCLUDED.sequence_number,
			ended_at=EXCLUDED.ended_at,
			score=EXCLUDED.score,
			passed=EXCLUDED.passed,
			scored_at=EXCLUDED.scored_at,
			frame_count=EXCLUDED.frame_count,
			lane_detected_count=EXCLUDED.lane_detected_count,
			avg_iou_score=EXCLUDED.avg_iou_score
	`, sessionID, testID, maneuverID, sequence, startedAt, endedAt, score, passed)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO session_results (
			id, test_id, session_id, maneuver_id, sequence_number, score, weight, passed,
			frame_count, lane_detected_pct, avg_iou, maneuver_type, critical_fail, created_at
		)
		VALUES ($1,$2,$1,$3,$4,$5,1.0,$6,650,95.0,0.93,'straight_line',false,NOW())
		ON CONFLICT (session_id) DO UPDATE SET
			maneuver_id=EXCLUDED.maneuver_id,
			sequence_number=EXCLUDED.sequence_number,
			score=EXCLUDED.score,
			weight=EXCLUDED.weight,
			passed=EXCLUDED.passed,
			frame_count=EXCLUDED.frame_count,
			lane_detected_pct=EXCLUDED.lane_detected_pct,
			avg_iou=EXCLUDED.avg_iou,
			maneuver_type=EXCLUDED.maneuver_type,
			critical_fail=EXCLUDED.critical_fail
	`, sessionID, testID, maneuverID, sequence, score, passed)
	return err
}

func upsertTestResult(ctx context.Context, tx pgx.Tx, id, testID uuid.UUID, score float64, passed bool, narrative string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO test_results (
			id, test_id, weighted_total_score, passed, pass_threshold,
			overall_narrative, strengths_narrative, weaknesses_narrative, recommended_focus, narrative_model,
			any_critical_fail, weakest_maneuver, score_breakdown, created_at
		)
		VALUES (
			$1,$2,$3,$4,70.0,
			$5,'Good lane control and smooth steering','Improve slow-speed precision','Practice parking entry speed','seed-generator-v1',
			false,'parking','{"straight_line": 88.4, "parking": 73.1}',NOW()
		)
		ON CONFLICT (test_id) DO UPDATE SET
			weighted_total_score=EXCLUDED.weighted_total_score,
			passed=EXCLUDED.passed,
			pass_threshold=EXCLUDED.pass_threshold,
			overall_narrative=EXCLUDED.overall_narrative,
			strengths_narrative=EXCLUDED.strengths_narrative,
			weaknesses_narrative=EXCLUDED.weaknesses_narrative,
			recommended_focus=EXCLUDED.recommended_focus,
			narrative_model=EXCLUDED.narrative_model,
			any_critical_fail=EXCLUDED.any_critical_fail,
			weakest_maneuver=EXCLUDED.weakest_maneuver,
			score_breakdown=EXCLUDED.score_breakdown
	`, id, testID, score, passed, narrative)
	return err
}

func upsertLegacySession(ctx context.Context, tx pgx.Tx, id, bookingID, candidateID, centerID, deviceID, actorID uuid.UUID, score float64, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO sessions (
			id, booking_id, candidate_id, test_center_id, device_id, status, score,
			recording_url, result_overlay_url, started_at, completed_at, finalized_at,
			created_at, updated_at, created_by, updated_by
		)
		VALUES (
			$1,$2,$3,$4,$5,'completed',$6,
			'seed-recording.mp4','seed-overlay.json',$7,$8,$8,
			$7,$7,$9,$9
		)
		ON CONFLICT (id) DO UPDATE SET
			booking_id=EXCLUDED.booking_id,
			candidate_id=EXCLUDED.candidate_id,
			test_center_id=EXCLUDED.test_center_id,
			device_id=EXCLUDED.device_id,
			status=EXCLUDED.status,
			score=EXCLUDED.score,
			recording_url=EXCLUDED.recording_url,
			result_overlay_url=EXCLUDED.result_overlay_url,
			started_at=EXCLUDED.started_at,
			completed_at=EXCLUDED.completed_at,
			finalized_at=EXCLUDED.finalized_at,
			updated_at=NOW(),
			updated_by=EXCLUDED.updated_by
	`, id, bookingID, candidateID, centerID, deviceID, score, now.Add(-50*time.Hour), now.Add(-48*time.Hour), actorID)
	return err
}

func upsertAppeal(ctx context.Context, tx pgx.Tx, id, testID, sessionID, candidateID, expertID uuid.UUID, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO appeals (
			id, session_id, candidate_id, expert_id, reason, status, resolution,
			created_at, updated_at, created_by, updated_by
		)
		VALUES (
			$1,$2,$3,$4,'Candidate disputes parking penalty','pending',NULL,
			$5,$5,$3,$3
		)
		ON CONFLICT (id) DO UPDATE SET
			session_id=EXCLUDED.session_id,
			candidate_id=EXCLUDED.candidate_id,
			expert_id=EXCLUDED.expert_id,
			reason=EXCLUDED.reason,
			status=EXCLUDED.status,
			resolution=NULL,
			updated_at=NOW(),
			updated_by=EXCLUDED.updated_by
	`, id, sessionID, candidateID, expertID, now)
	return err
}

func upsertPayment(ctx context.Context, tx pgx.Tx, id, bookingID uuid.UUID, amountCents int, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO payments (
			id, booking_id, amount_cents, currency, status, provider, provider_ref, attempt_number, created_at, updated_at
		)
		VALUES (
			$1,$2,$3,'ETB','success','chapa','seed-payment-ref-001',1,$4,$4
		)
		ON CONFLICT (id) DO UPDATE SET
			booking_id=EXCLUDED.booking_id,
			amount_cents=EXCLUDED.amount_cents,
			currency=EXCLUDED.currency,
			status=EXCLUDED.status,
			provider=EXCLUDED.provider,
			provider_ref=EXCLUDED.provider_ref,
			attempt_number=EXCLUDED.attempt_number,
			updated_at=NOW()
	`, id, bookingID, amountCents, now)
	return err
}

func tableExists(ctx context.Context, tx pgx.Tx, tableName string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+tableName).Scan(&exists)
	return exists, err
}

func timePtrAdd(t *time.Time, d time.Duration) *time.Time {
	if t == nil {
		return nil
	}
	out := t.Add(d)
	return &out
}

func intPtr(v int) *int { return &v }

func mustUUID(v string) uuid.UUID {
	id, err := uuid.Parse(v)
	if err != nil {
		panic(err)
	}
	return id
}

func printSummary() {
	fmt.Println("Seed completed successfully.")
	fmt.Println("")
	fmt.Println("Credentials:")
	fmt.Println("  super_admin: super@adlts.et / SuperAdmin123!")
	fmt.Println("  admin: admin@adlts.et / Admin123!")
	fmt.Println("  candidate: candidate@test.et / Candidate123!")
	fmt.Println("  institute: institute@test.et / Institute123!")
	fmt.Println("  expert: expert@test.et / Expert123!")
	fmt.Println("  transport_authority: authority@test.et / Authority123!")
	fmt.Println("")
	fmt.Println("Seeded test IDs:")
	fmt.Printf("  running:   %s\n", seedRunningTestID)
	fmt.Printf("  completed: %s\n", seedCompletedTest1ID)
	fmt.Printf("  completed: %s\n", seedCompletedTest2ID)
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  DATABASE_URL=postgres://user:pass@localhost:5432/adlts go run ./cmd/seed/main.go")
}

package store

import (
	"sync"
	"time"

	"adlts/internal/platform/domain"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Store struct {
	mu          sync.RWMutex
	Users       map[string]*domain.User
	Institutes  map[string]*domain.Institute
	Invitations map[string]*domain.Invitation
	Bookings    map[string]*domain.Booking
	Slots       map[string]*domain.Slot
	Devices     map[string]*domain.Device
	Exams       map[string]*domain.Exam
	Appeals     map[string]*domain.Appeal
	Frames      map[string]*domain.FrameAnalysis
	OTPHistory  map[string]time.Time
}

func New() *Store {
	return &Store{
		Users:       map[string]*domain.User{},
		Institutes:  map[string]*domain.Institute{},
		Invitations: map[string]*domain.Invitation{},
		Bookings:    map[string]*domain.Booking{},
		Slots:       map[string]*domain.Slot{},
		Devices:     map[string]*domain.Device{},
		Exams:       map[string]*domain.Exam{},
		Appeals:     map[string]*domain.Appeal{},
		Frames:      map[string]*domain.FrameAnalysis{},
		OTPHistory:  map[string]time.Time{},
	}
}

func Read[T any](s *Store, fn func() T) T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return fn()
}

func Write[T any](s *Store, fn func() T) T {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fn()
}

func (s *Store) FindUser(id string) (*domain.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.Users[id]
	return user, ok
}

func (s *Store) FindUserByEmail(email string) (*domain.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, user := range s.Users {
		if user.Email == email {
			return user, true
		}
	}
	return nil, false
}

func (s *Store) FindInstitute(id string) (*domain.Institute, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inst, ok := s.Institutes[id]
	return inst, ok
}

func (s *Store) FindBooking(id string) (*domain.Booking, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	booking, ok := s.Bookings[id]
	return booking, ok
}

func (s *Store) FindDevice(id string) (*domain.Device, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	device, ok := s.Devices[id]
	return device, ok
}

func (s *Store) FindExam(id string) (*domain.Exam, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	exam, ok := s.Exams[id]
	return exam, ok
}

func (s *Store) FindAppeal(id string) (*domain.Appeal, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	appeal, ok := s.Appeals[id]
	return appeal, ok
}

func (s *Store) InvitationsByEmail(email string) []*domain.Invitation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*domain.Invitation, 0)
	for _, invitation := range s.Invitations {
		if invitation.Email == email {
			result = append(result, invitation)
		}
	}
	return result
}

func NewID() string {
	return uuid.NewString()
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func SeedDemoData(s *Store) {
	now := time.Now().UTC()
	hash, _ := HashPassword("Password123!")

	authority := &domain.User{ID: NewID(), Name: "Transport Authority", Email: "authority@adlts.local", PasswordHash: hash, Role: domain.RoleAuthority, Status: domain.AccountActive, CreatedAt: now, UpdatedAt: now}
	admin := &domain.User{ID: NewID(), Name: "System Admin", Email: "admin@adlts.local", PasswordHash: hash, Role: domain.RoleAdmin, Status: domain.AccountActive, CreatedAt: now, UpdatedAt: now}
	instituteAdmin := &domain.User{ID: NewID(), Name: "Institute Admin", Email: "institute@adlts.local", PasswordHash: hash, Role: domain.RoleInstituteAdmin, Status: domain.AccountActive, CreatedAt: now, UpdatedAt: now}
	examiner := &domain.User{ID: NewID(), Name: "Examiner", Email: "examiner@adlts.local", PasswordHash: hash, Role: domain.RoleExaminer, Status: domain.AccountActive, CreatedAt: now, UpdatedAt: now}
	candidate := &domain.User{ID: NewID(), Name: "Candidate", Email: "candidate@adlts.local", PasswordHash: hash, Role: domain.RoleCandidate, Status: domain.AccountActive, CreatedAt: now, UpdatedAt: now}

	institute := &domain.Institute{ID: NewID(), Name: "Safe Drive School", Email: "school@adlts.local", Verified: true, CreatedAt: now}
	instituteAdmin.InstituteID = institute.ID
	candidate.InstituteID = institute.ID

	s.mu.Lock()
	defer s.mu.Unlock()
	s.Users[authority.ID] = authority
	s.Users[admin.ID] = admin
	s.Users[instituteAdmin.ID] = instituteAdmin
	s.Users[examiner.ID] = examiner
	s.Users[candidate.ID] = candidate
	s.Institutes[institute.ID] = institute

	slotOneID := NewID()
	slotTwoID := NewID()
	s.Slots[slotOneID] = &domain.Slot{ID: slotOneID, InstituteID: institute.ID, StartTime: now.Add(24 * time.Hour), EndTime: now.Add(25 * time.Hour), Capacity: 20, BookedCount: 2, Location: "S-Curve Track"}
	s.Slots[slotTwoID] = &domain.Slot{ID: slotTwoID, InstituteID: institute.ID, StartTime: now.Add(48 * time.Hour), EndTime: now.Add(49 * time.Hour), Capacity: 20, BookedCount: 1, Location: "Parking Bay"}

	deviceID := NewID()
	device := &domain.Device{ID: deviceID, MACAddress: "AA:BB:CC:DD:EE:FF", Name: "ESP32-CAM Demo", Secret: "demo-device-secret", Status: "online", CreatedAt: now, LastHeartbeat: &now}
	s.Devices[deviceID] = device

	booking := &domain.Booking{ID: NewID(), CandidateID: candidate.ID, InstituteID: institute.ID, Status: domain.BookingVerified, TrainingHours: 30, TrainingEvidenceURL: "https://storage.local/evidence/training-log.pdf", VerifiedBy: instituteAdmin.ID, VerifiedAt: &now, CreatedAt: now, UpdatedAt: now}
	s.Bookings[booking.ID] = booking

	exam := &domain.Exam{ID: NewID(), BookingID: booking.ID, CandidateID: candidate.ID, DeviceID: device.ID, ExaminerID: examiner.ID, Status: domain.ExamActive, Score: 87.5, Violations: []domain.Violation{{Code: "SIGN_DISREGARD", Message: "Stop sign ignored", Severity: "high", CreatedAt: now}}, Telemetry: domain.ExamTelemetry{Health: "nominal", CurrentScore: 87.5, ViolationCount: 1, UpdatedAt: now}, ResultOverlayURL: "https://storage.local/results/overlay.mp4", StartedAt: &now, CreatedAt: now, UpdatedAt: now}
	s.Exams[exam.ID] = exam

	appeal := &domain.Appeal{ID: NewID(), ExamID: exam.ID, CandidateID: candidate.ID, Reason: "Requested human review of a stop-sign event", Status: domain.AppealPending, CreatedAt: now, UpdatedAt: now}
	s.Appeals[appeal.ID] = appeal
}

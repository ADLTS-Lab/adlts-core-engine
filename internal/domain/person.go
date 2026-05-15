package domain

import (
	"time"

	"github.com/google/uuid"
)

type BasePerson struct {
	ID           uuid.UUID  `db:"id"`
	FirstName    string     `db:"first_name"`
	MiddleName   string     `db:"middle_name"`
	LastName     string     `db:"last_name"`
	FirstNameAm  *string    `db:"first_name_am"`  // optional Amharic
	MiddleNameAm *string    `db:"middle_name_am"`
	LastNameAm   *string    `db:"last_name_am"`
	Email        string     `db:"email"` // unique
	PasswordHash string     `db:"password_hash"`
	Status       UserStatus `db:"status"`
	Audit
}

type Candidate struct {
	BasePerson
	Phone     string    `db:"phone"`
	FayidaID  string    `db:"fayida_id"`
	BirthDate time.Time `db:"birth_date"`
	Gender    Gender    `db:"gender"`
	Address   Address
	PhotoURL  string `db:"photo_url"`
}

type Expert struct {
	BasePerson
	Phone      string    `db:"phone"`
	FayidaID   string    `db:"fayida_id"`
	EmployeeID string    `db:"employee_id"`
	BirthDate  time.Time `db:"birth_date"`
	Gender     Gender    `db:"gender"`
	PhotoURL   string    `db:"photo_url"`
}

type Admin struct {
	BasePerson
	TestCenterID uuid.UUID `db:"test_center_id"`
}

package domain

import "github.com/google/uuid"

// SuperAdmin is a platform-level operator account, not a domain person.
// It has a single display name, no personal details, and NO status field
// because a SuperAdmin account is always active by definition.
// Privileges: full read and write on all entities including admin CRUD.
type SuperAdmin struct {
	ID           uuid.UUID `db:"id"`
	Name         string    `db:"name"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	Audit
}

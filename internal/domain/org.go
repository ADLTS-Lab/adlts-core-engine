package domain

import "github.com/google/uuid"

type BaseOrg struct {
	ID           uuid.UUID `db:"id"`
	Name         string    `db:"name"`
	NameAm       *string   `db:"name_am"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	Phone        string    `db:"phone"`
	Address      Address
	LogoURL      string    `db:"logo_url"`
	Status       OrgStatus `db:"status"`
	Audit
}

type Institute struct {
	BaseOrg
}

type TransportAuthority struct {
	BaseOrg
}

type TestCenter struct {
	BaseOrg
}

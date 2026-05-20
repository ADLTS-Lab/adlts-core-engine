package domain

type UserStatus string

const (
	UserStatusActive              UserStatus = "active"
	UserStatusInactive            UserStatus = "inactive"
	UserStatusSuspended           UserStatus = "suspended"
	UserStatusPendingVerification UserStatus = "pending_verification"
)

type OrgStatus string

const (
	OrgStatusActive          OrgStatus = "active"
	OrgStatusInactive        OrgStatus = "inactive"
	OrgStatusSuspended       OrgStatus = "suspended"
	OrgStatusPendingApproval OrgStatus = "pending_approval"
)

type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
)

type BookingStatus string

const (
	BookingPendingVerification BookingStatus = "pending_verification"
	BookingVerified            BookingStatus = "verified"
	BookingRejected            BookingStatus = "rejected"
	BookingScheduled           BookingStatus = "scheduled"
)

type AppealStatus string

const (
	AppealPending  AppealStatus = "pending"
	AppealAccepted AppealStatus = "accepted"
	AppealRejected AppealStatus = "rejected"
)

// DeviceStatus, DeviceCommand, and SessionStatus (testing core) are in testing.go.

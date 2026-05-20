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
type SessionStatus string

const (
	SessionScheduled      SessionStatus = "scheduled"
	SessionInitiating     SessionStatus = "initiating"
	SessionActive         SessionStatus = "active"
	SessionCompleted      SessionStatus = "completed"
	SessionReviewRequired SessionStatus = "review_required"
	SessionFinalized      SessionStatus = "finalized"
	SessionAborted        SessionStatus = "aborted"
)

type AppealStatus string

const (
	AppealPending  AppealStatus = "pending"
	AppealAccepted AppealStatus = "accepted"
	AppealRejected AppealStatus = "rejected"
)

type DeviceCommand string

const (
	CmdHealth        DeviceCommand = "health"
	CmdHealthDetails DeviceCommand = "health-details"
	CmdStartTest     DeviceCommand = "start-test"
	CmdEndTest       DeviceCommand = "end-test"
	CmdAbort         DeviceCommand = "abort"
)

type DeviceStatus string

const (
	DeviceOnline    DeviceStatus = "online"
	DeviceOffline   DeviceStatus = "offline"
	DeviceStreaming  DeviceStatus = "streaming"
)

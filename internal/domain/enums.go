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



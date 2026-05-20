package testing

import (
	"errors"
)

// Sentinel errors for the Testing Core module.
// Handlers switch on these to produce the correct HTTP status codes.

var (
	// Device errors
	ErrDeviceNotFound  = errors.New("device not found")
	ErrDeviceCodeTaken = errors.New("device code already registered")
	ErrDeviceInUse     = errors.New("device is currently in use by another test")

	// Test plan errors
	ErrTestPlanNotFound   = errors.New("test plan not found")
	ErrTestPlanNotActive  = errors.New("test plan is not in active status")
	ErrManeuverNotFound   = errors.New("maneuver not found")

	// Test errors
	ErrNoPendingTest       = errors.New("no pending test found for this candidate and test center")
	ErrDuplicatePending    = errors.New("candidate already has a pending test at this test center")
	ErrNoActivePlan        = errors.New("no active test plan found for this level at this test center")
	ErrLevelNotAllowed     = errors.New("this device is not configured for the requested test level")
	ErrBookingWindowClosed = errors.New("device check-in is outside the allowed booking window")
	ErrInvalidLevelCode    = errors.New("test level code does not exist")

	// Recording errors
	ErrRecordingNotFound = errors.New("recording not found for this test")
)

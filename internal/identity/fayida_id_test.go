package identity

import (
	"strings"
	"testing"
	"time"
)

func validRegisterCandidateRequest() RegisterCandidateRequest {
	return RegisterCandidateRequest{
		FirstName: "Abel",
		LastName:  "Test",
		Email:     "candidate@test.et",
		Password:  "Candidate123!",
		Phone:     "911001122",
		FayidaID:  "FAYIDA-1234",
		BirthDate: time.Date(1995, 2, 1, 0, 0, 0, 0, time.UTC),
		Gender:    "male",
	}
}

func TestRegisterCandidateNormalizeLegacyFaydaID(t *testing.T) {
	req := validRegisterCandidateRequest()
	req.FayidaID = ""
	req.FaydaID = "FAYDA-LEGACY-1"

	req.NormalizeFayidaID()

	if req.FayidaID != "FAYDA-LEGACY-1" {
		t.Fatalf("expected FayidaID to be normalized from legacy field, got %q", req.FayidaID)
	}
	if err := req.validate(); err != nil {
		t.Fatalf("expected request to validate after normalization, got error: %v", err)
	}
}

func TestRegisterCandidateValidateRequiresFayidaID(t *testing.T) {
	req := validRegisterCandidateRequest()
	req.FayidaID = ""
	req.FaydaID = ""

	req.NormalizeFayidaID()
	err := req.validate()
	if err == nil {
		t.Fatalf("expected validation error when fayida id is missing")
	}
	if !strings.Contains(err.Error(), "fayida_id is required") {
		t.Fatalf("expected fayida_id error message, got %q", err.Error())
	}
}

func TestAcceptInvitationNormalizeLegacyFaydaID(t *testing.T) {
	req := AcceptInvitationRequest{
		FaydaID: "FAYDA-LEGACY-2",
	}
	req.NormalizeFayidaID()
	if req.FayidaID != "FAYDA-LEGACY-2" {
		t.Fatalf("expected FayidaID to be normalized from legacy field, got %q", req.FayidaID)
	}
}

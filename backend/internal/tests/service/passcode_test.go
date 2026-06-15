package service_test

import (
	"testing"

	"agentroom/backend/internal/service"
)

func TestRoomPasscodeHashDoesNotExposePlaintextAndMatchesTrimmedInput(t *testing.T) {
	hash := service.HashRoomPasscode("  open-sesame  ")
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash == "open-sesame" {
		t.Fatal("hash must not expose the plaintext passcode")
	}
	if !service.RoomPasscodeMatches(hash, "open-sesame") {
		t.Fatal("expected hash to match trimmed passcode")
	}
	if service.RoomPasscodeMatches(hash, "wrong") {
		t.Fatal("expected wrong passcode to fail")
	}
}

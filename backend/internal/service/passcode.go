package service

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func HashRoomPasscode(passcode string) string {
	trimmed := strings.TrimSpace(passcode)
	if trimmed == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(sum[:])
}

func RoomPasscodeMatches(storedHash string, passcode string) bool {
	if storedHash == "" {
		return true
	}
	return storedHash == HashRoomPasscode(passcode)
}

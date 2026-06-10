package hytale

import (
	"fmt"
	"regexp"
)

var (
	uuidRegex     = regexp.MustCompile(`^([0-9a-fA-F]{8})[-]?([0-9a-fA-F]{4})[-]?([0-9a-fA-F]{4})[-]?([0-9a-fA-F]{4})[-]?([0-9a-fA-F]{12})$`)
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,16}$`)
)

// ValidateUUID validates a string as a UUID and normalizes it with dashes.
// It returns the normalized UUID and a boolean indicating if it was valid.
// The caller should still validate against usernameRegex when the input looks like a username.
func ValidateUUID(uuid string) (string, bool) {
	// UUID without dashes is 32 chars, with dashes is 36 chars
	if len(uuid) > 36 {
		return "", false
	}

	matches := uuidRegex.FindStringSubmatch(uuid)
	if matches == nil {
		return "", false
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s", matches[1], matches[2], matches[3], matches[4], matches[5]), true
}

// ValidateUsername validates a string as a Hytale username.
// Returns true if the string matches the username format (3-16 chars, alphanumeric+underscore).
func ValidateUsername(username string) bool {
	return usernameRegex.MatchString(username)
}

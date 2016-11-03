package common

// UserIdentifier represents Slack user ID.
// Both REST API and RTM shares this format.
type UserIdentifier struct {
	ID string
}

// UnmarshalText parses a given slack user id to UserIdentifier
// This method is mainly used by encode/json.
func (identifier *UserIdentifier) UnmarshalText(b []byte) error {
	str := string(b)

	identifier.ID = str

	return nil
}

// MarshalText returns the stringified value of Channel
func (identifier *UserIdentifier) MarshalText() ([]byte, error) {
	return []byte(identifier.ID), nil
}

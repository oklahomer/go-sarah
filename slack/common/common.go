package common

type Channel struct {
	Name string
}

func NewChannel(name string) *Channel {
	return &Channel{
		Name: name,
	}
}

/*
UnmarshalText parses a given slack chanel information to Channel
This method is mainly used by encode/json.
*/
func (channel *Channel) UnmarshalText(b []byte) error {
	str := string(b)

	channel.Name = str

	return nil
}

/*
MarshalText returns the stringified value of Channel
*/
func (channel *Channel) MarshalText() ([]byte, error) {
	return []byte(channel.Name), nil
}

type UserIdentifier struct {
	ID string
}

/*
UnmarshalText parses a given slack user id to UserIdentifier
This method is mainly used by encode/json.
*/
func (identifier *UserIdentifier) UnmarshalText(b []byte) error {
	str := string(b)

	identifier.ID = str

	return nil
}

/*
MarshalText returns the stringified value of Channel
*/
func (identifier *UserIdentifier) MarshalText() ([]byte, error) {
	return []byte(identifier.ID), nil
}

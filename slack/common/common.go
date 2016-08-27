package common

type Channel struct {
	Name string
}

func NewChannel(name string) *Channel {
	return &Channel{
		Name: name,
	}
}

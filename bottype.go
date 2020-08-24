package sarah

// BotType indicates what bot implementation a particular Bot/Plugin is corresponding to.
type BotType string

// String returns a stringified form of BotType
func (botType BotType) String() string {
	return string(botType)
}

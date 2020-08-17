package sarah

// BotType indicates what bot implementation a particular Bot/Plugin is corresponding to.
type BotType string

// String returns a string field form of BotType
func (botType BotType) String() string {
	return string(botType)
}

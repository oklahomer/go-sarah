package sarah

// BotType tells what type of chat service a Bot or a plugin integrates with. e.g. slack, gitter, cli, etc...
// This can be used as a unique ID to distinguish one Bot implementation from another.
type BotType string

// String returns a stringified form of the BotType.
func (botType BotType) String() string {
	return string(botType)
}

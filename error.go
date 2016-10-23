package sarah

/*
BotNonContinuableError represents critical error that Bot can't continue its operation.
When Runner receives this, Runner must stop corresponding Bot.
*/
type BotNonContinuableError struct {
	err string
}

/*
Error returns detailed error about Bot's non-continuable state.
*/
func (e BotNonContinuableError) Error() string {
	return e.err
}

/*
NewBotNonContinuableError creates and return new BotNonContinuableError instance.
*/
func NewBotNonContinuableError(errorContent string) error {
	return &BotNonContinuableError{err: errorContent}
}

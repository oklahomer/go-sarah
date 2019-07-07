package sarah

import (
	"fmt"
)

// BotNonContinuableError represents critical error that Bot can't continue its operation.
// When Runner receives this, it must stop corresponding Bot, and should inform administrator by available mean.
type BotNonContinuableError struct {
	err string
}

// Error returns detailed error about Bot's non-continuable state.
func (e BotNonContinuableError) Error() string {
	return e.err
}

// NewBotNonContinuableError creates and return new BotNonContinuableError instance.
func NewBotNonContinuableError(errorContent string) error {
	return &BotNonContinuableError{err: errorContent}
}

// BlockedInputError indicates incoming input is blocked due to lack of resource.
// Excessive increase in message volume may result in this error.
// When this error occurs, Runner does not wait to enqueue input, but just skip the overflowing message and proceed.
//
// Possible cure includes having more workers and/or more worker queue size,
// but developers MUST aware that this modification may cause more concurrent Command.Execute and Bot.SendMessage operation.
// With that said, increase workers by setting bigger number to worker.Config.WorkerNum to allow more concurrent executions and minimize the delay;
// increase worker queue size by setting bigger number to worker.Config.QueueSize to allow delay and have same concurrent execution number.
type BlockedInputError struct {
	ContinuationCount int
}

// Error returns the detailed error about this blocking situation including the number of continuous occurrence.
// Do err.(*BlockedInputError).ContinuationCount to get the number of continuous occurrence.
// e.g. log if the remainder of this number divided by N is 0 to avoid excessive logging.
func (e BlockedInputError) Error() string {
	return fmt.Sprintf("continuously failed to enqueue input (%d continuation)", e.ContinuationCount)
}

// NewBlockedInputError creates and return new BlockedInputError instance.
func NewBlockedInputError(i int) error {
	return &BlockedInputError{ContinuationCount: i}
}

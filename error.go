package sarah

import (
	"fmt"
)

// BotNonContinuableError represents a critical error that Bot can't continue its operation.
// When Sarah receives this error, she must stop the failing Bot and should inform administrators with Alerter.
type BotNonContinuableError struct {
	err string
}

// Error returns a detailed message about the Bot's non-continuable state.
func (e BotNonContinuableError) Error() string {
	return e.err
}

// NewBotNonContinuableError creates and returns a new BotNonContinuableError instance.
func NewBotNonContinuableError(errorContent string) error {
	return &BotNonContinuableError{err: errorContent}
}

// BlockedInputError indicates the incoming input is blocked due to a lack of worker resources.
// An excessive increase in message volume may result in this error.
// Upon this occurrence, Sarah does not wait until the input can be enqueued, but just skip the overflowing message and proceed with its operation.
//
// Possible cure includes having more workers and/or more worker queue size,
// but developers MUST be aware that this modification may cause more concurrent Command.Execute and Bot.SendMessage operation.
// With that said, developers can set a bigger number to worker.Config.WorkerNum to increase the number of workers and allow more concurrent executions with less delay;
// set a bigger number to worker.Config.QueueSize to allow more delay with the same maximum number of concurrent executions.
type BlockedInputError struct {
	ContinuationCount int
}

// Error returns the detailed message about this blocking situation including the number of continuous occurrences.
// To get the number of continuous occurrences, call err.(*BlockedInputError).ContinuationCount.
// e.g. log if the remainder of this number divided by N is 0 to avoid excessive logging.
func (e BlockedInputError) Error() string {
	return fmt.Sprintf("continuously failed to enqueue input (%d continuation)", e.ContinuationCount)
}

// NewBlockedInputError creates and returns a new BlockedInputError instance.
func NewBlockedInputError(i int) error {
	return &BlockedInputError{ContinuationCount: i}
}

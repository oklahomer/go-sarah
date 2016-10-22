package sarah

import "time"

// Input defines interface that each incoming message must satisfy.
type Input interface {
	SenderKey() string
	Message() string
	SentAt() time.Time
	ReplyTo() OutputDestination
}

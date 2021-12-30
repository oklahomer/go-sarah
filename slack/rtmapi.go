package slack

import (
	"context"
	"fmt"
	"github.com/oklahomer/go-kasumi/logger"
	"github.com/oklahomer/go-kasumi/retry"
	"github.com/oklahomer/go-sarah/v4"
	"github.com/oklahomer/golack/v2/event"
	"github.com/oklahomer/golack/v2/rtmapi"
	"strings"
	"time"
)

const pingSignalChannelID = "ping"

type rtmAPIAdapter struct {
	config        *Config
	client        SlackClient
	handlePayload func(context.Context, *Config, rtmapi.DecodedPayload, func(sarah.Input) error)
}

var _ apiSpecificAdapter = (*rtmAPIAdapter)(nil)

func (r *rtmAPIAdapter) run(ctx context.Context, enqueueInput func(sarah.Input) error, notifyErr func(error)) {
	for {
		conn, err := r.connect(ctx)
		if err != nil {
			// Failed to establish a WebSocket connection with max retrials.
			// Notify the unrecoverable state and give up.
			notifyErr(sarah.NewBotNonContinuableError(err.Error()))
			return
		}

		// Create a connection specific context so each connection-scoped goroutine can receive the connection closing signal and eventually return.
		connCtx, connCancel := context.WithCancel(ctx)

		// This channel is not subject to close. This channel can be accessed in a parallel manner with nonBlockSignal function,
		// and the receiver is NOT waiting for a close signal. Let GC run when this channel is no longer referred.
		//
		// http://stackoverflow.com/a/8593986
		// "Note that it is only necessary to close a channel if the receiver is looking for a close.
		// Closing the channel is a control signal on the channel indicating that no more data follows."
		tryPing := make(chan struct{}, 1)

		go r.receivePayload(connCtx, conn, tryPing, enqueueInput)

		// Payload reception and other connection-related tasks must run in separate goroutines since receivePayload function
		// internally blocks till the per-connection context is cancelled.
		connErr := r.superviseConnection(connCtx, conn, tryPing)

		// superviseConnection returns when parent context is canceled or the connection is hopelessly unstable.
		// Close the current connection and do some cleanup.
		_ = conn.Close()
		connCancel()
		if connErr == nil {
			// Connection is intentionally closed by the caller.
			// No more interaction follows.
			return
		}

		logger.Errorf("Will try re-connection due to previous connection's fatal state: %+v", connErr)
	}
}

func (r *rtmAPIAdapter) connect(ctx context.Context) (rtmapi.Connection, error) {
	var conn rtmapi.Connection
	err := retry.WithPolicy(r.config.RetryPolicy, func() (e error) {
		conn, e = r.client.ConnectRTM(ctx)
		return e
	})
	return conn, err
}

func (r *rtmAPIAdapter) receivePayload(connCtx context.Context, payloadReceiver rtmapi.PayloadReceiver, tryPing chan<- struct{}, enqueueInput func(sarah.Input) error) {
	for {
		select {
		case <-connCtx.Done():
			logger.Info("Stop receiving payload due to context cancel")
			return

		default:
			payload, err := payloadReceiver.Receive()
			// TODO should io.EOF and io.ErrUnexpectedEOF treated differently than other errors?
			if err == event.ErrEmptyPayload {
				continue
			}

			switch err.(type) {
			case nil:
				// O.K. Do nothing and proceed to the payload handling

			case *event.MalformedPayloadError:
				logger.Warnf("Ignore malformed payload: %+v", err)
				continue

			case *rtmapi.UnexpectedMessageTypeError:
				logger.Warnf("Ignore a payload with unexpected message type: %+v", err)
				continue

			default:
				// Connection might not be stable or is closed already.
				logger.Infof("Try ping caused by error: %+v", err)
				nonBlockSignal(pingSignalChannelID, tryPing)
				continue
			}

			if payload == nil {
				continue
			}

			r.handlePayload(connCtx, r.config, payload, enqueueInput)
		}
	}
}

func (r *rtmAPIAdapter) superviseConnection(connCtx context.Context, payloadSender rtmapi.PayloadSender, tryPing chan struct{}) error {
	ticker := time.NewTicker(r.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-connCtx.Done():
			return nil

		case <-ticker.C:
			nonBlockSignal(pingSignalChannelID, tryPing)

		case <-tryPing:
			logger.Debug("Send ping")
			err := payloadSender.Ping()
			if err != nil {
				return fmt.Errorf("error on ping: %w", err)
			}
		}
	}
}

// DefaultRTMPayloadHandler receives incoming events, converts them to sarah.Input, and then passes them to enqueueInput.
// To replace this default behavior, define a function with the same signature and replace this.
//
//   myHandler := func(_ context.Context, config *Config, _ rtmapi.DecodedPayload, _ func(sarah.Input) error) {}
//   slackAdapter, _ := slack.NewAdapter(slackConfig, slack.WithRTMPayloadHandler(myHandler))
func DefaultRTMPayloadHandler(_ context.Context, config *Config, payload rtmapi.DecodedPayload, enqueueInput func(sarah.Input) error) {
	switch p := payload.(type) {
	case *rtmapi.OKReply:
		logger.Debugf("Successfully sent. ID: %d. Text: %s.", p.ReplyTo, p.Text)

	case *rtmapi.NGReply:
		logger.Errorf(
			"Something was wrong with previous message sending. id: %d. error code: %d. error message: %s.",
			p.ReplyTo, p.Error.Code, p.Error.Message)

	case *rtmapi.Pong:
		logger.Debug("Pong message received.")

	case *event.Hello:
		logger.Debugf("Successfully connected.")

	default:
		input, err := EventToInput(p)
		if err == ErrNonSupportedEvent {
			logger.Debugf("Event given, but no corresponding action is defined. %#v", payload)
			return
		}

		if err != nil {
			logger.Errorf("Failed to convert %T event: %s", p, err.Error())
			return
		}

		trimmed := strings.TrimSpace(input.Message())
		if config.HelpCommand != "" && trimmed == config.HelpCommand {
			// Help command
			help := sarah.NewHelpInput(input)
			_ = enqueueInput(help)
		} else if config.AbortCommand != "" && trimmed == config.AbortCommand {
			// Abort command
			abort := sarah.NewAbortInput(input)
			_ = enqueueInput(abort)
		} else {
			// Regular input
			_ = enqueueInput(input)
		}
	}
}

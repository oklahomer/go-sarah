package slack

import (
	"context"
	"github.com/oklahomer/go-sarah/v2"
	"github.com/oklahomer/go-sarah/v2/log"
	"github.com/oklahomer/go-sarah/v2/retry"
	"github.com/oklahomer/golack/v2/event"
	"github.com/oklahomer/golack/v2/rtmapi"
	"golang.org/x/xerrors"
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
			// Failed to establish WebSocket connection with max retrials.
			// Notify the unrecoverable state and give up.
			notifyErr(sarah.NewBotNonContinuableError(err.Error()))
			return
		}

		// Create connection specific context so each connection-scoped goroutine can receive connection closing message and eventually return.
		connCtx, connCancel := context.WithCancel(ctx)

		// This channel is not subject to close. This channel can be accessed in parallel manner with nonBlockSignal(),
		// and the receiver is NOT looking for close signal. Let GC run when this channel is no longer referred.
		//
		// http://stackoverflow.com/a/8593986
		// "Note that it is only necessary to close a channel if the receiver is looking for a close.
		// Closing the channel is a control signal on the channel indicating that no more data follows."
		tryPing := make(chan struct{}, 1)

		go r.receivePayload(connCtx, conn, tryPing, enqueueInput)

		// payload reception and other connection-related tasks must run in separate goroutines since receivePayload()
		// internally blocks til entire payload is being read and iterates it over and over.
		connErr := r.superviseConnection(connCtx, conn, tryPing)

		// superviseConnection returns when parent context is canceled or connection is hopelessly unstable
		// close current connection and do some cleanup
		_ = conn.Close()
		connCancel()
		if connErr == nil {
			// Connection is intentionally closed by caller.
			// No more interaction follows.
			return
		}

		log.Errorf("Will try re-connection due to previous connection's fatal state: %+v", connErr)
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
			log.Info("Stop receiving payload due to context cancel")
			return

		default:
			payload, err := payloadReceiver.Receive()
			// TODO should io.EOF and io.ErrUnexpectedEOF treated differently than other errors?
			if err == event.ErrEmptyPayload {
				continue
			} else if _, ok := err.(*event.MalformedPayloadError); ok {
				// Malformed payload was passed, but there is no programmable way to handle this error.
				// Leave log and proceed.
				log.Warnf("Ignore malformed payload: %+v", err)
			} else if _, ok := err.(*rtmapi.UnexpectedMessageTypeError); ok {
				log.Warnf("Ignore a payload with unexpected message type: %+v", err)
			} else if err != nil {
				// Connection might not be stable or is closed already.
				log.Debugf("Ping caused by error: %+v", err)
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
			log.Debug("Send ping")
			err := payloadSender.Ping()
			if err != nil {
				return xerrors.Errorf("error on ping: %w", err)
			}
		}
	}
}

// DefaultRTMPayloadHandler receives incoming events, convert them to sarah.Input and then pass them to enqueueInput.
// To replace this default behavior, define a function with the same signature and replace this.
//
//   myHandler := func(_ context.Context, config *Config, _ rtmapi.DecodedPayload, _ func(sarah.Input) error)
//   slackAdapter, _ := slack.NewAdapter(slackConfig, slack.WithRTMPayloadHandler(myHandler))
func DefaultRTMPayloadHandler(_ context.Context, config *Config, payload rtmapi.DecodedPayload, enqueueInput func(sarah.Input) error) {
	switch p := payload.(type) {
	case *rtmapi.OKReply:
		log.Debugf("Successfully sent. ID: %d. Text: %s.", p.ReplyTo, p.Text)

	case *rtmapi.NGReply:
		log.Errorf(
			"Something was wrong with previous message sending. id: %d. error code: %d. error message: %s.",
			p.ReplyTo, p.Error.Code, p.Error.Message)

	case *rtmapi.Pong:
		log.Debug("Pong message received.")

	case *event.Hello:
		log.Debugf("Successfully connected.")

	default:
		input, err := EventToInput(p)
		if err == ErrNonSupportedEvent {
			log.Debugf("Event given, but no corresponding action is defined. %#v", payload)
			return
		}

		if err != nil {
			log.Errorf("Failed to convert %T event: %s", p, err.Error())
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

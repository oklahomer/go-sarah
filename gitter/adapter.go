package gitter

import (
	"context"
	"errors"
	"fmt"
	"github.com/oklahomer/go-kasumi/logger"
	"github.com/oklahomer/go-kasumi/retry"
	"github.com/oklahomer/go-sarah/v4"
)

const (
	// GITTER is a dedicated sarah.BotType for Gitter integration.
	GITTER sarah.BotType = "gitter"
)

// AdapterOption defines a function's signature that Adapter's functional options must satisfy.
type AdapterOption func(adapter *Adapter)

// Adapter is a sarah.Adapter implementation for Gitter.
// This holds REST/Streaming API clients' instances.
type Adapter struct {
	config          *Config
	apiClient       APIClient
	streamingClient StreamingClient
}

var _ sarah.Adapter = (*Adapter)(nil)

// NewAdapter creates and returns a new Adapter instance.
func NewAdapter(config *Config, options ...AdapterOption) (*Adapter, error) {
	adapter := &Adapter{
		config:          config,
		apiClient:       NewRestAPIClient(config.Token),
		streamingClient: NewStreamingAPIClient(config.Token),
	}

	for _, opt := range options {
		opt(adapter)
	}

	return adapter, nil
}

// BotType returns a designated BotType for Gitter integration.
func (adapter *Adapter) BotType() sarah.BotType {
	return GITTER
}

// Run fetches all belonging Room information and connects to them.
// New goroutines are activated for each Room to connect, and the interactions run in a concurrent manner.
func (adapter *Adapter) Run(ctx context.Context, enqueueInput func(sarah.Input) error, notifyErr func(error)) {
	// Get belonging rooms.
	var rooms *Rooms
	err := retry.WithPolicy(adapter.config.RetryPolicy, func() (e error) {
		rooms, e = adapter.apiClient.Rooms(ctx)
		return e
	})
	if err != nil {
		notifyErr(sarah.NewBotNonContinuableError(err.Error()))
		return
	}

	// Connect to each room.
	for _, room := range *rooms {
		go adapter.runEachRoom(ctx, room, enqueueInput)
	}
}

// SendMessage lets sarah.Bot send a message to Gitter.
func (adapter *Adapter) SendMessage(ctx context.Context, output sarah.Output) {
	switch content := output.Content().(type) {
	case string:
		room, ok := output.Destination().(*Room)
		if !ok {
			logger.Errorf("Destination is not instance of Room. %#v.", output.Destination())
			return
		}
		_, err := adapter.apiClient.PostMessage(ctx, room, content)
		logger.Errorf("Failed posting message to %s: %+v", room.ID, err)

	default:
		logger.Warnf("Unexpected output %#v", output)

	}
}

func (adapter *Adapter) runEachRoom(ctx context.Context, room *Room, enqueueInput func(sarah.Input) error) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
			logger.Infof("Connecting to room: %s", room.ID)

			var conn Connection
			err := retry.WithPolicy(adapter.config.RetryPolicy, func() (e error) {
				conn, e = adapter.streamingClient.Connect(ctx, room)
				return e
			})
			if err != nil {
				logger.Warnf("Could not connect to room: %s. Error: %+v", room.ID, err)
				return
			}

			connErr := receiveMessageRecursive(conn, enqueueInput)
			_ = conn.Close()

			// TODO: Intentional connection close such as context.cancel also comes here.
			// It would be nice if we could detect such an event to distinguish an intentional behaviour and an unintentional connection error.
			// But, the truth is, the given error is just a privately defined error instance given by http package:
			//   var errRequestCanceled = errors.New("net/http: request canceled")
			// For now, let an error log appear and proceed to the next loop, select case with ctx.Done() will eventually return.
			logger.Errorf("Disconnected from room %s: %+v", room.ID, connErr)

		}
	}
}

func receiveMessageRecursive(messageReceiver MessageReceiver, enqueueInput func(sarah.Input) error) error {
	logger.Infof("Start receiving message")
	for {
		message, err := messageReceiver.Receive()

		var malformedErr *MalformedPayloadError
		if errors.Is(err, ErrEmptyPayload) {
			// https://developer.gitter.im/docs/streaming-api
			// Parsers must be tolerant of extra newline characters occasionally placed in between messages.
			// These characters are sent as periodic "keep-alive" messages to tell clients and NAT firewalls
			// that the connection is still alive during low message volume periods.
			continue

		} else if errors.As(err, &malformedErr) {
			logger.Warnf("Skipping malformed input: %+v", err)
			continue

		} else if err != nil {
			// At this point, assume the connection is unstable or is closed.
			// Let the caller proceed to reconnect or quit.
			return fmt.Errorf("failed to receive input: %w", err)

		}

		_ = enqueueInput(message)
	}
}

// NewResponse creates *sarah.CommandResponse with the given arguments.
func NewResponse(content string, options ...RespOption) (*sarah.CommandResponse, error) {
	stash := &respOptions{
		userContext: nil,
	}

	for _, opt := range options {
		opt(stash)
	}

	return &sarah.CommandResponse{
		Content:     content,
		UserContext: stash.userContext,
	}, nil
}

// RespWithNext sets a given fnc as part of the response's *sarah.UserContext.
// The next input from the same user will be passed to this fnc.
// sarah.UserContextStorage must be configured or otherwise, the function will be ignored.
func RespWithNext(fnc sarah.ContextualFunc) RespOption {
	return func(options *respOptions) {
		options.userContext = &sarah.UserContext{
			Next: fnc,
		}
	}
}

// RespWithNextSerializable sets the given arg as part of the response's *sarah.UserContext.
// The next input from the same user will be passed to the function defined in the arg.
// sarah.UserContextStorage must be configured or otherwise, the function will be ignored.
func RespWithNextSerializable(arg *sarah.SerializableArgument) RespOption {
	return func(options *respOptions) {
		options.userContext = &sarah.UserContext{
			Serializable: arg,
		}
	}
}

// RespOption defines a function's signature that NewResponse's functional option must satisfy.
type RespOption func(*respOptions)

type respOptions struct {
	userContext *sarah.UserContext
}

// APIClient is an interface that a Rest API client must satisfy.
// This is mainly defined to ease tests.
type APIClient interface {
	// Rooms fetch the list of rooms the token's owner belongs.
	Rooms(context.Context) (*Rooms, error)

	// PostMessage sends message to a given Room.
	PostMessage(context.Context, *Room, string) (*Message, error)
}

// StreamingClient is an interface that an HTTP Streaming client must satisfy.
// This is mainly defined to ease tests.
type StreamingClient interface {
	Connect(context.Context, *Room) (Connection, error)
}

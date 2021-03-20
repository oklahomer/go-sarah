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
	// GITTER is a dedicated BotType for gitter implementation.
	GITTER sarah.BotType = "gitter"
)

// AdapterOption defines function signature that Adapter's functional option must satisfy.
type AdapterOption func(adapter *Adapter)

// Adapter stores REST/Streaming API clients' instances to let users interact with gitter.
type Adapter struct {
	config          *Config
	apiClient       APIClient
	streamingClient StreamingClient
}

// NewAdapter creates and returns new Adapter instance.
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

// BotType returns gitter designated BotType.
func (adapter *Adapter) BotType() sarah.BotType {
	return GITTER
}

// Run fetches all belonging Room and connects to them.
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

// SendMessage let Bot send message to gitter.
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
			// It would be nice if we could detect such event to distinguish intentional behaviour and unintentional connection error.
			// But, the truth is, given error is just a privately defined error instance given by http package.
			// var errRequestCanceled = errors.New("net/http: request canceled")
			// For now, let error log appear and proceed to next loop, select case with ctx.Done() will eventually return.
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
			// Parsers must be tolerant of occasional extra newline characters placed between messages.
			// These characters are sent as periodic "keep-alive" messages to tell clients and NAT firewalls
			// that the connection is still alive during low message volume periods.
			continue

		} else if errors.As(err, &malformedErr) {
			logger.Warnf("Skipping malformed input: %+v", err)
			continue

		} else if err != nil {
			// At this point, assume connection is unstable or is closed.
			// Let caller proceed to reconnect or quit.
			return fmt.Errorf("failed to receive input: %w", err)

		}

		_ = enqueueInput(message)
	}
}

// NewResponse creates *sarah.CommandResponse with given arguments.
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

// RespWithNext sets given fnc as part of the response's *sarah.UserContext.
// The next input from the same user will be passed to this fnc.
// See sarah.UserContextStorage must be present or otherwise, fnc will be ignored.
func RespWithNext(fnc sarah.ContextualFunc) RespOption {
	return func(options *respOptions) {
		options.userContext = &sarah.UserContext{
			Next: fnc,
		}
	}
}

// RespWithNextSerializable sets given arg as part of the response's *sarah.UserContext.
// The next input from the same user will be passed to the function defined in the arg.
// See sarah.UserContextStorage must be present or otherwise, arg will be ignored.
func RespWithNextSerializable(arg *sarah.SerializableArgument) RespOption {
	return func(options *respOptions) {
		options.userContext = &sarah.UserContext{
			Serializable: arg,
		}
	}
}

// RespOption defines function signature that NewResponse's functional option must satisfy.
type RespOption func(*respOptions)

type respOptions struct {
	userContext *sarah.UserContext
}

// APIClient is an interface that Rest API client must satisfy.
// This is mainly defined to ease tests.
type APIClient interface {
	Rooms(context.Context) (*Rooms, error)
	PostMessage(context.Context, *Room, string) (*Message, error)
}

// StreamingClient is an interface that HTTP Streaming client must satisfy.
// This is mainly defined to ease tests.
type StreamingClient interface {
	Connect(context.Context, *Room) (Connection, error)
}

package gitter

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/retry"
	"golang.org/x/net/context"
	"time"
)

const (
	// GITTER is a dedicated BotType for gitter implementation.
	GITTER sarah.BotType = "gitter"
)

// Adapter stores REST/Streaming API clients' instances to let users interact with gitter.
type Adapter struct {
	config             *Config
	restAPIClient      *RestAPIClient
	streamingAPIClient *StreamingAPIClient
}

// NewAdapter creates and returns new Adapter instance.
func NewAdapter(config *Config) *Adapter {
	return &Adapter{
		config:             config,
		restAPIClient:      NewRestAPIClient(config.token),
		streamingAPIClient: NewStreamingAPIClient(config.token),
	}
}

// BotType returns gitter designated BotType.
func (adapter *Adapter) BotType() sarah.BotType {
	return GITTER
}

// Run fetches all belonging Room and connects to them.
func (adapter *Adapter) Run(ctx context.Context, receivedMessage chan<- sarah.Input, errCh chan<- error) {
	// fetch joined rooms
	rooms, err := fetchRooms(ctx, adapter.restAPIClient, adapter.config.retryLimit, adapter.config.retryInterval)
	if err != nil {
		errCh <- sarah.NewBotNonContinuableError(err.Error())
		return
	}

	for _, room := range *rooms {
		go adapter.runEachRoom(ctx, room, receivedMessage)
	}
}

// SendMessage let Bot send message to gitter.
func (adapter *Adapter) SendMessage(ctx context.Context, output sarah.Output) {
	switch content := output.Content().(type) {
	case string:
		room, ok := output.Destination().(*Room)
		if !ok {
			log.Errorf("Destination is not instance of Room. %#v.", output.Destination())
			return
		}
		adapter.restAPIClient.PostMessage(ctx, room, content)
	default:
		log.Warnf("unexpected output %#v", output)
	}
}

func (adapter *Adapter) runEachRoom(ctx context.Context, room *Room, receivedMessage chan<- sarah.Input) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			log.Infof("connecting to room: %s", room.ID)
			conn, err := connectRoom(ctx, adapter.streamingAPIClient, room, adapter.config.retryLimit, adapter.config.retryInterval)
			if err != nil {
				log.Warnf("could not connect to room: %s", room.ID)
				return
			}

			connErr := receiveMessageRecursive(conn, receivedMessage)
			conn.Close()
			if connErr == nil {
				// Connection is intentionally closed by caller.
				// No more interaction follows.
				return
			} else {
				// TODO: Intentional connection close such as context.cancel also comes here.
				// It would be nice if we could detect such event to distinguish intentional behaviour and unintentional connection error.
				// But, the truth is, given error is just a privately defined error instance given by http package.
				// var errRequestCanceled = errors.New("net/http: request canceled")
				// For now, let error log appear and proceed to next loop, select case with ctx.Done() will eventually return.
				log.Error(connErr.Error())
			}
		}
	}
}

func fetchRooms(ctx context.Context, fetcher RoomsFetcher, retrial uint, interval time.Duration) (*Rooms, error) {
	var rooms *Rooms
	err := retry.RetryInterval(retrial, func() error {
		r, e := fetcher.Rooms(ctx)
		rooms = r
		return e
	}, interval)

	return rooms, err
}

func receiveMessageRecursive(messageReceiver MessageReceiver, receivedMessage chan<- sarah.Input) error {
	log.Infof("start receiving message")
	for {
		message, err := messageReceiver.Receive()

		if err == EmptyPayloadError {
			// https://developer.gitter.im/docs/streaming-api
			// Parsers must be tolerant of occasional extra newline characters placed between messages.
			// These characters are sent as periodic "keep-alive" messages to tell clients and NAT firewalls
			// that the connection is still alive during low message volume periods.
			continue
		} else if malformedErr, ok := err.(*MalformedPayloadError); ok {
			log.Warnf("skipping malformed input: %s", malformedErr)
			continue
		} else if err != nil {
			// At this point, assume connection is unstable or is closed.
			// Let caller proceed to reconnect or quit.
			return err
		}

		receivedMessage <- message
	}
}

func connectRoom(ctx context.Context, connector StreamConnector, room *Room, retrial uint, interval time.Duration) (Connection, error) {
	var conn Connection
	err := retry.RetryInterval(retrial, func() error {
		r, e := connector.Connect(ctx, room)
		if e != nil {
			log.Error(e)
		}
		conn = r
		return e
	}, interval)

	return conn, err
}

// NewStringResponse can be used by plugin command to return string response to gitter.
func NewStringResponse(responseContent string) *sarah.CommandResponse {
	return &sarah.CommandResponse{Content: responseContent}
}

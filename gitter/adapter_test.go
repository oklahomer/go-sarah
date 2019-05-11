package gitter

import (
	"errors"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/log"
	"github.com/oklahomer/go-sarah/retry"
	"golang.org/x/net/context"
	"golang.org/x/xerrors"
	"io/ioutil"
	stdLogger "log"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	oldLogger := log.GetLogger()
	defer log.SetLogger(oldLogger)

	l := stdLogger.New(ioutil.Discard, "dummyLog", 0)
	logger := log.NewWithStandardLogger(l)
	log.SetLogger(logger)

	code := m.Run()

	os.Exit(code)
}

type DummyAPIClient struct {
	RoomsFunc       func(context.Context) (*Rooms, error)
	PostMessageFunc func(context.Context, *Room, string) (*Message, error)
}

func (c *DummyAPIClient) Rooms(ctx context.Context) (*Rooms, error) {
	return c.RoomsFunc(ctx)
}

func (c *DummyAPIClient) PostMessage(ctx context.Context, room *Room, message string) (*Message, error) {
	return c.PostMessageFunc(ctx, room, message)
}

type DummyStreamingClient struct {
	ConnectFunc func(context.Context, *Room) (Connection, error)
}

func (sc *DummyStreamingClient) Connect(ctx context.Context, room *Room) (Connection, error) {
	return sc.ConnectFunc(ctx, room)
}

type DummyConnection struct {
	ReceiveFunc func() (*RoomMessage, error)
	CloseFunc   func() error
}

func (c *DummyConnection) Receive() (*RoomMessage, error) {
	return c.ReceiveFunc()
}

func (c *DummyConnection) Close() error {
	return c.CloseFunc()
}

func TestNewAdapter(t *testing.T) {
	config := NewConfig()
	adapter, err := NewAdapter(config)
	if err != nil {
		t.Fatalf("Unexpected error returned: %s.", err.Error())
	}

	if adapter.config != config {
		t.Fatal("Supplied config is not set.")
	}
}

func TestNewAdapter_WithOptionError(t *testing.T) {
	expectedErr := errors.New("dummy")
	adapter, err := NewAdapter(NewConfig(), func(_ *Adapter) error {
		return expectedErr
	})

	if !xerrors.Is(err, expectedErr) {
		t.Errorf("Expected error is not returned: %#v.", err)
	}

	if adapter != nil {
		t.Error("Adapter instance should not be returned on error.")
	}
}

func TestAdapter_BotType(t *testing.T) {
	adapter := &Adapter{}

	if adapter.BotType() != GITTER {
		t.Errorf("Unexpected BotType is returned: %s.", adapter.BotType())
	}
}

func Test_receiveMessageRecursive(t *testing.T) {
	type value struct {
		message *RoomMessage
		err     error
	}
	values := []value{
		{
			message: &RoomMessage{},
			err:     nil,
		},
		{
			message: nil,
			err:     ErrEmptyPayload,
		},
		{
			message: nil,
			err:     &MalformedPayloadError{},
		},
		{
			message: nil,
			err:     errors.New("random error"),
		},
	}

	conn := &DummyConnection{
		ReceiveFunc: func() (*RoomMessage, error) {
			var v value
			v, values = values[0], values[1:]
			return v.message, v.err
		},
	}
	enqueueCnt := 0
	enqueuer := func(_ sarah.Input) error {
		enqueueCnt++
		return nil
	}
	_ = receiveMessageRecursive(conn, enqueuer)

	if enqueueCnt != 1 {
		t.Errorf("Enqueued %d times. Should enqueue only if no error is returned.", enqueueCnt)
	}
}

func TestAdapter_runEachRoom(t *testing.T) {
	var retryLimit uint = 1
	connect := make(chan struct{}, retryLimit)
	closed := make(chan struct{})
	adapter := &Adapter{
		streamingClient: &DummyStreamingClient{
			ConnectFunc: func(_ context.Context, _ *Room) (Connection, error) {
				connect <- struct{}{}
				return &DummyConnection{
					ReceiveFunc: func() (*RoomMessage, error) {
						return &RoomMessage{}, nil
					},
					CloseFunc: func() error {
						closed <- struct{}{}
						return nil
					},
				}, nil
			},
		},
		config: &Config{
			RetryPolicy: &retry.Policy{
				Trial: 1,
			},
		},
	}

	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)

	room := &Room{
		ID: "testID",
	}
	go adapter.runEachRoom(ctx, room, func(_ sarah.Input) error { return nil })

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-connect:
		// O.K.
	case <-time.NewTimer(10 * time.Second).C:
		t.Fatal("APIClient.Connect is not called.")
	}

	select {
	case <-closed:
		t.Fatalf("Connection.Close should never called while connection is stable.")
	case <-time.NewTimer(10 * time.Millisecond).C:
		// O.K.
	}
}

func TestAdapter_runEachRoom_ConnectionInitializationError(t *testing.T) {
	var retryLimit uint = 1
	connect := make(chan struct{}, retryLimit)
	adapter := &Adapter{
		streamingClient: &DummyStreamingClient{
			ConnectFunc: func(_ context.Context, _ *Room) (Connection, error) {
				connect <- struct{}{}
				return nil, errors.New("connection error")
			},
		},
		config: &Config{
			RetryPolicy: &retry.Policy{
				Trial: 1,
			},
		},
	}

	queue := make(chan struct{})
	enqueuer := func(_ sarah.Input) error {
		queue <- struct{}{}
		return nil
	}
	room := &Room{
		ID: "testID",
	}
	adapter.runEachRoom(context.TODO(), room, enqueuer) // No goroutine. Will end automatically.

	select {
	case <-queue:
		t.Fatalf("No message should be enqueued when connection is anavailable.")
	case <-time.NewTimer(10 * time.Millisecond).C:
		// O.K.
	}
}

func TestAdapter_runEachRoom_ConnectionError(t *testing.T) {
	connect := make(chan struct{}, 1)
	closed := make(chan struct{}, 1)
	adapter := &Adapter{
		streamingClient: &DummyStreamingClient{
			ConnectFunc: func(_ context.Context, _ *Room) (Connection, error) {
				select {
				case connect <- struct{}{}:
					// O.K.
				default:
					// Only the first one is wanted.
				}

				return &DummyConnection{
					ReceiveFunc: func() (*RoomMessage, error) {
						return nil, errors.New("message reception error")
					},
					CloseFunc: func() error {
						select {
						case closed <- struct{}{}:
							// O.K.
						default:
							// Only the first one is wanted.
						}

						return nil
					},
				}, nil
			},
		},
		config: &Config{
			RetryPolicy: &retry.Policy{
				Trial: 1,
			},
		},
	}

	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)
	queue := make(chan struct{})
	enqueuer := func(_ sarah.Input) error {
		queue <- struct{}{}
		return nil
	}
	room := &Room{
		ID: "testID",
	}
	go adapter.runEachRoom(ctx, room, enqueuer) // No goroutine. Will end automatically.

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-connect:
		// O.K.
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("StreamingAPIClient.Connect is not called.")
	}

	select {
	case <-closed:
		// O.K.
	case <-time.NewTimer(10 * time.Second).C:
		t.Error("StreamingAPIClient.Close is not called.")
	}
}

func TestAdapter_Run(t *testing.T) {
	givenRoom := make(chan string)
	roomID := "dummy"
	adapter := &Adapter{
		config: &Config{
			RetryPolicy: &retry.Policy{
				Trial: 1,
			},
		},
		apiClient: &DummyAPIClient{
			RoomsFunc: func(_ context.Context) (*Rooms, error) {
				return &Rooms{
					{
						ID: roomID,
					},
				}, nil
			},
		},
		streamingClient: &DummyStreamingClient{
			ConnectFunc: func(_ context.Context, room *Room) (Connection, error) {
				givenRoom <- room.ID
				return nil, errors.New("to be ignored")
			},
		},
	}

	adapter.Run(context.TODO(), func(sarah.Input) error { return nil }, func(error) {})
	time.Sleep(100 * time.Millisecond)

	select {
	case id := <-givenRoom:
		if id != roomID {
			t.Fatalf("Expected roomID is not given: %s", id)
		}
	case <-time.NewTimer(10 * time.Second).C:
		t.Fatal("StreamingAPIClient.Connect is not called.")
	}
}

func TestAdapter_Run_RestAPIClientRoomsError(t *testing.T) {
	adapter := &Adapter{
		config: &Config{
			RetryPolicy: &retry.Policy{
				Trial: 1,
			},
		},
		apiClient: &DummyAPIClient{
			RoomsFunc: func(_ context.Context) (*Rooms, error) {
				return nil, errors.New("room fetch error")
			},
		},
	}

	var err error
	notifyErr := func(e error) {
		err = e
	}
	adapter.Run(context.TODO(), func(sarah.Input) error { return nil }, notifyErr)

	if _, ok := err.(*sarah.BotNonContinuableError); !ok {
		t.Fatalf("Expected error is not returned: %#v.", err)
	}
}

func TestAdapter_SendMessage(t *testing.T) {
	called := false
	adapter := &Adapter{
		apiClient: &DummyAPIClient{
			PostMessageFunc: func(_ context.Context, _ *Room, _ string) (*Message, error) {
				called = true
				return nil, nil
			},
		},
	}
	output := sarah.NewOutputMessage(&Room{}, "text")

	adapter.SendMessage(context.TODO(), output)

	if !called {
		t.Error("APIClient.PostMessage is not called.")
	}
}

func TestAdapter_SendMessage_InvalidDestinationError(t *testing.T) {
	called := false
	adapter := &Adapter{
		apiClient: &DummyAPIClient{
			PostMessageFunc: func(_ context.Context, _ *Room, _ string) (*Message, error) {
				called = true
				return nil, nil
			},
		},
	}
	output := sarah.NewOutputMessage("invalid", "text")

	adapter.SendMessage(context.TODO(), output)

	if called {
		t.Error("APIClient.PostMessage is called with invalid destination.")
	}
}

func TestAdapter_SendMessage_InvalidContentTypeError(t *testing.T) {
	called := false
	adapter := &Adapter{
		apiClient: &DummyAPIClient{
			PostMessageFunc: func(_ context.Context, _ *Room, _ string) (*Message, error) {
				called = true
				return nil, nil
			},
		},
	}
	output := sarah.NewOutputMessage(&Room{}, 123)

	adapter.SendMessage(context.TODO(), output)

	if called {
		t.Error("APIClient.PostMessage is called with invalid content type.")
	}
}

func TestNewStringResponse(t *testing.T) {
	str := "abc"
	res := NewStringResponse(str)

	if res.Content != str {
		t.Errorf("Expected content is not returned: %s.", res.Content)
	}

	if res.UserContext != nil {
		t.Errorf("UserContext should not be returned: %#v.", res.UserContext)
	}
}

func TestNewStringResponseWithNext(t *testing.T) {
	str := "abc"
	next := func(_ context.Context, _ sarah.Input) (*sarah.CommandResponse, error) {
		return nil, nil
	}
	res := NewStringResponseWithNext(str, next)

	if res.Content != str {
		t.Errorf("Expected content is not returned: %s.", res.Content)
	}

	if res.UserContext == nil {
		t.Fatal("Expected UserContxt is not stored.")
	}

	if reflect.ValueOf(res.UserContext.Next).Pointer() != reflect.ValueOf(next).Pointer() {
		t.Fatalf("Expected next step is not returned: %T.", res.UserContext.Next)
	}
}

package slack

import (
	"context"
	"errors"
	"github.com/oklahomer/go-kasumi/retry"
	"github.com/oklahomer/go-sarah/v4"
	"github.com/oklahomer/golack/v2/event"
	"github.com/oklahomer/golack/v2/rtmapi"
	"reflect"
	"testing"
	"time"
)

type DummyConnection struct {
	ReceiveFunc func() (rtmapi.DecodedPayload, error)
	SendFunc    func(message *rtmapi.OutgoingMessage) error
	PingFunc    func() error
	CloseFunc   func() error
}

func (conn *DummyConnection) Receive() (rtmapi.DecodedPayload, error) {
	return conn.ReceiveFunc()
}

func (conn *DummyConnection) Send(message *rtmapi.OutgoingMessage) error {
	return conn.SendFunc(message)
}

func (conn *DummyConnection) Ping() error {
	return conn.PingFunc()
}

func (conn *DummyConnection) Close() error {
	return conn.CloseFunc()
}

func Test_rtmAPIAdapter_run(t *testing.T) {
	t.Run("Successful case", func(t *testing.T) {
		// Prepare an adapter that always success to establish a connection.
		closed := make(chan struct{}, 1)
		r := &rtmAPIAdapter{
			config: &Config{
				PingInterval: 30 * time.Second,
				RetryPolicy: &retry.Policy{
					Trial: 1,
				},
			},
			client: &DummyClient{
				ConnectRTMFunc: func(_ context.Context) (rtmapi.Connection, error) {
					return &DummyConnection{
						PingFunc: func() error {
							return nil
						},
						ReceiveFunc: func() (rtmapi.DecodedPayload, error) {
							return nil, nil
						},
						CloseFunc: func() error {
							closed <- struct{}{}
							return nil
						},
					}, nil
				},
			},
		}

		// Run in background
		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go r.run(ctx, func(_ sarah.Input) error { return nil }, func(err error) { errCh <- err })

		// Context cancellation should stop adapter and its internal goroutines.
		cancel()

		// Context cancellation should not cause an error state.
		select {
		case err := <-errCh:
			t.Errorf("Unexlected error is returned: %s", err.Error())

		case <-time.NewTimer(10 * time.Millisecond).C:
			// O.K.
		}

		// Connection.Close must be called when context is canceled.
		select {
		case <-closed:
		// O.K.

		case <-time.NewTimer(10 * time.Millisecond).C:
			t.Error("Connection close is not called even though the parent context is canceled.")

		}
	})

	t.Run("Connection error", func(t *testing.T) {
		// Prepare an adapter that always fails to establish a connection.
		r := &rtmAPIAdapter{
			config: &Config{
				PingInterval: 30 * time.Second,
				RetryPolicy: &retry.Policy{
					Trial: 1,
				},
			},
			client: &DummyClient{
				ConnectRTMFunc: func(_ context.Context) (rtmapi.Connection, error) {
					return nil, errors.New("ERROR")
				},
			},
		}

		// Run in background
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errCh := make(chan error, 1)
		go r.run(ctx, func(_ sarah.Input) error { return nil }, func(err error) { errCh <- err })

		// Connection error should return sarah.BotNonContinuableError.
		select {
		case err := <-errCh:
			var target *sarah.BotNonContinuableError
			if !errors.As(err, &target) {
				t.Errorf("Expected error is not returned: %#v", err)
			}

		case <-time.NewTimer(10 * time.Second).C:
			t.Error("Connection close is not called even though the parent context is canceled.")

		}
	})
}

func Test_rtmAPIAdapter_connect(t *testing.T) {
	t.Run("Successful case", func(t *testing.T) {
		// Prepare an apiSpecificAdapter with a client that provides dummy connection instance.
		r := &rtmAPIAdapter{
			config: &Config{
				RetryPolicy: &retry.Policy{
					Trial: 1,
				},
			},
			client: &DummyClient{
				ConnectRTMFunc: func(_ context.Context) (rtmapi.Connection, error) {
					return &DummyConnection{}, nil
				},
			},
		}

		// Try connect with the prepared apiSpecificAdapter.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		conn, err := r.connect(ctx)

		// See if successfully connected.
		if err != nil {
			t.Fatalf("Unexpected error is returned: %s.", err.Error())
		}
		if conn == nil {
			t.Error("Connection is not returned.")
		}
	})

	t.Run("Connection error", func(t *testing.T) {
		// Prepare and apiSpecificAdapter that try connecting up to 3 times but fails every time.
		expectedErr := errors.New("expectedErr error")
		r := &rtmAPIAdapter{
			config: &Config{
				RetryPolicy: &retry.Policy{
					Trial: 3,
				},
			},
			client: &DummyClient{
				ConnectRTMFunc: func(_ context.Context) (rtmapi.Connection, error) {
					return nil, expectedErr
				},
			},
		}

		// Try connect with the prepared apiSpecificAdapter.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_, err := r.connect(ctx)

		// See if expected error is returned.
		if err == nil {
			t.Fatal("Expected error is not returned.")
		}
		typedErr, ok := err.(*retry.Errors)
		if !ok {
			t.Fatalf("Returned error is not *retry.Errors: %T", err)
		}
		if len(*typedErr) != 3 {
			t.Errorf("The number of errors is not equal to that of retrials: %d", len(*typedErr))
		}
	})
}

func Test_rtmAPIAdapter_receivePayload(t *testing.T) {
	t.Run("Successful case", func(t *testing.T) {
		// Prepare an apiSpecificAdapter that notifies payload reception event.
		payloadGiven := make(chan struct{})
		r := &rtmAPIAdapter{
			handlePayload: func(_ context.Context, _ *Config, _ rtmapi.DecodedPayload, _ func(sarah.Input) error) {
				// Notify that a payload is given.
				payloadGiven <- struct{}{}
			},
		}

		// Prepare a dummy connection that receives empty struct as a payload.
		dummyPayload := struct{}{}
		conn := &DummyConnection{
			ReceiveFunc: func() (rtmapi.DecodedPayload, error) {
				return dummyPayload, nil
			},
		}

		// Run payload reception function in background.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go r.receivePayload(ctx, conn, make(chan struct{}), func(_ sarah.Input) error { return nil })

		// Check payload reception.
		select {
		case <-payloadGiven:
			// Payload is passed successfully. O.K.

		case <-time.NewTimer(10 * time.Second).C:
			t.Error("PayloadHandler is not called.")
		}
	})

	t.Run("Reception error", func(t *testing.T) {
		// Prepare an apiSpecificAdapter that never receives a payload.
		r := &rtmAPIAdapter{
			handlePayload: func(_ context.Context, _ *Config, _ rtmapi.DecodedPayload, _ func(sarah.Input) error) {
				t.Fatal("PayloadHandler should not be called.")
			},
		}

		// List up all possible errors
		errs := []error{
			event.ErrEmptyPayload,
			event.NewMalformedPayloadError("dummy"),
			&rtmapi.UnexpectedMessageTypeError{},
			errors.New("random error"),
		}
		conn := &DummyConnection{
			ReceiveFunc: func() (rtmapi.DecodedPayload, error) {
				if len(errs) == 0 {
					return nil, nil
				}
				e := errs[0]
				errs = errs[1:]
				return nil, e
			},
		}

		// Run payload reception function in background.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go r.receivePayload(ctx, conn, make(chan struct{}), func(_ sarah.Input) error { return nil })

		// Give long enough time to receive all errors.
		time.Sleep(100 * time.Millisecond)
	})
}

func Test_rtmAPIAdapter_superviseConnection(t *testing.T) {
	t.Run("Successful case", func(t *testing.T) {
		// Prepare a connection that never returns error state.
		pingCalled := make(chan struct{}, 1)
		conn := &DummyConnection{
			PingFunc: func() error {
				// Notify Ping() call via signal channel.
				select {
				case pingCalled <- struct{}{}:
					return nil

				default:
					// The signal channel is blocked due to duplicate calls. Just ignore.
					return nil
				}
			},
		}

		// Prepare and apiSpecificAdapter that pings every 10 milliseconds.
		pingInterval := 10 * time.Millisecond
		r := &rtmAPIAdapter{
			config: &Config{
				PingInterval: pingInterval,
			},
			client:        nil,
			handlePayload: nil,
		}

		// Run connection supervising function in background.
		ctx, cancel := context.WithCancel(context.Background())
		conErr := make(chan error)
		go func() {
			err := r.superviseConnection(ctx, conn, make(chan struct{}, 1))
			conErr <- err
		}()

		// Give long enough time to have at least one ping event before context cancellation.
		time.Sleep(pingInterval + 10*time.Millisecond)
		cancel()

		// Ensure Ping is sent.
		select {
		case <-pingCalled:
			// Ping() was called. O.K.

		case <-time.NewTimer(10 * time.Second).C:
			t.Error("Connection.Ping was not called.")
		}

		// The supervising function should return nil on context cancellation.
		select {
		case err := <-conErr:
			if err != nil {
				t.Errorf("Unexpected error was returned: %s.", err.Error())
			}

		case <-time.NewTimer(10 * time.Second).C:
			t.Error("Context was canceled, but superviseConnection did not return.")
		}
	})

	t.Run("Ping error", func(t *testing.T) {
		// Prepare a connection that returns error on Ping.
		expectedErr := errors.New("PING ERROR")
		conn := &DummyConnection{
			PingFunc: func() error {
				return expectedErr
			},
		}

		// Prepare an apiSpecificAdapter that try to send Ping every 10 milliseconds.
		pingInterval := 10 * time.Millisecond
		r := &rtmAPIAdapter{
			config: &Config{
				PingInterval: pingInterval,
			},
		}

		// Run connection supervising function in background.
		ctx, cancel := context.WithCancel(context.Background())
		conErr := make(chan error)
		go func() {
			err := r.superviseConnection(ctx, conn, make(chan struct{}, 1))
			conErr <- err
		}()

		// Give long enough time to have at least one ping event before context cancellation.
		time.Sleep(pingInterval + 10*time.Millisecond)
		cancel()

		// See if expected error is returned.
		select {
		case err := <-conErr:
			if !errors.Is(err, expectedErr) {
				t.Errorf("Expected error is not returned: %#v", err)
			}

		case <-time.NewTimer(10 * time.Second).C:
			t.Error("Error is not returned.")
		}
	})
}

func Test_rtmAPIAdapter_handleRTMPayload(t *testing.T) {
	helpCommand := ".help"
	abortCommand := ".abort"
	config := &Config{
		HelpCommand:  helpCommand,
		AbortCommand: ".abort",
	}
	inputs := []struct {
		payload   rtmapi.DecodedPayload
		inputType reflect.Type
	}{
		{
			payload: &rtmapi.OKReply{
				Reply: rtmapi.Reply{
					ReplyTo: 1,
					OK:      true,
				},
				Text: "OK",
			},
			inputType: nil,
		},
		{
			payload: &rtmapi.NGReply{
				Reply: rtmapi.Reply{
					ReplyTo: 1,
					OK:      false,
				},
				Error: struct {
					Code    int    `json:"code"`
					Message string `json:"msg"`
				}{
					Code:    404,
					Message: "Not Found",
				},
			},
			inputType: nil,
		},
		{
			payload:   &rtmapi.Pong{},
			inputType: nil,
		},
		{
			payload: &event.Message{
				ChannelID: event.ChannelID("abc"),
				UserID:    event.UserID("cde"),
				Text:      helpCommand,
				TimeStamp: &event.TimeStamp{
					Time: time.Now(),
				},
			},
			inputType: reflect.ValueOf(&sarah.HelpInput{}).Type(),
		},
		{
			payload: &event.Message{
				ChannelID: event.ChannelID("abc"),
				UserID:    event.UserID("cde"),
				Text:      abortCommand,
				TimeStamp: &event.TimeStamp{
					Time: time.Now(),
				},
			},
			inputType: reflect.ValueOf(&sarah.AbortInput{}).Type(),
		},
		{
			payload: &event.Message{
				ChannelID: event.ChannelID("abc"),
				UserID:    event.UserID("cde"),
				Text:      "foo",
				TimeStamp: &event.TimeStamp{
					Time: time.Now(),
				},
			},
			inputType: reflect.ValueOf(&Input{}).Type(),
		},
		{
			payload:   &event.PinAdded{},
			inputType: nil,
		},
	}

	for i, input := range inputs {
		var receivedType reflect.Type
		fnc := func(i sarah.Input) error {
			receivedType = reflect.ValueOf(i).Type()
			return nil
		}
		DefaultRTMPayloadHandler(context.TODO(), config, input.payload, fnc)

		if input.inputType == nil && receivedType != nil {
			t.Errorf("Input shuold not be passed this time: %s.", receivedType.String())
		} else if input.inputType == nil {
			// No test
			continue
		}

		if receivedType == nil {
			t.Error("No payload is received")
		} else if receivedType != input.inputType {
			t.Errorf("Unexpected input type is given on %d test: %s.", i, receivedType.String())
		}
	}
}

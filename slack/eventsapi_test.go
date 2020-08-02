package slack

import (
	"context"
	"github.com/oklahomer/go-sarah/v2"
	"github.com/oklahomer/golack/v2/event"
	"github.com/oklahomer/golack/v2/eventsapi"
	"golang.org/x/xerrors"
	"testing"
	"time"
)

func Test_eventsAPIAdapter_run(t *testing.T) {
	t.Run("Successful case", func(t *testing.T) {
		// Prepare an adapter with a client that run a server.
		// The server notifies a signal when it stops on context cancellation.
		closed := make(chan struct{}, 1)
		client := &DummyClient{
			RunServerFunc: func(ctx context.Context, receiver eventsapi.EventReceiver) <-chan error {
				<-ctx.Done()
				closed <- struct{}{}
				return make(chan error, 1)
			},
		}
		adapter := &eventsAPIAdapter{
			config:        nil,
			client:        client,
			handlePayload: DefaultEventsPayloadHandler,
		}

		ctx, cancel := context.WithCancel(context.Background())
		go adapter.run(ctx, func(_ sarah.Input) error { return nil }, func(err error) {})
		cancel()

		// Context cancellation should not cause an error state.
		select {
		case <-closed:
			// O.K.

		case <-time.NewTimer(10 * time.Millisecond).C:
			t.Error("Context cancellation is not propagated to running server.")
		}
	})

	t.Run("Running server returns an error", func(t *testing.T) {
		// Prepare an adapter with a client that fails to run a server.
		expectedErr := xerrors.New("ERROR")
		client := &DummyClient{
			RunServerFunc: func(_ context.Context, _ eventsapi.EventReceiver) <-chan error {
				ch := make(chan error, 1)
				ch <- expectedErr
				return ch
			},
		}
		adapter := &eventsAPIAdapter{
			config:        nil,
			client:        client,
			handlePayload: DefaultEventsPayloadHandler,
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errCh := make(chan error, 1)
		notifyErr := func(err error) {
			errCh <- err
		}
		go adapter.run(ctx, func(_ sarah.Input) error { return nil }, notifyErr)

		// Context cancellation should not cause an error state.
		select {
		case err := <-errCh:
			var target *sarah.BotNonContinuableError
			if !xerrors.As(err, &target) {
				t.Errorf("Expected error is not returned: %#v", err)
			}

		case <-time.NewTimer(10 * time.Millisecond).C:
			t.Error("Error is not returned event though server unexpectedly stopped.")
		}
	})
}

func TestDefaultEventsPayloadHandler(t *testing.T) {
	t.Run("Regular message", func(t *testing.T) {
		ev := &event.ChannelMessage{}
		wrapper := &eventsapi.EventWrapper{
			Event: ev,
		}

		config := &Config{}
		incoming := make(chan sarah.Input, 1)
		enqueueInput := func(input sarah.Input) error {
			incoming <- input
			return nil
		}
		DefaultEventsPayloadHandler(context.TODO(), config, wrapper, enqueueInput)

		select {
		case input := <-incoming:
			typed, ok := input.(*Input)
			if !ok {
				t.Fatalf("Unexpected input is given: %#v", input)
			}

			if typed.payload != ev {
				t.Errorf("Given payload does not match with the original one: %#v", typed.payload)
			}
		}
	})

	t.Run("Help message", func(t *testing.T) {
		ev := &event.ChannelMessage{
			Text: ".help",
			TimeStamp: &event.TimeStamp{
				Time: time.Time{},
			},
		}
		wrapper := &eventsapi.EventWrapper{
			Event: ev,
		}

		config := &Config{
			HelpCommand: ".help",
		}
		incoming := make(chan sarah.Input, 1)
		enqueueInput := func(input sarah.Input) error {
			incoming <- input
			return nil
		}
		DefaultEventsPayloadHandler(context.TODO(), config, wrapper, enqueueInput)

		select {
		case input := <-incoming:
			_, ok := input.(*sarah.HelpInput)
			if !ok {
				t.Fatalf("Unexpected input is given: %#v", input)
			}
		}
	})

	t.Run("Abort message", func(t *testing.T) {
		ev := &event.ChannelMessage{
			Text: ".abort",
			TimeStamp: &event.TimeStamp{
				Time: time.Time{},
			},
		}
		wrapper := &eventsapi.EventWrapper{
			Event: ev,
		}

		config := &Config{
			AbortCommand: ".abort",
		}
		incoming := make(chan sarah.Input, 1)
		enqueueInput := func(input sarah.Input) error {
			incoming <- input
			return nil
		}
		DefaultEventsPayloadHandler(context.TODO(), config, wrapper, enqueueInput)

		select {
		case input := <-incoming:
			_, ok := input.(*sarah.AbortInput)
			if !ok {
				t.Fatalf("Unexpected input is given: %#v", input)
			}
		}
	})

	t.Run("Unsupported message", func(t *testing.T) {
		wrapper := &eventsapi.EventWrapper{
			Event: struct{}{},
		}

		config := &Config{}
		incoming := make(chan sarah.Input, 1)
		enqueueInput := func(input sarah.Input) error {
			incoming <- input
			return nil
		}
		DefaultEventsPayloadHandler(context.TODO(), config, wrapper, enqueueInput)

		// See if uncontrollable input is skipped.
		select {
		case input := <-incoming:
			t.Errorf("Uncontrollable input is passed: %#v", input)

		default:
			// O.K.
		}
	})
}

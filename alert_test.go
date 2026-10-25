package sarah

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type DummyAlerter struct {
	AlertFunc func(context.Context, BotType, error) error
}

func (alerter *DummyAlerter) Alert(ctx context.Context, botType BotType, err error) error {
	return alerter.AlertFunc(ctx, botType, err)
}

func TestAlerters_appendAlerter(t *testing.T) {
	a := &alerters{}
	impl := &DummyAlerter{}
	a.appendAlerter(impl)

	if len(*a) != 1 {
		t.Fatalf("Expected 1 Alerter to be stored, but was %d.", len(*a))
	}
}

func TestAlerters_alertAll(t *testing.T) {
	a := &alerters{}
	err := a.alertAll(t.Context(), "FOO", errors.New("error"))
	if err != nil {
		t.Errorf("Expected no error to be returned, but got %s.", err.Error())
	}

	panicErr := errors.New("panic with an error")
	alertErr := errors.New("failed to alert")
	a = &alerters{
		&DummyAlerter{
			AlertFunc: func(_ context.Context, _ BotType, _ error) error {
				panic(panicErr)
			},
		},
		&DummyAlerter{
			AlertFunc: func(_ context.Context, _ BotType, _ error) error {
				panic("PANIC!!")
			},
		},
		&DummyAlerter{
			AlertFunc: func(_ context.Context, _ BotType, _ error) error {
				return alertErr
			},
		},
		&DummyAlerter{
			AlertFunc: func(_ context.Context, _ BotType, _ error) error {
				return nil
			},
		},
	}

	err = a.alertAll(t.Context(), "FOO", errors.New("error"))
	if err == nil {
		t.Fatal("Expected error to be returned")
	}

	// errors.Join keeps every failure reachable through errors.Is.
	if !errors.Is(err, panicErr) {
		t.Errorf("Panicked error is not wrapped: %+v", err)
	}
	if !errors.Is(err, alertErr) {
		t.Errorf("Returned error is not wrapped: %+v", err)
	}

	// The joined error exposes the failures in registration order, each with the wrapper naming its cause.
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("Expected a joined error, but was %T.", err)
	}
	wrapped := joined.Unwrap()
	if len(wrapped) != 3 {
		t.Fatalf("Expected 3 errors to be stored: %#v.", err)
	}

	// The first error wraps the error a panicking Alerter raised.
	if !errors.Is(wrapped[0], panicErr) || !strings.HasPrefix(wrapped[0].Error(), "panic on alerting via") {
		t.Errorf("Expected panic error is not wrapped: %+v", wrapped[0])
	}

	// The second error carries the non-error value a panicking Alerter raised.
	if !strings.HasPrefix(wrapped[1].Error(), "panic on alerting via") || !strings.HasSuffix(wrapped[1].Error(), "PANIC!!") {
		t.Errorf("Expected panic value is not wrapped: %+v", wrapped[1])
	}

	// The third error wraps the error Alerter.Alert returned.
	if !errors.Is(wrapped[2], alertErr) || !strings.HasPrefix(wrapped[2].Error(), "failed to send alert via") {
		t.Errorf("Expected alert error is not wrapped: %+v", wrapped[2])
	}

	// The message is every failure's message joined by newlines.
	expected := strings.Join([]string{wrapped[0].Error(), wrapped[1].Error(), wrapped[2].Error()}, "\n")
	if err.Error() != expected {
		t.Errorf("Unexpected joined message: %q", err.Error())
	}
}

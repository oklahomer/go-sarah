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

	wrappedErr := errors.New("panic with an error")
	a = &alerters{
		&DummyAlerter{
			AlertFunc: func(_ context.Context, _ BotType, _ error) error {
				panic(wrappedErr)
			},
		},
		&DummyAlerter{
			AlertFunc: func(_ context.Context, _ BotType, _ error) error {
				panic("PANIC!!")
			},
		},
		&DummyAlerter{
			AlertFunc: func(_ context.Context, _ BotType, _ error) error {
				return wrappedErr
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
	if !errors.Is(err, wrappedErr) {
		t.Errorf("Expected error is not wrapped: %+v", err)
	}

	// The joined error exposes the failures in registration order.
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("Expected a joined error, but was %T.", err)
	}
	wrapped := joined.Unwrap()
	if len(wrapped) != 3 {
		t.Fatalf("Expected 3 errors to be stored: %#v.", err)
	}

	// The first error contains an error derived from panic
	if !errors.Is(wrapped[0], wrappedErr) {
		t.Errorf("Expected error is not wrapped: %+v", wrapped[0])
	}

	// The second error contains a string derived from panic
	if !strings.HasSuffix(wrapped[1].Error(), "PANIC!!") {
		t.Errorf("Expected string is not wrapped: %+v", wrapped[1])
	}

	// The third error wraps a error derived from Alerter.Alert
	if !errors.Is(wrapped[2], wrappedErr) {
		t.Errorf("Expected error is not wrapped: %+v", wrapped[2])
	}
}

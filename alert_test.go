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

func TestAlertErrs_appendError(t *testing.T) {
	e := errors.New("foo")
	errs := &alertErrs{}
	errs.appendError(e)

	if len(*errs) != 1 {
		t.Errorf("Expected 1 error to be stored, but was %d.", len(*errs))
	}
}

func TestAlertErrs_isEmpty(t *testing.T) {
	errs := &alertErrs{}
	if !errs.isEmpty() {
		t.Error("Expected to be true, but was not.")
	}

	errs = &alertErrs{errors.New("foo")}
	if errs.isEmpty() {
		t.Error("Expected to be false, but was not.")
	}
}

func TestAlertErrs_Error(t *testing.T) {
	testSets := []struct {
		errs []error
	}{
		{errs: nil},
		{errs: []error{errors.New("single error string")}},
		{errs: []error{errors.New("1st error string"), errors.New("2nd error string")}},
	}

	for i, testSet := range testSets {
		var errs []string
		for _, err := range testSet.errs {
			errs = append(errs, err.Error())
		}
		expected := strings.Join(errs, "\n")

		e := &alertErrs{}
		*e = testSet.errs
		if e.Error() != expected {
			t.Errorf("Expected error is not returned on test %d: %s", i, e.Error())
		}
	}
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
	err := a.alertAll(context.TODO(), "FOO", errors.New("error"))
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

	err = a.alertAll(context.TODO(), "FOO", errors.New("error"))
	if err == nil {
		t.Fatal("Expected error to be returned")
	}

	typed, ok := err.(*alertErrs)
	if !ok {
		t.Fatalf("Expected error type of *alertErrs, but was %T.", err)
	}
	if len(*typed) != 3 {
		t.Fatalf("Expected 3 errors to be stored: %#v.", err)
	}

	// The first error contains an error derived from panic
	if !errors.Is((*typed)[0], wrappedErr) {
		t.Errorf("Expected error is not wrapped: %+v", (*typed)[0])
	}

	// The second error contains a string derived from panic
	if !strings.HasSuffix((*typed)[1].Error(), "PANIC!!") {
		t.Errorf("Expected string is not wrapped: %+v", (*typed)[1])
	}

	// The third error wraps a error derived from Alerter.Alert
	if !errors.Is((*typed)[2], wrappedErr) {
		t.Errorf("Expected error is not wrapped: %+v", (*typed)[2])
	}
}

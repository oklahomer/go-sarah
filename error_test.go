package sarah

import (
	"strconv"
	"strings"
	"testing"
)

func TestNewBlockedInputError(t *testing.T) {
	i := 123
	err := NewBlockedInputError(i)

	if err == nil {
		t.Fatal("Instance of BlockedInputError is not returned.")
	}

	if _, ok := err.(*BlockedInputError); !ok {
		t.Fatalf("Returned value is not instance of BBlockedInputError: %#v", err)
	}

	concreteErr := err.(*BlockedInputError)
	if concreteErr.ContinuationCount != i {
		t.Errorf("Returned instance has different count than expected one. Expected: %d. Returned: %d.", i, concreteErr.ContinuationCount)
	}
}

func TestBlockedInputError_Error(t *testing.T) {
	i := 123
	err := NewBlockedInputError(i)

	if !strings.Contains(err.Error(), strconv.Itoa(i)) {
		t.Errorf("Returned string does not contain the count of error occurrence: %s.", err.Error())
	}
}

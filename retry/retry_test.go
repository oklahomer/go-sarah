package retry

import (
	"fmt"
	"testing"
)

func TestRetry(t *testing.T) {
	trial := uint(3)
	i := 0
	err := Retry(trial, func() error {
		i++
		return fmt.Errorf("error on %d", i)
	})

	retryErr, ok := err.(*RetryErrors)
	if !ok {
		t.Errorf("returned error is not RetryErrors. %#v", err)
	}
	if uint(len(retryErr.Errors)) != trial {
		t.Errorf("something is wrong with retrial. %s.", err.Error())
	}
}

func TestSomeRetrial(t *testing.T) {
	trial := uint(3)
	i := uint(0)
	expectedStr := "abc"
	str := ""
	err := Retry(trial, func() error {
		i++
		if i >= trial {
			fmt.Errorf("error on %d", i)
		}

		str = expectedStr
		return nil
	})

	if err != nil {
		t.Errorf("error is returned where it was not expected. %s.", err.Error())
	}

	if str != expectedStr {
		t.Errorf("expected string is not returned. instead %s is returned.", str)
	}
}

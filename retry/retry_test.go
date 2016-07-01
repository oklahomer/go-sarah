package retry

import (
	"errors"
	"fmt"
	"testing"
)

func TestRetry(t *testing.T) {
	trial := uint(3)
	i := 0
	err := Retry(trial, func() error {
		i++
		return errors.New(fmt.Sprintf("error on %d", i))
	})

	if uint(len(err.Errors)) != trial {
		t.Errorf("something is wrong with retrial. %s.", err.Error())
	}
}

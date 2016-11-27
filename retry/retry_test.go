package retry

import (
	"errors"
	"fmt"
	"github.com/oklahomer/go-sarah/log"
	"strings"
	"testing"
	"time"
)

func TestRetry(t *testing.T) {
	trial := uint(3)

	retryTests := []struct {
		failCnt uint
		err     error
	}{
		{
			// Succeed on the last trial
			failCnt: trial - 1,
		},
		{
			// Keep failing
			failCnt: trial,
		},
	}

	for _, testSet := range retryTests {
		i := 0
		successStr := "abc"
		str := ""
		err := Retry(trial, func() error {
			i++
			if uint(i) <= testSet.failCnt {
				return fmt.Errorf("error on %d", i)
			}

			str = successStr
			return nil
		})

		if testSet.failCnt == trial {
			retryErr, ok := err.(*Errors)
			if !ok {
				t.Errorf("Returned error is not RetryErrors: %#v.", err)
			}
			if uint(len(retryErr.Errors)) != trial {
				t.Errorf("Something is wrong with retrial: %s.", err.Error())
			}
		} else {
			if err != nil {
				t.Errorf("Error is returned where it was not expected: %s.", err.Error())
			}

			if str != successStr {
				t.Errorf("Expected string is not returned: %s.", str)
			}
		}
	}
}

func TestNewErrors(t *testing.T) {
	errs := NewErrors()
	if errs.Errors == nil {
		t.Fatal("Internal error stash is not initialized.")
	}
}

func TestErrors_Error(t *testing.T) {
	errs := NewErrors()
	firstErr := errors.New("1st error.")
	errs.appendError(firstErr)
	secondErr := errors.New("2nd error.")
	errs.appendError(secondErr)

	if errs.Error() != strings.Join([]string{firstErr.Error(), secondErr.Error()}, "\n") {
		t.Errorf("Unexpected error message is returned: %s.", errs.Error())
	}
}

func TestWithInterval(t *testing.T) {
	i := 0
	var startAt time.Time
	var endAt time.Time
	interval := 100 * time.Millisecond
	WithInterval(2, func() error {
		i++
		log.Error(i)
		if i == 1 {
			startAt = time.Now()
		} else {
			endAt = time.Now()
			return nil
		}

		return errors.New("error")
	}, interval)

	elapsed := endAt.Sub(startAt)
	if elapsed.Nanoseconds() <= interval.Nanoseconds() {
		t.Errorf("Expected retry interval is %d, but actual interval was %d.", interval.Nanoseconds(), elapsed.Nanoseconds())
	}
}

func TestWithBackOff(t *testing.T) {
	i := 0
	var startAt time.Time
	var endAt time.Time
	interval := 100 * time.Millisecond
	factor := 0.01
	WithBackOff(2, func() error {
		i++
		log.Error(i)
		if i == 1 {
			startAt = time.Now()
		} else {
			endAt = time.Now()
			return nil
		}

		return errors.New("error")
	}, interval, factor)

	delta := factor * float64(interval)
	min := float64(interval) - delta

	elapsed := endAt.Sub(startAt)
	if float64(elapsed.Nanoseconds()) <= min {
		t.Errorf("Expected minimum retry interval is %d, but actual interval was %d.", min, elapsed.Nanoseconds())
	}
}

func Test_randInterval(t *testing.T) {
	interval := randInterval(5*time.Second, 0)
	if interval != 5*time.Second {
		t.Error("Returned interval differs from input while random factor is 0.")
	}

	mean := 100 * time.Second
	for i := range make([]int, 100) {
		factor := float64(i) / 100
		given := randInterval(mean, factor)
		delta := factor * float64(mean)
		min := float64(mean) - delta
		max := float64(mean) + delta
		if !(min <= float64(given) && float64(given) <= max) {
			t.Errorf("Returned interval is not in the range of expectation. Mean: %g. Factor: %g. Given: %g.", mean.Seconds(), factor, given.Seconds())
		}
	}
}

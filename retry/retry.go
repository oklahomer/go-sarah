package retry

import (
	"math/rand"
	"strings"
	"time"
)

type RetryErrors struct {
	Errors []error
}

func NewRetryErrors() *RetryErrors {
	return &RetryErrors{Errors: []error{}}
}

func (e *RetryErrors) Error() string {
	errs := []string{}
	for _, err := range e.Errors {
		errs = append(errs, err.Error())
	}
	return strings.Join(errs, "\n")
}

func (e *RetryErrors) append(err error) {
	e.Errors = append(e.Errors, err)
}

func Retry(trial uint, function func() error) error {
	return RetryInterval(trial, function, 0*time.Second)
}

func RetryInterval(trial uint, function func() error, interval time.Duration) error {
	return RetryBackOff(trial, interval, 0, function)
}

func RetryBackOff(trial uint, meanInterval time.Duration, randFactor float64, function func() error) error {
	errors := NewRetryErrors()
	for trial > 0 {
		trial--
		if err := function(); err == nil {
			return nil
		} else {
			errors.append(err)
		}

		if trial <= 0 {
			break
		} else if randFactor <= 0 || meanInterval <= 0 {
			time.Sleep(meanInterval)
		} else {
			interval := randInterval(meanInterval, randFactor)
			time.Sleep(interval)
		}
	}

	if len(errors.Errors) > 0 {
		return errors
	}

	return nil
}

func randInterval(intervalDuration time.Duration, randFactor float64) time.Duration {
	interval := float64(intervalDuration)
	delta := randFactor * interval
	minInterval := interval - delta
	maxInterval := interval + delta

	return time.Duration(minInterval + (rand.Float64() * (maxInterval - minInterval + 1)))
}

package retry

import (
	"math/rand"
	"strings"
	"time"
)

type Errors struct {
	Errors []error
}

func NewErrors() *Errors {
	return &Errors{Errors: []error{}}
}

func (e *Errors) Error() string {
	errs := []string{}
	for _, err := range e.Errors {
		errs = append(errs, err.Error())
	}
	return strings.Join(errs, "\n")
}

func (e *Errors) appendError(err error) {
	e.Errors = append(e.Errors, err)
}

func Retry(trial uint, function func() error) error {
	return WithInterval(trial, function, 0*time.Second)
}

func WithInterval(trial uint, function func() error, interval time.Duration) error {
	return WithBackOff(trial, interval, 0, function)
}

func WithBackOff(trial uint, meanInterval time.Duration, randFactor float64, function func() error) error {
	errors := NewErrors()
	for trial > 0 {
		trial--
		err := function()
		if err == nil {
			return nil
		}
		errors.appendError(err)

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

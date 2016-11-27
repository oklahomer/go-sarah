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
	return WithBackOff(trial, function, interval, 0)
}

func WithBackOff(trial uint, function func() error, meanInterval time.Duration, randFactor float64) error {
	errs := NewErrors()
	for trial > 0 {
		trial--
		err := function()
		if err == nil {
			return nil
		}
		errs.appendError(err)

		if trial <= 0 {
			// All trials failed
			break
		}

		if randFactor <= 0 || meanInterval <= 0 {
			time.Sleep(meanInterval)
		} else {
			interval := randInterval(meanInterval, randFactor)
			time.Sleep(interval)
		}
	}

	return errs
}

func randInterval(intervalDuration time.Duration, randFactor float64) time.Duration {
	if randFactor < 0 {
		randFactor = 0
	} else if randFactor > 1 {
		randFactor = 1
	}

	interval := float64(intervalDuration)
	delta := randFactor * interval
	minInterval := interval - delta
	maxInterval := interval + delta

	return time.Duration(minInterval + (rand.Float64() * (maxInterval - minInterval + 1)))
}

/*
Package retry provides general retry logic and designated error structure that contains multiple errors.
*/
package retry

import (
	"math/rand"
	"strings"
	"time"
)

// Errors is an alias for slice of error that contains ordered error that occurred during retrials.
// This implements Error method to satisfy error interface, which returns concatenated message of all belonging errors.
//
// Since this is an alias to []error, each belonging error is accessible in a way such as:
//
//  for i, err := range *errs { ... }
type Errors []error

// Error returns the concatenated message of all belonging errors.
// All err.Err() strings are joined with "\n".
func (e *Errors) Error() string {
	errs := []string{}
	for _, err := range *e {
		errs = append(errs, err.Error())
	}
	return strings.Join(errs, "\n")
}

func (e *Errors) appendError(err error) {
	*e = append(*e, err)
}

// Retry retries given function as many times as the maximum trial count.
// It quits retrial when the function returns no error, which is nil.
func Retry(trial uint, function func() error) error {
	return WithInterval(trial, function, 0*time.Second)
}

// WithInterval retries given function at interval.
func WithInterval(trial uint, function func() error, interval time.Duration) error {
	return WithBackOff(trial, function, interval, 0)
}

// WithBackOff retries given function at interval, but the interval differs every time.
// The base interval and randomization factor are specified as 3rd and 4th arguments.
func WithBackOff(trial uint, function func() error, meanInterval time.Duration, randFactor float64) error {
	errs := &Errors{}
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

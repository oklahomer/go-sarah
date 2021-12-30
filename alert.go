package sarah

import (
	"context"
	"fmt"
	"strings"
)

// Alerter notifies administrators when Sarah or a bot is in a critical state.
// This is recommended to design one Alerter implementation deal with one and only one communication channel.
// e.g. MailAlerter sends an e-mail to administrators; SMSAlerter sends an SMS message to administrators.
// To notify via multiple communication channels, register as many Alerter implementations as required with multiple RegisterAlerter calls.
type Alerter interface {
	// Alert sends a notification to administrators so they can acknowledge the current critical state.
	Alert(context.Context, BotType, error) error
}

type alertErrs []error

func (e *alertErrs) appendError(err error) {
	*e = append(*e, err)
}

func (e *alertErrs) isEmpty() bool {
	return len(*e) == 0
}

// Error returns stringified form of all stored errors.
func (e *alertErrs) Error() string {
	var errs []string
	for _, err := range *e {
		errs = append(errs, err.Error())
	}
	return strings.Join(errs, "\n")
}

type alerters []Alerter

func (a *alerters) appendAlerter(alerter Alerter) {
	*a = append(*a, alerter)
}

func (a *alerters) alertAll(ctx context.Context, botType BotType, err error) error {
	errs := &alertErrs{}
	for _, alerter := range *a {
		// Considering the irregular state of Bot's lifecycle and importance of alert,
		// it is safer to be panic-proof.
		func() {
			defer func() {
				if r := recover(); r != nil {
					e, ok := r.(error)
					if ok {
						errs.appendError(fmt.Errorf("panic on alerting via %T: %w", alerter, e))
						return
					}

					errs.appendError(fmt.Errorf("panic on alerting via %T: %+v", alerter, r))
				}
			}()

			err := alerter.Alert(ctx, botType, err)
			if err != nil {
				errs.appendError(fmt.Errorf("failed to send alert via %T: %w", alerter, err))
			}
		}()
	}

	if errs.isEmpty() {
		return nil
	}
	return errs
}

package sarah

import (
	"context"
	"errors"
	"fmt"
)

// Alerter notifies administrators when Sarah or a bot is in a critical state.
// This is recommended to design one Alerter implementation deal with one and only one communication channel.
// e.g. MailAlerter sends an e-mail to administrators; SMSAlerter sends an SMS message to administrators.
// To notify via multiple communication channels, register as many Alerter implementations as required with multiple RegisterAlerter calls.
type Alerter interface {
	// Alert sends a notification to administrators so they can acknowledge the current critical state.
	Alert(context.Context, BotType, error) error
}

type alerters []Alerter

func (a *alerters) appendAlerter(alerter Alerter) {
	*a = append(*a, alerter)
}

// alertAll lets every registered Alerter send the given error.
// Failures are collected and returned as one error joined with errors.Join, or nil when every Alerter succeeds.
func (a *alerters) alertAll(ctx context.Context, botType BotType, err error) error {
	var errs []error
	for _, alerter := range *a {
		// Considering the irregular state of Bot's lifecycle and importance of alert,
		// it is safer to be panic-proof.
		func() {
			defer func() {
				if r := recover(); r != nil {
					e, ok := r.(error)
					if ok {
						errs = append(errs, fmt.Errorf("panic on alerting via %T: %w", alerter, e))
						return
					}

					errs = append(errs, fmt.Errorf("panic on alerting via %T: %+v", alerter, r))
				}
			}()

			err := alerter.Alert(ctx, botType, err)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to send alert via %T: %w", alerter, err))
			}
		}()
	}

	return errors.Join(errs...)
}

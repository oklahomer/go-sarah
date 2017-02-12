package sarah

import "golang.org/x/net/context"

// Alerter can be used to report Bot's critical state to developer/administrator.
// Anything that implements this interface can be registered as Alerter via Runner.RegisterAlerter.
type Alerter interface {
	// Alert sends notification to developer/administrator so one may notify Bot's critical state.
	Alert(context.Context, BotType, error) error
}

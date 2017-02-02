package sarah

import "golang.org/x/net/context"

type DummyAlerter struct {
	AlertFunc func(context.Context, BotType, error) error
}

func (alerter *DummyAlerter) Alert(ctx context.Context, botType BotType, err error) error {
	return alerter.AlertFunc(ctx, botType, err)
}

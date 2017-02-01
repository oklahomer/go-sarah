package sarah

import "golang.org/x/net/context"

type DummyAlerter struct {
	AlertFunc func(context.Context, BotType, error)
}

func (alerter *DummyAlerter) Alert(ctx context.Context, botType BotType, err error) {
	alerter.AlertFunc(ctx, botType, err)
}

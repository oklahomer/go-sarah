package sarah

import "golang.org/x/net/context"

type Alerter interface {
	Alert(context.Context, BotType, error)
}

package sarah

import "context"

type DummyAdapter struct {
	BotTypeValue    BotType
	RunFunc         func(context.Context, func(Input) error, func(error))
	SendMessageFunc func(context.Context, Output)
}

func (adapter *DummyAdapter) BotType() BotType {
	return adapter.BotTypeValue
}

func (adapter *DummyAdapter) Run(ctx context.Context, enqueueInput func(Input) error, notifyErr func(error)) {
	adapter.RunFunc(ctx, enqueueInput, notifyErr)
}

func (adapter *DummyAdapter) SendMessage(ctx context.Context, output Output) {
	adapter.SendMessageFunc(ctx, output)
}

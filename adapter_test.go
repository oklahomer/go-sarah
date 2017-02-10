package sarah

import "golang.org/x/net/context"

type DummyAdapter struct {
	BotTypeValue    BotType
	RunFunc         func(context.Context, func(Input), func(error))
	SendMessageFunc func(context.Context, Output)
}

func (adapter *DummyAdapter) BotType() BotType {
	return adapter.BotTypeValue
}

func (adapter *DummyAdapter) Run(ctx context.Context, receiveInput func(Input), errNotifier func(error)) {
	adapter.RunFunc(ctx, receiveInput, errNotifier)
}

func (adapter *DummyAdapter) SendMessage(ctx context.Context, output Output) {
	adapter.SendMessageFunc(ctx, output)
}

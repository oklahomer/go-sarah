package slack

import (
	"context"
	"github.com/oklahomer/go-sarah/v2"
	"github.com/oklahomer/go-sarah/v2/log"
	"github.com/oklahomer/golack/v2/eventsapi"
	"net/http"
	"strings"
)

type eventsAPIAdapter struct {
	config        *Config
	client        SlackClient
	handlePayload func(context.Context, *Config, *eventsapi.EventWrapper, func(sarah.Input) error)
}

var _ apiSpecificAdapter = (*eventsAPIAdapter)(nil)

func (e *eventsAPIAdapter) run(ctx context.Context, enqueueInput func(sarah.Input) error, notifyErr func(error)) {
	receiver := eventsapi.NewDefaultEventReceiver(func(wrapper *eventsapi.EventWrapper) {
		e.handlePayload(ctx, e.config, wrapper, enqueueInput)
	})
	errChan := e.client.RunServer(ctx, receiver)

	select {
	case <-ctx.Done():
		// Context is canceled by caller
		return

	case err := <-errChan:
		if err == http.ErrServerClosed {
			// Server is intentionally stopped probably due to caller's context cancellation.
			return
		}

		notifyErr(sarah.NewBotNonContinuableError(err.Error()))
		return
	}
}

func DefaultEventsPayloadHandler(_ context.Context, config *Config, payload *eventsapi.EventWrapper, enqueueInput func(input sarah.Input) error) {
	input, err := EventToInput(payload.Event)
	if err == ErrNonSupportedEvent {
		log.Debugf("Event given, but no corresponding action is defined. %#v", payload)
		return
	}

	if err != nil {
		log.Errorf("Failed to convert %T event: %s", payload.Event, err.Error())
		return
	}

	trimmed := strings.TrimSpace(input.Message())
	if config.HelpCommand != "" && trimmed == config.HelpCommand {
		// Help command
		help := sarah.NewHelpInput(input)
		_ = enqueueInput(help)
	} else if config.AbortCommand != "" && trimmed == config.AbortCommand {
		// Abort command
		abort := sarah.NewAbortInput(input)
		_ = enqueueInput(abort)
	} else {
		// Regular input
		_ = enqueueInput(input)
	}
}

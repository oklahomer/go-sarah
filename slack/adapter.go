package slack

import (
	"context"
	"errors"
	"fmt"
	"github.com/oklahomer/go-kasumi/logger"
	"github.com/oklahomer/go-sarah/v4"
	"github.com/oklahomer/golack/v2"
	"github.com/oklahomer/golack/v2/event"
	"github.com/oklahomer/golack/v2/eventsapi"
	"github.com/oklahomer/golack/v2/rtmapi"
	"github.com/oklahomer/golack/v2/webapi"
	"time"
)

const (
	// SLACK is a designated sara.BotType for Slack integration.
	SLACK sarah.BotType = "slack"
)

// ErrNonSupportedEvent is returned when the given Slack event is not supported by this adapter.
var ErrNonSupportedEvent = errors.New("event not supported")

// AdapterOption defines a function's signature that Adapter's functional options must satisfy.
type AdapterOption func(adapter *Adapter)

// WithSlackClient creates an AdapterOption with the given SlackClient implementation.
// If this option is not given, NewAdapter tries to create golack instance with the given Config.
func WithSlackClient(client SlackClient) AdapterOption {
	return func(adapter *Adapter) {
		adapter.client = client
	}
}

// WithEventsPayloadHandler creates an AdapterOption with the given function to handle incoming Events API payloads.
// The simplest example to receive a message payload is to use a default payload handler as below:
//
//   slackAdapter, _ := slack.NewAdapter(slackConfig, slack.WithEventsPayloadHandler(slack.DefaultEventsPayloadHandler))
//
// See WithRTMPayloadHandler for the detailed usage. WithEventsPayloadHandler is just another form of payload handler to work with Events API.
func WithEventsPayloadHandler(fnc func(context.Context, *Config, *eventsapi.EventWrapper, func(sarah.Input) error)) AdapterOption {
	return func(adapter *Adapter) {
		adapter.apiSpecificAdapterBuilder = func(config *Config, client SlackClient) apiSpecificAdapter {
			return &eventsAPIAdapter{
				config:        adapter.config,
				client:        adapter.client,
				handlePayload: fnc,
			}
		}
	}
}

// WithRTMPayloadHandler creates an AdapterOption with the given function to handle incoming RTM payloads.
// The simplest example to receive message payloads is to use a default payload handler as below:
//
//   slackAdapter, _ := slack.NewAdapter(slackConfig, slack.WithRTMPayloadHandler(DefaultRTMPayloadHandler))
//
// However, Slack's RTM API defines a relatively large amount of payload types.
// To have a better user experience, developers may provide a customized callback function to handle different types of received payloads.
// In that case, one may implement a new payload handler and replace DefaultRTMPayloadHandler.
// Inside the customized payload handler, a developer may wish to have direct access to SlackClient to post some sort of message to Slack via Web API.
// To support such a scenario, wrap this function like below so the SlackClient can be accessed within its scope.
//
//  // Setup golack instance, which implements SlackClient interface.
//  golackConfig := golack.NewConfig()
//  golackConfig.Token = "XXXXXXX"
//  slackClient := golack.New(golackConfig)
//
//  rtmPayloadHandler := func(connCtx context.Context, config *Config, payload rtmapi.DecodedPayload, enqueueInput func(sarah.Input) error) {
// 		switch p := payload.(type) {
//  	case *event.PinAdded:
//			// Do something with pre-defined SlackClient
//    		// slackClient.PostMessage(connCtx, ...)
//
//    	default:
//    		input, err := EventToInput(p)
//    		if err == ErrNonSupportedEvent {
//     			log.Debugf("Event given, but no corresponding action is defined. %#v", payload)
//     			return
//    		}
//
//    		if err != nil {
//     			log.Errorf("Failed to convert %T event: %s", p, err.Error())
//     			return
//    		}
//
//    		trimmed := strings.TrimSpace(input.Message())
//    		if config.HelpCommand != "" && trimmed == config.HelpCommand {
//     			// Help command
//     			help := sarah.NewHelpInput(input)
//     			_ = enqueueInput(help)
//    		} else if config.AbortCommand != "" && trimmed == config.AbortCommand {
//     			// Abort command
//     			abort := sarah.NewAbortInput(input)
//     			_ = enqueueInput(abort)
//    		} else {
//     			// Regular input
//     			_ = enqueueInput(input)
//    		}
//    	}
//  }
//
//  slackConfig := slack.NewConfig()
//  slackAdapter, _ := slack.NewAdapter(slackConfig, slack.WithSlackClient(slackClient), slack.WithRTMPayloadHandler(rtmPayloadHandler))
//  slackBot, _ := sarah.NewBot(slackAdapter)
func WithRTMPayloadHandler(fnc func(context.Context, *Config, rtmapi.DecodedPayload, func(sarah.Input) error)) AdapterOption {
	return func(adapter *Adapter) {
		adapter.apiSpecificAdapterBuilder = func(config *Config, client SlackClient) apiSpecificAdapter {
			return &rtmAPIAdapter{
				config:        adapter.config,
				client:        adapter.client,
				handlePayload: fnc,
			}
		}
	}
}

// Adapter is a sarah.Adapter implementation that internally calls Slack Rest API and Real Time Messaging API to offer Bot developers an easy way to communicate with Slack.
//
//	slackConfig := slack.NewConfig()
//	slackConfig.Token = "XXXXXXXXXXXX" // Set token manually or feed slackConfig to json.Unmarshal or yaml.Unmarshal
//	slackAdapter, _ := slack.NewAdapter(slackConfig)
//	slackBot, _ := sarah.NewBot(slackAdapter)
//	sarah.RegisterBot(slackBot)
//
//	sarah.Run(context.TODO(), sarah.NewConfig())
type Adapter struct {
	config                    *Config
	client                    SlackClient
	apiSpecificAdapterBuilder func(config *Config, client SlackClient) apiSpecificAdapter
}

// NewAdapter creates a new Adapter with the given *Config and zero or more AdapterOption values.
func NewAdapter(config *Config, options ...AdapterOption) (*Adapter, error) {
	adapter := &Adapter{
		config: config,
	}

	for _, opt := range options {
		opt(adapter)
	}

	// See if a client is set by WithSlackClient option.
	// If not, use golack with the given configuration.
	if adapter.client == nil {
		if config.Token == "" {
			return nil, errors.New("Slack client must be provided with WithSlackClient option or must be configurable with given *Config")
		}

		golackConfig := golack.NewConfig()
		golackConfig.Token = config.Token
		golackConfig.AppSecret = config.AppSecret
		golackConfig.ListenPort = config.ListenPort
		if config.RequestTimeout != 0 {
			golackConfig.RequestTimeout = config.RequestTimeout
		}

		adapter.client = golack.New(golackConfig)
	}

	if adapter.apiSpecificAdapterBuilder == nil {
		return nil, errors.New("RTM or Events API configuration must be applied with WithRTMPayloadHandler or WithEventsPayloadHandler")
	}

	return adapter, nil
}

// BotType returns a designated BotType for Slack integration.
func (adapter *Adapter) BotType() sarah.BotType {
	return SLACK
}

// Run establishes a connection with Slack, supervises it, and tries to reconnect when the current connection is gone.
//
// When a message is sent from the Slack server, the payload is passed to Sarah via the function given as the 2nd argument -- enqueueInput.
// This function simply wraps a channel to prevent blocking situation. When workers are too busy and channel blocks, this function returns BlockedInputError.
//
// Upon a critical situation such as consecutive reconnection trial failures, such a state is notified to Sarah via the 3rd argument function -- notifyErr.
// Sarah cancels this Bot/Adapter and cleans up related resources when BotNonContinuableError is given to this function.
func (adapter *Adapter) Run(ctx context.Context, enqueueInput func(sarah.Input) error, notifyErr func(error)) {
	adapter.apiSpecificAdapterBuilder(adapter.config, adapter.client).run(ctx, enqueueInput, notifyErr)
}

// nonBlockSignal tries to send a signal to the given channel in a non-blocking manner.
// If no goroutine is listening to the channel or one is working on a task triggered by the previous signal,
// this method skips signaling rather than blocking till one is ready to read the channel.
//
// For a signaling purposes, an empty struct{} should be used.
// http://peter.bourgon.org/go-in-production/
//  "Use struct{} as a sentinel value, rather than bool or interface{}. For example, (snip) a signal channel is chan struct{}.
//  It unambiguously signals an explicit lack of information."
func nonBlockSignal(id string, target chan<- struct{}) {
	select {
	case target <- struct{}{}:
		// O.K

	default:
		// couldn't send because no goroutine is listening to the  channel or one is currently busy.
		logger.Debugf("Not sending signal to channel: %s", id)

	}
}

// SendMessage lets sarah.Bot send a message to Slack.
func (adapter *Adapter) SendMessage(ctx context.Context, output sarah.Output) {
	var message *webapi.PostMessage
	switch content := output.Content().(type) {
	case *webapi.PostMessage:
		message = content

	case string:
		channel, ok := output.Destination().(event.ChannelID)
		if !ok {
			logger.Errorf("Destination is not instance of Channel. %#v.", output.Destination())
			return
		}
		message = webapi.NewPostMessage(channel, content)

	case *sarah.CommandHelps:
		channelID, ok := output.Destination().(event.ChannelID)
		if !ok {
			logger.Errorf("Destination is not instance of Channel. %#v.", output.Destination())
			return
		}

		var fields []*webapi.AttachmentField
		for _, commandHelp := range *output.Content().(*sarah.CommandHelps) {
			fields = append(fields, &webapi.AttachmentField{
				Title: commandHelp.Identifier,
				Value: commandHelp.Instruction,
				Short: false,
			})
		}
		attachments := []*webapi.MessageAttachment{
			{
				Fallback: "Here are some input instructions.",
				Pretext:  "Help:",
				Title:    "",
				Fields:   fields,
			},
		}
		message = webapi.NewPostMessage(channelID, "").WithAttachments(attachments)

	default:
		logger.Warnf("Unexpected output %#v", output)
		return
	}

	resp, err := adapter.client.PostMessage(ctx, message)
	if err != nil {
		logger.Errorf("Something went wrong with Web API posting: %+v. %+v", err, message)
		return
	}

	if !resp.OK {
		logger.Errorf("Failed to post message %#v: %s", message, resp.Error)
	}
}

// Input is a sarah.Input implementation that represents a received message.
// Pass an incoming payload to EventToInput for a conversion.
type Input struct {
	Event           interface{}
	senderKey       string
	text            string
	timestamp       *event.TimeStamp
	threadTimeStamp *event.TimeStamp
	channelID       event.ChannelID
}

// SenderKey returns the message sender's id.
func (i *Input) SenderKey() string {
	return i.senderKey
}

// Message returns the received text.
func (i *Input) Message() string {
	return i.text
}

// SentAt returns when the message is sent.
func (i *Input) SentAt() time.Time {
	return i.timestamp.Time
}

// ReplyTo returns the Slack channel where the message was sent.
func (i *Input) ReplyTo() sarah.OutputDestination {
	return i.channelID
}

// EventToInput converts the given event payload to *Input.
func EventToInput(e interface{}) (sarah.Input, error) {
	switch typed := e.(type) {
	case *event.Message:
		return &Input{
			Event:           e,
			senderKey:       fmt.Sprintf("%s|%s", typed.ChannelID.String(), typed.UserID.String()),
			text:            typed.Text,
			timestamp:       typed.TimeStamp,
			threadTimeStamp: typed.ThreadTimeStamp,
			channelID:       typed.ChannelID,
		}, nil

	case *event.ChannelMessage:
		return &Input{
			Event:           e,
			senderKey:       fmt.Sprintf("%s|%s", typed.ChannelID.String(), typed.UserID.String()),
			text:            typed.Text,
			timestamp:       typed.TimeStamp,
			threadTimeStamp: typed.ThreadTimeStamp,
			channelID:       typed.ChannelID,
		}, nil

	default:
		return nil, ErrNonSupportedEvent
	}
}

// IsThreadMessage tells if the given message is sent in a thread.
// If the message is sent in a thread, this is encouraged to reply in a thread.
// NewResponse, therefore, defaults to send a response as a thread reply when the input is sent in a thread.
// Use RespAsThreadReply to modify the behavior.
func IsThreadMessage(input *Input) bool {
	if input.threadTimeStamp == nil {
		return false
	}

	if input.threadTimeStamp.OriginalValue == input.timestamp.OriginalValue {
		return false
	}

	return true
}

// NewResponse creates *sarah.CommandResponse with the given arguments.
// Simply pass a received sarah.Input instance and a text string to send a text message as a reply.
// To send a more customized reply message, pass as many options created by ResponseWith* functions as required.
//
// When an input is sent in a thread, this function defaults to send a response as a thread reply.
// To explicitly change such behavior, use RespAsThreadReply or RespReplyBroadcast.
func NewResponse(input sarah.Input, msg string, options ...RespOption) (*sarah.CommandResponse, error) {
	typed, ok := input.(*Input)
	if !ok {
		return nil, fmt.Errorf("%T is not currently supported to automatically generate response", input)
	}

	stash := &respOptions{
		attachments: []*webapi.MessageAttachment{},
		userContext: nil,
		linkNames:   1, // Linkify channel names and usernames. ref. https://api.slack.com/docs/message-formatting#parsing_modes
		parseMode:   webapi.ParseModeFull,
		unfurlLinks: true,
		unfurlMedia: true,
	}
	for _, opt := range options {
		opt(stash)
	}

	postMessage := webapi.NewPostMessage(typed.channelID, msg).
		WithAttachments(stash.attachments).
		WithLinkNames(stash.linkNames).
		WithParse(stash.parseMode).
		WithUnfurlLinks(stash.unfurlLinks).
		WithUnfurlMedia(stash.unfurlMedia)
	if replyInThread(typed, stash) {
		postMessage.
			WithThreadTimeStamp(threadTimeStamp(typed).String()).
			WithReplyBroadcast(stash.replyBroadcast)
	}
	return &sarah.CommandResponse{
		Content:     postMessage,
		UserContext: stash.userContext,
	}, nil
}

func replyInThread(input *Input, options *respOptions) bool {
	// If explicitly set by a user, follow such instruction.
	if options.asThreadReply != nil {
		return *options.asThreadReply
	}

	// If an input is given in a thread, then reply in thread.
	// Otherwise, post as a stand-alone message.
	return IsThreadMessage(input)
}

func threadTimeStamp(input *Input) *event.TimeStamp {
	if input.threadTimeStamp != nil {
		return input.threadTimeStamp
	}

	return input.timestamp
}

// RespAsThreadReply specifies if the response should be sent as a thread reply.
func RespAsThreadReply(asReply bool) RespOption {
	return func(options *respOptions) {
		options.asThreadReply = &asReply
	}
}

// RespReplyBroadcast decides if the thread reply should be broadcast.
// To activate this option, RespAsThreadReply must be set to true.
func RespReplyBroadcast(broadcast bool) RespOption {
	return func(options *respOptions) {
		options.replyBroadcast = broadcast
	}
}

// RespWithAttachments adds given attachments to the response.
func RespWithAttachments(attachments []*webapi.MessageAttachment) RespOption {
	return func(options *respOptions) {
		options.attachments = attachments
	}
}

// RespWithNext sets a given fnc as part of the response's *sarah.UserContext.
// The next input from the same user will be passed to this fnc.
// sarah.UserContextStorage must be configured or otherwise, the function will be ignored.
func RespWithNext(fnc sarah.ContextualFunc) RespOption {
	return func(options *respOptions) {
		options.userContext = &sarah.UserContext{
			Next: fnc,
		}
	}
}

// RespWithNextSerializable sets the given arg as part of the response's *sarah.UserContext.
// The next input from the same user will be passed to the function defined in the arg.
// sarah.UserContextStorage must be configured or otherwise, the function will be ignored.
func RespWithNextSerializable(arg *sarah.SerializableArgument) RespOption {
	return func(options *respOptions) {
		options.userContext = &sarah.UserContext{
			Serializable: arg,
		}
	}
}

// RespWithLinkNames sets the given linkNames value to the response.
// Set 1 to linkify channel names and usernames in the response.
// The default value is 1.
func RespWithLinkNames(linkNames int) RespOption {
	return func(options *respOptions) {
		options.linkNames = linkNames
	}
}

// RespWithParse sets the given mode value to the response.
// The default value is webapi.ParseModeFull.
func RespWithParse(mode webapi.ParseMode) RespOption {
	return func(options *respOptions) {
		options.parseMode = mode
	}
}

// RespWithUnfurlLinks sets the given unfurl value to the response.
// The default value is true.
func RespWithUnfurlLinks(unfurl bool) RespOption {
	return func(options *respOptions) {
		options.unfurlLinks = unfurl
	}
}

// RespWithUnfurlMedia sets the given unfurl value to the response.
// The default value is true.
func RespWithUnfurlMedia(unfurl bool) RespOption {
	return func(options *respOptions) {
		options.unfurlMedia = unfurl
	}
}

// RespOption defines a function's signature that NewResponse's functional option must satisfy.
type RespOption func(*respOptions)

type respOptions struct {
	attachments    []*webapi.MessageAttachment
	userContext    *sarah.UserContext
	linkNames      int
	parseMode      webapi.ParseMode
	unfurlLinks    bool
	unfurlMedia    bool
	asThreadReply  *bool
	replyBroadcast bool
}

type apiSpecificAdapter interface {
	run(ctx context.Context, enqueueInput func(sarah.Input) error, notifyErr func(error))
}

// SlackClient is an interface that covers golack's public methods.
type SlackClient interface {
	ConnectRTM(ctx context.Context) (rtmapi.Connection, error)
	PostMessage(ctx context.Context, message *webapi.PostMessage) (*webapi.APIResponse, error)
	RunServer(ctx context.Context, receiver eventsapi.EventReceiver) <-chan error
}

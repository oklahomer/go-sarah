package echo

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/gitter"
	"github.com/oklahomer/golack/rtmapi"
	"golang.org/x/net/context"
	"testing"
	"time"
)

type DummyInput struct {
	SenderKeyValue string
	MessageValue   string
	SentAtValue    time.Time
	ReplyToValue   sarah.OutputDestination
}

func (i *DummyInput) SenderKey() string {
	return i.SenderKeyValue
}

func (i *DummyInput) Message() string {
	return i.MessageValue
}

func (i *DummyInput) SentAt() time.Time {
	return i.SentAtValue
}

func (i *DummyInput) ReplyTo() sarah.OutputDestination {
	return i.ReplyToValue
}

func TestSlackCommandFunc(t *testing.T) {
	input := &DummyInput{
		SenderKeyValue: "userKey",
		MessageValue:   ".echo foo",
		SentAtValue:    time.Now(),
		ReplyToValue:   rtmapi.ChannelID("channelId"),
	}

	response, err := SlackCommandFunc(context.TODO(), input)

	if err != nil {
		t.Errorf("Unexpected error is returned: %s.", err.Error())
	}

	if response == nil {
		t.Fatal("Response should be returned.")
	}

	content, ok := response.Content.(string)
	if !ok {
		t.Fatalf("Response content should be a plain text: %#v.", response.Content)
	}

	if content != "foo" {
		t.Errorf("Unexpected response is returned: %s.", content)
	}

	if response.UserContext != nil {
		t.Fatalf("UserContext should never be returned with this Command: %#v.", response.UserContext)
	}
}

func TestGitterCommandFunc(t *testing.T) {
	input := &DummyInput{
		SenderKeyValue: "userKey",
		MessageValue:   ".echo foo",
		SentAtValue:    time.Now(),
		ReplyToValue:   &gitter.Room{},
	}

	response, err := GitterCommandFunc(context.TODO(), input)

	if err != nil {
		t.Errorf("Unexpected error is returned: %s.", err.Error())
	}

	if response == nil {
		t.Fatal("Response should be returned.")
	}

	content, ok := response.Content.(string)
	if !ok {
		t.Fatalf("Response content should be a plain text: %#v.", response.Content)
	}

	if content != "foo" {
		t.Errorf("Unexpected response is returned: %s.", content)
	}

	if response.UserContext != nil {
		t.Fatalf("UserContext should never be returned with this Command: %#v.", response.UserContext)
	}
}

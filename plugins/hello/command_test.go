package hello

import (
	"context"
	"github.com/oklahomer/go-sarah/slack"
	"github.com/oklahomer/golack/rtmapi"
	"testing"
)

func Test_slackFunc(t *testing.T) {
	input := slack.NewMessageInput(&rtmapi.Message{
		Text: ".hello",
	})
	response, err := slackFunc(context.TODO(), input)
	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	if response == nil {
		t.Fatal("Expected response is not returned.")
	}

	if response.UserContext != nil {
		t.Errorf("Unexpected UserContext is returned: %#v.", response.UserContext)
	}

	message, ok := response.Content.(*rtmapi.OutgoingMessage)
	if !ok {
		t.Fatalf("Returned content has unexpected type: %T", response.Content)
	}

	if message.Text != "Hello!" {
		t.Errorf("Unexpected text is returned: %s.", message.Text)
	}
}

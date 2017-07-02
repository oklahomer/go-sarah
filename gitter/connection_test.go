package gitter

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

type DummyConn struct {
	*strings.Reader
	CloseFunc func() error
}

func (c *DummyConn) Close() error {
	return c.CloseFunc()
}

func TestConnWrapper_Receive(t *testing.T) {
	// https://developer.gitter.im/docs/messages-resource
	readLine := `{
		"id": "53316dc47bfc1a000000000f",
		"text": "new message",
		"html": "Hi <span data-link-type=\"mention\" data-screen-name=\"suprememoocow\" class=\"mention\">@suprememoocow</span> !",
		"sent": "2014-03-25T11:51:32.289Z",
		"editedAt": "2014-03-25T11:51:32.289Z",
		"fromUser": {
			"id": "53307734c3599d1de448e192",
			"username": "malditogeek",
			"displayName": "Mauro Pompilio",
			"url": "/malditogeek",
			"avatarUrlSmall": "https://avatars.githubusercontent.com/u/14751?",
			"avatarUrlMedium": "https://avatars.githubusercontent.com/u/14751?"
		},
		"unread": false,
		"readBy": 0,
		"urls": [],
		"mentions": [{
			"screenName": "suprememoocow",
			"userId": "53307831c3599d1de448e19a"
			}],
		"v": 1
	}`
	readLine = regexp.MustCompile(`\r?\n`).ReplaceAllString(readLine, "")
	conn := &DummyConn{
		strings.NewReader(readLine + "\n"), // Delimiter
		nil,
	}
	wrapper := &connWrapper{
		readCloser: conn,
		Room:       &Room{},
	}

	roomMessage, err := wrapper.Receive()
	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	if roomMessage.ReceivedMessage.Text != "new message" {
		t.Errorf("Supplied line is not returned: %#v.", roomMessage)
	}
}

func TestConnWrapper_Close(t *testing.T) {
	expected := errors.New("close error")
	conn := &DummyConn{
		nil,
		func() error { return expected },
	}

	wrapper := &connWrapper{
		readCloser: conn,
		Room:       &Room{},
	}

	err := wrapper.Close()
	if err != expected {
		t.Errorf("Expected error is not returned: %s.", err.Error())
	}
}

func TestRoomMessage_Message(t *testing.T) {
	text := "text"
	message := &RoomMessage{
		ReceivedMessage: &Message{
			Text: text,
		},
	}

	if message.Message() != text {
		t.Errorf("Expected message is not returned: %s.", message.Message())
	}
}

func TestRoomMessage_ReplyTo(t *testing.T) {
	room := &Room{}
	message := &RoomMessage{
		Room: room,
	}

	if message.ReplyTo() != room {
		t.Errorf("Expected reply destination is returned: %#v", message.ReplyTo())
	}
}

func TestRoomMessage_SenderKey(t *testing.T) {
	userID := "userID"
	roomID := "roomID"
	room := &Room{
		ID: roomID,
	}
	message := &RoomMessage{
		Room: room,
		ReceivedMessage: &Message{
			FromUser: User{
				ID: userID,
			},
		},
	}

	if !(strings.Contains(message.SenderKey(), roomID)) {
		t.Errorf("Room ID is not contained: %s.", message.SenderKey())
	}

	if !(strings.Contains(message.SenderKey(), userID)) {
		t.Errorf("User ID is not contained: %s.", message.SenderKey())
	}
}

func TestRoomMessage_SentAt(t *testing.T) {
	now := time.Now()
	message := &RoomMessage{
		ReceivedMessage: &Message{
			SendTimeStamp: TimeStamp{
				Time: now,
			},
		},
	}

	if !message.SentAt().Equal(now) {
		t.Errorf("Expected TimeStamp is not returned: %s.", message.SentAt())
	}
}

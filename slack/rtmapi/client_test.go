package rtmapi

import (
	"bytes"
	"fmt"
	"golang.org/x/net/websocket"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

var webSocketServerAddress string
var once sync.Once

func echoServer(ws *websocket.Conn) {
	defer ws.Close()
	io.Copy(ws, ws)
}

func startServer() {
	http.Handle("/echo", websocket.Handler(echoServer))
	server := httptest.NewServer(nil)
	webSocketServerAddress = server.Listener.Addr().String()
}

func TestConnect(t *testing.T) {
	once.Do(startServer)

	// Establish connection
	url := fmt.Sprintf("ws://%s%s", webSocketServerAddress, "/echo")
	client := NewClient()
	conn, err := client.Connect(url)
	if err != nil {
		t.Errorf("webSocket connection error. %#v.", err)
		return
	}

	// Send message
	msg := []byte("hello, world\n")
	if _, err := conn.Write(msg); err != nil {
		t.Errorf("error on sending message over WebSocket connection. %#v.", err)
	}
	small_msg := make([]byte, 8)

	// Receive
	if _, err = conn.Read(small_msg); err != nil {
		t.Errorf("error on WebSocket paylaod receive. %#v.", err)
	}
	if !bytes.Equal(msg[:len(small_msg)], small_msg) {
		t.Errorf("error on received message comparison. expected %q got %q.", msg[:len(small_msg)], small_msg)
	}

	// Close connection
	if err := conn.Close(); err != nil {
		t.Errorf("error on WebSocket connection close. %#v.", err)
	}
}

func TestDecodePayload(t *testing.T) {
	input := []byte("{\"type\": \"message\", \"channel\": \"C2147483705\", \"user\": \"U2147483697\", \"text\": \"Hello, world!\", \"ts\": \"1355517523.000005\", \"edited\": { \"user\": \"U2147483697\", \"ts\": \"1355517536.000001\"}}")
	if event, err := DefaultPayloadDecoder(input); err == nil {
		if event == nil {
			t.Error("expecting event to be returned, but neither event or error is returned.")
			return
		}

		switch event.(type) {
		case *Message:
			// O.K.
		default:
			t.Errorf("expecting message event's pointer, but was not. %#v.", event)
		}
	} else {
		t.Errorf("expecting event, but error is returned. %#v", err)
	}
}

func TestDecodeReplyPayload(t *testing.T) {
	input := []byte("{\"ok\": true, \"reply_to\": 1, \"ts\": \"1355517523.000005\", \"text\": \"Hello world\"}")
	event, err := DefaultPayloadDecoder(input)

	if err != nil {
		t.Errorf("expecting nil error to be returned, but was %#v", err)
	}

	if event != nil {
		t.Errorf("expecting nil event to be returned, but was %#v", event)
	}
}

func TestDecodeReplyPayloadWithErrorStatus(t *testing.T) {
	input := []byte("{\"ok\": false, \"reply_to\": 1, \"ts\": \"1355517523.000005\", \"text\": \"Hello world\"}")
	event, err := DefaultPayloadDecoder(input)

	switch e := err.(type) {
	case nil:
		t.Errorf("error MUST be returned. returned event is... %#v", event)
	case *ReplyStatusError:
		if *e.Reply.OK {
			t.Error("reply status error is given, but it says OK.")
		}
	default:
		t.Errorf("something wrong with the reply payload decode. returened %#v and %#v", event, err)
	}

	if event != nil {
		t.Errorf("expecting nil event to be returned, but was %#v", event)
	}
}

func TestDecodePayloadWithUnknownFormat(t *testing.T) {
	input := []byte("{\"foo\": \"bar\"}")
	event, err := DefaultPayloadDecoder(input)

	switch err.(type) {
	case nil:
		t.Errorf("error MUST be returned. returned event is... %#v", event)
	case *MalformedPayloadError:
		// O.K.
	default:
		t.Errorf("something wrong with the reply payload decode. returened %#v and %#v", event, err)
	}

	if event != nil {
		t.Errorf("expecting nil event to be returned, but was %#v", event)
	}
}

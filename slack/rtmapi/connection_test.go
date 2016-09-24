package rtmapi

import (
	"encoding/json"
	"fmt"
	"golang.org/x/net/websocket"
	"reflect"
	"testing"
)

func TestDecodePayload(t *testing.T) {
	type output struct {
		payload reflect.Type
		err     reflect.Type
	}
	var decodeTests = []struct {
		input  string
		output output
	}{
		{
			`{"type": "message", "channel": "C2147483705", "user": "U2147483697", "text": "Hello, world!", "ts": "1355517523.000005", "edited": { "user": "U2147483697", "ts": "1355517536.000001"}}`,
			output{
				reflect.TypeOf(&Message{}),
				nil,
			},
		},
		{
			`{"ok": true, "reply_to": 1, "ts": "1355517523.000005", "text": "Hello world"}`,
			output{
				reflect.TypeOf(&WebSocketReply{}),
				nil,
			},
		},
		{
			`{"foo": "bar"}`,
			output{
				nil,
				reflect.TypeOf(&MalformedPayloadError{}),
			},
		},
	}

	for i, testSet := range decodeTests {
		testCnt := i + 1
		inputByte := []byte(testSet.input)
		payload, err := decodePayload(inputByte)

		if testSet.output.payload != reflect.TypeOf(payload) {
			t.Errorf("Test No. %d. expected return type of %s, but was %#v", testCnt, testSet.output.payload.Name(), err)
		}
		if testSet.output.err != reflect.TypeOf(err) {
			t.Errorf("Test No. %d. Expected return error type of %s, but was %#v", testCnt, testSet.output.err.Name(), err)
		}
	}
}

func TestConnWrapper_Send(t *testing.T) {
	once.Do(startServer)

	url := fmt.Sprintf("ws://%s%s", webSocketServerAddress, "/echo")
	conn, err := websocket.Dial(url, "", "http://localhost")
	if err != nil {
		t.Fatal("can't establish connection with test server")
	}
	defer conn.Close()

	connWrapper := newConnectionWrapper(conn)
	if err := connWrapper.Send(&Channel{Name: "dummy channel"}, "hello"); err != nil {
		t.Errorf("error on sending message over WebSocket connection. %#v.", err)
	}
}

func TestConnWrapper_Receive(t *testing.T) {
	once.Do(startServer)

	url := fmt.Sprintf("ws://%s%s", webSocketServerAddress, "/echo")
	conn, err := websocket.Dial(url, "", "http://localhost")
	if err != nil {
		t.Fatal("can't establish connection with test server")
	}
	defer conn.Close()

	type output struct {
		payload reflect.Type
		err     reflect.Type
	}
	testSets := []struct {
		input  string
		output output
	}{
		{
			`{"type": "message", "channel": "C12345", "user": "U6789", "text": "Hello world", "ts": "1355517523.000005"}`,
			output{
				payload: reflect.TypeOf(&Message{}),
				err:     nil,
			},
		},
		{
			`aaaaaaaa`,
			output{
				payload: nil,
				err:     reflect.TypeOf(&json.SyntaxError{}), // invalid character 'a' looking for beginning of value
			},
		},
		{
			" ",
			output{
				payload: nil,
				err:     reflect.TypeOf(&json.SyntaxError{}), // unexpected end of JSON input
			},
		},
	}

	connWrapper := newConnectionWrapper(conn)
	for i, testSet := range testSets {
		testCnt := i + 1
		conn.Write([]byte(testSet.input))
		decodedPayload, err := connWrapper.Receive()

		if testSet.output.payload != reflect.TypeOf(decodedPayload) {
			t.Errorf("Test No. %d. expected return type of %s, but was %#v", testCnt, testSet.output.payload.Name(), err)
		}
		if testSet.output.err != reflect.TypeOf(err) {
			t.Errorf("Test No. %d. Expected return error type of %s, but was %#v", testCnt, testSet.output.err.Name(), err)
		}
	}
}

func TestConnWrapper_Close(t *testing.T) {
	once.Do(startServer)

	url := fmt.Sprintf("ws://%s%s", webSocketServerAddress, "/echo")
	conn, err := websocket.Dial(url, "", "http://localhost")
	if err != nil {
		t.Fatal("can't establish connection with test server")
	}

	connWrapper := newConnectionWrapper(conn)

	if err := connWrapper.Close(); err != nil {
		t.Fatal("error on connection close")
	}

	if err := conn.Close(); err == nil {
		t.Fatal("net.OpError should be returned when WebSocket.Conn.Close is called multiple times.")
	}
}

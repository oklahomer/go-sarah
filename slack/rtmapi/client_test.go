package rtmapi

import (
	"fmt"
	"golang.org/x/net/context"
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
	connection, err := client.Connect(context.TODO(), url)
	if err != nil {
		t.Errorf("webSocket connection error. %#v.", err)
		return
	}

	connWrapper := connection.(*connWrapper)
	if !connWrapper.conn.IsClientConn() {
		t.Fatal("connection is not client originated")
	}
}

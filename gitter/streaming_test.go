package gitter

import (
	"fmt"
	"github.com/jarcoal/httpmock"
	"golang.org/x/net/context"
	"strings"
	"testing"
)

func TestNewStreamingAPIClient(t *testing.T) {
	token := "token"

	client := NewStreamingAPIClient(token)

	if client.token != token {
		t.Errorf("Supplied token is not set: %s.", client.token)
	}
}

func TestNewVersionSpecificStreamingAPIClient(t *testing.T) {
	token := "token"
	version := "v2"

	client := NewVersionSpecificStreamingAPIClient(version, token)

	if client.token != token {
		t.Errorf("Supplied token is not set: %s.", client.token)
	}

	if client.apiVersion != version {
		t.Errorf("Supplied version is not set: %s.", client.token)
	}
}

func TestStreamingAPIClient_buildEndPoint(t *testing.T) {
	version := "v1"
	client := &StreamingAPIClient{
		apiVersion: "v1",
	}

	room := &Room{
		ID: "foo",
	}
	endpoint := client.buildEndpoint(room)

	if !strings.HasPrefix(endpoint.Path, "/"+version) {
		t.Errorf("Buit path does not start with version: %s.", endpoint)
	}
}

func TestStreamingAPIClient_Connect(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	roomID := "foo"
	responder := httpmock.NewStringResponder(200, "OK")
	httpmock.RegisterResponder("GET", fmt.Sprintf("https://stream.gitter.im/v1/rooms/%s/chatMessages", roomID), responder)

	client := &StreamingAPIClient{
		apiVersion: "v1",
		token:      "dummy",
	}

	room := &Room{
		ID: "foo",
	}
	conn, err := client.Connect(context.TODO(), room)

	if err != nil {
		t.Fatalf("Unexpected error returned: %s.", err.Error())
	}

	if conn == nil {
		t.Error("Connection is not returned.")
	}
}

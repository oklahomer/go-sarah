package gitter

import (
	"fmt"
	"github.com/jarcoal/httpmock"
	"golang.org/x/net/context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestNewRestAPIClient(t *testing.T) {
	token := "dummy"
	client := NewRestAPIClient(token)

	if client == nil {
		t.Fatalf("Client is not returned.")
	}

	if client.token != token {
		t.Errorf("Supplied token is not set: %s.", client.token)
	}
}

func TestNewVersionSpecificRestAPIClient(t *testing.T) {
	token := "dummy"
	version := "v2"

	client := NewVersionSpecificRestAPIClient(token, version)

	if client == nil {
		t.Fatalf("Client is not returned.")
	}

	if client.token != token {
		t.Errorf("Supplied token is not set: %s.", client.token)
	}

	if client.apiVersion != version {
		t.Errorf("Supplied version is not set: %s.", client.apiVersion)
	}
}

func TestRestAPIClient_buildEndPoint(t *testing.T) {
	version := "v1"
	client := &RestAPIClient{
		apiVersion: "v1",
	}

	endpoint := client.buildEndpoint([]string{"path"})
	if !strings.HasPrefix(endpoint.Path, "/"+version) {
		t.Errorf("Buit path does not start with version: %s.", endpoint)
	}
}

func TestRestAPIClient_Get(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	type GetResponseDummy struct {
		Foo string
	}

	response := &GetResponseDummy{
		Foo: "foo",
	}
	responder, _ := httpmock.NewJsonResponder(200, response)
	httpmock.RegisterResponder("GET", "https://api.gitter.im/v1/bar", responder)

	client := &RestAPIClient{
		token:      "buzz",
		apiVersion: "v1",
	}

	returned := &GetResponseDummy{}
	err := client.Get(context.TODO(), []string{"bar"}, returned)

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	if returned.Foo != response.Foo {
		t.Errorf("Expected value is not returned: %s.", returned.Foo)
	}
}

func TestClient_Get_StatusError(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	type GetResponseDummy struct {
		Foo string
	}

	statusCode := 404
	responder := func(req *http.Request) (*http.Response, error) {
		resp := httpmock.NewStringResponse(statusCode, "foo bar")
		resp.Request = req // To let *http.Response.Request work
		return resp, nil
	}
	httpmock.RegisterResponder("GET", "https://api.gitter.im/v1/foo", responder)

	client := &RestAPIClient{
		token:      "buzz",
		apiVersion: "v1",
	}
	returned := &GetResponseDummy{}
	err := client.Get(context.TODO(), []string{"foo"}, returned)

	if err == nil {
		t.Errorf("error should return when %d is given.", statusCode)
	}
}

func TestRestAPIClient_Post(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	type PostResponseDummy struct {
		OK bool
	}

	responder, _ := httpmock.NewJsonResponder(200, &PostResponseDummy{OK: true})
	httpmock.RegisterResponder("POST", "https://api.gitter.im/v1/bar", responder)

	client := &RestAPIClient{
		token:      "bar",
		apiVersion: "v1",
	}
	returned := &PostResponseDummy{OK: true}
	err := client.Post(context.TODO(), []string{"bar"}, url.Values{}, returned)

	if err != nil {
		t.Errorf("something is wrong. %#v", err)
	}

	if !returned.OK {
		t.Error("Expected value is not returned")
	}
}

func TestRestAPIClient_Rooms(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	response := &Rooms{
		&Room{
			LastAccessTime: TimeStamp{
				OriginalValue: "2015-04-08T07:06:00.000Z",
			},
		},
	}
	responder, _ := httpmock.NewJsonResponder(200, response)
	httpmock.RegisterResponder("GET", "https://api.gitter.im/v1/rooms", responder)

	client := &RestAPIClient{
		token:      "buzz",
		apiVersion: "v1",
	}

	rooms, err := client.Rooms(context.TODO())

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	if rooms == nil {
		t.Fatal("Expected payload is not returned.")
	}
}

func TestRestAPIClient_PostMessage(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	roomID := "123"
	response := &Message{
		SendTimeStamp: TimeStamp{
			OriginalValue: "2015-04-08T07:06:00.000Z",
		},
		EditTimeStamp: TimeStamp{
			OriginalValue: "2015-04-08T07:06:00.000Z",
		},
	}
	responder, _ := httpmock.NewJsonResponder(200, response)
	httpmock.RegisterResponder("POST", fmt.Sprintf("https://api.gitter.im/v1/rooms/%s/chatMessages", roomID), responder)

	client := &RestAPIClient{
		token:      "bar",
		apiVersion: "v1",
	}

	room := &Room{
		ID: roomID,
		LastAccessTime: TimeStamp{
			OriginalValue: "2015-04-08T07:06:00.000Z",
		},
	}
	message, err := client.PostMessage(context.TODO(), room, "dummy")

	if err != nil {
		t.Errorf("something is wrong. %#v", err)
	}

	if message == nil {
		t.Error("Expected payload is not returned")
	}
}

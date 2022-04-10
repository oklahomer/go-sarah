package gitter

import (
	"context"
	"encoding/json"
	"io"
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
	type GetResponseDummy struct {
		Foo string
	}

	response := &GetResponseDummy{
		Foo: "foo",
	}
	resetClient := switchHTTPClient(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("Unexpected request method: %s.", req.Method)
		}

		bytes, err := json.Marshal(response)
		if err != nil {
			t.Fatalf("Unexpected json marshal error: %s.", err.Error())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(bytes))),
		}, nil
	})
	defer resetClient()

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
	resetClient := switchHTTPClient(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("Unexpected request method: %s.", req.Method)
		}

		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader("foo bar")),
		}, nil
	})
	defer resetClient()

	client := &RestAPIClient{
		token:      "buzz",
		apiVersion: "v1",
	}
	returned := struct{}{}
	err := client.Get(context.TODO(), []string{"foo"}, returned)

	if err == nil {
		t.Errorf("error should return when %d is given.", http.StatusNotFound)
	}
}

func TestRestAPIClient_Post(t *testing.T) {
	type PostResponseDummy struct {
		OK bool
	}

	resetClient := switchHTTPClient(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("Unexpected request method: %s.", req.Method)
		}

		bytes, err := json.Marshal(&PostResponseDummy{OK: true})
		if err != nil {
			t.Fatalf("Unexpected json marshal error: %s.", err.Error())
		}

		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(string(bytes))),
		}, nil
	})
	defer resetClient()

	client := &RestAPIClient{
		token:      "bar",
		apiVersion: "v1",
	}
	returned := &PostResponseDummy{}
	err := client.Post(context.TODO(), []string{"bar"}, url.Values{}, returned)

	if err != nil {
		t.Errorf("something is wrong. %#v", err)
	}

	if !returned.OK {
		t.Error("Expected value is not returned")
	}
}

func TestRestAPIClient_Rooms(t *testing.T) {
	resetClient := switchHTTPClient(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("Unexpected request method: %s.", req.Method)
		}

		response := &Rooms{
			&Room{
				LastAccessTime: TimeStamp{
					OriginalValue: "2015-04-08T07:06:00.000Z",
				},
			},
		}

		bytes, err := json.Marshal(response)
		if err != nil {
			t.Fatalf("Unexpected json marshal error: %s.", err.Error())
		}

		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(string(bytes))),
		}, nil
	})
	defer resetClient()

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
	resetClient := switchHTTPClient(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("Unexpected request method: %s.", req.Method)
		}

		response := &Message{
			SendTimeStamp: TimeStamp{
				OriginalValue: "2015-04-08T07:06:00.000Z",
			},
			EditTimeStamp: TimeStamp{
				OriginalValue: "2015-04-08T07:06:00.000Z",
			},
		}

		bytes, err := json.Marshal(response)
		if err != nil {
			t.Fatalf("Unexpected json marshal error: %s.", err.Error())
		}

		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(string(bytes))),
		}, nil
	})
	defer resetClient()

	client := &RestAPIClient{
		token:      "bar",
		apiVersion: "v1",
	}

	room := &Room{
		ID: "123",
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

type roundTripFnc func(*http.Request) (*http.Response, error)

func (fnc roundTripFnc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fnc(r)
}

func switchHTTPClient(fnc roundTripFnc) func() {
	oldClient := http.DefaultClient

	http.DefaultClient = &http.Client{
		Transport: fnc,
	}

	return func() {
		http.DefaultClient = oldClient
	}
}

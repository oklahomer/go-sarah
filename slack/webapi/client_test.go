package webapi

import (
	"github.com/jarcoal/httpmock"
	"golang.org/x/net/context"
	"net/http"
	"net/url"
	"testing"
)

type GetResponseDummy struct {
	APIResponse
	Foo string
}

func TestGet(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	jsonResponder, _ := httpmock.NewJsonResponder(
		200,
		&GetResponseDummy{
			APIResponse: APIResponse{OK: true},
			Foo:         "bar"})

	httpmock.RegisterResponder(
		"GET",
		"https://slack.com/api/foo",
		jsonResponder)

	client := NewClient(&Config{Token: "abc"})
	dummyResponse := &GetResponseDummy{}
	err := client.Get(context.TODO(), "foo", nil, dummyResponse)

	if err != nil {
		t.Errorf("something went wrong. %#v", err)
	}

	if dummyResponse.OK != true {
		t.Errorf("OK status is wrong. %#v", dummyResponse)
	}

	if dummyResponse.Foo != "bar" {
		t.Errorf("foo value is wrong. %#v", dummyResponse)
	}
}

func TestGetStatusError(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	statusCode := 404
	httpmock.RegisterResponder(
		"GET",
		"https://slack.com/api/foo",
		func(req *http.Request) (*http.Response, error) {
			resp := httpmock.NewStringResponse(statusCode, "foo bar")
			resp.Request = req // To let *http.Response.Request work
			return resp, nil
		})

	client := NewClient(&Config{Token: "abc"})
	dummyResponse := &GetResponseDummy{}
	err := client.Get(context.TODO(), "foo", nil, dummyResponse)

	if err == nil {
		t.Errorf("error should return when %d is given.", statusCode)
	}
}

func TestGetJSONError(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	responder := httpmock.NewStringResponder(200, "invalid json")
	httpmock.RegisterResponder(
		"GET",
		"https://slack.com/api/foo",
		responder)

	client := NewClient(&Config{Token: "abc"})
	dummyResponse := &GetResponseDummy{}
	err := client.Get(context.TODO(), "foo", nil, dummyResponse)

	if err == nil {
		t.Error("error should return")
	}
}

func TestRtmStart(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	jsonResponder, _ := httpmock.NewJsonResponder(
		200,
		&RtmStart{
			APIResponse: APIResponse{OK: true},
			URL:         "https://localhost/foo",
			Self:        nil})

	httpmock.RegisterResponder(
		"GET",
		"https://slack.com/api/rtm.start",
		jsonResponder)

	client := NewClient(&Config{Token: "abc"})
	rtmStart, err := client.RtmStart(context.TODO())

	if err != nil {
		t.Errorf("something went wrong. %#v", err)
	}

	if rtmStart.URL != "https://localhost/foo" {
		t.Errorf("URL is not returned properly. %#v", rtmStart)
	}
}

func TestPost(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	jsonResponder, _ := httpmock.NewJsonResponder(200, &APIResponse{OK: true})

	httpmock.RegisterResponder(
		"POST",
		"https://slack.com/api/foo",
		jsonResponder)

	client := NewClient(&Config{Token: "abc"})
	response := &APIResponse{}
	err := client.Post(context.TODO(), "foo", url.Values{}, response)

	if err != nil {
		t.Errorf("something is wrong. %#v", err)
	}
}

func TestPostStatusError(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	statusCode := 404
	httpmock.RegisterResponder(
		"POST",
		"https://slack.com/api/foo",
		func(req *http.Request) (*http.Response, error) {
			resp := httpmock.NewStringResponse(statusCode, "foo bar")
			resp.Request = req // To let *http.Response.Request work
			return resp, nil
		})

	client := NewClient(&Config{Token: "abc"})
	response := &APIResponse{}
	err := client.Post(context.TODO(), "foo", url.Values{}, response)

	if err == nil {
		t.Errorf("error should return when %d is given.", statusCode)
	}
}

func TestPostMessage(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	response := &APIResponse{OK: true}
	jsonResponder, _ := httpmock.NewJsonResponder(200, response)
	httpmock.RegisterResponder(
		"POST",
		"https://slack.com/api/chat.postMessage",
		jsonResponder)

	postMessage := NewPostMessage("channel", "my message")
	client := NewClient(&Config{Token: "abc"})
	response, err := client.PostMessage(context.TODO(), postMessage)

	if err != nil {
		t.Errorf("something is wrong. %#v", err)
	}

	if response.OK != true {
		t.Errorf("OK status is wrong. %#v", response)
	}
}

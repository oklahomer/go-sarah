package line

import (
	"errors"
	"github.com/jarcoal/httpmock"
	"golang.org/x/net/context"
	"testing"
)

func TestNewConfig(t *testing.T) {
	config := NewConfig()

	if config == nil {
		t.Fatal("Config struct is not retuned.")
	}

	if config.RequestTimeout == 0 {
		t.Error("Timeout value is not set.")
	}

	if config.Token != "" {
		t.Errorf("Token value is set: %s.", config.Token)
	}
}

func TestNew(t *testing.T) {
	config := NewConfig()
	client := New(config)

	if client == nil {
		t.Fatal("Client struct is not returned.")
	}

	if client.config == nil {
		t.Fatal("Config is not set.")
	}
}

func TestClient_Alert(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	responses := []*struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	}{
		{
			Status:  403,
			Message: "forbidden",
		},
		{
			Status:  200,
			Message: "ok",
		},
	}

	var errs []error
	for _, r := range responses {
		responder, _ := httpmock.NewJsonResponder(r.Status, r)
		httpmock.RegisterResponder("POST", Endpoint, responder)

		config := NewConfig()
		client := New(config)
		errs = append(errs, client.Alert(context.TODO(), "DUMMY", errors.New("message")))
	}

	if errs[0] == nil {
		t.Error("Expected error is not returned.")
	}

	if errs[1] != nil {
		t.Errorf("Unexpected error is returned: %s.", errs[1].Error())
	}
}

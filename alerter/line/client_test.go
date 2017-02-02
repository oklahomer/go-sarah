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

	returningResponse := &struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	}{
		Status:  200,
		Message: "ok",
	}
	responder, _ := httpmock.NewJsonResponder(200, returningResponse)
	httpmock.RegisterResponder("POST", Endpoint, responder)

	config := NewConfig()
	client := New(config)
	err := client.Alert(context.TODO(), "DUMMY", errors.New("message"))

	if err != nil {
		t.Errorf("Unexpected error is returned: %s.", err.Error())
	}
}

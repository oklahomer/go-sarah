package slack

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
	"testing"
)

func TestNewConfig(t *testing.T) {
	config := NewConfig()

	if config == nil {
		t.Fatal("config struct is not returned.")
	}

	if config.Token != "" {
		t.Errorf("token must be empty at this point, but was %s.", config.Token)
	}
}

func TestConfigUnmarshalYaml(t *testing.T) {
	config := NewConfig()

	oldQueueSize := config.SendingQueueSize
	token := "myToken"
	queueSize := oldQueueSize + 100
	yamlBytes := []byte(fmt.Sprintf("token: %s\nsending_queue_size: %d", token, queueSize))

	if err := yaml.Unmarshal(yamlBytes, config); err != nil {
		t.Fatalf("error on parsing given YAML structure: %s. %s.", string(yamlBytes), err.Error())
	}

	if config.Token != token {
		t.Errorf("given token was not set: %s.", config.Token)
	}

	if config.SendingQueueSize != queueSize {
		t.Errorf("queue size is not updated with given value: %d.", config.SendingQueueSize)
	}
}

func TestConfigUnmarshalJson(t *testing.T) {
	config := NewConfig()

	oldQueueSize := config.SendingQueueSize
	token := "myToken"
	queueSize := oldQueueSize + 100
	jsonBytes := []byte(fmt.Sprintf(`{"token": "%s", "sending_queue_size": %d}`, token, queueSize))

	if err := json.Unmarshal(jsonBytes, config); err != nil {
		t.Fatalf("error on parsing given JSON structure: %s. %s.", string(jsonBytes), err.Error())
	}

	if config.Token != token {
		t.Errorf("given token was not set: %s.", config.Token)
	}

	if config.SendingQueueSize != queueSize {
		t.Errorf("queue size is not updated with given value: %d.", config.SendingQueueSize)
	}
}

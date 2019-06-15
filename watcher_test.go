package sarah

import (
	"context"
	"strings"
	"testing"
)

func TestConfigNotFoundError_Error(t *testing.T) {
	var botType BotType = "dummy"
	id := "id"
	err := &ConfigNotFoundError{
		BotType: botType,
		ID:      id,
	}

	if !strings.Contains(err.Error(), botType.String()) {
		t.Errorf("Error string does not contain BotType: %s.", err.Error())
	}

	if !strings.Contains(err.Error(), id) {
		t.Errorf("Error string does not contain ID: %s.", err.Error())
	}
}

func TestNullConfigWatcher_Read(t *testing.T) {
	w := &nullConfigWatcher{}
	err := w.Read(context.TODO(), "dummy", "id", &struct{}{})
	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}
}

func TestNullConfigWatcher_Watch(t *testing.T) {
	w := &nullConfigWatcher{}
	err := w.Watch(context.TODO(), "dummy", "id", func() {})
	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}
}

func TestNullConfigWatcher_Unwatch(t *testing.T) {
	w := &nullConfigWatcher{}
	err := w.Unwatch("dummy")
	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}
}

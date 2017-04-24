package gitter

import (
	"github.com/oklahomer/go-sarah"
	"golang.org/x/net/context"
	"reflect"
	"testing"
)

func TestNewStringResponse(t *testing.T) {
	str := "abc"
	res := NewStringResponse(str)

	if res.Content != str {
		t.Errorf("Expected content is not returned: %s.", res.Content)
	}

	if res.UserContext != nil {
		t.Errorf("UserContext should not be returned: %#v.", res.UserContext)
	}
}

func TestNewStringResponseWithNext(t *testing.T) {
	str := "abc"
	next := func(_ context.Context, _ sarah.Input) (*sarah.CommandResponse, error) {
		return nil, nil
	}
	res := NewStringResponseWithNext(str, next)

	if res.Content != str {
		t.Errorf("Expected content is not returned: %s.", res.Content)
	}

	if res.UserContext == nil {
		t.Fatal("Expected UserContxt is not stored.")
	}

	if reflect.ValueOf(res.UserContext.Next).Pointer() != reflect.ValueOf(next).Pointer() {
		t.Fatalf("Expected next step is not returned: %#v.", res.UserContext.Next)
	}
}

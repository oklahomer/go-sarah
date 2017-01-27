package slack

import (
	"github.com/oklahomer/go-sarah"
	"golang.org/x/net/context"
	"testing"
)

func TestNewStringResponse(t *testing.T) {
	str := "abc"
	res := NewStringResponse(str)

	if res.Content != str {
		t.Errorf("expected content is not returned: %s.", res.Content)
	}
}

func TestNewStringResponseWithNext(t *testing.T) {
	str := "abc"
	next := func(_ context.Context, _ sarah.Input) (*sarah.CommandResponse, error) {
		return nil, nil
	}
	res := NewStringResponseWithNext(str, next)

	if res.Content != str {
		t.Errorf("expected content is not returned: %s.", res.Content)
	}

	if res.Next == nil {
		t.Fatalf("expected next step is not returned: %#v.", res.Next)
	}
}

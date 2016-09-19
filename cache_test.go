package sarah

import (
	"golang.org/x/net/context"
	"testing"
	"time"
)

func TestNewCachedUserContexts(t *testing.T) {
	contexts := NewCachedUserContexts(3*time.Minute, 10*time.Minute)
	if contexts.cache == nil {
		t.Fatal("cache is not initialized.")
	}

	if contexts.cache.ItemCount() > 0 {
		t.Fatal("some value is stored by default.")
	}
}

func TestCachedUserContexts_CRUD(t *testing.T) {
	contexts := NewCachedUserContexts(3*time.Minute, 10*time.Minute)

	key := "myKey"
	if empty := contexts.Get(key); empty != nil {
		t.Fatalf("nil should return on empty cache. %#v.", empty)
	}

	contexts.Set(key, NewUserContext(func(ctx context.Context, input BotInput) (*PluginResponse, error) { return nil, nil }))
	if val := contexts.Get(key); val == nil {
		t.Fatal("expected value is not stored")
	}

	contexts.Delete(key)
	if empty := contexts.Get(key); empty != nil {
		t.Fatalf("nil should return after cache deletion. %#v.", empty)
	}

	contexts.Set(key, NewUserContext(func(ctx context.Context, input BotInput) (*PluginResponse, error) { return nil, nil }))
	contexts.Flush()
	if contexts.cache.ItemCount() > 0 {
		t.Fatal("some value is stored after flush.")
	}
}

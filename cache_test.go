package sarah

import (
	"golang.org/x/net/context"
	"testing"
	"time"
)

func TestNewCachedUserContexts(t *testing.T) {
	contexts := NewCachedUserContexts(NewCacheConfig())
	if contexts.cache == nil {
		t.Fatal("Cache is not initialized.")
	}

	if contexts.cache.ItemCount() > 0 {
		t.Fatal("Some value is stored by default.")
	}
}

func TestCachedUserContexts_CRUD(t *testing.T) {
	contexts := NewCachedUserContexts(NewCacheConfig())

	key := "myKey"
	if empty, _ := contexts.Get(key); empty != nil {
		t.Fatalf("nil should return on empty cache. %#v.", empty)
	}

	contexts.Set(key, NewUserContext(func(ctx context.Context, input Input) (*CommandResponse, error) { return nil, nil }))
	if val, _ := contexts.Get(key); val == nil {
		t.Fatal("Expected value is not stored.")
	}

	contexts.Delete(key)
	if empty, _ := contexts.Get(key); empty != nil {
		t.Fatalf("nil should return after cache deletion. %#v.", empty)
	}

	contexts.Set(key, NewUserContext(func(ctx context.Context, input Input) (*CommandResponse, error) { return nil, nil }))
	contexts.Flush()
	if contexts.cache.ItemCount() > 0 {
		t.Fatal("Some value is stored after flush.")
	}

	invalidKey := "invalidStoredType"
	contexts.cache.Set(invalidKey, &struct{}{}, 10*time.Second)
	invalidVal, getErr := contexts.Get(invalidKey)
	if getErr == nil {
		t.Error("Error must be returnd for invalid stored value.")
	}
	if invalidVal != nil {
		t.Errorf("Invalid stored value shouldn't be returned: %T", invalidVal)
	}
}

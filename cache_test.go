package sarah

import (
	"github.com/patrickmn/go-cache"
	"golang.org/x/net/context"
	"testing"
	"time"
)

type DummyCachedUserContexts struct {
	GetFunc    func(string) (*UserContext, error)
	SetFunc    func(string, *UserContext)
	DeleteFunc func(string)
	FlushFunc  func()
}

func (cache *DummyCachedUserContexts) Get(key string) (*UserContext, error) {
	return cache.GetFunc(key)
}

func (cache *DummyCachedUserContexts) Set(key string, userContext *UserContext) {
	cache.SetFunc(key, userContext)
}

func (cache *DummyCachedUserContexts) Delete(key string) {
	cache.DeleteFunc(key)
}

func (cache *DummyCachedUserContexts) Flush() {
	cache.FlushFunc()
}

func TestNewCachedUserContexts(t *testing.T) {
	contexts := NewCachedUserContexts(NewCacheConfig())
	if contexts == nil {
		t.Fatal("Cache is not initialized.")
	}
}

func TestCachedUserContexts_CRUD(t *testing.T) {
	cachedContexts := &CachedUserContexts{
		cache: cache.New(3*time.Minute, 10*time.Minute),
	}

	key := "myKey"
	if empty, _ := cachedContexts.Get(key); empty != nil {
		t.Fatalf("nil should return on empty cache. %#v.", empty)
	}

	cachedContexts.Set(key, NewUserContext(func(ctx context.Context, input Input) (*CommandResponse, error) { return nil, nil }))
	if val, _ := cachedContexts.Get(key); val == nil {
		t.Fatal("Expected value is not stored.")
	}

	cachedContexts.Delete(key)
	if empty, _ := cachedContexts.Get(key); empty != nil {
		t.Fatalf("nil should return after cache deletion. %#v.", empty)
	}

	cachedContexts.Set(key, NewUserContext(func(ctx context.Context, input Input) (*CommandResponse, error) { return nil, nil }))
	cachedContexts.Flush()
	if cachedContexts.cache.ItemCount() > 0 {
		t.Fatal("Some value is stored after flush.")
	}

	invalidKey := "invalidStoredType"
	cachedContexts.cache.Set(invalidKey, &struct{}{}, 10*time.Second)
	invalidVal, getErr := cachedContexts.Get(invalidKey)
	if getErr == nil {
		t.Error("Error must be returned for invalid stored value.")
	}
	if invalidVal != nil {
		t.Errorf("Invalid stored value shouldn't be returned: %T", invalidVal)
	}
}

package sarah

import (
	"github.com/patrickmn/go-cache"
	"golang.org/x/net/context"
	"testing"
	"time"
)

type DummyUserContextStorage struct {
	GetFunc    func(string) (ContextualFunc, error)
	SetFunc    func(string, *UserContext) error
	DeleteFunc func(string) error
	FlushFunc  func() error
}

func (storage *DummyUserContextStorage) Get(key string) (ContextualFunc, error) {
	return storage.GetFunc(key)
}

func (storage *DummyUserContextStorage) Set(key string, userContext *UserContext) error {
	return storage.SetFunc(key, userContext)
}

func (storage *DummyUserContextStorage) Delete(key string) error {
	return storage.DeleteFunc(key)
}

func (storage *DummyUserContextStorage) Flush() error {
	return storage.FlushFunc()
}

func TestNewUserContextStorage(t *testing.T) {
	storage := NewUserContextStorage(NewCacheConfig())
	if storage == nil {
		t.Fatal("Storage is not initialized.")
	}
}

func TestDefaultUserContextStorage_Set_WithEmptyNext(t *testing.T) {
	storage := &defaultUserContextStorage{
		cache: cache.New(3*time.Minute, 10*time.Minute),
	}

	err := storage.Set("key", &UserContext{})

	if err == nil {
		t.Error("Expected error is not returned.")
	}
}

func TestDefaultUserContextStorage_CRUD(t *testing.T) {
	storage := &defaultUserContextStorage{
		cache: cache.New(3*time.Minute, 10*time.Minute),
	}

	key := "myKey"
	if empty, _ := storage.Get(key); empty != nil {
		t.Fatalf("nil should return on empty storage. %#v.", empty)
	}

	storage.Set(key, NewUserContext(func(ctx context.Context, input Input) (*CommandResponse, error) { return nil, nil }))
	if val, _ := storage.Get(key); val == nil {
		t.Fatal("Expected value is not stored.")
	}

	storage.Delete(key)
	if empty, _ := storage.Get(key); empty != nil {
		t.Fatalf("nil should return after deletion. %#v.", empty)
	}

	storage.Set(key, NewUserContext(func(ctx context.Context, input Input) (*CommandResponse, error) { return nil, nil }))
	storage.Flush()
	if storage.cache.ItemCount() > 0 {
		t.Fatal("Some value is stored after flush.")
	}

	invalidKey := "invalidStoredType"
	storage.cache.Set(invalidKey, &struct{}{}, 10*time.Second)
	invalidVal, getErr := storage.Get(invalidKey)
	if getErr == nil {
		t.Error("Error must be returned for invalid stored value.")
	}
	if invalidVal != nil {
		t.Errorf("Invalid stored value shouldn't be returned: %T", invalidVal)
	}
}

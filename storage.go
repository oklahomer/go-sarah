package sarah

import (
	"context"
	"errors"
	"fmt"
	"github.com/patrickmn/go-cache"
	"time"
)

// CacheConfig contains some configuration variables for cache mechanism.
type CacheConfig struct {
	ExpiresIn       time.Duration `json:"expires_in" yaml:"expires_in"`
	CleanupInterval time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"`
}

// NewCacheConfig creates and returns new CacheConfig instance with default settings.
// Use json.Unmarshal, yaml.Unmarshal, or manual manipulation to overload default values.
func NewCacheConfig() *CacheConfig {
	return &CacheConfig{
		ExpiresIn:       3 * time.Minute,
		CleanupInterval: 10 * time.Minute,
	}
}

// ContextualFunc defines a function signature that defines user's next step.
// When a function or instance method is given as CommandResponse.Next, Bot implementation must store this with Input.SenderKey.
// On user's next input, inside of Bot.Respond, Bot retrieves stored ContextualFunc and execute this.
// If CommandResponse.Next is given again as part of result, the same step must be followed.
type ContextualFunc func(context.Context, Input) (*CommandResponse, error)

// SerializableArgument defines user context data to be stored in external storage.
// This data is read from storage on next user input, deserialized, and executed to continue previous conversation.
type SerializableArgument struct {
	FuncIdentifier string
	Argument       interface{}
}

// UserContext represents a user's conversational context.
// If this is returned as part of CommandResponse, user is considered "in the middle of conversation,"
// which means the next input of the user MUST be fed to a function declared in UserContext to continue the conversation.
// This has higher priority than finding and executing Command by checking Command.Match against Input.
//
// Currently this structure supports two forms of context storage.
// See GitHub issue, https://github.com/oklahomer/go-sarah/issues/34, for detailed motives.
type UserContext struct {
	// Next contains a function to be called on next user input.
	// Default implementation of UserContextStorage, defaultUserContextStorage, uses this to store conversational contexts.
	//
	// Since this is a plain function, this is stored in the exact same memory space the Bot is currently running,
	// which means this function can not be shared with other Bot instance or can not be stored in external storage such as Redis.
	// To store user context in externally, set Serializable to store serialized arguments in external storage.
	Next ContextualFunc

	// Serializable, on the other hand, contains arguments and function identifier to be stored in external storage.
	// When user input is given next time, serialized SerializableArgument is fetched from storage, deserialized, and fed to pre-registered function.
	// Pre-registered function is identified by SerializableArgument.FuncIdentifier.
	// A reference implementation is available at https://github.com/oklahomer/go-sarah-rediscontext
	Serializable *SerializableArgument
}

// NewUserContext creates and returns new UserContext with given ContextualFunc.
// Once this instance is stored in Bot's internal storage, the next input from the same user must be fed to this ContextualFunc so the conversation continues.
func NewUserContext(next ContextualFunc) *UserContext {
	return &UserContext{
		Next: next,
	}
}

// UserContextStorage defines an interface of Bot's storage mechanism for users' conversational contexts.
type UserContextStorage interface {
	Get(string) (ContextualFunc, error)
	Set(string, *UserContext) error
	Delete(string) error
	Flush() error
}

// defaultUserContextStorage is the default implementation of UserContexts.
// This stores user contexts in-memory.
type defaultUserContextStorage struct {
	cache *cache.Cache
}

// NewUserContextStorage creates and returns new defaultUserContextStorage instance to store users' conversational contexts.
func NewUserContextStorage(config *CacheConfig) UserContextStorage {
	return &defaultUserContextStorage{
		cache: cache.New(config.ExpiresIn, config.CleanupInterval),
	}
}

// Get searches for user's stored state with given user key, and return it if any found.
func (storage *defaultUserContextStorage) Get(key string) (ContextualFunc, error) {
	val, hasKey := storage.cache.Get(key)
	if !hasKey {
		return nil, nil
	}

	switch v := val.(type) {
	case *UserContext:
		return v.Next, nil

	default:
		return nil, fmt.Errorf("cached value has illegal type of %T", v)

	}
}

// Delete removes currently stored user's conversational context.
// This does nothing if corresponding stored context is not found.
func (storage *defaultUserContextStorage) Delete(key string) error {
	storage.cache.Delete(key)
	return nil
}

// Set stores given UserContext.
// Stored context is tied to given key, which represents a particular user.
func (storage *defaultUserContextStorage) Set(key string, userContext *UserContext) error {
	if userContext.Next == nil {
		return errors.New("required UserContext.Next is not set. defaultUserContextStorage only supports in-memory ContextualFunc cache")
	}

	storage.cache.Set(key, userContext, cache.DefaultExpiration)
	return nil
}

// Flush removes all stored UserContext from its storage.
func (storage *defaultUserContextStorage) Flush() error {
	storage.cache.Flush()
	return nil
}

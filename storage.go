package sarah

import (
	"context"
	"errors"
	"fmt"
	"github.com/patrickmn/go-cache"
	"time"
)

// CacheConfig contains some configuration values for the default UserContextStorage implementation.
type CacheConfig struct {
	// ExpiresIn declares how long a stored UserContext lives.
	ExpiresIn time.Duration `json:"expires_in" yaml:"expires_in"`

	// CleanupInterval declares how often the expired items are removed from the storage.
	// The default UserContextStorage's cache mechanism still holds references to expired values until a cleanup function runs and completely removes the expired values.
	// However, cached items are considered "expired" once the expiration time is over, and they are not returned to the caller even though the value is still cached.
	CleanupInterval time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"`
}

// NewCacheConfig creates and returns a new CacheConfig instance with the default setting values.
// Use json.Unmarshal, yaml.Unmarshal, or manual manipulation to override them.
func NewCacheConfig() *CacheConfig {
	return &CacheConfig{
		ExpiresIn:       3 * time.Minute,
		CleanupInterval: 10 * time.Minute,
	}
}

// ContextualFunc is a function's signature that declares the user's next step.
// When a function or instance method is given as UserContext.Next, Bot implementation must store that with Input.SenderKey to UserContextStorage.
// On the next user input, in Bot.Respond, Bot retrieves the stored ContextualFunc from UserContextStorage and executes this.
type ContextualFunc func(context.Context, Input) (*CommandResponse, error)

// SerializableArgument defines the user context data to be stored in external storage.
// UserContextStorage implementation receives this, serializes this, and stores this to external storage.
type SerializableArgument struct {
	// FuncIdentifier is a unique identifier of the function to be executed on the next user input.
	// A developer needs to register a series of functions beforehand following the UserContextStorage implementation's instruction
	// so the matching function can be fetched by this identifier.
	FuncIdentifier string

	// Argument is an argument to be passed to the function fetched by FuncIdentifier.
	// Therefore, its type must be equal to the one the fetched function receives as an argument.
	Argument interface{}
}

// UserContext represents a user's conversational context.
// If this is returned as part of CommandResponse, the user is considered "in the middle of a conversation,"
// which means the next input of the user MUST be fed to a function declared by UserContext to continue the conversation.
// This conversational context has a higher priority than executing a Command found by checking Command.Match against the user's Input.
//
// Currently, this structure supports two forms of context storage.
// One to store the context in the process memory space; another to store the serialized context in the external storage.
//
// Set one of Next or Serializable depending on the usage and the UserContextStorage implementation.
type UserContext struct {
	// Next contains a function to be called on the next user input.
	// The default implementation of UserContextStorage -- defaultUserContextStorage -- uses this to store conversational contexts.
	//
	// Since this is a plain function, this is stored in the exact same memory space the Bot is currently running,
	// which means this function can not be shared with other Bot instances or can not be stored in external storage such as Redis.
	// To store the user context in external storage, set Serializable and use a UserContextStorage implementation that integrates with external storage.
	Next ContextualFunc

	// Serializable, on contrary to Next, contains a function identifier and its arguments to be stored in external storage.
	// When the user input is given next time, the serialized SerializableArgument is fetched from storage, deserialized,
	// and its arguments are fed to pre-registered function.
	// The pre-registered function is identified by SerializableArgument.FuncIdentifier.
	// A reference implementation is available at https://github.com/oklahomer/go-sarah-rediscontext
	Serializable *SerializableArgument
}

// NewUserContext creates and returns a new UserContext with the given ContextualFunc.
// Once this instance is stored in the Bot's internal storage, the next input from the same user must be passed to this ContextualFunc so the conversation continues.
func NewUserContext(next ContextualFunc) *UserContext {
	return &UserContext{
		Next: next,
	}
}

// UserContextStorage defines an interface of the Bot's storage mechanism to store the users' conversational contexts.
type UserContextStorage interface {
	// Get searches for the user's stored state with the given user key, and return it if one is found.
	Get(string) (ContextualFunc, error)

	// Set stores the given UserContext.
	// The stored context is tied to the given key, which represents a particular user.
	Set(string, *UserContext) error

	// Delete removes a currently stored user's conversational context.
	// This does nothing if a corresponding context is not stored.
	Delete(string) error

	// Flush removes all stored UserContext values.
	Flush() error
}

// defaultUserContextStorage is the default implementation of UserContextStorage.
// This stores user contexts in the process memory space.
type defaultUserContextStorage struct {
	cache *cache.Cache
}

// NewUserContextStorage creates and returns a new defaultUserContextStorage instance to store users' conversational contexts.
func NewUserContextStorage(config *CacheConfig) UserContextStorage {
	return &defaultUserContextStorage{
		cache: cache.New(config.ExpiresIn, config.CleanupInterval),
	}
}

// Get searches for the user's stored state with the given user key, and return it if one is found.
func (storage *defaultUserContextStorage) Get(key string) (ContextualFunc, error) {
	val, hasKey := storage.cache.Get(key)
	if !hasKey || val == nil {
		return nil, nil
	}

	switch v := val.(type) {
	case *UserContext:
		return v.Next, nil

	default:
		return nil, fmt.Errorf("cached value has illegal type of %T", v)

	}
}

// Delete removes a currently stored user's conversational context.
// This does nothing if a corresponding context is not stored.
func (storage *defaultUserContextStorage) Delete(key string) error {
	storage.cache.Delete(key)
	return nil
}

// Set stores the given UserContext.
// The stored context is tied to the given key, which represents a particular user.
func (storage *defaultUserContextStorage) Set(key string, userContext *UserContext) error {
	if userContext.Next == nil {
		return errors.New("required UserContext.Next is not set. defaultUserContextStorage only supports in-memory ContextualFunc cache")
	}

	storage.cache.Set(key, userContext, cache.DefaultExpiration)
	return nil
}

// Flush removes all stored UserContext values.
func (storage *defaultUserContextStorage) Flush() error {
	storage.cache.Flush()
	return nil
}

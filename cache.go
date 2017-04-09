package sarah

import (
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

// UserContext represents a user's conversational context.
// If this is present, user is considered "in the middle of conversation,"
// which means the next input of the user MUST be fed to UserContext.Next to continue the conversation.
// This has higher priority than finding and executing Command by checking Command.Match against Input.
type UserContext struct {
	Next ContextualFunc
}

// NewUserContext creates and returns new UserContext with given ContextualFunc.
// Once this instance is stored in Bot's internal cache, the next input from the same user must be fed to this ContextualFunc so the conversation continues.
func NewUserContext(next ContextualFunc) *UserContext {
	return &UserContext{
		Next: next,
	}
}

// UserContexts defines an interface of Bot's cache mechanism for users' conversational contexts.
type UserContexts interface {
	Get(string) (*UserContext, error)
	Set(string, *UserContext) error
	Delete(string) error
	Flush() error
}

// CachedUserContexts is the default implementation of UserContexts.
// This stores user contexts in-memory.
type CachedUserContexts struct {
	cache *cache.Cache
}

// NewCachedUserContexts creates and returns new CachedUserContexts instance to cache users' conversational contexts.
func NewCachedUserContexts(config *CacheConfig) UserContexts {
	return &CachedUserContexts{
		cache: cache.New(config.ExpiresIn, config.CleanupInterval),
	}
}

// Get searches for cached user's state by given user key, and return if any found.
func (contexts *CachedUserContexts) Get(key string) (*UserContext, error) {
	val, hasKey := contexts.cache.Get(key)
	if !hasKey || val == nil {
		return nil, nil
	}

	switch v := val.(type) {
	case *UserContext:
		return v, nil
	default:
		return nil, fmt.Errorf("cached value has illegal type of %T", v)
	}
}

// Delete removes currently stored user's conversational context.
// This does nothing if corresponding cache is not found.
func (contexts *CachedUserContexts) Delete(key string) error {
	contexts.cache.Delete(key)
	return nil
}

// Set stores given UserContext.
// Stored context is tied to given key, which represents a particular user.
func (contexts *CachedUserContexts) Set(key string, userContext *UserContext) error {
	contexts.cache.Set(key, userContext, cache.DefaultExpiration)
	return nil
}

// Flush removes all stored UserContext from its in-memory cache.
func (contexts *CachedUserContexts) Flush() error {
	contexts.cache.Flush()
	return nil
}

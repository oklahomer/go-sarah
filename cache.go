package sarah

import (
	"fmt"
	"github.com/patrickmn/go-cache"
	"time"
)

type CacheConfig struct {
	ExpiresIn       time.Duration `json:"expires_in" yaml:"expires_in"`
	CleanupInterval time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"`
}

func NewCacheConfig() *CacheConfig {
	return &CacheConfig{
		ExpiresIn:       3 * time.Minute,
		CleanupInterval: 10 * time.Minute,
	}
}

type UserContext struct {
	Next ContextualFunc
}

func NewUserContext(next ContextualFunc) *UserContext {
	return &UserContext{
		Next: next,
	}
}

type UserContexts interface {
	Get(string) (*UserContext, error)
	Set(string, *UserContext)
	Delete(string)
	Flush()
}

type CachedUserContexts struct {
	cache *cache.Cache
}

func NewCachedUserContexts(config *CacheConfig) UserContexts {
	return &CachedUserContexts{
		cache: cache.New(config.ExpiresIn, config.CleanupInterval),
	}
}

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

func (contexts *CachedUserContexts) Delete(key string) {
	contexts.cache.Delete(key)
}

func (contexts *CachedUserContexts) Set(key string, userContext *UserContext) {
	contexts.cache.Set(key, userContext, cache.DefaultExpiration)
}

func (contexts *CachedUserContexts) Flush() {
	contexts.cache.Flush()
}

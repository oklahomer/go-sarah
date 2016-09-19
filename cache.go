package sarah

import (
	"github.com/Sirupsen/logrus"
	"github.com/patrickmn/go-cache"
	"time"
)

type UserContext struct {
	Next ContextualFunc
}

func NewUserContext(next ContextualFunc) *UserContext {
	return &UserContext{
		Next: next,
	}
}

type CachedUserContexts struct {
	cache *cache.Cache
}

func NewCachedUserContexts(expire, cleanupInterval time.Duration) *CachedUserContexts {
	return &CachedUserContexts{
		cache: cache.New(expire, cleanupInterval),
	}
}

func (contexts *CachedUserContexts) Get(key string) *UserContext {
	val, hasKey := contexts.cache.Get(key)
	if !hasKey || val == nil {
		return nil
	}

	switch v := val.(type) {
	case *UserContext:
		return v
	default:
		logrus.Errorf("cached value has illegal type of %#v", v)
		return nil
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

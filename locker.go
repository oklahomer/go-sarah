package sarah

import (
	"fmt"
	"sync"
)

var configLocker = &configRWLocker{
	pluginMutex: map[string]*sync.RWMutex{},
	mutex:       sync.Mutex{},
}

// configRWLocker provides a locking mechanism for Command/ScheduledTask to safely read and write the config struct in a concurrent manner.
// This was introduced to solve a race condition caused by concurrent live re-configuration and Command/ScheduledTask execution.
// Detailed description can be found at https://github.com/oklahomer/go-sarah/issues/44.
//
// Mutex instance is created and managed per Command/ScheduledTask ID because some may share the same identifier
// and hence may refer to the same resource.
type configRWLocker struct {
	pluginMutex map[string]*sync.RWMutex
	mutex       sync.Mutex
}

func (cl *configRWLocker) get(botType BotType, pluginID string) *sync.RWMutex {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	lockID := fmt.Sprintf("botType:%s::id:%s", botType.String(), pluginID)
	locker, ok := cl.pluginMutex[lockID]
	if !ok {
		locker = &sync.RWMutex{}
		cl.pluginMutex[lockID] = locker
	}

	return locker
}

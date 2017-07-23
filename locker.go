package sarah

import (
	"fmt"
	"sync"
)

var configLocker = &configRWLocker{
	fileMutex: map[string]*sync.RWMutex{},
	mutex:     sync.Mutex{},
}

// configRWLocker provides locking mechanism for Command/ScheduledTask to safely read and write config struct.
// This was introduced to solve race condition caused by concurrent live re-configuration and Command/ScheduledTask execution.
// Detailed description can be found at https://github.com/oklahomer/go-sarah/issues/44.
//
// Mutex instance is created and managed per file path
// because ScheduledTask and Command may share same Identifier and hence may refer to same configuration file.
type configRWLocker struct {
	fileMutex map[string]*sync.RWMutex
	mutex     sync.Mutex
}

func (cl *configRWLocker) get(botType BotType, pluginID string) *sync.RWMutex {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	lockID := fmt.Sprintf("botType:%s::id:%s", botType.String(), pluginID)
	locker, ok := cl.fileMutex[lockID]
	if !ok {
		locker = &sync.RWMutex{}
		cl.fileMutex[lockID] = locker
	}

	return locker
}

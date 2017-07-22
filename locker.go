package sarah

import (
	"fmt"
	"path/filepath"
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

func (cl *configRWLocker) get(configDir string, pluginID string) *sync.RWMutex {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	absDir, err := filepath.Abs(configDir)
	if err != nil {
		panic(fmt.Sprintf("failed to get absolute path to configuration files: %s", err.Error()))
	}
	lockID := fmt.Sprintf("dir:%s::id:%s", absDir, pluginID)

	locker, ok := cl.fileMutex[lockID]
	if !ok {
		locker = &sync.RWMutex{}
		cl.fileMutex[lockID] = locker
	}

	return locker
}

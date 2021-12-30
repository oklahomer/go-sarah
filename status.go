package sarah

import (
	"errors"
	"github.com/oklahomer/go-kasumi/logger"
	"sync"
)

var runnerStatus = &status{}

// ErrRunnerAlreadyRunning indicates that Run is already called and the process is running.
// The second or later initiations are prevented by returning this error so the initially activated process is protected.
var ErrRunnerAlreadyRunning = errors.New("go-sarah's process is already running")

// CurrentStatus returns the current status of go-sarah.
// This can still be called even when Run is not called, yet.
// So developers can safely run two different goroutines:
//
//   - One that sets up the bot configuration and calls Run.
//   - Another that periodically calls CurrentStatus and monitors status.
//     When Status.Running is false and Status.Bots field is empty, then the bot is not initiated yet.
func CurrentStatus() Status {
	return runnerStatus.snapshot()
}

// Status represents the current status of Sarah and all registered Bots.
type Status struct {
	// Running indicates if Sarah is currently "running."
	// Sarah is considered running when Run is called and at least one of its belonging Bot is actively running.
	Running bool

	// Bots holds a list of BotStatus values where each value represents its corresponding Bot's status.
	Bots []BotStatus
}

// BotStatus represents the current status of a Bot.
type BotStatus struct {
	// Type represents a BotType the corresponding Bot.BotType returns.
	Type BotType

	// Running indicates if the Bot is currently "running."
	// The Bot is considered running when Bot.Run is already called and its process is context.Context is not yet canceled.
	// When this returns false, the state is final and the Bot is never recovered unless the process is rebooted.
	// In other words, a Bot is "running" even if the connection with the chat service is unstable and recovery is in progress.
	Running bool
}

type status struct {
	bots     []*botStatus
	finished chan struct{}
	mutex    sync.RWMutex
}

func (s *status) running() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	finished := s.finished
	if finished == nil {
		// This status instance is created but Run is not called yet.
		// This channel field is populated when status.start is called via Run.
		return false
	}

	select {
	case <-finished:
		return false

	default:
		return true

	}
}

func (s *status) start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.finished != nil {
		return ErrRunnerAlreadyRunning
	}

	s.finished = make(chan struct{})
	return nil
}

func (s *status) addBot(bot Bot) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	botStatus := &botStatus{
		botType:  bot.BotType(),
		finished: make(chan struct{}),
	}
	s.bots = append(s.bots, botStatus)
}

func (s *status) stopBot(bot Bot) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for _, bs := range s.bots {
		if bs.botType == bot.BotType() {
			bs.stop()
		}
	}
}

func (s *status) snapshot() Status {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var bots []BotStatus
	for _, botStatus := range s.bots {
		bs := BotStatus{
			Type:    botStatus.botType,
			Running: botStatus.running(),
		}
		bots = append(bots, bs)
	}
	return Status{
		Running: s.running(),
		Bots:    bots,
	}
}

func (s *status) stop() {
	defer func() {
		if recover() != nil {
			// O.K.
			// Comes here when channel is already closed.
			// This method is not expected to be called multiple times,
			// but recover here to avoid panic.
			logger.Warn("Multiple status.stop() calls occurred.")
		}
	}()

	close(s.finished)
}

type botStatus struct {
	botType  BotType
	finished chan struct{}
}

func (bs *botStatus) running() bool {
	select {
	case <-bs.finished:
		return false

	default:
		return true

	}
}

func (bs *botStatus) stop() {
	defer func() {
		if recover() != nil {
			// O.K.
			// Comes here when channel is already closed.
			// This method is not expected to be called multiple times,
			// but recover here to avoid panic.
			logger.Warnf("Multiple botStatus.stop() calls for %s occurred.", bs.botType)
		}
	}()

	close(bs.finished)
}

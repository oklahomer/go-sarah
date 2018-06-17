package sarah

import (
	"github.com/oklahomer/go-sarah/log"
	"sync"
)

// Status represents the current status of the bot system including Runner and all registered Bots.
type Status struct {
	Running bool
	Bots    []BotStatus
}

// BotStatus represents the current status of a Bot.
type BotStatus struct {
	Type    BotType
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
		// NewRunner() is called, status instance is created, but Runner.Run() is not called yet.
		// This channel field is populated when status.start() is called via Runner.Run().
		return false
	}

	select {
	case <-finished:
		return false

	default:
		return true

	}
}

func (s *status) start() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.finished = make(chan struct{})
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

	snapshot := Status{
		Running: s.running(),
	}
	for _, botStatus := range s.bots {
		bs := BotStatus{
			Type:    botStatus.botType,
			Running: botStatus.running(),
		}
		snapshot.Bots = append(snapshot.Bots, bs)
	}

	return snapshot
}

func (s *status) stop() {
	defer func() {
		if recover() != nil {
			// O.K.
			// Comes here when channel is already closed.
			// stop() is not expected to be called multiple times,
			// but recover here to avoid panic.
			log.Warn("Multiple status.stop() call occurred.")
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
			// stop() is not expected to be called multiple times,
			// but recover here to avoid panic.
			log.Warnf("Multiple botStatus.stop() call for %s occurred.", bs.botType)
		}
	}()

	close(bs.finished)
}

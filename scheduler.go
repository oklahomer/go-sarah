package sarah

import (
	"context"
	"github.com/oklahomer/go-sarah/v2/log"
	"github.com/robfig/cron/v3"
	"golang.org/x/xerrors"
	"time"
)

type scheduler interface {
	remove(BotType, string)
	update(BotType, ScheduledTask, func()) error
}

type taskScheduler struct {
	cron         *cron.Cron
	removingTask chan *removingTask
	updatingTask chan *updatingTask
}

func (s *taskScheduler) remove(botType BotType, taskID string) {
	remove := &removingTask{
		botType: botType,
		taskID:  taskID,
	}
	s.removingTask <- remove
}

func (s *taskScheduler) update(botType BotType, task ScheduledTask, fn func()) error {
	add := &updatingTask{
		botType: botType,
		task:    task,
		fn:      fn,
		err:     make(chan error, 1),
	}
	s.updatingTask <- add

	return <-add.err
}

type removingTask struct {
	botType BotType
	taskID  string
}

type updatingTask struct {
	botType BotType
	task    ScheduledTask
	fn      func()
	err     chan error
}

func runScheduler(ctx context.Context, location *time.Location) scheduler {
	c := cron.New(cron.WithLocation(location))
	// TODO set logger
	//c.ErrorLog = log.New(...)

	c.Start()

	s := &taskScheduler{
		cron:         c,
		removingTask: make(chan *removingTask, 1),
		updatingTask: make(chan *updatingTask, 1),
	}

	go s.receiveEvent(ctx)

	return s
}

func (s *taskScheduler) receiveEvent(ctx context.Context) {
	schedule := make(map[BotType]map[string]cron.EntryID)
	removeFunc := func(botType BotType, taskID string) {
		botSchedule, ok := schedule[botType]
		if !ok {
			// Task is not registered for the given bot
			return
		}

		storedID, ok := botSchedule[taskID]
		if !ok {
			// Given task is not registered
			return
		}

		delete(botSchedule, taskID)
		s.cron.Remove(storedID)
	}

	for {
		select {
		case <-ctx.Done():
			log.Info("Stop cron jobs due to context cancel.")
			s.cron.Stop()
			return

		case remove := <-s.removingTask:
			removeFunc(remove.botType, remove.taskID)

		case add := <-s.updatingTask:
			if add.task.Schedule() == "" {
				add.err <- xerrors.Errorf("empty schedule is given for %s", add.task.Identifier())
				continue
			}

			removeFunc(add.botType, add.task.Identifier())

			id, err := s.cron.AddFunc(add.task.Schedule(), add.fn)
			if err != nil {
				add.err <- err
				break
			}

			if _, ok := schedule[add.botType]; !ok {
				schedule[add.botType] = make(map[string]cron.EntryID)
			}
			schedule[add.botType][add.task.Identifier()] = id
			add.err <- nil
		}
	}
}

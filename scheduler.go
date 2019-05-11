package sarah

import (
	"github.com/oklahomer/cron"
	"github.com/oklahomer/go-sarah/log"
	"golang.org/x/net/context"
	"golang.org/x/xerrors"
	"time"
)

type scheduler interface {
	remove(BotType, string) error
	update(BotType, ScheduledTask, func()) error
}

type taskScheduler struct {
	cron         *cron.Cron
	removingTask chan *removingTask
	updatingTask chan *updatingTask
}

func (s *taskScheduler) remove(botType BotType, taskID string) error {
	remove := &removingTask{
		botType: botType,
		taskID:  taskID,
		err:     make(chan error, 1),
	}
	s.removingTask <- remove

	return <-remove.err
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
	err     chan error
}

type updatingTask struct {
	botType BotType
	task    ScheduledTask
	fn      func()
	err     chan error
}

func runScheduler(ctx context.Context, location *time.Location) scheduler {
	c := cron.NewWithLocation(location)
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
	removeFunc := func(botType BotType, taskID string) error {
		botSchedule, ok := schedule[botType]
		if !ok {
			return xerrors.Errorf("registered task for %s is not found with ID of %s", botType, taskID)
		}

		storedID, ok := botSchedule[taskID]
		if !ok {
			return xerrors.Errorf("task for %s is not found with ID of %s", botType, taskID)
		}

		delete(botSchedule, taskID)
		s.cron.Remove(storedID)

		return nil
	}

	for {
		select {
		case <-ctx.Done():
			log.Info("Stop cron jobs due to context cancel.")
			s.cron.Stop()
			return

		case remove := <-s.removingTask:
			remove.err <- removeFunc(remove.botType, remove.taskID)

		case add := <-s.updatingTask:
			if add.task.Schedule() == "" {
				add.err <- xerrors.Errorf("empty schedule is given for %s", add.task.Identifier())
				continue
			}

			_ = removeFunc(add.botType, add.task.Identifier())

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

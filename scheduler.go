package sarah

import (
	"context"
	"fmt"
	"github.com/oklahomer/go-kasumi/logger"
	"github.com/robfig/cron/v3"
	"strings"
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
	c := cron.New(cron.WithLocation(location), cron.WithLogger(&cronLogAdapter{l: logger.GetLogger()}))
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
			logger.Info("Stop cron jobs due to context cancellation.")
			s.cron.Stop()
			return

		case remove := <-s.removingTask:
			removeFunc(remove.botType, remove.taskID)

		case add := <-s.updatingTask:
			if add.task.Schedule() == "" {
				add.err <- fmt.Errorf("empty schedule is given for %s", add.task.Identifier())
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

type cronLogAdapter struct {
	l logger.Logger
}

var _ cron.Logger = (*cronLogAdapter)(nil)

func (c *cronLogAdapter) Info(msg string, keysAndValues ...interface{}) {
	converted := c.convertKeyValues(keysAndValues)
	args := append([]interface{}{msg}, converted...)
	format := c.formatString(len(args))
	c.l.Infof(format, args...)
}

func (c *cronLogAdapter) Error(err error, msg string, keysAndValues ...interface{}) {
	converted := c.convertKeyValues(keysAndValues)
	args := append([]interface{}{msg, "error", err}, converted...)
	format := c.formatString(len(args))
	c.l.Errorf(format, args...)
}

func (c *cronLogAdapter) convertKeyValues(keysAndValues []interface{}) []interface{} {
	formatted := make([]interface{}, len(keysAndValues))
	for i, arg := range keysAndValues {
		switch typed := arg.(type) {
		case time.Time:
			str := typed.Format(time.RFC3339)
			formatted[i] = str

		case fmt.Stringer:
			str := typed.String()
			formatted[i] = str

		default:
			formatted[i] = typed

		}
	}

	return formatted
}

func (c *cronLogAdapter) formatString(numKeysAndValues int) string {
	var sb strings.Builder

	// The first argument is always a stringified log message.
	sb.WriteString("%s")

	// Set a delimiter when there more arguments to follow.
	// Note that the first element is already taken care of.
	if numKeysAndValues > 1 {
		sb.WriteString(", ")
	}

	// Arbitrary arguments are to be formatted with %v.
	// robfig/cron's code suggests that the key is always string,
	// but the type is actually interface{}.
	// That library's logger implementation also format with %v, after all.
	for i := range numKeysAndValues / 2 {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("%v=%v")
	}

	return sb.String()
}

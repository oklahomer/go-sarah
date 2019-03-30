package sarah

import (
	"golang.org/x/net/context"
	"testing"
	"time"
)

type DummyScheduler struct {
	RemoveFunc func(BotType, string) error
	UpdateFunc func(BotType, ScheduledTask, func()) error
}

func (s *DummyScheduler) remove(botType BotType, taskID string) error {
	return s.RemoveFunc(botType, taskID)
}

func (s *DummyScheduler) update(botType BotType, task ScheduledTask, fn func()) error {
	return s.UpdateFunc(botType, task, fn)
}

func Test_runScheduler(t *testing.T) {
	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()
	scheduler := runScheduler(ctx, time.UTC)

	if scheduler == nil {
		t.Fatal("scheduler is nil")
	}

	c := scheduler.(*taskScheduler).cron
	if c == nil {
		t.Fatal("Internal cron instance is not initialized")
	}

	loc := c.Location()
	if loc != time.UTC {
		t.Errorf("Assigned time.Location is not set: %s.", loc.String())
	}
}

func TestTaskScheduler_updateAndRemove(t *testing.T) {
	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()
	scheduler := runScheduler(ctx, time.Local)

	taskID := "id"
	task := &scheduledTask{
		identifier: taskID,
		taskFunc: func(_ context.Context, _ ...TaskConfig) ([]*ScheduledTaskResult, error) {
			return nil, nil
		},
		schedule: " ",
	}

	var storedBotType BotType = "Foo"
	if err := scheduler.update(storedBotType, task, func() { return }); err == nil {
		t.Fatal("Error should return on invalid schedule value.")
	}

	task.schedule = "@daily"
	if err := scheduler.update(storedBotType, task, func() { return }); err != nil {
		t.Fatalf("Error is returned on valid schedule value: %s", err.Error())
	}

	jobCnt := len(scheduler.(*taskScheduler).cron.Entries())
	if jobCnt != 1 {
		t.Fatalf("1 job is expected: %d.", jobCnt)
	}

	if err := scheduler.remove("irrelevantBotType", taskID); err == nil {
		t.Fatal("Error should be returned for irrelevant removal.")
	}

	if err := scheduler.remove(storedBotType, "irrelevantID"); err == nil {
		t.Fatal("Error should be returned for irrelevant removal.")
	}

	if err := scheduler.remove(storedBotType, taskID); err != nil {
		t.Fatalf("Error should not be returned for actual removal: %s.", err.Error())
	}

	jobCnt = len(scheduler.(*taskScheduler).cron.Entries())
	if jobCnt != 0 {
		t.Fatalf("0 job is expected: %d.", jobCnt)
	}
}

func TestTaskScheduler_updateWithEmptySchedule(t *testing.T) {
	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()
	scheduler := runScheduler(ctx, time.Local)

	err := scheduler.update("dummy", &DummyScheduledTask{}, func() { return })

	if err == nil {
		t.Error("Expected error is not returned.")
	}
}

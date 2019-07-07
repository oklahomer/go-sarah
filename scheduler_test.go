package sarah

import (
	"context"
	"testing"
	"time"
)

type DummyScheduler struct {
	RemoveFunc func(BotType, string)
	UpdateFunc func(BotType, ScheduledTask, func()) error
}

func (s *DummyScheduler) remove(botType BotType, taskID string) {
	s.RemoveFunc(botType, taskID)
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
	if err := scheduler.update(storedBotType, task, func() {}); err == nil {
		t.Fatal("Error should return on invalid schedule value.")
	}

	task.schedule = "@daily"
	if err := scheduler.update(storedBotType, task, func() {}); err != nil {
		t.Fatalf("Error is returned on valid schedule value: %s", err.Error())
	}
	time.Sleep(10 * time.Millisecond)
	jobCnt := len(scheduler.(*taskScheduler).cron.Entries())
	if jobCnt != 1 {
		t.Fatalf("1 job is expected: %d.", jobCnt)
	}

	// Irrelevant call cause no trouble
	scheduler.remove("irrelevantBotType", taskID)
	scheduler.remove(storedBotType, "irrelevantID")

	// Remove a registered job
	scheduler.remove(storedBotType, taskID)
	time.Sleep(10 * time.Millisecond)
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

	err := scheduler.update("dummy", &DummyScheduledTask{}, func() {})

	if err == nil {
		t.Error("Expected error is not returned.")
	}
}

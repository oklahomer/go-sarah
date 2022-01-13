package sarah

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/oklahomer/go-kasumi/logger"
	"io"
	"io/ioutil"
	"log"
	"strconv"
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

func Test_cronLogAdapter_Info(t *testing.T) {
	buffer := bytes.NewBuffer([]byte{})
	c := &cronLogAdapter{
		l: logger.NewWithStandardLogger(log.New(buffer, "", 0)),
	}

	tests := []struct {
		msg      string
		args     []interface{}
		expected string
	}{
		{
			msg:      "foo",
			args:     []interface{}{},
			expected: "[INFO] foo\n",
		},
		{
			msg: "foo bar",
			args: []interface{}{
				"key1", "value1",
				"key2", "value2",
			},
			expected: "[INFO] foo bar, key1=value1, key2=value2\n",
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, _ = io.Copy(ioutil.Discard, buffer) // Make sure the buffer is empty.

			c.Info(tt.msg, tt.args...)

			passed := buffer.String()
			if passed != tt.expected {
				t.Errorf("Expected string is not passed: %s", passed)
			}
		})
	}
}

type stringer struct {
}

func (s *stringer) String() string {
	return "Hello, 世界"
}

func Test_cronLogger_Error(t *testing.T) {
	buffer := bytes.NewBuffer([]byte{})
	c := &cronLogAdapter{
		l: logger.NewWithStandardLogger(log.New(buffer, "", 0)),
	}

	tests := []struct {
		msg      string
		err      error
		args     []interface{}
		expected string
	}{
		{
			msg:      "foo",
			err:      errors.New("this is an error"),
			args:     []interface{}{},
			expected: "[ERROR] foo, error=this is an error\n",
		},
		{
			msg:      "foo bar",
			err:      fmt.Errorf("this is an error: %w", errors.New("embedded")),
			args:     []interface{}{},
			expected: "[ERROR] foo bar, error=this is an error: embedded\n",
		},
		{
			msg: "foo bar",
			err: fmt.Errorf("this is an error: %w", errors.New("embedded")),
			args: []interface{}{
				"key1",
				"value1",
				"key2",
				"value2",
			},
			expected: "[ERROR] foo bar, error=this is an error: embedded, key1=value1, key2=value2\n",
		},
		{
			msg: "foo bar",
			err: fmt.Errorf("this is an error: %w", errors.New("embedded")),
			args: []interface{}{
				"key", "value",
				"string", &stringer{},
				"time", func() time.Time {
					t, _ := time.Parse(time.RFC3339, "2022-01-09T16:22:00+09:00")
					return t
				}()},
			expected: "[ERROR] foo bar, error=this is an error: embedded, key=value, string=Hello, 世界, time=2022-01-09T16:22:00+09:00\n",
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, _ = io.Copy(ioutil.Discard, buffer) // Make sure the buffer is empty.

			c.Error(tt.err, tt.msg, tt.args...)

			passed := buffer.String()
			if passed != tt.expected {
				t.Errorf("Expected string is not passed: %s", passed)
			}
		})
	}
}

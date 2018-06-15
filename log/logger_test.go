package log

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"sync"
	"testing"
	"time"
)

type DummyLogger struct {
	DebugFunc  func(args ...interface{})
	DebugfFunc func(format string, args ...interface{})

	InfoFunc  func(args ...interface{})
	InfofFunc func(format string, args ...interface{})

	WarnFunc  func(args ...interface{})
	WarnfFunc func(format string, args ...interface{})

	ErrorFunc  func(args ...interface{})
	ErrorfFunc func(format string, args ...interface{})
}

func (l *DummyLogger) Debug(args ...interface{}) {
	l.DebugFunc(args...)
}

func (l *DummyLogger) Debugf(format string, args ...interface{}) {
	l.DebugfFunc(format, args...)
}

func (l *DummyLogger) Info(args ...interface{}) {
	l.InfoFunc(args...)
}

func (l *DummyLogger) Infof(format string, args ...interface{}) {
	l.InfofFunc(format, args...)
}

func (l *DummyLogger) Warn(args ...interface{}) {
	l.WarnFunc(args...)
}

func (l *DummyLogger) Warnf(format string, args ...interface{}) {
	l.WarnfFunc(format, args...)
}

func (l *DummyLogger) Error(args ...interface{}) {
	l.ErrorFunc(args...)
}

func (l *DummyLogger) Errorf(format string, args ...interface{}) {
	l.ErrorfFunc(format, args...)
}

func TestLevel_String(t *testing.T) {
	var DummyLevel Level = 999
	testSets := []struct {
		level Level
		str   string
	}{
		{
			DebugLevel,
			"DEBUG",
		},
		{
			InfoLevel,
			"INFO",
		},
		{
			WarnLevel,
			"WARN",
		},
		{
			ErrorLevel,
			"ERROR",
		},
		{
			DummyLevel,
			"UNKNOWN",
		},
	}

	for i, set := range testSets {
		if set.level.String() != set.str {
			t.Errorf("Expected string value is not returned on test #%d: %s.", i, set.level.String())
		}
	}
}

func Test_newDefaultLogger(t *testing.T) {
	l := newDefaultLogger()

	if l == nil {
		t.Fatal("Instance of defaultLogger is not returned.")
	}

	if _, ok := l.(*defaultLogger); !ok {
		t.Fatalf("Returned instance is not defaultLogger type: %#v.", l)
	}
}

func TestNewWithStandardLogger(t *testing.T) {
	standardLogger := log.New(ioutil.Discard, "", 0)
	l := NewWithStandardLogger(standardLogger)

	if l == nil {
		t.Fatal("Instance of defaultLogger is not returned.")
	}

	if l.(*defaultLogger).logger != standardLogger {
		t.Fatal("Given standard logger is not set.")
	}
}

func TestEachLevel(t *testing.T) {
	b := bytes.NewBuffer([]byte{})
	impl := logger.(*defaultLogger)
	old := impl.logger
	impl.logger = log.New(b, "", 0)
	defer func() {
		impl.logger = old
	}()

	testSets := []struct {
		level   Level
		logFunc func(args ...interface{})
	}{
		// Access via logger instance
		{
			level:   DebugLevel,
			logFunc: logger.Debug,
		},
		{
			level:   InfoLevel,
			logFunc: logger.Info,
		},
		{
			level:   WarnLevel,
			logFunc: logger.Warn,
		},
		{
			level:   ErrorLevel,
			logFunc: logger.Error,
		},

		// Access to pre-set logger statically
		{
			level:   DebugLevel,
			logFunc: Debug,
		},
		{
			level:   InfoLevel,
			logFunc: Info,
		},
		{
			level:   WarnLevel,
			logFunc: Warn,
		},
		{
			level:   ErrorLevel,
			logFunc: Error,
		},
	}

	for i, test := range testSets {
		_, _ = io.Copy(ioutil.Discard, b) // make sure the buffer is reset before each test
		input := "test"
		test.logFunc(input, i)
		expected := fmt.Sprintf("[%s] %s %d\n", test.level.String(), input, i)
		if expected != b.String() {
			t.Errorf("Expected logging output is not given: %s", b.String())
		}
	}
}

func TestEachLevelWithFormat(t *testing.T) {
	b := bytes.NewBuffer([]byte{})
	impl := logger.(*defaultLogger)
	old := impl.logger
	impl.logger = log.New(b, "", 0)
	defer func() {
		impl.logger = old
	}()

	testSets := []struct {
		level   Level
		logFunc func(string, ...interface{})
	}{
		// Access via logger instance
		{
			level:   DebugLevel,
			logFunc: logger.Debugf,
		},
		{
			level:   InfoLevel,
			logFunc: logger.Infof,
		},
		{
			level:   WarnLevel,
			logFunc: logger.Warnf,
		},
		{
			level:   ErrorLevel,
			logFunc: logger.Errorf,
		},

		// Access to pre-set logger statically
		{
			level:   DebugLevel,
			logFunc: Debugf,
		},
		{
			level:   InfoLevel,
			logFunc: Infof,
		},
		{
			level:   WarnLevel,
			logFunc: Warnf,
		},
		{
			level:   ErrorLevel,
			logFunc: Errorf,
		},
	}

	for i, test := range testSets {
		_, _ = io.Copy(ioutil.Discard, b) // make sure the buffer is reset before each test
		input := "test"
		format := "%d : %s"
		test.logFunc(format, i, input)
		expected := fmt.Sprintf("[%s] %s\n", test.level, fmt.Sprintf(format, i, input))
		if expected != b.String() {
			t.Errorf("Expected logging output is not given: %s", b.String())
		}
	}
}

func TestSetOutputLevel(t *testing.T) {
	b := bytes.NewBuffer([]byte{})
	impl := logger.(*defaultLogger)
	oldLogger := impl.logger
	oldOutputLevel := outputLevel
	impl.logger = log.New(b, "", 0)
	defer func() {
		impl.logger = oldLogger
		outputLevel = oldOutputLevel
	}()

	testSets := []struct {
		level   []Level
		logFunc func(string, ...interface{})
	}{
		{
			level:   []Level{DebugLevel},
			logFunc: logger.Debugf,
		},
		{
			level:   []Level{DebugLevel, InfoLevel},
			logFunc: logger.Infof,
		},
		{
			level:   []Level{DebugLevel, InfoLevel, WarnLevel},
			logFunc: logger.Warnf,
		},
		{
			level:   []Level{DebugLevel, InfoLevel, WarnLevel, ErrorLevel},
			logFunc: logger.Errorf,
		},
	}

	for i, test := range testSets {
		for _, level := range []Level{DebugLevel, InfoLevel, WarnLevel, ErrorLevel} {
			SetOutputLevel(level)

			for _, outputAllowedLevel := range test.level {
				_, _ = io.Copy(ioutil.Discard, b) // make sure the buffer is reset before each test
				input := "test"
				format := "%d : %s"
				test.logFunc(format, i, input)
				if level == outputAllowedLevel {
					if b.String() == "" {
						t.Errorf("Log output was expected, but there was none. OutputLevel: %s, Given Level: %s.", outputAllowedLevel.String(), level.String())
					}
				}
			}
		}
	}
}

func TestSetLogger(t *testing.T) {
	old := logger.(*defaultLogger)
	defer func() {
		logger = old
	}()

	newLogger := &DummyLogger{}
	SetLogger(newLogger)

	if logger != newLogger {
		t.Errorf("Assigned logger is not set: %#v.", logger)
	}
}

func Test_concurrentAccess(t *testing.T) {
	impl := logger.(*defaultLogger)
	old := impl.logger
	impl.logger = log.New(ioutil.Discard, "", 0)
	defer func() {
		impl.logger = old
	}()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		levels := []Level{
			DebugLevel, InfoLevel, WarnLevel, ErrorLevel,
		}
		rand.Seed(time.Now().Unix())
		for range make([]int, 1000) {
			SetOutputLevel(levels[rand.Intn(len(levels))])
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		for range make([]int, 1000) {
			newLogger := &DummyLogger{
				DebugFunc: func(args ...interface{}) {
					// O.K.
				},
			}
			SetLogger(newLogger)
		}

	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		for range make([]int, 1000) {
			Debug("foo")
		}
	}()

	wg.Wait()
}

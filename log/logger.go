/*
Package log provides logging mechanism including replaceable Logger interface and its default implementation.
Developers may replace default implementation and output level with her desired Logger implementation in a thread-safe manner as below:

	type MyLogger struct {}

	var _ Logger = (*MyLogger)(nil)

	func (*MyLogger) Debug(args ...interface{}) {}

	func (*MyLogger) Debugf(format string, args ...interface{}) {}

	func (*MyLogger) Info(args ...interface{}) {}

	func (*MyLogger) Infof(format string, args ...interface{}) {}

	func (*MyLogger) Warn(args ...interface{}) {}

	func (*MyLogger) Warnf(format string, args ...interface{}) {}

	func (*MyLogger) Error(args ...interface{}) {}

	func (*MyLogger) Errorf(format string, args ...interface{}) {}

	l := &MyLogger{}

	// These methods are thread-safe
	log.SetLogger(l)
	log.SetOutputLevel(log.InfoLevel)

	log.Info("Output via new Logger impl.")
*/
package log

import (
	"fmt"
	"log"
	"os"
	"sync"
)

// Level indicates what logging level the output is representing.
// This typically indicates the severity of a particular logging event.
type Level uint

var (
	outputLevel = DebugLevel
	logger      = newDefaultLogger()

	// mutex avoids race condition caused by concurrent call to SetLogger(), SetOutputLevel() and logging method.
	mutex sync.RWMutex
)

const (
	// ErrorLevel indicates the error state of events. This must be noted and be fixed.
	// In practical situation, fix may include lowering the corresponding event's log level.
	ErrorLevel Level = iota

	// WarnLevel represents those events that are not critical, but deserves to note.
	// Event with this level may not necessarily be considered as an error;
	// however frequent occurrence deserves developer's attention and may be subject to bug-fix.
	WarnLevel

	// InfoLevel is used to inform what is happening inside the application.
	InfoLevel

	// DebugLevel indicates the output is logged for debugging purpose.
	// This level is not suitable for production usage.
	DebugLevel
)

// String returns the stringified representation of log level.
func (level Level) String() string {
	switch level {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	}

	return "UNKNOWN"
}

// Logger defines the interface that can be used as logging tool in this application.
// Developer may provide a customized logger via SetLogger() to modify behavior.
// By default, instance of defaultLogger is set as default Logger just like http's DefaultClient.
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})

	Info(args ...interface{})
	Infof(format string, args ...interface{})

	Warn(args ...interface{})
	Warnf(format string, args ...interface{})

	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}

type defaultLogger struct {
	logger *log.Logger
}

var _ Logger = (*defaultLogger)(nil)

func (l *defaultLogger) Debug(args ...interface{}) {
	l.out(DebugLevel, args...)
}

func (l *defaultLogger) Debugf(format string, args ...interface{}) {
	l.outf(DebugLevel, format, args...)
}

func (l *defaultLogger) Info(args ...interface{}) {
	l.out(InfoLevel, args...)
}

func (l *defaultLogger) Infof(format string, args ...interface{}) {
	l.outf(InfoLevel, format, args...)
}

func (l *defaultLogger) Warn(args ...interface{}) {
	l.out(WarnLevel, args...)
}

func (l *defaultLogger) Warnf(format string, args ...interface{}) {
	l.outf(WarnLevel, format, args...)
}
func (l *defaultLogger) Error(args ...interface{}) {
	l.out(ErrorLevel, args...)
}

func (l *defaultLogger) Errorf(format string, args ...interface{}) {
	l.outf(ErrorLevel, format, args...)
}

func (l *defaultLogger) out(level Level, args ...interface{}) {
	// combine level identifier and given arguments for variadic function call
	leveledArgs := append([]interface{}{"[" + level.String() + "]"}, args...)
	_ = l.logger.Output(4, fmt.Sprintln(leveledArgs...))
}

func (l *defaultLogger) outf(level Level, format string, args ...interface{}) {
	// combine level identifier and given arguments for variadic function call
	leveledArgs := append([]interface{}{level}, args...)
	_ = l.logger.Output(4, fmt.Sprintf("[%s] "+format, leveledArgs...))
}

func newDefaultLogger() Logger {
	return NewWithStandardLogger(log.New(os.Stdout, "sarah ", log.LstdFlags|log.Llongfile))
}

// NewWithStandardLogger creates an instance of defaultLogger with Go's standard log.Logger.
// This can be used when implementing Logger interface is too much of a task, but still a bit of modification to defaultLogger is required.
//
// Returning Logger can be fed to SetLogger() to replace old defaultLogger.
func NewWithStandardLogger(l *log.Logger) Logger {
	return &defaultLogger{
		logger: l,
	}
}

// SetLogger receives struct that satisfies Logger interface, and sets this as logger.
// From this call forward, any call to logging method proxies arguments to corresponding logging method of given Logger.
// e.g. call to log.Info points to Logger.Info.
//
// This method is "thread-safe."
func SetLogger(l Logger) {
	mutex.Lock()
	defer mutex.Unlock()
	logger = l
}

// GetLogger returns currently set Logger.
// Once a preferred Logger implementation is set via log.SetLogger,
// developer may use its method by calling this package's function: log.Debug, log.Debugf and others.
//
// However when developer wishes to retrieve the Logger instance, this function helps.
// This is particularly useful in such situation where Logger implementation must be temporarily switched but must be
// switched back when a task is done.
// Example follows.
//
//	import (
//		"github.com/oklahomer/go-sarah/log"
//		"io/ioutil"
//		stdLogger "log"
//		"os"
//		"testing"
//	)
//
//	func TestMain(m *testing.M) {
//		oldLogger := log.GetLogger()
//		defer log.SetLogger(oldLogger)
//
//		l := stdLogger.New(ioutil.Discard, "dummyLog", 0)
// 		logger := log.NewWithStandardLogger(l)
//		log.SetLogger(logger)
//
//		code := m.Run()
//
//		os.Exit(code)
//	}
func GetLogger() Logger {
	mutex.RLock()
	defer mutex.RUnlock()
	return logger
}

// SetOutputLevel sets what logging level to output.
// Application may call logging method any time, but Logger only outputs if the corresponding log level is equal to or higher than the level set here.
// e.g. When InfoLevel is set, output with Debug() and Debugf() are ignored.
//
// This method is "thread-safe."
//
// DebugLevel is set by default, so this should be explicitly overridden with higher logging level on production environment
// to avoid printing undesired sensitive data.
func SetOutputLevel(level Level) {
	mutex.Lock()
	defer mutex.Unlock()
	outputLevel = level
}

// Debug outputs given arguments via pre-set Logger implementation.
// Logging level must be left with the default setting or be set to DebugLevel via SetOutputLevel().
func Debug(args ...interface{}) {
	mutex.RLock()
	defer mutex.RUnlock()
	if outputLevel >= DebugLevel {
		logger.Debug(args...)
	}
}

// Debugf outputs given arguments with format via pre-set Logger implementation.
// Logging level must be left with the default setting or be set to DebugLevel via SetOutputLevel().
func Debugf(format string, args ...interface{}) {
	mutex.RLock()
	defer mutex.RUnlock()
	if outputLevel >= DebugLevel {
		logger.Debugf(format, args...)
	}
}

// Info outputs given arguments via pre-set Logger implementation.
// Logging level must be set to DebugLevel or InfoLevel via SetOutputLevel().
func Info(args ...interface{}) {
	mutex.RLock()
	defer mutex.RUnlock()
	if outputLevel >= InfoLevel {
		logger.Info(args...)
	}
}

// Infof outputs given arguments with format via pre-set Logger implementation.
// Logging level must be set to DebugLevel or InfoLevel via SetOutputLevel().
func Infof(format string, args ...interface{}) {
	mutex.RLock()
	defer mutex.RUnlock()
	if outputLevel >= InfoLevel {
		logger.Infof(format, args...)
	}
}

// Warn outputs given arguments via pre-set Logger implementation.
// Logging level must be set to DebugLevel, InfoLevel or WarnLevel via SetOutputLevel().
func Warn(args ...interface{}) {
	mutex.RLock()
	defer mutex.RUnlock()
	if outputLevel >= WarnLevel {
		logger.Warn(args...)
	}
}

// Warnf outputs given arguments with format via pre-set Logger implementation.
// Logging level must be set to DebugLevel, InfoLevel or WarnLevel via SetOutputLevel().
func Warnf(format string, args ...interface{}) {
	mutex.RLock()
	defer mutex.RUnlock()
	if outputLevel >= WarnLevel {
		logger.Warnf(format, args...)
	}
}

// Error outputs given arguments via pre-set Logger implementation.
func Error(args ...interface{}) {
	mutex.RLock()
	defer mutex.RUnlock()
	if outputLevel >= ErrorLevel {
		logger.Error(args...)
	}
}

// Errorf outputs given arguments with format via pre-set Logger implementation.
func Errorf(format string, args ...interface{}) {
	mutex.RLock()
	defer mutex.RUnlock()
	if outputLevel >= ErrorLevel {
		logger.Errorf(format, args...)
	}
}

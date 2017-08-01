/*
Package log provides logging mechanism including replaceable Logger interface and its default implementation.
*/
package log

import (
	"fmt"
	"log"
	"os"
	"sync"
)

// Level indicates what logging level the output is representing.
// This typically indicates the severity of particular logging event.
type Level uint

var (
	outputLevel = DebugLevel
	logger      = newDefaultLogger()
	mutex       = &sync.Mutex{}
)

const (
	// ErrorLevel indicates the error state of events. This must be noted and be fixed.
	// In practical situation, fix may include lowering of the log level.
	ErrorLevel Level = iota

	// WarnLevel represents those events that are not critical, but deserves to note.
	WarnLevel

	// InfoLevel is used to inform what is happening inside the application.
	InfoLevel

	// DebugLevel indicates the output is logged for debugging purpose.
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
// Developer may provide a customized logger via SetLogger to modify behavior.
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
	l.logger.Output(4, fmt.Sprintln(leveledArgs...))
}

func (l *defaultLogger) outf(level Level, format string, args ...interface{}) {
	// combine level identifier and given arguments for variadic function call
	leveledArgs := append([]interface{}{level}, args...)
	l.logger.Output(4, fmt.Sprintf("[%s] "+format, leveledArgs...))
}

func newDefaultLogger() Logger {
	return NewWithStandardLogger(log.New(os.Stdout, "sarah ", log.LstdFlags|log.Llongfile))
}

// NewWithStandardLogger creates an instance of defaultLogger with Go's standard log.Logger.
// This can be used when implementing Logger interface is too much of a task, but still a bit of modification to defaultLogger is required.
//
// Returning Logger can be fed to SetLogger to replace old defaultLogger.
func NewWithStandardLogger(l *log.Logger) Logger {
	return &defaultLogger{
		logger: l,
	}
}

// SetLogger receives struct that satisfies Logger interface, and set this as logger.
// From this call forward, any call to logging method proxies arguments to corresponding logging method of given Logger.
// e.g. call to log.Info points to Logger.Info.
//
// This method is "thread-safe."
func SetLogger(l Logger) {
	mutex.Lock()
	defer mutex.Unlock()
	logger = l
}

// SetOutputLevel sets what logging level to output.
// Application may call logging method any time, but Logger only outputs if the corresponding log level is equal to or higher than the level set here.
func SetOutputLevel(level Level) {
	mutex.Lock()
	defer mutex.Unlock()
	outputLevel = level
}

// Debug outputs given arguments via pre-set Logger implementation.
// Logging level must be set to DebugLevel via logger.SetOutputLevel
func Debug(args ...interface{}) {
	if outputLevel >= DebugLevel {
		logger.Debug(args...)
	}
}

// Debugf outputs given arguments with format via pre-set Logger implementation.
// Logging level must be set to DebugLevel via logger.SetOutputLevel
func Debugf(format string, args ...interface{}) {
	if outputLevel >= DebugLevel {
		logger.Debugf(format, args...)
	}
}

// Info outputs given arguments via pre-set Logger implementation.
// Logging level must be set to DebugLevel or InfoLevel via logger.SetOutputLevel
func Info(args ...interface{}) {
	if outputLevel >= InfoLevel {
		logger.Info(args...)
	}
}

// Infof outputs given arguments with format via pre-set Logger implementation.
// Logging level must be set to DebugLevel or InfoLevel via logger.SetOutputLevel
func Infof(format string, args ...interface{}) {
	if outputLevel >= InfoLevel {
		logger.Infof(format, args...)
	}
}

// Warn outputs given arguments via pre-set Logger implementation.
// Logging level must be set to DebugLevel, InfoLevel or WarnLevel via logger.SetOutputLevel
func Warn(args ...interface{}) {
	if outputLevel >= WarnLevel {
		logger.Warn(args...)
	}
}

// Warnf outputs given arguments with format via pre-set Logger implementation.
// Logging level must be set to DebugLevel, InfoLevel or WarnLevel via logger.SetOutputLevel
func Warnf(format string, args ...interface{}) {
	if outputLevel >= WarnLevel {
		logger.Warnf(format, args...)
	}
}

// Error outputs given arguments via pre-set Logger implementation.
func Error(args ...interface{}) {
	if outputLevel >= ErrorLevel {
		logger.Error(args...)
	}
}

// Errorf outputs given arguments with format via pre-set Logger implementation.
func Errorf(format string, args ...interface{}) {
	if outputLevel >= ErrorLevel {
		logger.Errorf(format, args...)
	}
}

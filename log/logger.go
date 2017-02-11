package log

import (
	"fmt"
	"log"
	"os"
	"sync"
)

// Level indicates what logging level the output is representing.
// This typically indicates the severity of particular logging event.
type Level string

var (
	logger = newDefaultLogger()
	mutex  = &sync.Mutex{}
)

const (
	// DebugLevel indicates the output is logged for debugging purpose.
	DebugLevel Level = "DEBUG"

	// InfoLevel is used to inform what is happening inside the application.
	InfoLevel Level = "INFO"

	// WarnLevel represents those events that are not critical, but deserves to note.
	WarnLevel Level = "WARN"

	// ErrorLevel indicates the error state of events. This must be noted and be fixed.
	// In practical situation, fix may include lowering of the log level.
	ErrorLevel Level = "ERROR"
)

// String returns the stringified representation of log level.
func (level Level) String() string {
	return string(level)
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
	return &defaultLogger{
		logger: log.New(os.Stdout, "sarah ", log.LstdFlags|log.Llongfile),
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

func Debug(args ...interface{}) {
	logger.Debug(args...)
}

func Debugf(format string, args ...interface{}) {
	logger.Debugf(format, args...)
}

func Info(args ...interface{}) {
	logger.Info(args...)
}

func Infof(format string, args ...interface{}) {
	logger.Infof(format, args...)
}

func Warn(args ...interface{}) {
	logger.Warn(args...)
}

func Warnf(format string, args ...interface{}) {
	logger.Warnf(format, args...)
}

func Error(args ...interface{}) {
	logger.Error(args...)
}

func Errorf(format string, args ...interface{}) {
	logger.Errorf(format, args...)
}

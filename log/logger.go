package log

import (
	"fmt"
	"log"
	"os"
	"sync"
)

type LogLevel string

var (
	logger Logger = newDefaultLogger()
	mutex         = &sync.Mutex{}
)

const (
	DebugLevel LogLevel = "DEBUG"
	InfoLevel           = "INFO"
	WarnLevel           = "WARN"
	ErrorLevel          = "ERROR"
)

func (level LogLevel) String() string {
	return string(level)
}

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

func (l *defaultLogger) out(level LogLevel, args ...interface{}) {
	// combine level identifier and given arguments for variadic function call
	leveledArgs := append([]interface{}{"[" + level.String() + "]"}, args...)
	l.logger.Output(4, fmt.Sprintln(leveledArgs...))
}

func (l *defaultLogger) outf(level LogLevel, format string, args ...interface{}) {
	// combine level identifier and given arguments for variadic function call
	leveledArgs := append([]interface{}{level}, args...)
	l.logger.Output(4, fmt.Sprintf("[%s] "+format, leveledArgs...))
}

func newDefaultLogger() Logger {
	return &defaultLogger{
		logger: log.New(os.Stdout, "sarah ", log.LstdFlags|log.Llongfile),
	}
}

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

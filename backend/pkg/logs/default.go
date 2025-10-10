// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/coze-dev/coze-loop/backend/pkg/consts"
)

var logger Logger = newDefaultLogger()

// SetLogger sets the logger.
// Note that this method is not concurrent-safe.
func SetLogger(l Logger) {
	logger = l
}

// SetLogLevel sets the level of logs below which logs will not be output.
// The default log level is LevelInfo.
// Note that this method is not concurrent-safe.
func SetLogLevel(level LogLevel) {
	logger.SetLevel(level)
}

func DefaultLogger() Logger {
	return logger
}

func Debug(format string, v ...interface{}) {
	logger.Debug(format, v...)
}

func Info(format string, v ...interface{}) {
	logger.Info(format, v...)
}

func Warn(format string, v ...interface{}) {
	logger.Warn(format, v...)
}

func Error(format string, v ...interface{}) {
	logger.Error(format, v...)
}

func Fatal(format string, v ...interface{}) {
	logger.Fatal(format, v...)
}

func CtxDebug(ctx context.Context, format string, v ...interface{}) {
	logger.CtxDebug(ctx, format, v...)
}

func CtxInfo(ctx context.Context, format string, v ...interface{}) {
	logger.CtxInfo(ctx, format, v...)
}

func CtxWarn(ctx context.Context, format string, v ...interface{}) {
	logger.CtxWarn(ctx, format, v...)
}

func CtxError(ctx context.Context, format string, v ...interface{}) {
	logger.CtxError(ctx, format, v...)
}

func CtxFatal(ctx context.Context, format string, v ...interface{}) {
	logger.CtxFatal(ctx, format, v...)
}

func NewLogID() string {
	return logger.NewLogID()
}

func GetLogID(ctx context.Context) string {
	return logger.GetLogID(ctx)
}

func SetLogID(ctx context.Context, logID string) context.Context {
	return logger.SetLogID(ctx, logID)
}

// DefaultLogger Default Logger using logrus.
type defaultLogger struct {
	log *logrus.Logger
}

func (l *defaultLogger) NewLogID() string {
	return uuid.New().String()
}

func (l *defaultLogger) GetLogID(ctx context.Context) string {
	logID, _ := ctx.Value(consts.CtxKeyLogID).(string)
	return logID
}

func (l *defaultLogger) SetLogID(ctx context.Context, logID string) context.Context {
	ctx = context.WithValue(ctx, consts.CtxKeyLogID, logID) //nolint:staticcheck
	return ctx
}

func newDefaultLogger() Logger {
	log := logrus.New()
	log.SetFormatter(&customFormatter{})
	log.SetLevel(logrus.InfoLevel)
	return &defaultLogger{log: log}
}

func (l *defaultLogger) GetLevel() LogLevel {
	switch l.log.GetLevel() {
	case logrus.DebugLevel:
		return DebugLevel
	case logrus.InfoLevel:
		return InfoLevel
	case logrus.WarnLevel:
		return WarnLevel
	case logrus.ErrorLevel:
		return ErrorLevel
	case logrus.FatalLevel:
		return FatalLevel
	default:
		return 0
	}
}

func (l *defaultLogger) SetLevel(level LogLevel) {
	var logrusLevel logrus.Level
	switch level {
	case DebugLevel:
		logrusLevel = logrus.DebugLevel
	case InfoLevel:
		logrusLevel = logrus.InfoLevel
	case WarnLevel:
		logrusLevel = logrus.WarnLevel
	case ErrorLevel:
		logrusLevel = logrus.ErrorLevel
	case FatalLevel:
		logrusLevel = logrus.FatalLevel
	}
	l.log.SetLevel(logrusLevel)
}

func (l *defaultLogger) Debug(format string, v ...interface{}) {
	l.log.Debugf(format, v...)
}

func (l *defaultLogger) Info(format string, v ...interface{}) {
	l.log.Infof(format, v...)
}

func (l *defaultLogger) Warn(format string, v ...interface{}) {
	l.log.Warnf(format, v...)
}

func (l *defaultLogger) Error(format string, v ...interface{}) {
	l.log.Errorf(format, v...)
}

func (l *defaultLogger) Fatal(format string, v ...interface{}) {
	l.log.Fatalf(format, v...)
}

func (l *defaultLogger) CtxDebug(ctx context.Context, format string, v ...interface{}) {
	l.log.WithContext(ctx).Debugf(format, v...)
}

func (l *defaultLogger) CtxInfo(ctx context.Context, format string, v ...interface{}) {
	l.log.WithContext(ctx).Infof(format, v...)
}

func (l *defaultLogger) CtxWarn(ctx context.Context, format string, v ...interface{}) {
	l.log.WithContext(ctx).Warnf(format, v...)
}

func (l *defaultLogger) CtxError(ctx context.Context, format string, v ...interface{}) {
	l.log.WithContext(ctx).Errorf(format, v...)
}

func (l *defaultLogger) CtxFatal(ctx context.Context, format string, v ...interface{}) {
	l.log.WithContext(ctx).Fatalf(format, v...)
}

func (l *defaultLogger) Flush() {}

type customFormatter struct{}

func (f *customFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	timestamp := entry.Time.Format("2006-01-02 15:04:05,000")
	level := strings.ToUpper(entry.Level.String())

	skip := 9
	if entry.Context != nil {
		skip = 8
	}
	_, file, line, ok := runtime.Caller(skip)
	if ok {
		file = filepath.Base(file) // 只获取文件名
	}
	var logid any
	logid = ""
	if entry.Context != nil {
		if id := entry.Context.Value(consts.CtxKeyLogID); id != nil {
			logid = id
		}
	}

	logLine := fmt.Sprintf("%s %s %s:%d %s %s\n",
		level,
		timestamp,
		file,
		line,
		logid,
		entry.Message,
	)

	return []byte(logLine), nil
}

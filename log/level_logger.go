package log

import "log/slog"

// LevelLogger is a logger that supports logging with a specified log level.
type LevelLogger struct {
	logger *slog.Logger
}

// NewLevelLogger creates a new LevelLogger.
func NewLevelLogger(logger *slog.Logger) *LevelLogger {
	return &LevelLogger{logger: logger}
}

// Log logs a message with the specified log level.
func (l *LevelLogger) Log(level LogLevel, msg string, args ...any) {
	switch level {
	case DEBUG:
		l.logger.Debug(msg, args...)
	case INFO:
		l.logger.Info(msg, args...)
	case WARN:
		l.logger.Warn(msg, args...)
	case ERROR:
		l.logger.Error(msg, args...)
	}
}

// Debug logs at DEBUG level.
func (l *LevelLogger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

// Info logs at INFO level.
func (l *LevelLogger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

// Warn logs at WARN level.
func (l *LevelLogger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

// Error logs at ERROR level.
func (l *LevelLogger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

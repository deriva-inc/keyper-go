package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// ANSI Color Codes
const (
	ColorTimestamp = "\x1b[38;2;128;128;128m"
	ColorError     = "\x1b[38;2;246;62;2m"
	ColorWarn      = "\x1b[38;2;243;183;0m"
	ColorInfo      = "\x1b[38;2;79;117;155m"
	ColorDebug     = "\x1b[38;2;0;155;114m"
	ColorReset     = "\x1b[0m"
)

// LogLevel type
type LogLevel int

const (
	LevelInfo LogLevel = iota
	LevelWarn
	LevelError
	LevelDebug
)

// Logger struct holds the underlying loggers for different levels.
type Logger struct {
	info  *log.Logger
	warn  *log.Logger
	err   *log.Logger
	debug *log.Logger
}

// New creates a new custom logger instance.
func New() *Logger {
	return &Logger{
		info:  createLogger(os.Stdout, LevelInfo),
		warn:  createLogger(os.Stdout, LevelWarn),
		err:   createLogger(os.Stderr, LevelError), // Errors go to stderr
		debug: createLogger(os.Stdout, LevelDebug),
	}
}

// createLogger is a helper to set up a logger for a specific level.
func createLogger(out io.Writer, level LogLevel) *log.Logger {
	var levelStr, color string
	switch level {
	case LevelInfo:
		levelStr, color = "INFO", ColorInfo
	case LevelWarn:
		levelStr, color = "WARN", ColorWarn
	case LevelError:
		levelStr, color = "ERROR", ColorError
	case LevelDebug:
		levelStr, color = "DEBUG", ColorDebug
	}

	// Format: [TIMESTAMP] | [LEVEL] : message
	prefix := fmt.Sprintf(
		"%s%s%s | %s%s%s : ",
		ColorTimestamp,
		time.Now().Format(time.RFC3339),
		ColorReset,
		color,
		levelStr,
		ColorReset,
	)

	// We use log.Lmsgprefix to ensure the prefix is respected on every line,
	// and 0 for the flag to avoid the standard logger's default timestamp/file info.
	return log.New(out, prefix, log.Lmsgprefix)
}

// Info logs a message with the info level.
func (l *Logger) Info(format string, v ...interface{}) {
	l.info.Printf(colorize(ColorInfo, format), v...)
}

// Warn logs a message with the warn level.
func (l *Logger) Warn(format string, v ...interface{}) {
	l.warn.Printf(colorize(ColorWarn, format), v...)
}

// Error logs a message with the error level.
func (l *Logger) Error(format string, v ...interface{}) {
	l.err.Printf(colorize(ColorError, format), v...)
}

// Debug logs a message with the debug level.
func (l *Logger) Debug(format string, v ...interface{}) {
	l.debug.Printf(colorize(ColorDebug, format), v...)
}

// colorize wraps the given text with a color and a reset code.
func colorize(color, text string) string {
	return fmt.Sprintf("%s%s%s", color, text, ColorReset)
}

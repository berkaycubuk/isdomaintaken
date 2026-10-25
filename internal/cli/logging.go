package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Level represents diagnostic log severity.
// Logs always go to stderr so stdout stays clean for agent parsing.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger writes leveled diagnostic messages to stderr.
// It respects --verbose (shows Debug) and --quiet (shows only Error).
type Logger struct {
	w       io.Writer
	level   Level
	quiet   bool
	verbose bool
}

// NewLogger returns a Logger that writes to the given writer.
// verbose enables Debug. quiet suppresses everything below Error.
func NewLogger(w io.Writer, verbose, quiet bool) *Logger {
	lvl := LevelInfo
	if verbose {
		lvl = LevelDebug
	}
	if quiet {
		lvl = LevelError
	}
	return &Logger{w: w, level: lvl, quiet: quiet, verbose: verbose}
}

// Stderr returns a Logger writing to os.Stderr with the given verbosity flags.
func Stderr(verbose, quiet bool) *Logger {
	return NewLogger(os.Stderr, verbose, quiet)
}

func (l *Logger) shouldLog(target Level) bool {
	return target >= l.level
}

// Debug logs a debug message (shown only with --verbose).
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.shouldLog(LevelDebug) {
		fmt.Fprintf(l.w, "DEBUG: "+format+"\n", args...)
	}
}

// Info logs an informational message (default level, suppressed by --quiet).
func (l *Logger) Info(format string, args ...interface{}) {
	if l.shouldLog(LevelInfo) {
		fmt.Fprintf(l.w, format+"\n", args...)
	}
}

// Warn logs a warning message.
func (l *Logger) Warn(format string, args ...interface{}) {
	if l.shouldLog(LevelWarn) {
		fmt.Fprintf(l.w, "WARNING: "+format+"\n", args...)
	}
}

// Error logs an error message (always shown unless completely silent).
func (l *Logger) Error(format string, args ...interface{}) {
	if l.shouldLog(LevelError) {
		fmt.Fprintf(l.w, "ERROR: "+format+"\n", args...)
	}
}

// Logf logs at the given level with a format string.
func (l *Logger) Logf(level Level, format string, args ...interface{}) {
	if !l.shouldLog(level) {
		return
	}
	prefix := ""
	switch level {
	case LevelDebug:
		prefix = "DEBUG: "
	case LevelWarn:
		prefix = "WARNING: "
	case LevelError:
		prefix = "ERROR: "
	}
	fmt.Fprintf(l.w, prefix+format+"\n", args...)
}

// Progress writes a progress line to stderr by overwriting the current line
// using a carriage return. Useful for progress bars in agent mode.
func Progress(w io.Writer, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(w, "\r%s", strings.TrimRight(msg, "\n"))
}

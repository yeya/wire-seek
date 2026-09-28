// Package output provides a simple leveled output system for CLI applications.
package output

import (
	"fmt"
	"io"
	"os"
)

// Level represents the verbosity level
type Level int

const (
	// LevelQuiet outputs only essential results (for scripting)
	LevelQuiet Level = iota
	// LevelNormal outputs standard progress and results
	LevelNormal
	// LevelVerbose outputs detailed diagnostic information
	LevelVerbose
)

// Logger handles leveled output
type Logger struct {
	level  Level
	out    io.Writer
	errOut io.Writer
}

// New creates a new Logger with the specified verbosity level
func New(level Level) *Logger {
	return &Logger{
		level:  level,
		out:    os.Stdout,
		errOut: os.Stderr,
	}
}

// Level returns the current verbosity level
func (l *Logger) Level() Level {
	return l.level
}

// Debug prints diagnostic output (only in verbose mode)
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level >= LevelVerbose {
		_, _ = fmt.Fprintf(l.out, format, args...)
	}
}

// Info prints standard progress information (normal and verbose modes)
func (l *Logger) Info(format string, args ...interface{}) {
	if l.level >= LevelNormal {
		_, _ = fmt.Fprintf(l.out, format, args...)
	}
}

// Result prints output that is always shown regardless of level
func (l *Logger) Result(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(l.out, format, args...)
}

// Error prints error messages to stderr (always shown)
func (l *Logger) Error(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(l.errOut, format, args...)
}

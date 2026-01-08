package logcapture

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// global is the global log capture instance.
var (
	global   *Capture
	globalMu sync.RWMutex
)

// DefaultLogDir returns the default log directory.
func DefaultLogDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp"
	}
	return filepath.Join(home, ".cache", "dorikin")
}

// DefaultLogFile returns the default debug log file path.
func DefaultLogFile() string {
	return filepath.Join(DefaultLogDir(), "debug.log")
}

// Init initializes the global log capture instance.
// It should be called once at application startup.
func Init(opts Options) (*Capture, error) {
	globalMu.Lock()
	defer globalMu.Unlock()

	if global != nil {
		return global, nil
	}

	global = New(opts)
	return global, nil
}

// Global returns the global Capture instance, creating one if needed.
func Global() *Capture {
	globalMu.RLock()
	if global != nil {
		globalMu.RUnlock()
		return global
	}
	globalMu.RUnlock()

	// Create default instance
	globalMu.Lock()
	defer globalMu.Unlock()

	if global == nil {
		global = New(Options{})
	}
	return global
}

// SetGlobal sets the global capture instance.
// This is useful for testing or custom configurations.
func SetGlobal(c *Capture) {
	globalMu.Lock()
	defer globalMu.Unlock()
	global = c
}

// Shutdown stops the global capture and cleans up resources.
func Shutdown() {
	globalMu.Lock()
	defer globalMu.Unlock()

	if global != nil {
		global.Stop()
		global = nil
	}
}

// Debug logs a debug message.
func Debug(format string, args ...any) {
	if c := Global(); c != nil {
		c.Log(LevelDebug, fmt.Sprintf(format, args...))
	}
}

// Info logs an info message.
func Info(format string, args ...any) {
	if c := Global(); c != nil {
		c.Log(LevelInfo, fmt.Sprintf(format, args...))
	}
}

// Warn logs a warning message.
func Warn(format string, args ...any) {
	if c := Global(); c != nil {
		c.Log(LevelWarn, fmt.Sprintf(format, args...))
	}
}

// Error logs an error message.
func Error(format string, args ...any) {
	if c := Global(); c != nil {
		c.Log(LevelError, fmt.Sprintf(format, args...))
	}
}

// GetEntries returns all log entries from the global capture.
func GetEntries() []Entry {
	if c := Global(); c != nil {
		return c.Entries()
	}
	return nil
}

// GetEntryCount returns the entry count from the global capture.
func GetEntryCount() int {
	if c := Global(); c != nil {
		return c.EntryCount()
	}
	return 0
}

// ClearEntries clears all entries from the global capture.
func ClearEntries() {
	if c := Global(); c != nil {
		c.Clear()
	}
}

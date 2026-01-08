// Package logcapture provides log capture and buffering for the TUI.
// It captures stderr output (including klog) and stores entries in a ring buffer
// that can be displayed in the TUI or written to a file.
package logcapture

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Level represents log severity.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String returns the string representation of the level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Entry represents a single log entry.
type Entry struct {
	Time    time.Time
	Level   Level
	Message string
	Source  string // "klog", "app", "stderr"
}

// Logger is the interface for logging operations.
type Logger interface {
	Log(level Level, msg string)
	Entries() []Entry
	Clear()
	EntryCount() int
}

// Capturer is the interface for stderr capture.
type Capturer interface {
	Start() error
	Stop()
}

// Options configures the log capture system.
type Options struct {
	MaxEntries int    // Ring buffer size (default: 1000)
	LogFile    string // Optional file path for persistent logging
	Debug      bool   // Enable debug level logging
}

// DefaultMaxEntries is the default ring buffer size.
const DefaultMaxEntries = 1000

// Capture implements both Logger and Capturer interfaces.
type Capture struct {
	mu         sync.RWMutex
	entries    []Entry
	maxEntries int
	debug      bool

	// Stderr capture
	oldStderr  *os.File
	pipeReader *os.File
	pipeWriter *os.File
	running    bool
	stopCh     chan struct{}
	wg         sync.WaitGroup

	// File output
	fileWriter *os.File
}

// Ensure Capture implements interfaces.
var (
	_ Logger   = (*Capture)(nil)
	_ Capturer = (*Capture)(nil)
)

// klog pattern: I0107 22:58:06.921062 96218 request.go:752] "message"
var klogPattern = regexp.MustCompile(`^([IWEF])(\d{4}) (\d{2}:\d{2}:\d{2}\.\d+)\s+\d+\s+\S+\]\s*(.*)$`)

// New creates a new Capture instance.
func New(opts Options) *Capture {
	maxEntries := opts.MaxEntries
	if maxEntries <= 0 {
		maxEntries = DefaultMaxEntries
	}

	c := &Capture{
		entries:    make([]Entry, 0, maxEntries),
		maxEntries: maxEntries,
		debug:      opts.Debug,
		stopCh:     make(chan struct{}),
	}

	// Open log file if specified
	if opts.LogFile != "" {
		if err := c.openLogFile(opts.LogFile); err != nil {
			// Log to stderr before capture starts
			fmt.Fprintf(os.Stderr, "warning: could not open log file %s: %v\n", opts.LogFile, err)
		}
	}

	return c
}

// openLogFile opens the log file for writing.
func (c *Capture) openLogFile(path string) error {
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}

	c.fileWriter = f
	return nil
}

// Start begins capturing stderr output.
func (c *Capture) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return nil
	}

	// Create pipe
	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("creating pipe: %w", err)
	}

	// Save old stderr and redirect
	c.oldStderr = os.Stderr
	c.pipeReader = r
	c.pipeWriter = w
	os.Stderr = w

	c.running = true
	c.stopCh = make(chan struct{})

	// Start reader goroutine
	c.wg.Add(1)
	go c.readLoop()

	return nil
}

// Stop stops capturing stderr and restores the original stderr.
func (c *Capture) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.running = false
	c.mu.Unlock()

	// Signal stop
	close(c.stopCh)

	// Restore stderr first so we can write errors if needed
	if c.oldStderr != nil {
		os.Stderr = c.oldStderr
	}

	// Close pipe writer to unblock reader
	if c.pipeWriter != nil {
		c.pipeWriter.Close()
	}

	// Wait for reader to finish
	c.wg.Wait()

	// Close pipe reader
	if c.pipeReader != nil {
		c.pipeReader.Close()
	}

	// Close file writer
	if c.fileWriter != nil {
		c.fileWriter.Close()
	}
}

// readLoop reads from the pipe and processes log lines.
func (c *Capture) readLoop() {
	defer c.wg.Done()

	scanner := bufio.NewScanner(c.pipeReader)
	for scanner.Scan() {
		line := scanner.Text()
		entry := c.parseLine(line)
		c.addEntry(entry)
	}
}

// parseLine parses a log line and returns an Entry.
func (c *Capture) parseLine(line string) Entry {
	entry := Entry{
		Time:    time.Now(),
		Level:   LevelInfo,
		Message: line,
		Source:  "stderr",
	}

	// Try to parse klog format
	if matches := klogPattern.FindStringSubmatch(line); matches != nil {
		entry.Source = "klog"

		// Parse level
		switch matches[1] {
		case "I":
			entry.Level = LevelInfo
		case "W":
			entry.Level = LevelWarn
		case "E":
			entry.Level = LevelError
		case "F":
			entry.Level = LevelError // Fatal as Error
		}

		// Parse time (using today's date + time from log)
		if t, err := time.Parse("15:04:05.000000", matches[3]); err == nil {
			now := time.Now()
			entry.Time = time.Date(now.Year(), now.Month(), now.Day(),
				t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), now.Location())
		}

		entry.Message = strings.TrimSpace(matches[4])
	}

	return entry
}

// addEntry adds an entry to the ring buffer.
func (c *Capture) addEntry(entry Entry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Add to ring buffer
	if len(c.entries) >= c.maxEntries {
		// Remove oldest entry
		c.entries = c.entries[1:]
	}
	c.entries = append(c.entries, entry)

	// Write to file if configured
	if c.fileWriter != nil {
		c.writeToFile(entry)
	}
}

// writeToFile writes an entry to the log file.
func (c *Capture) writeToFile(entry Entry) {
	line := fmt.Sprintf("%s  %-5s  [%s] %s\n",
		entry.Time.Format("2006-01-02 15:04:05.000"),
		entry.Level.String(),
		entry.Source,
		entry.Message,
	)
	_, _ = c.fileWriter.WriteString(line) // Ignore error - best effort logging
}

// Log adds a log entry directly (not from stderr capture).
func (c *Capture) Log(level Level, msg string) {
	// Skip debug messages if debug mode is disabled
	if level == LevelDebug && !c.debug {
		return
	}

	entry := Entry{
		Time:    time.Now(),
		Level:   level,
		Message: msg,
		Source:  "app",
	}
	c.addEntry(entry)
}

// Entries returns a copy of all log entries.
func (c *Capture) Entries() []Entry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]Entry, len(c.entries))
	copy(result, c.entries)
	return result
}

// Clear removes all log entries.
func (c *Capture) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = c.entries[:0]
}

// EntryCount returns the number of entries without copying.
func (c *Capture) EntryCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}

// FilteredEntries returns entries filtered by minimum level.
func (c *Capture) FilteredEntries(minLevel Level) []Entry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []Entry
	for _, e := range c.entries {
		if e.Level >= minLevel {
			result = append(result, e)
		}
	}
	return result
}

package logcapture

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		c := New(Options{})
		if c.maxEntries != DefaultMaxEntries {
			t.Errorf("maxEntries = %d, want %d", c.maxEntries, DefaultMaxEntries)
		}
	})

	t.Run("custom max entries", func(t *testing.T) {
		c := New(Options{MaxEntries: 100})
		if c.maxEntries != 100 {
			t.Errorf("maxEntries = %d, want %d", c.maxEntries, 100)
		}
	})
}

func TestCapture_Log(t *testing.T) {
	c := New(Options{Debug: true})

	c.Log(LevelInfo, "test message")

	entries := c.Entries()
	if len(entries) != 1 {
		t.Fatalf("entries count = %d, want 1", len(entries))
	}

	if entries[0].Level != LevelInfo {
		t.Errorf("level = %v, want %v", entries[0].Level, LevelInfo)
	}
	if entries[0].Message != "test message" {
		t.Errorf("message = %q, want %q", entries[0].Message, "test message")
	}
	if entries[0].Source != "app" {
		t.Errorf("source = %q, want %q", entries[0].Source, "app")
	}
}

func TestCapture_LogDebugDisabled(t *testing.T) {
	c := New(Options{Debug: false})

	c.Log(LevelDebug, "debug message")
	c.Log(LevelInfo, "info message")

	entries := c.Entries()
	if len(entries) != 1 {
		t.Fatalf("entries count = %d, want 1 (debug should be filtered)", len(entries))
	}

	if entries[0].Message != "info message" {
		t.Errorf("message = %q, want %q", entries[0].Message, "info message")
	}
}

func TestCapture_RingBuffer(t *testing.T) {
	c := New(Options{MaxEntries: 5})

	// Add 7 entries
	for i := 0; i < 7; i++ {
		c.Log(LevelInfo, fmt.Sprintf("message %d", i))
	}

	entries := c.Entries()
	if len(entries) != 5 {
		t.Fatalf("entries count = %d, want 5", len(entries))
	}

	// Should have messages 2-6 (oldest 0,1 evicted)
	if entries[0].Message != "message 2" {
		t.Errorf("first entry message = %q, want %q", entries[0].Message, "message 2")
	}
	if entries[4].Message != "message 6" {
		t.Errorf("last entry message = %q, want %q", entries[4].Message, "message 6")
	}
}

func TestCapture_Clear(t *testing.T) {
	c := New(Options{})

	c.Log(LevelInfo, "message 1")
	c.Log(LevelInfo, "message 2")

	if c.EntryCount() != 2 {
		t.Fatalf("entry count before clear = %d, want 2", c.EntryCount())
	}

	c.Clear()

	if c.EntryCount() != 0 {
		t.Errorf("entry count after clear = %d, want 0", c.EntryCount())
	}
}

func TestCapture_FilteredEntries(t *testing.T) {
	c := New(Options{Debug: true})

	c.Log(LevelDebug, "debug")
	c.Log(LevelInfo, "info")
	c.Log(LevelWarn, "warn")
	c.Log(LevelError, "error")

	tests := []struct {
		minLevel Level
		want     int
	}{
		{LevelDebug, 4},
		{LevelInfo, 3},
		{LevelWarn, 2},
		{LevelError, 1},
	}

	for _, tt := range tests {
		t.Run(tt.minLevel.String(), func(t *testing.T) {
			filtered := c.FilteredEntries(tt.minLevel)
			if len(filtered) != tt.want {
				t.Errorf("filtered count = %d, want %d", len(filtered), tt.want)
			}
		})
	}
}

func TestCapture_ThreadSafety(t *testing.T) {
	c := New(Options{MaxEntries: 100})

	var wg sync.WaitGroup
	numGoroutines := 10
	numMessages := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numMessages; j++ {
				c.Log(LevelInfo, fmt.Sprintf("goroutine %d message %d", id, j))
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numMessages; j++ {
				_ = c.Entries()
				_ = c.EntryCount()
			}
		}()
	}

	wg.Wait()

	// Should have maxEntries (ring buffer limit)
	if c.EntryCount() != 100 {
		t.Errorf("entry count = %d, want 100", c.EntryCount())
	}
}

func TestCapture_ParseKlogLine(t *testing.T) {
	c := New(Options{})

	tests := []struct {
		name    string
		line    string
		wantLvl Level
		wantSrc string
		wantMsg string
	}{
		{
			name:    "klog info",
			line:    `I0107 22:58:06.921062 96218 request.go:752] "Waited before sending request"`,
			wantLvl: LevelInfo,
			wantSrc: "klog",
			wantMsg: `"Waited before sending request"`,
		},
		{
			name:    "klog warning",
			line:    `W0107 10:30:00.123456 12345 file.go:100] warning message`,
			wantLvl: LevelWarn,
			wantSrc: "klog",
			wantMsg: "warning message",
		},
		{
			name:    "klog error",
			line:    `E0107 10:30:00.123456 12345 file.go:100] error message`,
			wantLvl: LevelError,
			wantSrc: "klog",
			wantMsg: "error message",
		},
		{
			name:    "plain text",
			line:    "some plain text message",
			wantLvl: LevelInfo,
			wantSrc: "stderr",
			wantMsg: "some plain text message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := c.parseLine(tt.line)

			if entry.Level != tt.wantLvl {
				t.Errorf("level = %v, want %v", entry.Level, tt.wantLvl)
			}
			if entry.Source != tt.wantSrc {
				t.Errorf("source = %q, want %q", entry.Source, tt.wantSrc)
			}
			if entry.Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", entry.Message, tt.wantMsg)
			}
		})
	}
}

func TestCapture_FileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	c := New(Options{
		LogFile: logFile,
		Debug:   true,
	})

	c.Log(LevelInfo, "test message")
	c.Log(LevelError, "error message")

	// Give time for file write
	time.Sleep(10 * time.Millisecond)

	// Close file writer
	if c.fileWriter != nil {
		c.fileWriter.Sync()
	}

	// Read file
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Error("log file is empty")
	}

	contentStr := string(content)
	if !contains(contentStr, "test message") {
		t.Error("log file missing 'test message'")
	}
	if !contains(contentStr, "error message") {
		t.Error("log file missing 'error message'")
	}
}

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("Level.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s != "" && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := range len(s) - len(substr) + 1 {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

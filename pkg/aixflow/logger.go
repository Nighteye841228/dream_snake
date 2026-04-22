package aixflow

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// SmartLogger implements Principle 3: Intelligent Logging.
// It keeps logs in a fixed-size circular buffer and only persists them to disk
// when an explicit error flag is triggered (Dump is called).
//
// The buffer is a true ring: writes are O(1) with zero allocations and constant
// memory footprint regardless of how many messages pass through.
type SmartLogger struct {
	mu     sync.Mutex
	buffer []string // fixed-size slice of length == limit
	head   int      // next write index
	count  int      // number of valid entries (0..limit)
	limit  int
}

// NewSmartLogger initializes a SmartLogger with a fixed-size ring buffer.
// A non-positive limit is clamped to 1 to preserve the invariant that at least
// one slot is always writable.
func NewSmartLogger(limit int) *SmartLogger {
	if limit < 1 {
		limit = 1
	}
	return &SmartLogger{
		buffer: make([]string, limit),
		limit:  limit,
	}
}

// Log records a message to the in-memory ring buffer. O(1), zero disk I/O,
// zero allocations beyond the formatted string itself.
func (l *SmartLogger) Log(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format(time.RFC3339)
	entry := fmt.Sprintf("[%s] %s", timestamp, message)

	l.buffer[l.head] = entry
	l.head = (l.head + 1) % l.limit
	if l.count < l.limit {
		l.count++
	}
}

// Dump persists the buffered logs to the specified file path, oldest first.
// This should be called during the Task's Undo() phase to preserve the
// "crime scene."
func (l *SmartLogger) Dump(path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.count == 0 {
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file for dumping: %w", err)
	}
	defer f.Close()

	header := fmt.Sprintf("\n--- ERROR DETECTED AT %s | DUMPING CRIME SCENE SNAPSHOT ---\n", time.Now().Format(time.RFC3339))
	if _, err := f.WriteString(header); err != nil {
		return err
	}

	// Oldest entry lives `count` slots behind head.
	start := (l.head - l.count + l.limit) % l.limit
	for i := 0; i < l.count; i++ {
		idx := (start + i) % l.limit
		if _, err := f.WriteString(l.buffer[idx] + "\n"); err != nil {
			return err
		}
	}

	l.reset()
	return nil
}

// Clear flushes the ring buffer without writing to disk.
func (l *SmartLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.reset()
}

// reset clears logical state and releases retained strings for GC.
// Must be called with l.mu held.
func (l *SmartLogger) reset() {
	for i := range l.buffer {
		l.buffer[i] = ""
	}
	l.head = 0
	l.count = 0
}

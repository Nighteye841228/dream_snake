package aixflow

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// SmartLogger implements Principle 3: Intelligent Logging.
// It keeps logs in an in-memory buffer and only persists them to disk
// when an explicit error flag is triggered (Dump is called).
type SmartLogger struct {
	mu     sync.Mutex
	buffer []string
	limit  int
}

// NewSmartLogger initializes a SmartLogger with a maximum memory buffer size.
func NewSmartLogger(limit int) *SmartLogger {
	return &SmartLogger{
		buffer: make([]string, 0, limit),
		limit:  limit,
	}
}

// Log records a message to the in-memory buffer. This is a high-speed
// O(1) operation with zero disk I/O.
func (l *SmartLogger) Log(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format(time.RFC3339)
	entry := fmt.Sprintf("[%s] %s", timestamp, message)

	if len(l.buffer) >= l.limit {
		// Simple ring-buffer behavior: drop oldest
		l.buffer = l.buffer[1:]
	}
	l.buffer = append(l.buffer, entry)
}

// Dump persists the buffered logs to the specified file path.
// This should be called during the Task's Undo() phase to preserve the "crime scene."
func (l *SmartLogger) Dump(path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.buffer) == 0 {
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

	for _, entry := range l.buffer {
		if _, err := f.WriteString(entry + "\n"); err != nil {
			return err
		}
	}

	// Clear buffer after dump to prevent redundant writes
	l.buffer = l.buffer[:0]
	return nil
}

// Clear flushes the memory buffer without writing to disk.
func (l *SmartLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buffer = l.buffer[:0]
}

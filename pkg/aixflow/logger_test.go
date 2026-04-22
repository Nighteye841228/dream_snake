package aixflow_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nighteye841228/aix-flow/pkg/aixflow"
)

func TestSmartLogger_LogAndDump(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_error.log")
	
	logger := aixflow.NewSmartLogger(5)

	// 1. Log some messages (In-memory only)
	logger.Log("Event 1")
	logger.Log("Event 2")
	logger.Log("Event 3")

	// Verify file does not exist yet
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatal("Log file should not exist before Dump is called")
	}

	// 2. Dump logs to disk
	err := logger.Dump(logPath)
	if err != nil {
		t.Fatalf("Dump failed: %v", err)
	}

	// 3. Verify content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	strContent := string(content)
	if !strings.Contains(strContent, "Event 1") || !strings.Contains(strContent, "Event 3") {
		t.Errorf("Log content missing expected events: %s", strContent)
	}
}

func TestSmartLogger_RingBufferBehavior(t *testing.T) {
	// Limit is 3
	logger := aixflow.NewSmartLogger(3)

	logger.Log("E1")
	logger.Log("E2")
	logger.Log("E3")
	logger.Log("E4") // This should push out E1

	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "ring.log")
	logger.Dump(logPath)

	content, _ := os.ReadFile(logPath)
	strContent := string(content)

	if strings.Contains(strContent, "E1") {
		t.Error("E1 should have been dropped by the ring buffer")
	}
	if !strings.Contains(strContent, "E4") {
		t.Error("E4 should be present in the log")
	}
}

func TestSmartLogger_Clear(t *testing.T) {
	logger := aixflow.NewSmartLogger(10)
	logger.Log("Sensitive data")
	logger.Clear()

	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "clear.log")
	logger.Dump(logPath)

	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Error("Log file should not be created if buffer was cleared")
	}
}

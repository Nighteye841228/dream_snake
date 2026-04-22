package downloader_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nighteye841228/aix-flow/pkg/aixflow"
	"github.com/nighteye841228/aix-flow/pkg/downloader"
)

func TestDownloadTask_Success(t *testing.T) {
	// Mock an HTTP server that supports Range requests.
	content := "HelloWorld" // 10 bytes
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "bytes=0-4" {
			w.Header().Set("Content-Range", "bytes 0-4/10")
			w.WriteHeader(http.StatusPartialContent)
			io.WriteString(w, content[0:5]) // Returns "Hello"
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "chunk_0.tmp")

	chunk := downloader.Chunk{Index: 0, Start: 0, End: 4, Size: 5}
	task := downloader.NewDownloadTask(ts.URL, chunk, tempFile)
	runner := aixflow.NewAtomicRunner()

	err := runner.Run(context.Background(), task)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify temp file content (Side effects are isolated to the temp file).
	data, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read temp file: %v", err)
	}
	if string(data) != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", string(data))
	}
}

func TestDownloadTask_FailureTriggersUndo(t *testing.T) {
	// Mock a server that returns a 500 Internal Server Error.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "chunk_1.tmp")

	// Pre-create a file to simulate a partial download artifact.
	os.WriteFile(tempFile, []byte("partial..."), 0644)

	chunk := downloader.Chunk{Index: 1, Start: 5, End: 9, Size: 5}
	task := downloader.NewDownloadTask(ts.URL, chunk, tempFile)
	runner := aixflow.NewAtomicRunner()

	err := runner.Run(context.Background(), task)
	if err == nil {
		t.Fatal("Expected error, got success")
	}

	// Verify Undo success: the temporary file must be deleted (Principle 2: Partial Rollback).
	if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
		t.Errorf("Expected temp file to be deleted by Undo, but it still exists")
	}
}

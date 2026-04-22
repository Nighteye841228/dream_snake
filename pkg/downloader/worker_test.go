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

	// Verify the published path holds the expected content.
	data, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read published file: %v", err)
	}
	if string(data) != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", string(data))
	}

	// Two-phase contract: the .partial staging file must not survive a success.
	if _, err := os.Stat(task.PartialPath()); !os.IsNotExist(err) {
		t.Errorf("Expected .partial to be renamed away on success, but it still exists")
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

	chunk := downloader.Chunk{Index: 1, Start: 5, End: 9, Size: 5}
	task := downloader.NewDownloadTask(ts.URL, chunk, tempFile)

	// Pre-create a stale .partial to simulate a crashed prior attempt.
	stalePartial := task.PartialPath()
	if err := os.WriteFile(stalePartial, []byte("stale residue..."), 0644); err != nil {
		t.Fatalf("Failed to seed stale partial: %v", err)
	}

	runner := aixflow.NewAtomicRunner()
	err := runner.Run(context.Background(), task)
	if err == nil {
		t.Fatal("Expected error, got success")
	}

	// Undo must purge the staging file (Principle 2: Partial Rollback).
	if _, err := os.Stat(stalePartial); !os.IsNotExist(err) {
		t.Errorf("Expected .partial to be removed by Undo, but it still exists")
	}
}

// TestDownloadTask_PreservesCompletedNeighbour verifies the two-phase contract:
// if a previously completed chunk file already exists at TempPath, a failed
// re-attempt must not destroy it. Anything at TempPath is sacred — only
// .partial is fair game for cleanup.
func TestDownloadTask_PreservesCompletedNeighbour(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "chunk_2.tmp")

	completed := []byte("verified bytes from a successful prior run")
	if err := os.WriteFile(tempFile, completed, 0644); err != nil {
		t.Fatalf("Failed to seed completed file: %v", err)
	}

	chunk := downloader.Chunk{Index: 2, Start: 0, End: 4, Size: 5}
	task := downloader.NewDownloadTask(ts.URL, chunk, tempFile)
	runner := aixflow.NewAtomicRunner()

	if err := runner.Run(context.Background(), task); err == nil {
		t.Fatal("Expected error, got success")
	}

	got, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Completed neighbour file disappeared: %v", err)
	}
	if string(got) != string(completed) {
		t.Errorf("Completed neighbour was mutated by Undo: got %q", string(got))
	}
}

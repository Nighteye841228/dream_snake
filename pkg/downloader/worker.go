package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

// defaultClient is shared across DownloadTasks that do not inject their own client.
// Reusing a single client preserves the underlying connection pool, which matters
// when many chunks target the same host.
var defaultClient = &http.Client{}

// DownloadTask implements the aixflow.Task interface, responsible for
// downloading a single file chunk and writing it to a temporary file.
type DownloadTask struct {
	URL      string
	Chunk    Chunk
	TempPath string

	// Client is optional. When nil, a shared default client is used.
	// [MANUAL INTERVENTION POINT: Transport Tuning]
	// The Senior Engineer should inject a tuned *http.Client (timeouts, MaxIdleConns,
	// proxy, TLS config) when running against production endpoints.
	Client *http.Client
}

// NewDownloadTask initializes a new DownloadTask using the shared default client.
func NewDownloadTask(url string, chunk Chunk, tempPath string) *DownloadTask {
	return &DownloadTask{
		URL:      url,
		Chunk:    chunk,
		TempPath: tempPath,
	}
}

// Execute performs the download logic. It writes data to a temporary path,
// adhering to the principle of side-effect isolation.
func (t *DownloadTask) Execute(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set the Range header for chunked download
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", t.Chunk.Start, t.Chunk.End))

	client := t.Client
	if client == nil {
		client = defaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request execution failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
	}

	// Create temporary file (isolating I/O side effects to this file)
	out, err := os.Create(t.TempPath)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer out.Close()

	// Stream data to the temporary file
	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}

	// Validate written size against expected chunk size
	if written != t.Chunk.Size {
		return fmt.Errorf("downloaded size mismatch: got %d, expected %d", written, t.Chunk.Size)
	}

	return nil
}

// Undo handles cleanup in the event of execution failure (Principle 2: Partial Rollback).
func (t *DownloadTask) Undo(ctx context.Context) error {
	// Remove the temporary file if it exists to restore the environment to a clean state.
	if _, err := os.Stat(t.TempPath); err == nil {
		if rmErr := os.Remove(t.TempPath); rmErr != nil {
			return fmt.Errorf("failed to remove temporary file during undo: %w", rmErr)
		}
	}
	return nil
}

package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

// partialSuffix is appended to TempPath while a chunk is being written. The
// final atomic rename to TempPath only occurs after the downloaded bytes are
// fully flushed and the size matches the expected chunk length.
const partialSuffix = ".partial"

// defaultClient is shared across DownloadTasks that do not inject their own
// client. Reusing a single client preserves the underlying connection pool,
// which matters when many chunks target the same host.
var defaultClient = &http.Client{}

// DownloadTask implements the aixflow.Task interface, responsible for
// downloading a single file chunk to TempPath via a two-phase write:
//
//  1. Stream bytes to TempPath + ".partial".
//  2. Verify the written length matches Chunk.Size.
//  3. os.Rename(.partial, TempPath) — atomic publish on POSIX.
//
// This guarantees that any file present at TempPath represents a complete,
// size-validated download. A crashed or interrupted attempt leaves only a
// .partial file behind, never a half-written TempPath that GetPendingChunks
// would mistake for completed work.
type DownloadTask struct {
	URL      string
	Chunk    Chunk
	TempPath string

	// Client is optional. When nil, a shared default client is used.
	// [MANUAL INTERVENTION POINT: Transport Tuning]
	// The Senior Engineer should inject a tuned *http.Client (timeouts,
	// MaxIdleConns, proxy, TLS config) when running against production
	// endpoints.
	Client *http.Client
}

// NewDownloadTask initializes a new DownloadTask using the shared default
// client.
func NewDownloadTask(url string, chunk Chunk, tempPath string) *DownloadTask {
	return &DownloadTask{
		URL:      url,
		Chunk:    chunk,
		TempPath: tempPath,
	}
}

// PartialPath returns the staging path used during the two-phase write. Exposed
// so callers (resume logic, custodial sweepers) can locate or clean up
// abandoned partial downloads.
func (t *DownloadTask) PartialPath() string {
	return t.TempPath + partialSuffix
}

// Execute performs the download. Side effects are confined to the .partial
// staging file until the size check passes, at which point an atomic rename
// publishes the chunk at TempPath.
func (t *DownloadTask) Execute(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
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

	partial := t.PartialPath()
	out, err := os.Create(partial)
	if err != nil {
		return fmt.Errorf("failed to create partial file: %w", err)
	}

	written, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("failed to write to partial file: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close partial file: %w", closeErr)
	}

	if written != t.Chunk.Size {
		return fmt.Errorf("downloaded size mismatch: got %d, expected %d", written, t.Chunk.Size)
	}

	// Atomic publish: only complete, size-verified bytes ever appear at TempPath.
	if err := os.Rename(partial, t.TempPath); err != nil {
		return fmt.Errorf("failed to publish chunk: %w", err)
	}

	return nil
}

// Undo cleans up the staging file in the event of execution failure (Principle
// 2: Partial Rollback). The published TempPath is intentionally left alone:
// under the two-phase contract, anything at TempPath is a verified completed
// chunk, even across process restarts.
func (t *DownloadTask) Undo(ctx context.Context) error {
	partial := t.PartialPath()
	if _, err := os.Stat(partial); err == nil {
		if rmErr := os.Remove(partial); rmErr != nil {
			return fmt.Errorf("failed to remove partial file during undo: %w", rmErr)
		}
	}
	return nil
}

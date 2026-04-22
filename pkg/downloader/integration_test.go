package downloader_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nighteye841228/aix-flow/pkg/aixflow"
	"github.com/nighteye841228/aix-flow/pkg/downloader"
)

// TestFullDownloadPipeline_Integration validates the complete E2E workflow:
// Calculation -> Concurrent Download -> Merging -> Secure Post-processing.
func TestFullDownloadPipeline_Integration(t *testing.T) {
	const totalSize = 1024 * 1024 // 1 MB
	const chunkSize = 256 * 1024  // 256 KB per chunk
	
	// Generate mock file data.
	var sb strings.Builder
	for sb.Len() < totalSize {
		sb.WriteString("0123456789")
	}
	mockFileData := sb.String()[:totalSize]

	// Initialize mock server with Range support.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			w.Header().Set("Content-Length", strconv.Itoa(totalSize))
			io.WriteString(w, mockFileData)
			return
		}

		parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
		start, _ := strconv.Atoi(parts[0])
		end, _ := strconv.Atoi(parts[1])

		if start >= totalSize || end >= totalSize || start > end {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}

		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, totalSize))
		w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
		w.WriteHeader(http.StatusPartialContent)
		io.WriteString(w, mockFileData[start:end+1])
	}))
	defer ts.Close()

	tempDir := t.TempDir()
	rawMergedFile := filepath.Join(tempDir, "merged_raw.bin")
	finalEncryptedFile := filepath.Join(tempDir, "final_secure.enc")
	aesKey := []byte("12345678901234567890123456789012")

	baseRunner := aixflow.NewAtomicRunner()
	runner := aixflow.NewBudgetedRunner(baseRunner, 5*time.Second)
	ctx := context.Background()

	// Step A: Calculate Chunks
	chunks, err := downloader.CalculateChunks(totalSize, chunkSize)
	if err != nil {
		t.Fatalf("Calculation failed: %v", err)
	}

	// Step B: Download all chunks.
	var chunkPaths []string
	for _, chunk := range chunks {
		chunkPath := filepath.Join(tempDir, fmt.Sprintf("chunk_%d.tmp", chunk.Index))
		chunkPaths = append(chunkPaths, chunkPath)

		task := downloader.NewDownloadTask(ts.URL, chunk, chunkPath)
		if err := runner.Run(ctx, task); err != nil {
			t.Fatalf("Download failed for chunk %d: %v", chunk.Index, err)
		}
	}

	// Step C: Merge chunks.
	mergeTask := downloader.NewMergeTask(chunkPaths, rawMergedFile)
	if err := runner.Run(ctx, mergeTask); err != nil {
		t.Fatalf("Merging failed: %v", err)
	}

	// Step D: Secure Post-processing.
	pipelineTask := downloader.NewPipelineTask(rawMergedFile, finalEncryptedFile, aesKey)
	if err := runner.Run(ctx, pipelineTask); err != nil {
		t.Fatalf("Pipeline execution failed: %v", err)
	}

	// Verify Final Integrity.
	if len(pipelineTask.FinalHash) != 64 {
		t.Errorf("Invalid SHA-256 hash length: %s", pipelineTask.FinalHash)
	}

	t.Logf("Integration Test Passed! Hash: %s", pipelineTask.FinalHash)
}

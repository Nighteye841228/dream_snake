package downloader_test

import (
	"testing"

	"github.com/nighteye841228/aix-flow/pkg/downloader"
)

func TestCalculateChunks(t *testing.T) {
	t.Run("ExactDivision", func(t *testing.T) {
		chunks, err := downloader.CalculateChunks(100, 25)
		if err != nil {
			t.Fatalf("Calculation failed: %v", err)
		}
		if len(chunks) != 4 {
			t.Errorf("Expected 4 chunks, got %d", len(chunks))
		}
		if chunks[0].Start != 0 || chunks[0].End != 24 {
			t.Errorf("Chunk 0 mismatch: %+v", chunks[0])
		}
	})

	t.Run("WithRemainder", func(t *testing.T) {
		chunks, err := downloader.CalculateChunks(10, 3)
		if err != nil {
			t.Fatalf("Calculation failed: %v", err)
		}
		// Expect sizes: 3, 3, 3, 1
		if len(chunks) != 4 {
			t.Errorf("Expected 4 chunks, got %d", len(chunks))
		}
		last := chunks[len(chunks)-1]
		if last.Start != 9 || last.End != 9 || last.Size != 1 {
			t.Errorf("Last chunk mismatch: %+v", last)
		}
	})
}

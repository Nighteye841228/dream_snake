package downloader

import (
	"fmt"
	"os"
)

// Chunk represents a segment of a file to be downloaded.
type Chunk struct {
	Index int   // Unique index of the chunk
	Start int64 // Start byte position
	End   int64 // End byte position
	Size  int64 // Expected size in bytes
}

// CalculateChunks divides a total size into multiple chunks of a given max size.
func CalculateChunks(totalSize, maxChunkSize int64) ([]Chunk, error) {
	if totalSize <= 0 || maxChunkSize <= 0 {
		return nil, fmt.Errorf("invalid size: totalSize=%d, maxChunkSize=%d", totalSize, maxChunkSize)
	}

	var chunks []Chunk
	index := 0
	for start := int64(0); start < totalSize; start += maxChunkSize {
		end := start + maxChunkSize - 1
		if end >= totalSize {
			end = totalSize - 1
		}
		chunks = append(chunks, Chunk{
			Index: index,
			Start: start,
			End:   end,
			Size:  end - start + 1,
		})
		index++
	}
	return chunks, nil
}

// GetPendingChunks filters a list of chunks, returning only those whose 
// corresponding temporary files do not already exist (supporting resumable downloads).
func GetPendingChunks(chunks []Chunk, tempDir string) []Chunk {
	var pending []Chunk
	for _, chunk := range chunks {
		path := fmt.Sprintf("%s/chunk_%d.tmp", tempDir, chunk.Index)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			pending = append(pending, chunk)
		}
	}
	return pending
}

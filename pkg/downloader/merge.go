package downloader

import (
	"context"
	"fmt"
	"io"
	"os"
)

// MergeTask implements the aixflow.Task interface, responsible for consolidating 
// multiple temporary chunk files into a single unified file.
type MergeTask struct {
	ChunkPaths []string
	OutputPath string
}

// NewMergeTask initializes a new MergeTask.
func NewMergeTask(chunkPaths []string, outputPath string) *MergeTask {
	return &MergeTask{
		ChunkPaths: chunkPaths,
		OutputPath: outputPath,
	}
}

// Execute reads each chunk sequentially and writes it to the output file.
func (t *MergeTask) Execute(ctx context.Context) error {
	// Create the unified target file
	outFile, err := os.Create(t.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to create merged output file: %w", err)
	}
	defer outFile.Close()

	// Sequentially read and append each chunk
	for _, chunkPath := range t.ChunkPaths {
		if err := ctx.Err(); err != nil {
			return err
		}

		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			return fmt.Errorf("failed to open chunk file %s: %w", chunkPath, err)
		}

		_, copyErr := io.Copy(outFile, chunkFile)
		chunkFile.Close()

		if copyErr != nil {
			return fmt.Errorf("failed to copy chunk %s: %w", chunkPath, copyErr)
		}
	}

	return nil
}

// Undo cleans up the partial merged file if execution fails (Principle 2: Partial Rollback).
func (t *MergeTask) Undo(ctx context.Context) error {
	if _, err := os.Stat(t.OutputPath); err == nil {
		if rmErr := os.Remove(t.OutputPath); rmErr != nil {
			return fmt.Errorf("failed to remove merged output file during undo: %w", rmErr)
		}
	}
	return nil
}

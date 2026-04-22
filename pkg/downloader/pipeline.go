package downloader

import (
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// PipelineTask implements the aixflow.Task interface, handling post-processing: 
// compression, encryption, and integrity hashing of the downloaded file.
type PipelineTask struct {
	InputPath  string
	OutputPath string
	Key        []byte // 32 bytes for AES-256

	// Stores the resulting SHA-256 hash upon successful execution.
	FinalHash string
}

// NewPipelineTask initializes a new PipelineTask for secure post-processing.
func NewPipelineTask(inputPath, outputPath string, key []byte) *PipelineTask {
	return &PipelineTask{
		InputPath:  inputPath,
		OutputPath: outputPath,
		Key:        key,
	}
}

// Execute performs stream processing: Read Source -> Gzip Compress -> AES Encrypt -> 
// Write Target + Calculate SHA-256 Hash.
func (t *PipelineTask) Execute(ctx context.Context) error {
	// 1. Open source file
	inFile, err := os.Open(t.InputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inFile.Close()

	// 2. Create target file (Side effect; requires Undo on failure)
	outFile, err := os.Create(t.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// 3. Initialize SHA-256 hasher
	hasher := sha256.New()

	// 4. Use MultiWriter to write encrypted data to both the file and the hasher
	multiWriter := io.MultiWriter(outFile, hasher)

	// 5. Initialize AES-CTR cipher stream
	block, err := aes.NewCipher(t.Key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}
	
	// Generate random Initialization Vector (IV)
	iv := make([]byte, block.BlockSize())
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return fmt.Errorf("failed to generate IV: %w", err)
	}
	
	// Prepend IV to the file for decryption purposes
	if _, err := multiWriter.Write(iv); err != nil {
		return fmt.Errorf("failed to write IV: %w", err)
	}

	stream := cipher.NewCTR(block, iv)
	streamWriter := &cipher.StreamWriter{S: stream, W: multiWriter}

	// 6. Initialize Gzip writer
	gzipWriter := gzip.NewWriter(streamWriter)

	// 7. Stream processing: Copy data through the pipeline (Gzip -> AES -> File+Hash)
	if err := ctx.Err(); err != nil {
		return err
	}
	
	if _, err := io.Copy(gzipWriter, inFile); err != nil {
		return fmt.Errorf("pipeline streaming failure: %w", err)
	}

	// 8. Close writers in sequence to flush buffers
	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}
	if err := streamWriter.Close(); err != nil {
		return fmt.Errorf("failed to close stream writer: %w", err)
	}

	// 9. Persist the final calculated hash
	t.FinalHash = hex.EncodeToString(hasher.Sum(nil))

	return nil
}

// Undo handles the removal of the output file in case of failure (Principle 2: Partial Rollback).
func (t *PipelineTask) Undo(ctx context.Context) error {
	if _, err := os.Stat(t.OutputPath); err == nil {
		if rmErr := os.Remove(t.OutputPath); rmErr != nil {
			return fmt.Errorf("failed to remove output file during undo: %w", rmErr)
		}
	}
	return nil
}

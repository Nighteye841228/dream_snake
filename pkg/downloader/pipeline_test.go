package downloader_test

import (
	"context"
	"crypto/aes"
	"os"
	"path/filepath"
	"testing"

	"github.com/nighteye841228/aix-flow/pkg/aixflow"
	"github.com/nighteye841228/aix-flow/pkg/downloader"
)

func TestPipelineTask_Success(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "input.txt")
	outputPath := filepath.Join(tempDir, "output.enc")

	originalData := []byte("Sensitive data that requires secure processing.")
	os.WriteFile(inputPath, originalData, 0644)

	key := []byte("0123456789abcdef0123456789abcdef") // 32 bytes for AES-256
	task := downloader.NewPipelineTask(inputPath, outputPath, key)
	runner := aixflow.NewAtomicRunner()

	err := runner.Run(context.Background(), task)
	if err != nil {
		t.Fatalf("Pipeline execution failed: %v", err)
	}

	if task.FinalHash == "" {
		t.Error("FinalHash was not populated")
	}

	stat, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Expected output file to exist: %v", err)
	}

	if stat.Size() <= int64(aes.BlockSize) {
		t.Errorf("Output file size is insufficient: %d", stat.Size())
	}
	
	t.Run("FailureTriggersUndo", func(t *testing.T) {
		badInputPath := filepath.Join(tempDir, "non_existent.txt")
		badOutputPath := filepath.Join(tempDir, "cleanup_target.enc")
		
		// Pre-create garbage to simulate execution residue.
		os.WriteFile(badOutputPath, []byte("residue..."), 0644)

		badTask := downloader.NewPipelineTask(badInputPath, badOutputPath, key)
		err := runner.Run(context.Background(), badTask)
		if err == nil {
			t.Fatal("Expected failure due to missing input file")
		}

		// Verify Undo: garbage file should be removed.
		if _, err := os.Stat(badOutputPath); !os.IsNotExist(err) {
			t.Errorf("Expected output file to be purged by Undo, but it still exists")
		}
	})
}

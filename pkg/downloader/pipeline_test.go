package downloader_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"io"
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

	// At minimum: 8-byte salt + 4-byte length prefix + non-empty sealed chunk
	// (which is plaintext + 16-byte GCM tag).
	if stat.Size() <= int64(8+4+aes.BlockSize) {
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

// TestPipelineTask_RoundTrip encrypts a payload, then decrypts the on-disk
// format using the format spec documented on PipelineTask. This proves the
// AES-GCM stream is well-formed and authenticated — a flipped bit anywhere in
// any sealed chunk would fail aead.Open() during decryption.
func TestPipelineTask_RoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "plain.bin")
	outputPath := filepath.Join(tempDir, "secure.enc")

	// Use payload that spans multiple GCM chunks to exercise counter increment.
	original := bytes.Repeat([]byte("AIX-FLOW-ROUND-TRIP-"), 20000) // ~400 KB
	if err := os.WriteFile(inputPath, original, 0644); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	key := []byte("12345678901234567890123456789012") // 32 bytes
	task := downloader.NewPipelineTask(inputPath, outputPath, key)
	if err := aixflow.NewAtomicRunner().Run(context.Background(), task); err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	encrypted, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read encrypted: %v", err)
	}

	decompressed, err := decryptAndGunzip(key, encrypted)
	if err != nil {
		t.Fatalf("round-trip failed: %v", err)
	}
	if !bytes.Equal(decompressed, original) {
		t.Errorf("round-trip mismatch: lengths got=%d want=%d", len(decompressed), len(original))
	}

	t.Run("TamperingIsDetected", func(t *testing.T) {
		// Flip a byte deep in the first sealed chunk (past salt + length prefix).
		tampered := bytes.Clone(encrypted)
		tampered[8+4+10] ^= 0x01

		if _, err := decryptAndGunzip(key, tampered); err == nil {
			t.Error("Expected GCM authentication failure on tampered ciphertext")
		}
	})
}

// decryptAndGunzip reverses the on-disk format produced by PipelineTask.
// Layout: [8-byte salt][repeated: [4-byte length N][N bytes sealed chunk]].
func decryptAndGunzip(key, blob []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(blob) < 8 {
		return nil, fmt.Errorf("blob shorter than salt header")
	}
	salt := blob[:8]
	rest := blob[8:]

	var compressed bytes.Buffer
	var counter uint32
	for len(rest) > 0 {
		if len(rest) < 4 {
			return nil, fmt.Errorf("truncated length prefix")
		}
		n := binary.BigEndian.Uint32(rest[:4])
		rest = rest[4:]
		if uint32(len(rest)) < n {
			return nil, fmt.Errorf("truncated sealed chunk: need %d, have %d", n, len(rest))
		}
		sealed := rest[:n]
		rest = rest[n:]

		nonce := make([]byte, 12)
		copy(nonce, salt)
		binary.BigEndian.PutUint32(nonce[8:], counter)

		opened, err := aead.Open(nil, nonce, sealed, nil)
		if err != nil {
			return nil, fmt.Errorf("aead open chunk %d: %w", counter, err)
		}
		compressed.Write(opened)
		counter++
	}

	gz, err := gzip.NewReader(&compressed)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()
	return io.ReadAll(gz)
}

package downloader

import (
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// gcmChunkSize is the plaintext block size processed by a single GCM Seal call.
// Smaller chunks bound the latency of a Seal/Open call; larger chunks reduce
// per-chunk overhead. 64 KiB is a well-trodden middle ground.
const gcmChunkSize = 64 * 1024

// PipelineTask implements the aixflow.Task interface, handling post-processing:
// compression, authenticated encryption, and integrity hashing of the
// downloaded file.
//
// On-disk format produced by this task:
//
//	[8-byte random salt]
//	repeated until EOF:
//	    [4-byte big-endian sealed-chunk length N]
//	    [N bytes of (ciphertext || 16-byte GCM tag)]
//
// Per-chunk nonce = salt (8 bytes) || uint32 counter (4 bytes), giving an
// authenticated, tamper-evident stream that composes cleanly with io.Writer.
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

// Execute performs stream processing:
// Read Source -> Gzip Compress -> AES-256-GCM Encrypt (chunked AEAD) ->
// Write Target + Calculate SHA-256 Hash.
func (t *PipelineTask) Execute(ctx context.Context) error {
	inFile, err := os.Open(t.InputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inFile.Close()

	outFile, err := os.Create(t.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	hasher := sha256.New()
	multiWriter := io.MultiWriter(outFile, hasher)

	gw, err := newGCMWriter(t.Key, multiWriter)
	if err != nil {
		return fmt.Errorf("failed to initialise GCM writer: %w", err)
	}

	gzipWriter := gzip.NewWriter(gw)

	if err := ctx.Err(); err != nil {
		return err
	}

	if _, err := io.Copy(gzipWriter, inFile); err != nil {
		return fmt.Errorf("pipeline streaming failure: %w", err)
	}

	// Flush in order: gzip first (emits trailer through gw), then gw (seals
	// any partial final chunk).
	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("failed to close GCM writer: %w", err)
	}

	t.FinalHash = hex.EncodeToString(hasher.Sum(nil))
	return nil
}

// Undo handles the removal of the output file in case of failure (Principle 2:
// Partial Rollback).
func (t *PipelineTask) Undo(ctx context.Context) error {
	if _, err := os.Stat(t.OutputPath); err == nil {
		if rmErr := os.Remove(t.OutputPath); rmErr != nil {
			return fmt.Errorf("failed to remove output file during undo: %w", rmErr)
		}
	}
	return nil
}

// gcmWriter buffers plaintext into fixed-size chunks and seals each chunk with
// AES-GCM under a deterministic per-chunk nonce. This adapts the non-streaming
// AEAD primitive to an io.Writer interface so it can sit in the gzip -> encrypt
// -> hash pipeline without changing the rest of the chain.
//
// [MANUAL INTERVENTION POINT: AEAD Choice]
// AES-GCM was chosen over CTR to provide authenticated encryption: tampering
// with any sealed chunk causes Open to fail loudly. The 12-byte nonce is built
// as salt(8) || counter(4); reusing a (key, nonce) pair across writes would be
// catastrophic, so the salt is freshly generated per Execute. Engineers cloning
// this writer for other use sites MUST preserve that invariant.
type gcmWriter struct {
	aead       cipher.AEAD
	underlying io.Writer
	salt       []byte
	counter    uint32
	buf        []byte
}

func newGCMWriter(key []byte, w io.Writer) (*gcmWriter, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	salt := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	if _, err := w.Write(salt); err != nil {
		return nil, fmt.Errorf("failed to write salt header: %w", err)
	}

	return &gcmWriter{
		aead:       aead,
		underlying: w,
		salt:       salt,
		buf:        make([]byte, 0, gcmChunkSize),
	}, nil
}

// Write buffers plaintext, sealing & flushing whenever a full chunk accumulates.
func (g *gcmWriter) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		space := gcmChunkSize - len(g.buf)
		n := len(p)
		if n > space {
			n = space
		}
		g.buf = append(g.buf, p[:n]...)
		p = p[n:]
		written += n
		if len(g.buf) == gcmChunkSize {
			if err := g.flushChunk(); err != nil {
				return written, err
			}
		}
	}
	return written, nil
}

// Close seals any buffered residue. Safe to call multiple times.
func (g *gcmWriter) Close() error {
	return g.flushChunk()
}

func (g *gcmWriter) flushChunk() error {
	if len(g.buf) == 0 {
		return nil
	}

	nonce := make([]byte, 12)
	copy(nonce, g.salt)
	binary.BigEndian.PutUint32(nonce[8:], g.counter)

	sealed := g.aead.Seal(nil, nonce, g.buf, nil)

	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(sealed)))
	if _, err := g.underlying.Write(lenBuf[:]); err != nil {
		return err
	}
	if _, err := g.underlying.Write(sealed); err != nil {
		return err
	}

	g.buf = g.buf[:0]
	g.counter++
	return nil
}

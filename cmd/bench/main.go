package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nighteye841228/aix-flow/pkg/aixflow"
	"github.com/nighteye841228/aix-flow/pkg/downloader"
)

type LogMode int

const (
	NoLog LogMode = iota
	SmartLog
	HeavyLog
)

// BenchTask wraps a core task to simulate various logging overheads and error scenarios.
type BenchTask struct {
	Task        aixflow.Task
	LogMode     LogMode
	LogFile     string
	ShouldErr   bool
	SmartLogger *aixflow.SmartLogger
}

func (t *BenchTask) Execute(ctx context.Context) error {
	// Simulate logging overhead based on mode.
	if t.LogMode == HeavyLog {
		f, _ := os.OpenFile(t.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		logger := log.New(f, "[HEAVY] ", log.LstdFlags)
		for i := 0; i < 5000; i++ {
			logger.Printf("Executing task... timestamp=%v, context=%+v, metadata=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n", time.Now(), ctx)
		}
		f.Close()
	} else if t.LogMode == SmartLog && t.SmartLogger != nil {
		// Use the official SmartLogger to simulate memory-based logging.
		for i := 0; i < 5000; i++ {
			t.SmartLogger.Log(fmt.Sprintf("Executing task... timestamp=%v, context=%+v, metadata=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", time.Now(), ctx))
		}
	}

	err := t.Task.Execute(ctx)
	if err != nil {
		return err
	}

	if t.ShouldErr {
		return errors.New("simulated error at end of task execution")
	}
	return nil
}

func (t *BenchTask) Undo(ctx context.Context) error {
	if t.LogMode == SmartLog && t.SmartLogger != nil {
		// Dump memory logs to disk only upon failure.
		_ = t.SmartLogger.Dump(t.LogFile)
	}
	return t.Task.Undo(ctx)
}

// WanSimulator simulates real-world WAN bandwidth constraints for a single connection.
type WanSimulator struct {
	Handler     http.Handler
	BytesPerSec int // Throughput limit per connection
}

type slowResponseWriter struct {
	http.ResponseWriter
	bytesPerSec int
}

func (w *slowResponseWriter) Write(p []byte) (int, error) {
	start := time.Now()
	n, err := w.ResponseWriter.Write(p)
	
	if n > 0 && w.bytesPerSec > 0 {
		expectedDuration := time.Duration(n) * time.Second / time.Duration(w.bytesPerSec)
		elapsed := time.Since(start)
		if elapsed < expectedDuration {
			time.Sleep(expectedDuration - elapsed)
		}
	}
	return n, err
}

func (ws *WanSimulator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slowW := &slowResponseWriter{
		ResponseWriter: w,
		bytesPerSec:    ws.BytesPerSec,
	}
	ws.Handler.ServeHTTP(slowW, r)
}

func runDownloadChunks(tsURL string, destDir string, totalSize int64, chunkSize int64, logMode LogMode, logPath string, failChunkIndex int) (time.Duration, error) {
	start := time.Now()
	runner := aixflow.NewAtomicRunner()
	ctx := context.Background()
	
	var sLogger *aixflow.SmartLogger
	if logMode == SmartLog {
		sLogger = aixflow.NewSmartLogger(10000)
	}

	chunks, err := downloader.CalculateChunks(totalSize, chunkSize)
	if err != nil {
		return 0, err
	}

	var chunkPaths []string
	for _, chunk := range chunks {
		chunkPath := filepath.Join(destDir, fmt.Sprintf("chunk_%d.tmp", chunk.Index))
		chunkPaths = append(chunkPaths, chunkPath)

		baseTask := downloader.NewDownloadTask(tsURL, chunk, chunkPath)
		benchTask := &BenchTask{
			Task:        baseTask,
			LogMode:     logMode,
			LogFile:     logPath,
			ShouldErr:   chunk.Index == failChunkIndex,
			SmartLogger: sLogger,
		}
		
		if err := runner.Run(ctx, benchTask); err != nil {
			return time.Since(start), err
		}
	}

	mergedFile := filepath.Join(destDir, "merged.bin")
	mergeTask := downloader.NewMergeTask(chunkPaths, mergedFile)
	runner.Run(ctx, mergeTask)

	return time.Since(start), nil
}

func runParallelChunks(tsURL string, destDir string, totalSize int64, chunkSize int64, simulateCorruption bool) time.Duration {
	start := time.Now()
	chunks, _ := downloader.CalculateChunks(totalSize, chunkSize)
	runner := aixflow.NewAtomicRunner()
	
	var wg sync.WaitGroup
	errCh := make(chan downloader.Chunk, len(chunks))

	// Concurrent Download Phase
	for _, chunk := range chunks {
		wg.Add(1)
		go func(c downloader.Chunk) {
			defer wg.Done()
			chunkPath := filepath.Join(destDir, fmt.Sprintf("pchunk_%d.tmp", c.Index))
			task := downloader.NewDownloadTask(tsURL, c, chunkPath)
			
			bTask := &BenchTask{Task: task, ShouldErr: simulateCorruption && c.Index == 50}
			if err := runner.Run(context.Background(), bTask); err != nil {
				errCh <- c
			}
		}(chunk)
	}
	wg.Wait()
	close(errCh)

	// Localized Recovery Phase
	for failedChunk := range errCh {
		chunkPath := filepath.Join(destDir, fmt.Sprintf("pchunk_%d.tmp", failedChunk.Index))
		task := downloader.NewDownloadTask(tsURL, failedChunk, chunkPath)
		runner.Run(context.Background(), task)
	}

	return time.Since(start)
}

func runTraditionalSingle(tsURL string, destDir string, totalSize int64, simulateCorruption bool) time.Duration {
	start := time.Now()
	runner := aixflow.NewAtomicRunner()
	chunkPath := filepath.Join(destDir, "single_huge.tmp")
	
	if simulateCorruption {
		// Traditional mode failure simulation at 50%.
		halfChunk := downloader.Chunk{Index: 0, Start: 0, End: (totalSize / 2) - 1, Size: totalSize / 2}
		bTaskHalf := &BenchTask{Task: downloader.NewDownloadTask(tsURL, halfChunk, chunkPath), ShouldErr: true}
		runner.Run(context.Background(), bTaskHalf)
	}

	// Traditional recovery: must restart the entire 1GB download.
	fullChunk := downloader.Chunk{Index: 0, Start: 0, End: totalSize - 1, Size: totalSize}
	runner.Run(context.Background(), downloader.NewDownloadTask(tsURL, fullChunk, chunkPath))

	return time.Since(start)
}

func main() {
	fmt.Println("Initializing benchmark environment...")
	tempDir, _ := os.MkdirTemp("", "aixflow-bench-*")
	defer os.RemoveAll(tempDir)

	sizes := map[string]int64{
		"1KB":    1024,
		"1MB":    1024 * 1024,
		"100MB":  100 * 1024 * 1024,
		"1000MB": 1000 * 1024 * 1024,
	}

	for name, size := range sizes {
		path := filepath.Join(tempDir, name+".bin")
		f, _ := os.Create(path)
		f.Truncate(size)
		f.Close()
	}

	fs := http.FileServer(http.Dir(tempDir))
	wanFs := &WanSimulator{Handler: fs, BytesPerSec: 100 * 1024 * 1024} // 100MB/s per-connection limit
	ts := httptest.NewServer(wanFs)
	defer ts.Close()

	var report string
	report += "# AI-Native Architecture: Depth Performance and Fault Tolerance Report\n\n"
	report += "This report quantifies the system's performance under normal conditions and explores the architectural advantages of AI-Native principles when \"AI-generated code encounters errors\" or \"network interruptions occur.\"\n\n"

	// Test 1: Baseline Performance
	report += "## 1. Baseline Download Matrix (10MB Chunk Strategy)\n"
	report += "| File Size | Total Duration | Number of Chunks |\n"
	report += "| :--- | :--- | :--- |\n"
	testSizes := []string{"1KB", "1MB", "100MB", "1000MB"}
	for _, name := range testSizes {
		duration, _ := runDownloadChunks(ts.URL+"/"+name+".bin", tempDir, sizes[name], 10*1024*1024, NoLog, "", -1)
		report += fmt.Sprintf("| %s | %v | %d Chunks |\n", name, duration, (sizes[name]/(10*1024*1024))+1)
	}
	report += "\n"

	// Test 2: Logging Overhead
	report += "## 2. Logging Strategy and Failure-Exit Overhead (100MB)\n"
	url100MB := ts.URL + "/100MB.bin"
	logPath := filepath.Join(tempDir, "bench.log")

	durNoLogNorm, _ := runDownloadChunks(url100MB, tempDir, sizes["100MB"], 10*1024*1024, NoLog, logPath, -1)
	durSmartNorm, _ := runDownloadChunks(url100MB, tempDir, sizes["100MB"], 10*1024*1024, SmartLog, logPath, -1)
	durHeavyNorm, _ := runDownloadChunks(url100MB, tempDir, sizes["100MB"], 10*1024*1024, HeavyLog, logPath, -1)
	durNoLogErr, _ := runDownloadChunks(url100MB, tempDir, sizes["100MB"], 10*1024*1024, NoLog, logPath, 9)
	durSmartErr, _ := runDownloadChunks(url100MB, tempDir, sizes["100MB"], 10*1024*1024, SmartLog, logPath, 9)
	durHeavyErr, _ := runDownloadChunks(url100MB, tempDir, sizes["100MB"], 10*1024*1024, HeavyLog, logPath, 9)

	report += "| Logging Strategy | Normal Completion | Failure + Undo Trigger | Strategy Description |\n"
	report += "| :--- | :--- | :--- | :--- |\n"
	report += fmt.Sprintf("| **No Logging** | %v | %v | Performance baseline. |\n", durNoLogNorm, durNoLogErr)
	report += fmt.Sprintf("| **Smart Log (Ring Buffer)** | %v | %v | Maintains memory logs; dumps to disk only on error. |\n", durSmartNorm, durSmartErr)
	report += fmt.Sprintf("| **Heavy Disk Logging** | %v | %v | Synchronous disk I/O; introduces significant latency. |\n", durHeavyNorm, durHeavyErr)
	report += "\n"

	// Test 3: Fault Tolerance
	report += "## 3. Massive File Fault Tolerance and Retry Performance (1000MB / 1GB)\n"
	url1000MB := ts.URL + "/1000MB.bin"
	durSingleNorm := runTraditionalSingle(url1000MB, tempDir, sizes["1000MB"], false)
	durParallelNorm := runParallelChunks(url1000MB, tempDir, sizes["1000MB"], 10*1024*1024, false)
	durSingleCorrupt := runTraditionalSingle(url1000MB, tempDir, sizes["1000MB"], true)
	durParallelCorrupt := runParallelChunks(url1000MB, tempDir, sizes["1000MB"], 10*1024*1024, true)

	report += "| Architectural Pattern | Successful Download | 50% Corruption/Interruption | Fault Tolerance Analysis |\n"
	report += "| :--- | :--- | :--- | :--- |\n"
	report += fmt.Sprintf("| **Traditional Monolithic** | %v | %v | Requires full restart on failure. |\n", durSingleNorm, durSingleCorrupt)
	report += fmt.Sprintf("| **AIX-Flow Concurrent Atomic** | %v | %v | Localized rollback and retry of specific chunk. |\n", durParallelNorm, durParallelCorrupt)

	os.WriteFile("/Users/nighteye1228/Documents/dream_snake/docs/performance_report.md", []byte(report), 0644)
	fmt.Println("Benchmark complete. Report generated at docs/performance_report.md")
}

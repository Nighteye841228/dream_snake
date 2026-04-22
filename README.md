# AIX-Flow: The AI-Native Atomic Execution Engine

[![Go Report Card](https://goreportcard.com/badge/github.com/nighteye841228/aix-flow)](https://goreportcard.com/report/github.com/nighteye841228/aix-flow)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**AIX-Flow** is a robust, high-performance execution framework designed specifically for the AI era of software engineering. It directly addresses the inherent unpredictability and unreliability of AI-generated code by enforcing strict architectural boundaries: side-effect isolation, atomic execution with guaranteed rollbacks, and intelligent resource budgeting.

To demonstrate the power of these principles, this repository includes a **high-performance, concurrent Chunk Downloader** built entirely on top of the AIX-Flow engine. It features parallel chunking, automatic failure recovery, and a secure post-processing pipeline (Gzip compression + AES-256 encryption + SHA-256 integrity hashing).

## 🚀 Core Architectural Principles (The "AI-Native" Manifesto)

As AI writes more of our code, the role of the Software Engineer shifts from writing syntax to designing resilient guardrails. AIX-Flow is built on four core principles:

1. **Strict Side-Effect Isolation**: Tasks must never mutate the final system state until fully verified. Temporary files and isolated buffers are mandatory.
2. **Atomic Execution & Partial Rollback (Undo)**: Every operation is an independent transaction. If an AI-generated task or network operation fails, the system automatically invokes a localized `Undo()` to clean up partial states, preventing system corruption without requiring a full restart.
3. **Smart Logging & Time Budgets**: To prevent I/O bottlenecks caused by massive debug logging, logs are kept in a memory Ring Buffer and only dumped to disk upon failure. Strict context timeouts prevent infinite loops.
4. **Contract-Driven Design**: Sequence diagrams and interface contracts must be defined *before* implementation, guiding the AI's generation process.

## 📊 Performance & Fault-Tolerance Benchmark

By breaking down large, monolithic operations into micro, atomic tasks, we not only gain immense fault tolerance but also unlock massive parallel computing capabilities.

*Benchmark: Downloading a 1GB file over a simulated WAN (100MB/s per connection limit).*

| Architecture | Success Duration | Duration upon 50% Failure / Interruption | Fault-Tolerance Analysis |
| :--- | :--- | :--- | :--- |
| **Traditional Monolithic** | `12.56s` | `18.78s` | **0% Fault Tolerance.** A failure at 500MB requires discarding all progress and restarting the 1GB download from scratch, wasting time and bandwidth. |
| **AIX-Flow (Concurrent Atomic)** | `0.87s` | `1.01s` | **14x Faster & Near-Zero Cost Recovery.** Bypasses single-connection limits via 101 concurrent chunk tasks. If a chunk fails, the engine only rolls back and retries that specific 10MB chunk. |

*Intelligent Logging Overhead Benchmark (100MB Task)*:
- Traditional Heavy Disk Logging: `1.50s` (I/O Bottleneck)
- **AIX-Flow Smart Ring Buffer**: `1.38s` (Zero Overhead, preserves the exact state upon failure)

## 🛠️ Usage

### Defining an Atomic Task

Implement the `aixflow.Task` interface:

```go
type MyTask struct {
    TempPath string
}

func (t *MyTask) Execute(ctx context.Context) error {
    // 1. Perform isolated work (e.g., download to a temp file)
    return nil
}

func (t *MyTask) Undo(ctx context.Context) error {
    // 2. Clean up any partial state if Execute fails
    os.Remove(t.TempPath)
    return nil
}
```

### Executing with the Engine

```go
runner := aixflow.NewAtomicRunner()
// Wrap with a 5-second budget to prevent AI-hallucinated infinite loops
budgetRunner := aixflow.NewBudgetedRunner(runner, 5*time.Second)

err := budgetRunner.Run(context.Background(), &MyTask{TempPath: "/tmp/isolated.bin"})
```

## 📁 Project Structure

- `pkg/aixflow/`: The core atomic execution and budgeting engine.
- `pkg/downloader/`: A production-ready concurrent downloader showcasing the engine's capabilities.
- `cmd/bench/`: Comprehensive benchmarking suite for performance and fault-tolerance validation.
- `docs/`: Design contracts and detailed performance reports.

## 📄 License

MIT License

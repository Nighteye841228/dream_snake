# AIX-Flow: The AI-Native Atomic Execution Framework

[ English ] | [ [繁體中文](https://github.com/Nighteye841228/dream_snake/blob/main/README_zh.md) ]

[![Go Report Card](https://goreportcard.com/badge/github.com/nighteye841228/aix-flow)](https://goreportcard.com/report/github.com/nighteye841228/aix-flow)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**AIX-Flow** is a **safety-first architectural framework** designed to solve the "Hallucination Risk" in AI-generated software. It enforces a strict **Atomic Governance** model, ensuring that unpredictable AI logic cannot leave your system in a corrupted state.

---

## 🧠 The Architectural Problem: "AI Hallucination in I/O"
As AI agents increasingly write production code, the biggest threat is **Non-Deterministic Side Effects**. AI often fails to handle edge-case failures of third-party APIs (e.g., partial HTTP responses or corrupted streams), leading to "silent corruption."

**AIX-Flow shifts the "Error Strategy" from the AI to a Human-Defined Framework.**

---

## 🚀 Human-in-the-Loop Governance
AIX-Flow is built for **Architectural Supervision**. Below is the core contract where the Senior Engineer maintains control over the AI-generated logic:

### Manual Intervention Point: Error Policy & Rollback
In `pkg/aixflow/atomic.go`, we define the boundary where human intelligence must oversee AI execution:

```go
// [MANUAL INTERVENTION POINT: Error Policy Definition]
// While AI agents generate the core logic of Execute and Undo, the Senior Engineer 
// MUST manually define the "Error Boundary." The engineer decides which third-party 
// API errors are recoverable (requiring a retry) versus which are fatal (requiring 
// an immediate Undo).
type Task interface {
    // Execute performs the core logic (AI-Generated).
    Execute(ctx context.Context) error

    // Undo reverts any partial side effects (AI-Generated, Human-Verified).
    // [CRITICAL INTERVENTION]: The rollback logic must be manually verified to ensure
    // it leaves the system in a deterministic "Zero-State."
    Undo(ctx context.Context) error
}
```

---

## 📊 Quantitative Benchmarks (Proof of Architecture)

### A. The "Smart Logging" Advantage (100MB Task)
*Traditional logging slows down execution by over 6x when AI-scale metadata is recorded.*

| Logging Strategy | Normal Completion | Performance Loss | Architecture Benefit |
| :--- | :--- | :--- | :--- |
| **Traditional Disk Logging** | `1.50s` | **+600%** | I/O Bottleneck from constant Disk Writes. |
| **AIX-Flow Smart Log** | `0.23s` | **0%** | Memory-speed logging. Full trace available on failure. |

### B. Fault-Tolerance Cost: Atomic vs. Monolithic (1GB Task)
*Comparing recovery costs when a 3rd-party API fails at 50% completion.*

| Architecture Pattern | Duration (Success) | Duration (50% Failure) | Fault-Tolerance Analysis |
| :--- | :--- | :--- | :--- |
| **Monolithic (Non-Atomic)** | `12.56s` | `18.78s` | **Zero Resilience**. Must restart from 0MB. |
| **AIX-Flow (Concurrent Atomic)**| `0.87s` | `1.01s` | **Instant Recovery**. Only the failed 10MB chunk is rolled back. |

---

## 📁 Project Structure
- `pkg/aixflow/`: The core atomic and budgeting engine.
- `pkg/downloader/`: A concurrent downloader built on AIX-Flow principles.
- `cmd/bench/`: The WAN-simulated benchmark suite.
- `docs/`: Sequence diagrams and architectural contracts.

## 📄 License
MIT License

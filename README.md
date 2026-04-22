# AIX-Flow: The AI-Native Atomic Execution Framework

[![Go Report Card](https://goreportcard.com/badge/github.com/nighteye841228/aix-flow)](https://goreportcard.com/report/github.com/nighteye841228/aix-flow)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**AIX-Flow** is not just a downloader; it is a **safety-first architectural framework** designed to solve the "Hallucination Risk" in AI-generated software. It enforces a strict **Atomic Governance** model, ensuring that unpredictable AI logic cannot leave your system in a corrupted state.

## 🧠 Why AIX-Flow? (The Architectural Problem)
As AI agents (like Gemini, GPT-4) increasingly write production code, the biggest threat is no longer syntax errors, but **Non-Deterministic Side Effects**. AI agents often fail to handle edge-case failures of third-party APIs (e.g., partial HTTP responses, rate limits, or corrupted streams), leading to "silent corruption."

**AIX-Flow solves this by shifting the "Error Strategy" from the AI to a Human-Defined Framework.**

---

## 🚀 Core Architectural Pillars

### 1. Atomic Transactional Governance
Every unit of work is a `Task` with a mandatory `Undo()` method. 
- **The AI** writes the `Execute` logic.
- **The Senior Engineer** manually reviews and defines the **Undo Policy**, ensuring that if a task fails at *any* point, the system is restored to its "Zero-State" automatically.

### 2. Side-Effect Isolation
No task is allowed to touch production data directly. All I/O is redirected to isolated buffers or temporary paths. The final system state is only mutated through a human-verified `MergeTask`.

### 3. Intelligent "Flight Recorder" Logging
AI debugging requires massive amounts of metadata. However, traditional disk logging causes severe I/O bottlenecks. 
- **Smart Ring Buffer**: Logs are kept in memory and are **only dumped to disk upon an Error Flag**. This provides a full "crime scene snapshot" for the AI to debug itself, with zero performance loss during normal operation.

---

## 📊 Quantitative Benchmarks (Proof of Architecture)

We measured the framework's performance using a 1GB file over a simulated WAN environment. These results prove that **Atomic Design** yields both **Safety** and **Performance**.

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

## 🛠️ Manual Intervention & Governance
This framework is designed for **Human-in-the-Loop** development. Engineers MUST manually intervene at the following points:

1.  **Error Boundary Definition**: In `pkg/aixflow/atomic.go`, the engineer decides which external API errors are recoverable.
2.  **Rollback Verification**: The `Undo()` logic for each task must be manually audited to ensure no residue is left behind.
3.  **Pipeline Integrity**: Engineers must verify the streaming SHA-256 implementation to prevent partial data bypasses.

## 📁 Project Structure
- `pkg/aixflow/`: The core atomic and budgeting engine.
- `pkg/downloader/`: A concurrent downloader built on AIX-Flow principles.
- `cmd/bench/`: The WAN-simulated benchmark suite.
- `docs/`: Sequence diagrams and architectural contracts.

## 📄 License
MIT License

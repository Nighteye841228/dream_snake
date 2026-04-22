# Concurrent Chunk Downloader: Architectural Contract

This document defines the interaction contract for the Chunk Downloader. All implementations must strictly adhere to the following logic to ensure atomicity and side-effect isolation.

## 1. System Design Principles
- **Isolate Side Effects**: Downloads are initially written to temporary chunk files (`chunk_n.tmp`). The production file is only touched during the `MergeTask` phase.
- **Atomic Operations**: Each download, merge, and post-processing step is encapsulated as an `aixflow.Task`.
- **Automatic Rollback**: If a task fails, its corresponding `Undo()` is invoked to clean up temporary artifacts.

## 2. Sequence Diagram Contract

```mermaid
sequenceDiagram
    participant App as Main Application
    participant Calc as ChunkCalculator
    participant Runner as AIX-Flow Engine
    participant Worker as DownloadTask (Atomic)
    participant Merger as MergeTask (Atomic)
    participant Pipeline as SecurePipeline (Atomic)

    App->>Calc: CalculateChunks(FileSize, ChunkSize)
    Calc-->>App: List of Chunks (Range offsets)

    loop For Each Chunk
        App->>Runner: Run(DownloadTask)
        Runner->>Worker: Execute(Download)
        alt Success
            Worker-->>Runner: Success (Temp File Created)
        else Failure
            Worker->>Worker: Undo (Delete Partial Temp File)
            Runner-->>App: Return Error
        end
    end

    App->>Runner: Run(MergeTask)
    Merger->>Merger: Execute (Combine all Temp Files)
    alt Failure
        Merger->>Merger: Undo (Delete Partial Merged File)
    end

    App->>Runner: Run(SecurePipeline)
    Pipeline->>Pipeline: Execute (Gzip -> AES -> SHA256)
    alt Success
        Pipeline-->>App: Final Secure File + Integrity Hash
    else Failure
        Pipeline->>Pipeline: Undo (Cleanup Output)
    end
```

## 3. Post-Processing Pipeline Contract
- **Compression**: Gzip (standard compression level).
- **Encryption**: AES-256-CTR with random IV prepended to the ciphertext.
- **Integrity Verification**: SHA-256 hash calculated simultaneously during the encryption streaming process.

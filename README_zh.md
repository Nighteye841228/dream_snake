# AIX-Flow: AI 原生原子化執行引擎

[![Go Report Card](https://goreportcard.com/badge/github.com/nighteye841228/aix-flow)](https://goreportcard.com/report/github.com/nighteye841228/aix-flow)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**AIX-Flow** 是一個專為「AI 軟體工程時代」設計的高效能、高容錯執行框架。它透過強制實施嚴格的架構邊界：副作用隔離、具備保證還原能力的原子化執行，以及智慧型資源預算控制，來從根本上解決 AI 生成程式碼不可預測與不可靠的痛點。

為了展示這些架構原則的威力，本專案包含了一個完全建構在 AIX-Flow 引擎之上的**高效能並行分塊下載器 (Chunk Downloader)**。它具備平行下載、自動錯誤復原，以及安全的後處理管線（Gzip 壓縮 + AES-256 加密 + SHA-256 完整性校驗）。

## 🚀 核心架構原則 (AI 原生宣言)

當 AI 替我們撰寫越來越多的程式碼，軟體工程師的角色將從「撰寫語法」轉變為「設計具備韌性的護欄」。AIX-Flow 建立在以下四大核心原則之上：

1. **嚴格的副作用隔離**：任務在完全驗證成功之前，絕對不能改變系統的最終狀態。必須強制使用暫存檔與隔離的緩衝區。
2. **原子化執行與局部還原 (Undo)**：每一個操作都是獨立的交易 (Transaction)。如果 AI 生成的邏輯或網路操作失敗，系統會自動呼叫該任務局部的 `Undo()` 來清理殘留狀態，防止系統損壞，且無需重啟整個服務。
3. **智慧型日誌與時間預算**：為了防止 AI 除錯所需的海量 Log 造成硬碟 I/O 瓶頸，日誌平時只存放在記憶體 (Ring Buffer) 中，僅在發生錯誤時才寫入實體磁碟。同時，強制實施 Context Timeout，防止 AI 產生無窮迴圈或 Deadlocks。
4. **合約驅動設計**：在實作前必須先定義循序圖 (Sequence Diagrams) 與介面合約，以此作為約束 AI 生成過程的邊界。

## 📊 效能與容錯基準測試

透過將龐大的單體操作拆解為微小的原子化任務，我們不僅獲得了極強的容錯能力，更順勢解鎖了龐大的平行運算效能。

*測試情境：在模擬外網環境 (單一連線限速 100MB/s) 下下載 1GB 巨型檔案。*

| 架構模式 | 順利下載耗時 | 遭遇 50% 損壞/中斷之最終耗時 | 容錯成本分析 |
| :--- | :--- | :--- | :--- |
| **傳統單體架構** | `12.56 秒` | `18.78 秒` | **0% 容錯率。** 在 500MB 時中斷需全部重頭來過，平白浪費大量的時間與網路頻寬。 |
| **AIX-Flow (並行原子架構)** | `0.87 秒` | `1.01 秒` | **速度提升 14 倍，且復原成本趨近於零。** 透過 101 條連線繞過限速。若發生中斷，引擎僅需局部還原 (Undo) 損壞的那 1 個 10MB 分塊並重試。 |

*智慧型日誌 Overhead 測試 (100MB 任務)*:
- 傳統海量實體磁碟寫入: `1.50 秒` (產生嚴重的 I/O 延遲)
- **AIX-Flow 智慧型記憶體日誌**: `1.38 秒` (幾乎零效能損耗，且能在出錯時完美保留案發現場)

## 🛠️ 使用方式

### 定義一個原子化任務

實作 `aixflow.Task` 介面：

```go
type MyTask struct {
    TempPath string
}

func (t *MyTask) Execute(ctx context.Context) error {
    // 1. 執行隔離的操作 (例如：下載資料到暫存檔)
    return nil
}

func (t *MyTask) Undo(ctx context.Context) error {
    // 2. 如果 Execute 失敗，清理任何殘留的狀態
    os.Remove(t.TempPath)
    return nil
}
```

### 透過引擎執行任務

```go
runner := aixflow.NewAtomicRunner()
// 加上 5 秒的時間預算控制，防止 AI 產生的邏輯卡死
budgetRunner := aixflow.NewBudgetedRunner(runner, 5*time.Second)

err := budgetRunner.Run(context.Background(), &MyTask{TempPath: "/tmp/isolated.bin"})
```

## 📁 專案結構

- `pkg/aixflow/`: 核心的原子化執行與預算控制引擎。
- `pkg/downloader/`: 一個基於此引擎打造，具備生產力標準的並行下載器。
- `cmd/bench/`: 完整的基準測試腳本，用於驗證效能與容錯能力。
- `docs/`: 設計合約與詳細的效能檢測報告。

## 📄 授權條款

MIT License

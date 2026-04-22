# AIX-Flow: AI 原生原子化執行框架

[ [English](https://github.com/Nighteye841228/dream_snake/blob/main/README.md) ] | [ 繁體中文 ]

[![Go Report Card](https://goreportcard.com/badge/github.com/nighteye841228/aix-flow)](https://goreportcard.com/report/github.com/nighteye841228/aix-flow)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**AIX-Flow** 不僅僅是一個下載器；它是一個**安全性優先的架構框架**，旨在解決 AI 生成軟體中的「幻覺風險」。它強制實施嚴格的**原子化治理 (Atomic Governance)** 模型，確保不可預測的 AI 邏輯不會讓您的系統陷入損壞狀態。

---

## 🧠 架構性痛點：「AI 在 I/O 中的幻覺」
隨著 AI 代理撰寫越來越多的生產程式碼，最大的威脅是**不可預測的副作用**。AI 往往無法處理第三方 API 的極端失敗情境（例如部分 HTTP 回應或損壞的串流），從而導致系統發生「無聲的資料損壞」。

**AIX-Flow 的核心價值在於將「錯誤處理策略」從 AI 手中收回，交還給由人類定義的框架。**

---

## 🚀 人機協作治理 (Human-in-the-Loop)
AIX-Flow 專為**「架構監督」**而生。以下是資深工程師用來約束 AI 生成邏輯的核心合約：

### 手動介入點：錯誤策略與還原
在 `pkg/aixflow/atomic.go` 中，我們定義了人類智慧必須介入監督 AI 執行的邊界：

```go
// [MANUAL INTERVENTION POINT: Error Policy Definition]
// 當 AI 代理生成 Execute 與 Undo 的核心邏輯時，資深工程師
// 必須手動定義「錯誤邊界」。由工程師決定哪些第三方 API 錯誤
// 是可以復原的（需要重試），哪些是致命的（需要立即執行 Undo）。
type Task interface {
    // Execute 執行核心邏輯 (由 AI 生成)。
    Execute(ctx context.Context) error

    // Undo 負責撤銷局部副作用 (由 AI 生成，需經人工驗證)。
    // [關鍵介入]：還原邏輯必須經過手動驗證，以確保系統
    // 能夠回到確定的「零狀態 (Zero-State)」。
    Undo(ctx context.Context) error
}
```

---

## 📊 量化基準測試 (架構實證)

### A. 「智慧型日誌」的效能優勢 (100MB 任務)
*當需要記錄 AI 等級的大量元數據時，傳統日誌會讓執行速度慢下 6 倍以上。*

| 日誌策略 | 正常完成耗時 | 效能損耗 | 架構效益 |
| :--- | :--- | :--- | :--- |
| **傳統磁碟日誌** | `1.50s` | **+600%** | 頻繁的磁碟寫入造成 I/O 瓶頸。 |
| **AIX-Flow 智慧型日誌** | `0.23s` | **0%** | 記憶體等級的紀錄速度，出錯時仍保有完整案發現場。 |

### B. 容錯成本比對：原子化 vs. 單體架構 (1GB 任務)
*模擬第三方 API 在下載進度 50% 時發生失敗後的復原成本。*

| 架構模式 | 順利完成耗時 | 發生 50% 失敗後耗時 | 容錯能力分析 |
| :--- | :--- | :--- | :--- |
| **傳統單體架構 (Non-Atomic)** | `12.56s` | `18.78s` | **零韌性**。必須從 0MB 開始重新下載。 |
| **AIX-Flow (並行原子化)** | `0.87s` | `1.01s` | **瞬間復原**。僅需復原並重試失敗的該 10MB 分塊。 |

---

## 📁 專案結構
- `pkg/aixflow/`: 核心原子化與預算控制引擎。
- `pkg/downloader/`: 基於 AIX-Flow 原則建立的並行下載器。
- `cmd/bench/`: 模擬 WAN 環境的基準測試套件。
- `docs/`: 循序圖與架構合約文件。

## 📄 授權條款
MIT License

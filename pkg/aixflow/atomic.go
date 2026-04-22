package aixflow

import "context"

// Task defines the interface for an atomic unit of work.
//
// [MANUAL INTERVENTION POINT: Error Policy Definition]
// While AI agents generate the core logic of Execute and Undo, the Senior Engineer 
// MUST manually define the "Error Boundary." The engineer decides which third-party 
// API errors are recoverable (requiring a retry) versus which are fatal (requiring 
// an immediate Undo).
type Task interface {
	// Execute performs the core logic. 
	// Engineer Note: Ensure all side effects are isolated (e.g., writing to .tmp files).
	Execute(ctx context.Context) error

	// Undo reverts any partial side effects.
	// [CRITICAL INTERVENTION]: The rollback logic must be manually verified to ensure
	// it leaves the system in a deterministic "Zero-State."
	Undo(ctx context.Context) error
}

// AtomicRunner manages the transaction lifecycle.
type AtomicRunner struct{}

func NewAtomicRunner() *AtomicRunner {
	return &AtomicRunner{}
}

// Run executes a task and manages the failure state.
func (r *AtomicRunner) Run(ctx context.Context, task Task) error {
	err := task.Execute(ctx)
	if err != nil {
		// [MANUAL INTERVENTION POINT: Error Governance]
		// This is where the engineer's defined policy is enacted. 
		// Instead of allowing AI to "hallucinate" an error handling path, 
		// we force a deterministic rollback (Undo) followed by error escalation.
		undoCtx := context.WithoutCancel(ctx)
		_ = task.Undo(undoCtx) 
		return err 
	}
	return nil
}

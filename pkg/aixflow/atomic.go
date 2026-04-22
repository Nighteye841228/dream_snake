package aixflow

import "context"

// Task defines an interface for an atomic unit of work.
// To adhere to "Principle 2: Error-Free Functional Atomicity and Partial Rollback," 
// all operations must implement Undo to maintain system integrity.
type Task interface {
	// Execute performs the core logic. If it returns an error, 
	// the AtomicRunner must invoke Undo().
	Execute(ctx context.Context) error

	// Undo is responsible for reverting any side effects caused by Execute 
	// (e.g., deleting temporary files, restoring memory states).
	Undo(ctx context.Context) error
}

// AtomicRunner manages task execution and ensures atomicity.
type AtomicRunner struct{}

// NewAtomicRunner creates a new instance of AtomicRunner.
func NewAtomicRunner() *AtomicRunner {
	return &AtomicRunner{}
}

// Run executes a task. If an error occurs, it automatically attempts recovery 
// via Undo and returns the original error.
func (r *AtomicRunner) Run(ctx context.Context, task Task) error {
	err := task.Execute(ctx)
	if err != nil {
		// An error occurred; trigger Undo (Principle 2: Partial Rollback).
		// Use context.WithoutCancel to ensure Undo can execute even if the original 
		// context has timed out or been canceled.
		undoCtx := context.WithoutCancel(ctx)
		undoErr := task.Undo(undoCtx)
		if undoErr != nil {
			// In a production environment, Undo failures should be logged via 
			// the SmartLogger as critical system errors.
		}
		return err // Return the original error for diagnosis.
	}
	return nil
}

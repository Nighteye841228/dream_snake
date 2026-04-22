package aixflow

import (
	"context"
	"time"
)

// Runner defines an interface for executing tasks.
type Runner interface {
	Run(ctx context.Context, task Task) error
}

// BudgetedRunner is a decorator that enforces an execution time budget 
// (Principle 3: Intelligent Logging and Time Budgets).
type BudgetedRunner struct {
	Base    Runner
	Timeout time.Duration
}

// NewBudgetedRunner creates a new BudgetedRunner that wraps an existing runner 
// with a timeout constraint.
func NewBudgetedRunner(base Runner, timeout time.Duration) *BudgetedRunner {
	return &BudgetedRunner{
		Base:    base,
		Timeout: timeout,
	}
}

// Run executes the task within the specified time budget.
// If the budget is exceeded, the context is canceled, triggering the Task's
// internal timeout handling and subsequent Undo logic.
//
// [MANUAL INTERVENTION POINT: Budget Calibration]
// In the AI era, infinite loops or stalled I/O are real risks. The Senior Engineer
// MUST calibrate Timeout based on the worst acceptable latency for the wrapped Task —
// AI-generated tasks should never run unbounded.
func (r *BudgetedRunner) Run(ctx context.Context, task Task) error {
	budgetCtx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	return r.Base.Run(budgetCtx, task)
}

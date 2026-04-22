package aixflow_test

import (
	"context"
	"testing"
	"time"

	"github.com/nighteye841228/aix-flow/pkg/aixflow"
)

// SlowTask simulates a long-running operation to test timeout constraints.
type SlowTask struct {
	Duration   time.Duration
	UndoCalled bool
}

func (t *SlowTask) Execute(ctx context.Context) error {
	select {
	case <-time.After(t.Duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *SlowTask) Undo(ctx context.Context) error {
	t.UndoCalled = true
	return nil
}

func TestBudgetedRunner_Timeout(t *testing.T) {
	base := aixflow.NewAtomicRunner()
	// Set a 10ms budget for a 100ms task.
	runner := aixflow.NewBudgetedRunner(base, 10*time.Millisecond)
	task := &SlowTask{Duration: 100 * time.Millisecond}

	err := runner.Run(context.Background(), task)
	if err == nil {
		t.Fatal("Expected timeout error, got success")
	}

	if !task.UndoCalled {
		t.Error("Undo should be triggered after budget exhaustion")
	}
}

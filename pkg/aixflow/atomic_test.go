package aixflow_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nighteye841228/aix-flow/pkg/aixflow"
)

// MockTask is a mock implementation of the Task interface for testing.
type MockTask struct {
	ExecuteCalled bool
	UndoCalled    bool
	ShouldFail    bool
}

func (t *MockTask) Execute(ctx context.Context) error {
	t.ExecuteCalled = true
	if t.ShouldFail {
		return errors.New("execution failed")
	}
	return nil
}

func (t *MockTask) Undo(ctx context.Context) error {
	t.UndoCalled = true
	return nil
}

func TestAtomicRunner_Run_Success(t *testing.T) {
	runner := aixflow.NewAtomicRunner()
	task := &MockTask{ShouldFail: false}

	err := runner.Run(context.Background(), task)
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	if !task.ExecuteCalled {
		t.Error("Execute was not called")
	}
	if task.UndoCalled {
		t.Error("Undo should not be called on success")
	}
}

func TestAtomicRunner_Run_FailureTriggersUndo(t *testing.T) {
	runner := aixflow.NewAtomicRunner()
	task := &MockTask{ShouldFail: true}

	err := runner.Run(context.Background(), task)
	if err == nil {
		t.Error("Expected error, got nil")
	}

	if !task.ExecuteCalled {
		t.Error("Execute was not called")
	}
	if !task.UndoCalled {
		t.Error("Undo was not called upon failure")
	}
}

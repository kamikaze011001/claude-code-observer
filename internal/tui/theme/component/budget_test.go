package component

import "testing"

func TestBudget_Allocate_HappyPath(t *testing.T) {
	b := Budget(40)
	b.Fixed("a", 10)
	b.Fixed("b", 12)
	b.Gutters(2, 1) // 2 gutters × 1 cell
	got := b.Flex("c")
	if got != 16 {
		t.Errorf("flex got %d want 16", got)
	}
	if err := b.Validate(); err != nil {
		t.Errorf("validate: %v", err)
	}
	if w := b.Width("c"); w != 16 {
		t.Errorf("flex width: got %d want 16", w)
	}
}

func TestBudget_Allocate_Overflow_FailsValidate(t *testing.T) {
	b := Budget(20)
	b.Fixed("a", 15)
	b.Fixed("b", 10)
	if err := b.Validate(); err == nil {
		t.Errorf("expected overflow error")
	}
}

func TestBudget_Validate_ExactFit(t *testing.T) {
	b := Budget(20)
	b.Fixed("a", 10)
	b.Fixed("b", 10)
	if err := b.Validate(); err != nil {
		t.Errorf("exact-fit: got %v want nil", err)
	}
}

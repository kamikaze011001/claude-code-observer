package component

import "testing"

func TestBudget_Allocate_HappyPath(t *testing.T) {
	b := Budget(40)
	b.Fixed("a", 10)
	b.Fixed("b", 12)
	b.Gutters(2, 1) // 2 gutters × 1 cell
	rest := b.Remaining()
	if rest != 16 {
		t.Errorf("remaining: got %d want 16", rest)
	}
	b.Flex("c", rest)
	if err := b.Validate(); err != nil {
		t.Errorf("validate: %v", err)
	}
	if got := b.Width("c"); got != 16 {
		t.Errorf("flex width: got %d want 16", got)
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

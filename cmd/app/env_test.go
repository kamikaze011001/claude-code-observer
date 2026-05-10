package main

import (
	"testing"
)

func TestParsePositiveIntEnv_UnsetReturnsDefault(t *testing.T) {
	t.Setenv("CCO_TEST_VAR", "")
	got, err := parsePositiveIntEnv("CCO_TEST_VAR", 30)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != 30 {
		t.Fatalf("got %d, want 30", got)
	}
}

func TestParsePositiveIntEnv_ValidValue(t *testing.T) {
	t.Setenv("CCO_TEST_VAR", "42")
	got, err := parsePositiveIntEnv("CCO_TEST_VAR", 30)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
}

func TestParsePositiveIntEnv_ZeroIsError(t *testing.T) {
	t.Setenv("CCO_TEST_VAR", "0")
	if _, err := parsePositiveIntEnv("CCO_TEST_VAR", 30); err == nil {
		t.Fatal("want error for 0, got nil")
	}
}

func TestParsePositiveIntEnv_NegativeIsError(t *testing.T) {
	t.Setenv("CCO_TEST_VAR", "-1")
	if _, err := parsePositiveIntEnv("CCO_TEST_VAR", 30); err == nil {
		t.Fatal("want error for -1, got nil")
	}
}

func TestParsePositiveIntEnv_NonIntegerIsError(t *testing.T) {
	t.Setenv("CCO_TEST_VAR", "abc")
	if _, err := parsePositiveIntEnv("CCO_TEST_VAR", 30); err == nil {
		t.Fatal("want error for abc, got nil")
	}
}

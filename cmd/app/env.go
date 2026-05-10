package main

import (
	"fmt"
	"os"
	"strconv"
)

// parsePositiveIntEnv reads name from the environment. If unset or empty,
// returns def. If set, the value must parse as a positive integer; otherwise
// an error is returned. Used at startup to fail fast on misconfiguration
// rather than silently falling back to defaults.
func parsePositiveIntEnv(name string, def int) (int, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s=%q: not an integer", name, raw)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s=%d: must be a positive integer", name, n)
	}
	return n, nil
}

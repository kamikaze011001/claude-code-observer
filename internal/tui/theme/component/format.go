package component

import "fmt"

// humanDuration formats a duration in seconds as e.g. "45s", "12m30s", "3h05m".
// Returns "" for non-positive values.
func humanDuration(sec int64) string {
	if sec <= 0 {
		return ""
	}
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	if sec < 3600 {
		return fmt.Sprintf("%dm%02ds", sec/60, sec%60)
	}
	return fmt.Sprintf("%dh%02dm", sec/3600, (sec%3600)/60)
}

// HumanInt formats large integers compactly: "1.2M", "38k", "999".
func HumanInt(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%dk", n/1_000)
	}
	return fmt.Sprintf("%d", n)
}

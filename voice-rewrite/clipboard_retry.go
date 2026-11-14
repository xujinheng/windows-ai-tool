package main

import "time"

// With the common two-attempt, 750ms-per-attempt configuration, an ignored
// first Ctrl+C should be retried after 450ms rather than waiting all 750ms.
// The second attempt receives the unused 300ms, keeping the combined budget
// identical. User-specified nondefault budgets retain their original behavior.
func selectionCopyAttemptTimeout(base time.Duration, attempt, attempts int) time.Duration {
	if attempts == 2 && base >= 600*time.Millisecond && base <= 1500*time.Millisecond {
		if attempt == 0 {
			return 450 * time.Millisecond
		}
		return base*2 - 450*time.Millisecond
	}
	return base
}

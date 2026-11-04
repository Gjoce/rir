package util

import "time"

const (
	JoinWindowBefore = 10 * time.Minute
	JoinWindowAfter  = 60 * time.Minute
)

// IsWithinJoinWindow reports whether now falls inside the allowed join
// window for a consultation scheduled at scheduledAt.
func IsWithinJoinWindow(scheduledAt, now time.Time) bool {
	if scheduledAt.IsZero() {
		return false
	}
	open := scheduledAt.Add(-JoinWindowBefore)
	close := scheduledAt.Add(JoinWindowAfter)

	return !now.Before(open) && now.Before(close)
}

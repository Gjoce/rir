package html

import (
	"testing"
	"time"
)

func TestAnswerTargetOpinionDeadline(t *testing.T) {
	start := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	deadline := start.Add(24 * time.Hour) // expires_at_after_video, set by the database

	tests := []struct {
		name       string
		now        time.Time
		wantPast   bool
		wantPassed bool
	}{
		{"before the call", start.Add(-time.Minute), false, false},
		{"right after the call", start.Add(time.Minute), true, false},
		{"one minute before the deadline", deadline.Add(-time.Minute), true, false},
		{"at the deadline", deadline, true, true},
		{"a day late", deadline.Add(24 * time.Hour), true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := answerTarget("doctor", "scheduled", "video", &start, &deadline, tt.now)
			if got.ScheduledAtPast != tt.wantPast || got.OpinionDeadlinePassed != tt.wantPassed {
				t.Errorf("past = %v, deadline passed = %v; want %v, %v",
					got.ScheduledAtPast, got.OpinionDeadlinePassed, tt.wantPast, tt.wantPassed)
			}
		})
	}

	written := answerTarget("doctor", "pending", "written", nil, nil, deadline.Add(time.Hour))
	if written.OpinionDeadlinePassed {
		t.Error("a written consultation has no opinion deadline")
	}
}

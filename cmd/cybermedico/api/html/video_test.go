package html

import (
	"strings"
	"testing"
	"time"
)

func TestJoinNotice(t *testing.T) {
	start := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC) // 12:00 in Skopje
	scheduled := func(status string) videoConsultation {
		return videoConsultation{ConsultationStatus: status, ConsultationType: "video", ScheduledAt: &start}
	}

	tests := []struct {
		name        string
		c           videoConsultation
		linkExpired bool
		now         time.Time
		wantNotice  bool
		wantTitle   string
		wantReload  time.Duration
	}{
		{"an hour early", scheduled("scheduled"), false, start.Add(-time.Hour), true, "hasn't started", 50 * time.Minute},
		{"two days early does not reload", scheduled("scheduled"), false, start.Add(-48 * time.Hour), true, "hasn't started", 0},
		{"window just opened", scheduled("scheduled"), false, start.Add(-10 * time.Minute), false, "", 0},
		{"during the call", scheduled("scheduled"), false, start.Add(30 * time.Minute), false, "", 0},
		{"window closed", scheduled("scheduled"), false, start.Add(60 * time.Minute), true, "has passed", 0},
		{"expired link", scheduled("scheduled"), true, start.Add(30 * time.Minute), true, "has passed", 0},
		{"finished", scheduled("finished"), false, start, true, "has finished", 0},
		{"not scheduled", videoConsultation{ConsultationStatus: "pending", ConsultationType: "video"}, false, start, true, "can't be joined", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notice, ok := joinNotice(tt.c, tt.linkExpired, tt.now)
			if ok != tt.wantNotice {
				t.Fatalf("notice = %v, want %v", ok, tt.wantNotice)
			}
			if !ok {
				return
			}
			if !strings.Contains(notice.Title, tt.wantTitle) {
				t.Errorf("title = %q, want it to contain %q", notice.Title, tt.wantTitle)
			}
			if notice.ReloadIn != tt.wantReload {
				t.Errorf("reload in = %v, want %v", notice.ReloadIn, tt.wantReload)
			}
			if tt.c.ConsultationStatus == "scheduled" && notice.AppointmentTime != "01.06.2026 12:00" {
				t.Errorf("appointment time = %q, want Skopje time", notice.AppointmentTime)
			}
		})
	}
}

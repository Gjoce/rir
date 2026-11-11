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

// AppointmentTimeFormat is how appointment times are shown to patients, the same
// as in the emails.
const AppointmentTimeFormat = "02.01.2006 15:04"

// FormatAppointmentTime formats t in Europe/Skopje, the clinic's time zone,
// wherever the app runs.
func FormatAppointmentTime(t time.Time) string {
	return FormatInSkopje(t, AppointmentTimeFormat)
}

// FormatInSkopje formats t with layout in Europe/Skopje.
func FormatInSkopje(t time.Time, layout string) string {
	location, err := time.LoadLocation("Europe/Skopje")
	if err != nil {
		return t.UTC().Format(layout) + " UTC"
	}

	return t.In(location).Format(layout)
}

// JoinWindowOpensAt returns when patients and doctors may first join.
func JoinWindowOpensAt(scheduledAt time.Time) time.Time {
	return scheduledAt.Add(-JoinWindowBefore)
}

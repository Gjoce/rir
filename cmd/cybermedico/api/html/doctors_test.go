package html

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func renderDoctorsPage(t *testing.T, doctors []DoctorRow) string {
	t.Helper()

	var buff bytes.Buffer
	if err := DoctorsPage(doctors).Render(context.Background(), &buff); err != nil {
		t.Fatalf("DoctorsPage.Render() error = %v", err)
	}

	return buff.String()
}

func TestDoctorsPageRendersAllDoctors(t *testing.T) {
	doctors := []DoctorRow{
		{
			ID:             "22222222-2222-2222-2222-222222222221",
			DisplayName:    "Dr. Emily Carter",
			Specialization: "Cardiology",
			Workplace:      "University Clinic for Cardiology, Skopje",
			Available:      true,
		},
		{
			ID:             "22222222-2222-2222-2222-222222222222",
			DisplayName:    "Dr. James Wilson",
			Specialization: "Neurology",
			Workplace:      "University Clinic for Neurology, Skopje",
			Available:      false,
		},
	}

	html := renderDoctorsPage(t, doctors)

	wants := []string{
		"Dr. Emily Carter",
		"Cardiology",
		"University Clinic for Cardiology, Skopje",
		"Dr. James Wilson",
		"Neurology",
		"University Clinic for Neurology, Skopje",
		">Available<",
		">Unavailable<",
		"Mark unavailable",
		"Mark available",
		`action="/doctors/22222222-2222-2222-2222-222222222221/availability"`,
		`action="/doctors/22222222-2222-2222-2222-222222222222/availability"`,
	}
	for _, want := range wants {
		t.Run(want, func(t *testing.T) {
			if !strings.Contains(html, want) {
				t.Errorf("DoctorsPage() does not contain %q", want)
			}
		})
	}

	// Each form sends the opposite of the doctor's current state.
	forms := []struct {
		name string
		want string
	}{
		{
			"available doctor's form sends available=false",
			`action="/doctors/22222222-2222-2222-2222-222222222221/availability" style="margin: 0;"><input type="hidden" name="available" value="false">`,
		},
		{
			"unavailable doctor's form sends available=true",
			`action="/doctors/22222222-2222-2222-2222-222222222222/availability" style="margin: 0;"><input type="hidden" name="available" value="true">`,
		},
	}
	for _, tt := range forms {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(html, tt.want) {
				t.Errorf("DoctorsPage() does not contain %q", tt.want)
			}
		})
	}
}

func TestDoctorsPageEmpty(t *testing.T) {
	html := renderDoctorsPage(t, nil)

	if !strings.Contains(html, "No doctors yet.") {
		t.Errorf("DoctorsPage(nil) does not contain %q", "No doctors yet.")
	}
	if strings.Contains(html, "<table") {
		t.Errorf("DoctorsPage(nil) should not render a table")
	}
}

func TestAvailabilityLabel(t *testing.T) {
	tests := []struct {
		name      string
		available bool
		want      string
	}{
		{"available doctor", true, "Available"},
		{"unavailable doctor", false, "Unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := availabilityLabel(tt.available); got != tt.want {
				t.Errorf("availabilityLabel(%v) = %q, want %q", tt.available, got, tt.want)
			}
		})
	}
}

func TestAvailabilityAction(t *testing.T) {
	tests := []struct {
		name      string
		available bool
		wantLabel string
		wantNext  string
	}{
		{"available doctor can be marked unavailable", true, "Mark unavailable", "false"},
		{"unavailable doctor can be marked available", false, "Mark available", "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label, next := availabilityAction(tt.available)
			if label != tt.wantLabel || next != tt.wantNext {
				t.Errorf("availabilityAction(%v) = (%q, %q), want (%q, %q)",
					tt.available, label, next, tt.wantLabel, tt.wantNext)
			}
		})
	}
}

package deviceevidence

import (
	"strings"
	"testing"
	"time"
)

func TestNewDraftIsCompleteAndValidAsDraft(t *testing.T) {
	now := time.Date(2026, time.August, 24, 6, 0, 0, 0, time.UTC)
	manifest, err := NewDraft("Asha Reviewer", "https://pilot.example.test", strings.Repeat("a", 40), now)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.CreatedAt != now || len(manifest.Devices) != len(requiredDeviceClasses) {
		t.Fatalf("unexpected draft: %#v", manifest)
	}
	if errors := Validate(manifest, ValidateDraft); len(errors) != 0 {
		t.Fatalf("draft validation errors: %v", errors)
	}
	if errors := Validate(manifest, ValidateComplete); len(errors) == 0 {
		t.Fatal("pending draft must not pass complete validation")
	}
}

func TestNewDraftRejectsPlaceholderReviewerAndCredentialURL(t *testing.T) {
	commit := strings.Repeat("b", 40)
	if _, err := NewDraft("Pending", "https://pilot.example.test", commit, time.Now()); err == nil {
		t.Fatal("placeholder reviewer was accepted")
	}
	if _, err := NewDraft("Asha Reviewer", "https://user:secret@pilot.example.test", commit, time.Now()); err == nil {
		t.Fatal("credential-bearing URL was accepted")
	}
}

func TestCompleteValidationRejectsOpenHighDefect(t *testing.T) {
	manifest, err := NewDraft("Asha Reviewer", "https://pilot.example.test", strings.Repeat("c", 40), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	completeTime := time.Now().UTC()
	for index := range manifest.Devices {
		device := &manifest.Devices[index]
		device.Status = "PASS"
		device.DeviceModel = "Physical device model"
		device.OSBrowser = "Observed OS and browser"
		device.Viewport = "390x844 portrait"
		device.Locale = "en-IN"
		device.AssistiveTechnology = "Observed screen reader"
		device.CompletedAt = &completeTime
		for key := range device.NetworkProfiles {
			device.NetworkProfiles[key] = "PASS"
		}
		for key := range device.Journeys {
			device.Journeys[key] = "PASS"
		}
		for _, kind := range requiredEvidenceKinds {
			device.Evidence = append(device.Evidence, Evidence{Kind: kind, URI: "https://evidence.example.test/" + kind, SHA256: strings.Repeat("d", 64), CapturedAt: completeTime, RetentionUntil: completeTime.AddDate(1, 0, 0)})
		}
	}
	manifest.Status = "PASS"
	if errors := Validate(manifest, ValidateComplete); len(errors) != 0 {
		t.Fatalf("complete evidence was rejected: %v", errors)
	}
	manifest.Devices[0].Defects = []Defect{{ID: "UI-1", Severity: "high", Status: "open"}}
	if errors := Validate(manifest, ValidateComplete); len(errors) == 0 {
		t.Fatal("open high defect was accepted")
	}
}

package deviceevidence

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type ValidationMode string

const (
	ValidateDraft    ValidationMode = "draft"
	ValidateComplete ValidationMode = "complete"
)

func Validate(manifest Manifest, mode ValidationMode) []error {
	errs := make([]error, 0)
	if mode != ValidateDraft && mode != ValidateComplete {
		return []error{fmt.Errorf("validation mode must be draft or complete")}
	}
	if manifest.SchemaVersion != SchemaVersion {
		errs = append(errs, fmt.Errorf("schema_version must be %s", SchemaVersion))
	}
	if !regexp.MustCompile(`^device-[0-9]{8}T[0-9]{6}Z-[0-9a-f]{8}$`).MatchString(manifest.RunID) {
		errs = append(errs, fmt.Errorf("run_id is not a generated device evidence run ID"))
	}
	if !isHex(strings.ToLower(manifest.CommitSHA), 40) {
		errs = append(errs, fmt.Errorf("commit_sha must be a full 40-character hexadecimal Git SHA"))
	}
	if err := validateReviewer(strings.TrimSpace(manifest.Reviewer)); err != nil {
		errs = append(errs, err)
	}
	if err := validateTargetURL(strings.TrimSpace(manifest.TargetURL)); err != nil {
		errs = append(errs, err)
	}
	if manifest.CreatedAt.IsZero() {
		errs = append(errs, fmt.Errorf("created_at is required"))
	}
	if manifest.TestData != requiredTestData {
		errs = append(errs, fmt.Errorf("test_data must preserve the approved exact INR 1.23 reversible fixture"))
	}

	devices := make(map[string]DeviceResult, len(manifest.Devices))
	allowedDevices := make(map[string]bool, len(requiredDeviceClasses))
	for _, class := range requiredDeviceClasses {
		allowedDevices[class] = true
	}
	for _, device := range manifest.Devices {
		if !allowedDevices[device.DeviceClass] {
			errs = append(errs, fmt.Errorf("unknown device class %q", device.DeviceClass))
			continue
		}
		if _, exists := devices[device.DeviceClass]; exists {
			errs = append(errs, fmt.Errorf("device class %q is duplicated", device.DeviceClass))
			continue
		}
		devices[device.DeviceClass] = device
	}
	for _, class := range requiredDeviceClasses {
		device, ok := devices[class]
		if !ok {
			errs = append(errs, fmt.Errorf("device class %q is missing", class))
			continue
		}
		errs = append(errs, validateDevice(device, manifest.Reviewer, manifest.CreatedAt, mode)...)
	}
	if mode == ValidateComplete && manifest.Status != "PASS" {
		errs = append(errs, fmt.Errorf("status must be PASS for complete evidence"))
	}
	return errs
}

func validateDevice(device DeviceResult, runReviewer string, runCreatedAt time.Time, mode ValidationMode) []error {
	errs := make([]error, 0)
	if device.Reviewer != runReviewer {
		errs = append(errs, fmt.Errorf("%s: reviewer must match the run reviewer", device.DeviceClass))
	}
	for _, profile := range requiredNetworkProfiles {
		result, ok := device.NetworkProfiles[profile]
		if !ok {
			errs = append(errs, fmt.Errorf("%s: network profile %s is missing", device.DeviceClass, profile))
		} else if mode == ValidateComplete && result != "PASS" {
			errs = append(errs, fmt.Errorf("%s: network profile %s must be PASS", device.DeviceClass, profile))
		}
	}
	for _, journey := range requiredJourneys {
		result, ok := device.Journeys[journey]
		if !ok {
			errs = append(errs, fmt.Errorf("%s: journey %s is missing", device.DeviceClass, journey))
		} else if mode == ValidateComplete && result != "PASS" {
			errs = append(errs, fmt.Errorf("%s: journey %s must be PASS", device.DeviceClass, journey))
		}
	}
	if mode == ValidateDraft {
		return errs
	}
	if device.Status != "PASS" {
		errs = append(errs, fmt.Errorf("%s: status must be PASS", device.DeviceClass))
	}
	for field, value := range map[string]string{
		"device_model":         device.DeviceModel,
		"os_browser":           device.OSBrowser,
		"viewport":             device.Viewport,
		"locale":               device.Locale,
		"assistive_technology": device.AssistiveTechnology,
	} {
		if strings.TrimSpace(value) == "" || strings.EqualFold(value, "pending") {
			errs = append(errs, fmt.Errorf("%s: %s must contain observed physical-device evidence", device.DeviceClass, field))
		}
	}
	if device.CompletedAt == nil || device.CompletedAt.IsZero() {
		errs = append(errs, fmt.Errorf("%s: completed_at is required", device.DeviceClass))
	} else if device.CompletedAt.Before(runCreatedAt) {
		errs = append(errs, fmt.Errorf("%s: completed_at cannot precede the evidence run", device.DeviceClass))
	}
	kinds := make(map[string]bool, len(device.Evidence))
	for index, evidence := range device.Evidence {
		kinds[evidence.Kind] = true
		if err := validateEvidenceURL(evidence.URI); err != nil {
			errs = append(errs, fmt.Errorf("%s: evidence %d must use a credential-free HTTPS URL", device.DeviceClass, index+1))
		}
		if !isHex(strings.ToLower(evidence.SHA256), 64) {
			errs = append(errs, fmt.Errorf("%s: evidence %d must include a SHA-256 digest", device.DeviceClass, index+1))
		}
		if evidence.CapturedAt.IsZero() || evidence.RetentionUntil.IsZero() || !evidence.RetentionUntil.After(evidence.CapturedAt) {
			errs = append(errs, fmt.Errorf("%s: evidence %d needs capture and future retention timestamps", device.DeviceClass, index+1))
		} else if evidence.CapturedAt.Before(runCreatedAt) {
			errs = append(errs, fmt.Errorf("%s: evidence %d was captured before the evidence run", device.DeviceClass, index+1))
		}
	}
	for _, kind := range requiredEvidenceKinds {
		if !kinds[kind] {
			errs = append(errs, fmt.Errorf("%s: evidence kind %s is missing", device.DeviceClass, kind))
		}
	}
	for _, defect := range device.Defects {
		severity := strings.ToLower(strings.TrimSpace(defect.Severity))
		status := strings.ToLower(strings.TrimSpace(defect.Status))
		if strings.TrimSpace(defect.ID) == "" {
			errs = append(errs, fmt.Errorf("%s: every defect needs an identifier", device.DeviceClass))
		}
		if severity != "critical" && severity != "high" && severity != "medium" && severity != "low" {
			errs = append(errs, fmt.Errorf("%s: defect %s has invalid severity %q", device.DeviceClass, defect.ID, defect.Severity))
		}
		if status != "closed" && status != "accepted" {
			errs = append(errs, fmt.Errorf("%s: defect %s must be closed or explicitly accepted", device.DeviceClass, defect.ID))
		}
		if (severity == "critical" || severity == "high") && status != "closed" {
			errs = append(errs, fmt.Errorf("%s: %s defect %s is not closed", device.DeviceClass, severity, defect.ID))
		}
		if status == "closed" {
			if err := validateEvidenceURL(defect.RetestURI); err != nil {
				errs = append(errs, fmt.Errorf("%s: closed defect %s needs credential-free HTTPS retest evidence", device.DeviceClass, defect.ID))
			}
		}
		if status == "accepted" {
			if err := validateEvidenceURL(defect.DecisionURI); err != nil {
				errs = append(errs, fmt.Errorf("%s: accepted defect %s needs a credential-free HTTPS decision reference", device.DeviceClass, defect.ID))
			}
		}
	}
	return errs
}

func validateEvidenceURL(raw string) error {
	if err := validateTargetURL(raw); err != nil || !strings.HasPrefix(strings.ToLower(raw), "https://") {
		return fmt.Errorf("evidence URL must use HTTPS")
	}
	return nil
}

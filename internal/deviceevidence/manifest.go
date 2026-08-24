package deviceevidence

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const SchemaVersion = "1.0"

var (
	requiredDeviceClasses   = []string{"iphone", "android", "tablet", "laptop", "desktop"}
	requiredNetworkProfiles = []string{"normal", "slow", "offline_before_submit", "lost_response_after_submit"}
	requiredJourneys        = []string{"navigation_and_focus", "account_investigation", "exact_transfer_and_compensation", "same_key_retry", "zoom_reflow_and_long_values", "assistive_technology"}
	requiredEvidenceKinds   = []string{"journey_recording", "retry_recording", "accessibility_notes"}
	requiredTestData        = TestData{
		TenantID:             "00000000-0000-4000-8000-000000000001",
		OperatorSubjectID:    "demo-operator",
		Currency:             "INR",
		AmountMinor:          "123",
		DisplayAmount:        "INR 1.23",
		ForwardDebitID:       "10000000-0000-4000-8000-000000000001",
		ForwardCreditID:      "10000000-0000-4000-8000-000000000004",
		CompensationDebitID:  "10000000-0000-4000-8000-000000000004",
		CompensationCreditID: "10000000-0000-4000-8000-000000000001",
	}
)

type Manifest struct {
	SchemaVersion string         `json:"schema_version"`
	RunID         string         `json:"run_id"`
	CommitSHA     string         `json:"commit_sha"`
	TargetURL     string         `json:"target_url"`
	Reviewer      string         `json:"reviewer"`
	CreatedAt     time.Time      `json:"created_at"`
	Status        string         `json:"status"`
	TestData      TestData       `json:"test_data"`
	Devices       []DeviceResult `json:"devices"`
}

type TestData struct {
	TenantID             string `json:"tenant_id"`
	OperatorSubjectID    string `json:"operator_subject_id"`
	Currency             string `json:"currency"`
	AmountMinor          string `json:"amount_minor"`
	DisplayAmount        string `json:"display_amount"`
	ForwardDebitID       string `json:"forward_debit_account_id"`
	ForwardCreditID      string `json:"forward_credit_account_id"`
	CompensationDebitID  string `json:"compensation_debit_account_id"`
	CompensationCreditID string `json:"compensation_credit_account_id"`
}

type DeviceResult struct {
	DeviceClass         string            `json:"device_class"`
	Status              string            `json:"status"`
	DeviceModel         string            `json:"device_model"`
	OSBrowser           string            `json:"os_browser"`
	Viewport            string            `json:"viewport"`
	Locale              string            `json:"locale"`
	AssistiveTechnology string            `json:"assistive_technology"`
	NetworkProfiles     map[string]string `json:"network_profiles"`
	Journeys            map[string]string `json:"journeys"`
	Evidence            []Evidence        `json:"evidence"`
	Defects             []Defect          `json:"defects"`
	Reviewer            string            `json:"reviewer"`
	CompletedAt         *time.Time        `json:"completed_at"`
}

type Evidence struct {
	Kind           string    `json:"kind"`
	URI            string    `json:"uri"`
	SHA256         string    `json:"sha256"`
	CapturedAt     time.Time `json:"captured_at"`
	RetentionUntil time.Time `json:"retention_until"`
}

type Defect struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Status      string `json:"status"`
	RetestURI   string `json:"retest_uri"`
	DecisionURI string `json:"decision_uri"`
}

func NewDraft(reviewer, targetURL, commitSHA string, now time.Time) (Manifest, error) {
	reviewer = strings.TrimSpace(reviewer)
	targetURL = strings.TrimSpace(targetURL)
	commitSHA = strings.ToLower(strings.TrimSpace(commitSHA))
	if err := validateReviewer(reviewer); err != nil {
		return Manifest{}, err
	}
	if err := validateTargetURL(targetURL); err != nil {
		return Manifest{}, err
	}
	if !isHex(commitSHA, 40) {
		return Manifest{}, fmt.Errorf("commit SHA must be the full 40-character hexadecimal Git SHA")
	}
	randomSuffix := make([]byte, 4)
	if _, err := rand.Read(randomSuffix); err != nil {
		return Manifest{}, fmt.Errorf("generate run identifier: %w", err)
	}
	now = now.UTC().Truncate(time.Second)
	runID := fmt.Sprintf("device-%s-%s", now.Format("20060102T150405Z"), hex.EncodeToString(randomSuffix))
	devices := make([]DeviceResult, 0, len(requiredDeviceClasses))
	for _, class := range requiredDeviceClasses {
		network := make(map[string]string, len(requiredNetworkProfiles))
		for _, profile := range requiredNetworkProfiles {
			network[profile] = "PENDING"
		}
		journeys := make(map[string]string, len(requiredJourneys))
		for _, journey := range requiredJourneys {
			journeys[journey] = "PENDING"
		}
		devices = append(devices, DeviceResult{
			DeviceClass:         class,
			Status:              "PENDING",
			DeviceModel:         "PENDING",
			OSBrowser:           "PENDING",
			Viewport:            "PENDING",
			Locale:              "PENDING",
			AssistiveTechnology: "PENDING",
			NetworkProfiles:     network,
			Journeys:            journeys,
			Evidence:            []Evidence{},
			Defects:             []Defect{},
			Reviewer:            reviewer,
		})
	}
	return Manifest{
		SchemaVersion: SchemaVersion,
		RunID:         runID,
		CommitSHA:     commitSHA,
		TargetURL:     targetURL,
		Reviewer:      reviewer,
		CreatedAt:     now,
		Status:        "PENDING",
		TestData:      requiredTestData,
		Devices:       devices,
	}, nil
}

func validateTargetURL(raw string) error {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("target URL must be an absolute http or https URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("target URL must not contain credentials, query parameters, or a fragment")
	}
	return nil
}

func validateReviewer(reviewer string) error {
	if len(reviewer) < 3 || strings.EqualFold(reviewer, "pending") || strings.EqualFold(reviewer, "unassigned") {
		return fmt.Errorf("reviewer must identify a named accountable person")
	}
	return nil
}

func isHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

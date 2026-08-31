package investigation

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	SavedViewFilterSchemaVersion = 1
	MaxSavedViewsPerOperator     = 25
)

var (
	ErrInvalidSavedView    = errors.New("invalid saved investigation view")
	ErrSavedViewConflict   = errors.New("saved investigation view conflict")
	ErrSavedViewLimit      = errors.New("saved investigation view limit reached")
	ErrSavedViewNotFound   = errors.New("saved investigation view not found")
	ErrSavedViewVersion    = errors.New("saved investigation view version conflict")
	canonicalSavedViewUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	transferSavedViewQuery = regexp.MustCompile(`^[0-9a-f-]{1,128}$`)
	eventSavedViewType     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

type SavedView struct {
	ID                  string            `json:"saved_view_id"`
	Name                string            `json:"name"`
	FilterSchemaVersion string            `json:"filter_schema_version"`
	Domain              string            `json:"domain"`
	Filters             map[string]string `json:"filters"`
	TargetPath          string            `json:"target_path"`
	Version             string            `json:"version"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

type SavedViewPage struct {
	Views       []SavedView `json:"views"`
	GeneratedAt time.Time   `json:"generated_at"`
}

type SavedViewCreate struct {
	TenantID, ActorID, Name, Domain, CorrelationID string
	FilterSchemaVersion                            int
	Filters                                        map[string]string
	Access                                         SavedViewAccess
	OccurredAt                                     time.Time
}

type SavedViewRename struct {
	TenantID, ActorID, SavedViewID, Name, CorrelationID string
	ExpectedVersion                                     int64
	Access                                              SavedViewAccess
	OccurredAt                                          time.Time
}

type SavedViewDelete struct {
	TenantID, ActorID, SavedViewID, CorrelationID string
	ExpectedVersion                               int64
	Access                                        SavedViewAccess
	OccurredAt                                    time.Time
}

type SavedViewRepository interface {
	ListSavedViews(context.Context, string, string, SavedViewAccess) (SavedViewPage, error)
	CreateSavedView(context.Context, SavedViewCreate) (SavedView, error)
	RenameSavedView(context.Context, SavedViewRename) (SavedView, error)
	DeleteSavedView(context.Context, SavedViewDelete) error
}

type SavedViewAccess struct {
	Accounts, Transfers, Funding, FundingApprovals, Corrections, CorrectionApprovals, Events, Webhooks bool
}

func (access SavedViewAccess) AnyReadableDomain() bool {
	return access.Accounts || access.Transfers || access.Funding || access.FundingApprovals || access.Corrections || access.CorrectionApprovals || access.Events || access.Webhooks
}

func NormalizeSavedViewName(value string) (string, error) {
	if !utf8.ValidString(value) || value != strings.TrimSpace(value) || utf8.RuneCountInString(value) < 1 || utf8.RuneCountInString(value) > 80 {
		return "", ErrInvalidSavedView
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return "", ErrInvalidSavedView
		}
	}
	return value, nil
}

func ParseSavedViewVersion(value string, allowZero bool) (int64, error) {
	if value == "" || len(value) > 19 || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") {
		return 0, ErrInvalidSavedView
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 || (!allowZero && parsed == 0) {
		return 0, ErrInvalidSavedView
	}
	return parsed, nil
}

func NormalizeSavedViewDefinition(domain string, schemaVersion int, input map[string]string) (map[string]string, string, error) {
	if schemaVersion != SavedViewFilterSchemaVersion || len(input) < 1 || len(input) > 8 {
		return nil, "", ErrInvalidSavedView
	}
	rules, path, ok := savedViewRules(domain)
	if !ok {
		return nil, "", ErrInvalidSavedView
	}
	filters := make(map[string]string, len(input))
	for key, raw := range input {
		rule, allowed := rules[key]
		if !allowed || raw == "" || raw != strings.TrimSpace(raw) {
			return nil, "", ErrInvalidSavedView
		}
		value, valid := rule(raw)
		if !valid {
			return nil, "", ErrInvalidSavedView
		}
		filters[key] = value
	}
	if !savedViewCombinationValid(domain, filters) {
		return nil, "", ErrInvalidSavedView
	}
	query := url.Values{}
	keys := make([]string, 0, len(filters))
	for key := range filters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		query.Set(key, filters[key])
	}
	return filters, path + "?" + query.Encode(), nil
}

type savedViewRule func(string) (string, bool)

func savedViewRules(domain string) (map[string]savedViewRule, string, bool) {
	enum := func(values ...string) savedViewRule {
		allowed := make(map[string]struct{}, len(values))
		for _, value := range values {
			allowed[value] = struct{}{}
		}
		return func(value string) (string, bool) { _, ok := allowed[value]; return value, ok }
	}
	uuid := func(value string) (string, bool) {
		value = strings.ToLower(value)
		return value, canonicalSavedViewUUID.MatchString(value)
	}
	rfc3339 := func(value string) (string, bool) {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return "", false
		}
		return parsed.UTC().Format(time.RFC3339Nano), true
	}
	date := func(value string) (string, bool) {
		parsed, err := time.Parse("2006-01-02", value)
		return value, err == nil && parsed.Format("2006-01-02") == value
	}
	eventType := func(value string) (string, bool) {
		return value, len(value) <= 128 && eventSavedViewType.MatchString(value)
	}
	switch domain {
	case "accounts":
		return map[string]savedViewRule{"status": enum("active", "frozen", "closed"), "category": enum("operating", "customer_funds", "payroll", "payables", "expenses", "reserve")}, "/accounts", true
	case "transfers":
		return map[string]savedViewRule{"q": func(value string) (string, bool) {
			value = strings.ToLower(value)
			return value, transferSavedViewQuery.MatchString(value)
		}, "accountId": uuid, "status": enum("pending", "posted", "rejected"), "from": rfc3339, "to": rfc3339}, "/transfers", true
	case "funding":
		return map[string]savedViewRule{"status": enum("requested", "approved", "posted", "rejected", "compensated")}, "/funding", true
	case "approvals":
		return map[string]savedViewRule{"domain": enum("funding", "correction"), "status": enum("funding:requested", "funding:approved", "funding:posted", "funding:rejected", "funding:compensated", "correction:requested", "correction:approved", "correction:rejected", "correction:cancelled", "correction:expired", "correction:posted"), "age": enum("under_24h", "over_24h", "over_7d", "over_30d"), "requested_after": date, "requested_before": date, "actionable_by_me": enum("true")}, "/approvals", true
	case "corrections":
		return map[string]savedViewRule{"status": enum("requested", "approved", "posted", "rejected", "cancelled", "expired")}, "/corrections", true
	case "events":
		return map[string]savedViewRule{"eventType": eventType, "state": enum("pending", "retrying", "published", "dead"), "endpointId": uuid, "relatedId": uuid, "correlationId": uuid, "from": rfc3339, "to": rfc3339}, "/events", true
	case "webhooks":
		return map[string]savedViewRule{"status": enum("pending_verification", "active", "disabled"), "eventType": eventType}, "/webhooks", true
	default:
		return nil, "", false
	}
}

func savedViewCombinationValid(domain string, filters map[string]string) bool {
	if from, ok := filters["from"]; ok {
		if to, exists := filters["to"]; exists && from > to {
			return false
		}
	}
	if after, ok := filters["requested_after"]; ok {
		if before, exists := filters["requested_before"]; exists && after > before {
			return false
		}
	}
	if domain == "approvals" && filters["domain"] != "" && filters["status"] != "" && !strings.HasPrefix(filters["status"], filters["domain"]+":") {
		return false
	}
	return true
}

func SavedViewDefinitionAllowed(domain string, filters map[string]string, access SavedViewAccess) bool {
	switch domain {
	case "accounts":
		return access.Accounts
	case "transfers":
		return access.Transfers
	case "funding":
		return access.Funding
	case "approvals":
		switch filters["domain"] {
		case "funding":
			return access.FundingApprovals
		case "correction":
			return access.CorrectionApprovals
		default:
			return access.FundingApprovals || access.CorrectionApprovals
		}
	case "corrections":
		return access.Corrections
	case "events":
		return access.Events
	case "webhooks":
		return access.Webhooks
	default:
		return false
	}
}

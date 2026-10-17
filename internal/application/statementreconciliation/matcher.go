package statementreconciliation

import "sort"

type LedgerItem struct {
	ID          string `json:"posting_id"`
	Reference   string `json:"external_reference"`
	Direction   string `json:"direction"`
	AmountMinor int64  `json:"amount_minor,string"`
	Currency    string `json:"currency"`
}
type ExactMatch struct {
	RowID     string `json:"row_id"`
	PostingID string `json:"posting_id"`
}
type RowMatch struct {
	RowID      string   `json:"row_id"`
	Status     string   `json:"status"`
	Candidates []string `json:"candidates"`
}
type MatchResult struct {
	Matches           []ExactMatch `json:"matches"`
	Rows              []RowMatch   `json:"rows"`
	UnmatchedPostings []string     `json:"unmatched_postings"`
}
type matchKey struct {
	reference, direction, currency string
	minor                          int64
}

// MatchExact uses both sides of the candidate graph. A shared posting or several
// eligible postings is ambiguous for every affected row; ordering cannot choose
// a winner. The caller persists its fixed snapshot with the result.
func MatchExact(rows []Row, items []LedgerItem) MatchResult {
	result := MatchResult{Matches: []ExactMatch{}, Rows: []RowMatch{}, UnmatchedPostings: []string{}}
	byKey := map[matchKey][]string{}
	rowCounts := map[matchKey]int{}
	byReference := map[string]bool{}
	for _, item := range items {
		key := matchKey{item.Reference, item.Direction, item.Currency, item.AmountMinor}
		byKey[key] = append(byKey[key], item.ID)
		byReference[item.Reference] = true
	}
	for _, row := range rows {
		rowCounts[matchKey{row.Reference, row.Direction, row.Currency, row.AmountMinor}]++
	}
	used := map[string]bool{}
	for _, row := range rows {
		key := matchKey{row.Reference, row.Direction, row.Currency, row.AmountMinor}
		candidates := append([]string{}, byKey[key]...)
		sort.Strings(candidates)
		status := "unmatched_external"
		if len(candidates) > 0 {
			status = "ambiguous"
			if len(candidates) == 1 && rowCounts[key] == 1 {
				status = "matched"
				used[candidates[0]] = true
				result.Matches = append(result.Matches, ExactMatch{row.ID, candidates[0]})
			}
		} else if byReference[row.Reference] {
			status = "amount_or_direction_difference"
		}
		result.Rows = append(result.Rows, RowMatch{row.ID, status, candidates})
	}
	for _, item := range items {
		if !used[item.ID] {
			result.UnmatchedPostings = append(result.UnmatchedPostings, item.ID)
		}
	}
	sort.Strings(result.UnmatchedPostings)
	return result
}

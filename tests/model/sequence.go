package model

import "math/rand"

type Operation struct {
	Kind      string `json:"kind"`
	Minor     int64  `json:"minor"`
	Reference int    `json:"reference"`
	Reverse   bool   `json:"reverse"`
}

// The stable seed and explicit operations both remain in a failure artifact.
// Prefix operations guarantee that each selected boundary is reached.
func Generate(seed int64, count int) []Operation {
	if count < 10 || count > 256 {
		panic("model sequence requires 10-256 operations")
	}
	random := rand.New(rand.NewSource(seed))
	kinds := []string{"transfer", "replay", "conflict", "fund", "correct", "lost-response", "rollback", "concurrent", "insufficient"}
	result := make([]Operation, count)
	for i := range result {
		kind := kinds[i%len(kinds)]
		if i >= len(kinds) {
			kind = kinds[random.Intn(len(kinds))]
		}
		result[i] = Operation{Kind: kind, Minor: int64(random.Intn(500) + 1), Reference: random.Intn(i + 1), Reverse: random.Intn(2) == 1}
		if kind == "insufficient" {
			result[i].Minor = 1_000_000
		}
	}
	result[0] = Operation{Kind: "transfer", Minor: 100}
	result[4].Reference = 0
	result[8] = Operation{Kind: "insufficient", Minor: 1_000_000}
	result[9] = Operation{Kind: "replay", Reference: 8}
	return result
}

// Minimize performs bounded deletion. The predicate must reproduce the same
// mismatch class, not an unrelated infrastructure or invalid-input failure.
func Minimize(input []Operation, budget int, stillFails func([]Operation) bool) []Operation {
	best := append([]Operation(nil), input...)
	for width := len(best) / 2; width >= 1 && budget > 0; width /= 2 {
		for start := 0; start+width <= len(best) && budget > 0; {
			candidate := append(append([]Operation(nil), best[:start]...), best[start+width:]...)
			budget--
			if len(candidate) > 0 && stillFails(candidate) {
				best = candidate
			} else {
				start += width
			}
		}
	}
	return best
}

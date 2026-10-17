package integration_test

import (
	"errors"
	"testing"
)

func capacityPairIndex(profile string, sequence int64) (int, error) {
	if sequence < 0 {
		return 0, errors.New("nonnegative capacity sequence required")
	}
	switch profile {
	case "mixed":
		slot := sequence % 12
		if slot >= 9 {
			return 2, nil
		}
		if slot >= 6 {
			return 1, nil
		}
		return 0, nil
	case "independent-pairs":
		return int(sequence % 2), nil
	case "hot-pair":
		return 0, nil
	case "multi-tenant":
		return int(sequence%2) * 2, nil
	default:
		return 0, errors.New("unsupported fixed capacity profile")
	}
}

func TestCapacityProfilesKeepTenantAndContentionComparisonsSeparate(t *testing.T) {
	for _, profile := range []string{"mixed", "independent-pairs", "hot-pair", "multi-tenant"} {
		counts := [3]int{}
		for i := int64(0); i < 120; i++ {
			pair, err := capacityPairIndex(profile, i)
			if err != nil || pair < 0 || pair > 2 {
				t.Fatal("invalid bounded account pair")
			}
			counts[pair]++
		}
		if profile == "hot-pair" && counts != [3]int{120, 0, 0} || profile == "independent-pairs" && counts != [3]int{60, 60, 0} || profile == "multi-tenant" && counts != [3]int{60, 0, 60} {
			t.Fatalf("contention or tenant comparison mixed its profiles: %s %v", profile, counts)
		}
	}
	if _, err := capacityPairIndex("unbounded-tenant-input", 0); err == nil {
		t.Fatal("unknown workload selected a default")
	}
}

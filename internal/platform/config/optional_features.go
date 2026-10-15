package config

import (
	"fmt"
	"os"
	"strings"
)

// Optional study surfaces use one exact flag vocabulary on the API and BFF.
func optionalFlag(name string) (bool, error) {
	switch strings.TrimSpace(os.Getenv(name)) {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, fmt.Errorf("%s must be true or false", name)
	}
}

func validateOptionalFeatures(environment string, study, live bool) error {
	if study && live {
		return fmt.Errorf("study and collaboration are separate qualified feature sets")
	}
	if (study || live) && environment != "development" {
		return fmt.Errorf("optional study features require the local development environment")
	}
	return nil
}

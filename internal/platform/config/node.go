package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var nodeIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,39}$`)

func configuredNodeID() (string, error) {
	id := strings.TrimSpace(os.Getenv("LEDGERSYNC_NODE_ID"))
	if id == "" {
		id, _ = os.Hostname()
	}
	if !nodeIDPattern.MatchString(id) {
		return "", fmt.Errorf("LEDGERSYNC_NODE_ID must be a bounded hostname or unique node name")
	}
	return id, nil
}

// NodeIdentity survives a process restart. Container replacement uses a new
// identity unless the operator explicitly retains a unique node name.
func (c Config) NodeIdentity(role string) string { return "ledgersync/" + role + "/" + c.NodeID }

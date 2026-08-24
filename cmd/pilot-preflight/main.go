package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/pilotgate"
)

func main() {
	evidencePath := flag.String("evidence-file", "", "path to the secret-free managed pilot evidence JSON")
	requireRestore := flag.Bool("require-restore", false, "require provider-backed PITR restore evidence")
	flag.Parse()
	if *evidencePath == "" {
		fmt.Fprintln(os.Stderr, "usage: pilot-preflight --evidence-file <path> [--require-restore]")
		os.Exit(2)
	}

	encoded, err := os.ReadFile(*evidencePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pilot evidence file could not be read")
		os.Exit(1)
	}
	var evidence pilotgate.Evidence
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&evidence); err != nil {
		fmt.Fprintln(os.Stderr, "pilot evidence JSON is invalid")
		os.Exit(1)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		fmt.Fprintln(os.Stderr, "pilot evidence JSON must contain exactly one object")
		os.Exit(1)
	}
	if err := evidence.Validate(*requireRestore); err != nil {
		fmt.Fprintf(os.Stderr, "pilot preflight failed:\n%v\n", err)
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "pass", "revision": evidence.Revision, "provider_restore_required": *requireRestore})
}

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/deviceevidence"
)

func main() {
	if len(os.Args) < 2 {
		fail(errors.New("usage: device-evidence create|validate [options]"))
	}
	var err error
	switch os.Args[1] {
	case "create":
		err = create(os.Args[2:])
	case "validate":
		err = validate(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q; use create or validate", os.Args[1])
	}
	if err != nil {
		fail(err)
	}
}

func create(arguments []string) error {
	flags := flag.NewFlagSet("create", flag.ContinueOnError)
	reviewer := flags.String("reviewer", "", "named accountable reviewer")
	targetURL := flags.String("target-url", "", "credential-free physical-device target URL")
	outputRoot := flags.String("output-root", filepath.FromSlash(".tmp/device-evidence"), "local ignored evidence workspace")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if strings.TrimSpace(*reviewer) == "" || strings.TrimSpace(*targetURL) == "" {
		return errors.New("create requires -reviewer and -target-url")
	}
	commitSHA, err := gitOutput("rev-parse", "HEAD")
	if err != nil {
		return err
	}
	dirty, err := gitOutput("status", "--porcelain")
	if err != nil {
		return err
	}
	if dirty != "" {
		return errors.New("refusing to bind evidence to a dirty working tree; commit or restore changes first")
	}
	manifest, err := deviceevidence.NewDraft(*reviewer, *targetURL, commitSHA, time.Now())
	if err != nil {
		return err
	}
	runDirectory := filepath.Join(*outputRoot, manifest.RunID)
	if _, err := os.Stat(runDirectory); err == nil {
		return fmt.Errorf("run directory already exists: %s", runDirectory)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(runDirectory, 0o750); err != nil {
		return fmt.Errorf("create run directory: %w", err)
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	manifestBytes = append(manifestBytes, '\n')
	if err := os.WriteFile(filepath.Join(runDirectory, "manifest.json"), manifestBytes, 0o640); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(runDirectory, "checklist.md"), []byte(deviceevidence.Checklist(manifest)), 0o640); err != nil {
		return fmt.Errorf("write checklist: %w", err)
	}
	fmt.Printf("Created %s\n", runDirectory)
	fmt.Printf("Run ID: %s\nCommit: %s\n", manifest.RunID, manifest.CommitSHA)
	return nil
}

func validate(arguments []string) error {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	manifestPath := flags.String("manifest", "", "path to a device evidence manifest")
	mode := flags.String("mode", string(deviceevidence.ValidateComplete), "draft or complete")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if strings.TrimSpace(*manifestPath) == "" {
		return errors.New("validate requires -manifest")
	}
	contents, err := os.ReadFile(*manifestPath)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(contents)))
	decoder.DisallowUnknownFields()
	var manifest deviceevidence.Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("decode manifest: multiple JSON values are not allowed")
		}
		return fmt.Errorf("decode manifest trailer: %w", err)
	}
	errs := deviceevidence.Validate(manifest, deviceevidence.ValidationMode(*mode))
	if len(errs) > 0 {
		messages := make([]string, 0, len(errs))
		for _, validationErr := range errs {
			messages = append(messages, "- "+validationErr.Error())
		}
		return fmt.Errorf("manifest validation failed:\n%s", strings.Join(messages, "\n"))
	}
	fmt.Printf("%s validation passed for %s at commit %s\n", *mode, manifest.RunID, manifest.CommitSHA)
	return nil
}

func gitOutput(arguments ...string) (string, error) {
	command := exec.Command("git", arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

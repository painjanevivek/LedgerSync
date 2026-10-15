package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const modulePrefix = "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/"

func forbiddenImport(layer, dependency string) bool {
	local := strings.TrimPrefix(dependency, modulePrefix)
	if layer == "domain" {
		if strings.HasPrefix(dependency, modulePrefix) && !strings.HasPrefix(local, "internal/domain/") {
			return true
		}
		// The identifier value type currently implements sql.Scanner/driver.Valuer.
		// Only database/sql/driver is retained; concrete persistence belongs outside domain.
		return dependency == "database/sql" || dependency == "net/http" || strings.HasPrefix(dependency, "net/http/") || strings.Contains(dependency, "github.com/jackc/pgx") || strings.Contains(dependency, "github.com/redis/")
	}
	if layer == "application" {
		for _, prefix := range []string{"internal/platform/", "internal/transport/", "internal/bootstrap", "cmd/"} {
			if strings.HasPrefix(local, prefix) {
				return true
			}
		}
		return dependency == "database/sql" || dependency == "crypto/tls" || dependency == "net/http" || strings.HasPrefix(dependency, "net/http/") || strings.Contains(dependency, "github.com/jackc/pgx") || strings.Contains(dependency, "github.com/redis/")
	}
	return false
}

func TestDependencyBoundaryPolicy(t *testing.T) {
	for _, test := range []struct {
		layer, dependency string
		forbidden         bool
	}{
		{"domain", modulePrefix + "internal/application/transfers", true},
		{"domain", modulePrefix + "internal/platform/db", true},
		{"domain", "database/sql", true},
		{"domain", "database/sql/driver", false},
		{"domain", modulePrefix + "internal/domain/money", false},
		{"domain", "net/http", true},
		{"application", modulePrefix + "internal/transport/http", true},
		{"application", modulePrefix + "internal/bootstrap", true},
		{"application", "github.com/jackc/pgx/v5", true},
		{"application", modulePrefix + "internal/domain/money", false},
		{"application", modulePrefix + "internal/application/lease", false},
		{"application", "net/http", true},
		// Endpoint validation uses net.ParseIP without opening a connection.
		{"application", "net", false},
		{"application", "crypto/tls", true},
	} {
		t.Run(test.layer+"/"+test.dependency, func(t *testing.T) {
			if got := forbiddenImport(test.layer, test.dependency); got != test.forbidden {
				t.Fatalf("forbidden=%v, want %v", got, test.forbidden)
			}
		})
	}
}

func TestProductionDependencyBoundaries(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal("repository root unavailable:", err)
	}
	for _, layer := range []string{"domain", "application"} {
		count := 0
		err := filepath.WalkDir(filepath.Join(root, "internal", layer), func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			count++
			file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, specification := range file.Imports {
				dependency, err := strconv.Unquote(specification.Path.Value)
				if err != nil {
					return err
				}
				if forbiddenImport(layer, dependency) {
					relative, _ := filepath.Rel(root, name)
					t.Errorf("%s imports forbidden dependency %s; see ADR-001", relative, dependency)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			t.Fatalf("no %s production files examined", layer)
		}
		t.Logf("Checked %d %s production files", count, layer)
	}
}

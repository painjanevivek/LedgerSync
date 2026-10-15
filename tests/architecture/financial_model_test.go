package architecture_test

import (
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestFinancialOracleUsesOnlyStandardLibrary(t *testing.T) {
	root := filepath.Join("..", "model")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		path := filepath.Join(root, entry.Name())
		source, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, dependency := range source.Imports {
			value, err := strconv.Unquote(dependency.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			pkg, err := build.Default.Import(value, "", build.FindOnly)
			if err != nil || !pkg.Goroot {
				t.Errorf("independent oracle %s imports non-standard dependency %s", entry.Name(), value)
			}
		}
	}
}

package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var directFinancialWrite = regexp.MustCompile(`(?i)\b(?:INSERT\s+INTO|UPDATE|DELETE\s+FROM|TRUNCATE(?:\s+TABLE)?)\s+(?:public\.)?(accounts|account_opening_balances|account_balance_projections|account_owners|account_credit_permissions|transfers|journal_transactions|ledger_postings|funding_events|funding_velocity_events|transfer_velocity_events|transfer_velocity_totals|transfer_corrections|approval_records|audit_events)\b`)

func TestRuntimeFinancialWritesUseControlledCapabilities(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "internal", "platform", "db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlStart := regexp.MustCompile(`^(WITH|SELECT|INSERT|UPDATE|DELETE|TRUNCATE)\s`)
	err = filepath.WalkDir(root, func(path string, item fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			set := token.NewFileSet()
			source, err := parser.ParseFile(set, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(source, func(node ast.Node) bool {
				literal, ok := node.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					return true
				}
				value, err := strconv.Unquote(literal.Value)
				if err != nil {
					return true
				}
				value = strings.TrimSpace(value)
				if sqlStart.MatchString(value) && directFinancialWrite.MatchString(value) {
					t.Errorf("ordinary financial writer at %s; use a controlled capability", set.Position(literal.Pos()))
				}
				return true
			})
		}
		if strings.HasSuffix(path, ".sql") {
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if directFinancialWrite.Match(body) {
				t.Errorf("query template %s contains a direct financial writer", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

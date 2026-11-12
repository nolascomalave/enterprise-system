package transaction_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoDirectTransactionsOutsideManager enforces the mandatory rule in
// .opencode/rules/transaction-guardrails.md: only the transaction manager and the
// tenancy driver may open transactions.
func TestNoDirectTransactionsOutsideManager(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	root := filepath.Join(wd, "..", "..", "..")

	allowed := map[string]bool{
		filepath.Join("internal", "infrastructure", "transaction", "manager.go"): true,
		filepath.Join("internal", "infrastructure", "enttenant", "driver.go"):    true,
	}

	var violations []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(rel, "internal"+string(filepath.Separator)) &&
			!strings.HasPrefix(rel, "cmd"+string(filepath.Separator)) {
			return nil
		}
		if strings.HasSuffix(rel, "_test.go") || allowed[rel] {
			return nil
		}

		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(source)
		if strings.Contains(text, "client.Tx(") || strings.Contains(text, ".Tx(ctx)") {
			violations = append(violations, rel)
		}
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, violations,
		"open transactions only through TransactionManager.WithinTransaction; offenders: %v", violations)
}

package regex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRulesFromFile(t *testing.T) {
	tmp := t.TempDir()
	yaml := `rules:
  - id: test-rule
    name: Test Rule
    severity: HIGH
    pattern: 'secret[_-]?key\s*=\s*\S+'
    tags:
      - test
`
	err := os.WriteFile(filepath.Join(tmp, "test.yaml"), []byte(yaml), 0o644)
	require.NoError(t, err)

	rules, err := LoadRulesFromFile(filepath.Join(tmp, "test.yaml"))
	require.NoError(t, err)
	assert.Len(t, rules, 1)
	assert.Equal(t, "test-rule", rules[0].ID)

	matches := rules[0].Match("secret_key = abc123def456")
	assert.NotEmpty(t, matches)
}

func TestLoadRulesFromDir(t *testing.T) {
	rulesDir := filepath.Join("..", "..", "..", "rules")
	if _, err := os.Stat(rulesDir); os.IsNotExist(err) {
		t.Skip("rulesディレクトリが見つかりません")
	}

	rules, err := LoadRulesFromDir(rulesDir)
	require.NoError(t, err)
	assert.NotEmpty(t, rules)
}

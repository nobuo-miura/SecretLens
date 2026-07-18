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

func TestParseRulesValidation(t *testing.T) {
	tmp := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(tmp, name)
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
		return p
	}

	// idなし
	_, err := LoadRulesFromFile(write("noid.yaml", "rules:\n  - name: X\n    severity: HIGH\n    pattern: 'x'\n"))
	assert.ErrorContains(t, err, "id")

	// nameなし
	_, err = LoadRulesFromFile(write("noname.yaml", "rules:\n  - id: a\n    severity: HIGH\n    pattern: 'x'\n"))
	assert.ErrorContains(t, err, "name")

	// patternなし
	_, err = LoadRulesFromFile(write("nopat.yaml", "rules:\n  - id: a\n    name: X\n    severity: HIGH\n"))
	assert.ErrorContains(t, err, "pattern")

	// 不正severity
	_, err = LoadRulesFromFile(write("badsev.yaml", "rules:\n  - id: a\n    name: X\n    severity: URGENT\n    pattern: 'x'\n"))
	assert.ErrorContains(t, err, "severity")

	// 未知のYAML項目（typo）
	_, err = LoadRulesFromFile(write("typo.yaml", "rules:\n  - id: a\n    name: X\n    severity: HIGH\n    pattern: 'x'\n    entoropy_min: 3.0\n"))
	assert.Error(t, err)
}

func TestLoadRulesFromDirDuplicateID(t *testing.T) {
	tmp := t.TempDir()
	rule := "rules:\n  - id: dup\n    name: A\n    severity: HIGH\n    pattern: 'x'\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "a.yaml"), []byte(rule), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "b.yaml"), []byte(rule), 0o644))

	_, err := LoadRulesFromDir(tmp)
	assert.ErrorContains(t, err, "重複")
}

func TestMergeRules(t *testing.T) {
	base := []Rule{{ID: "a", Severity: "HIGH"}, {ID: "b", Severity: "LOW"}}
	overrides := []Rule{{ID: "b", Severity: "CRITICAL"}, {ID: "c", Severity: "MEDIUM"}}

	merged := MergeRules(base, overrides)
	require.Len(t, merged, 3)
	assert.Equal(t, "HIGH", merged[0].Severity)     // aはそのまま
	assert.Equal(t, "CRITICAL", merged[1].Severity) // bは上書き
	assert.Equal(t, "c", merged[2].ID)              // cは追加
}

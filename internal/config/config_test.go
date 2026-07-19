package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secretlens.yml")
	content := `source: all
format: sarif
out: results.sarif
fail_on: HIGH
rules_dir: ./custom-rules
baseline: my-baseline.json
exclude:
  - "**/vendor/**"
  - "**/*.md"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "all", cfg.Source)
	assert.Equal(t, "sarif", cfg.Format)
	assert.Equal(t, "HIGH", cfg.FailOn)
	assert.Equal(t, []string{"**/vendor/**", "**/*.md"}, cfg.Exclude)

	// 相対パスは設定ファイルのディレクトリ基準に正規化される
	assert.Equal(t, filepath.Join(dir, "custom-rules"), cfg.RulesDir)
	assert.Equal(t, filepath.Join(dir, "my-baseline.json"), cfg.Baseline)
	assert.Equal(t, filepath.Join(dir, "results.sarif"), cfg.Out)
}

func TestLoadAbsolutePathsUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secretlens.yml")
	require.NoError(t, os.WriteFile(path, []byte("rules_dir: /abs/rules\n"), 0o644))

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "/abs/rules", cfg.RulesDir)
}

func TestLoadUnknownFieldFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secretlens.yml")
	require.NoError(t, os.WriteFile(path, []byte("formt: json\n"), 0o644))

	_, err := Load(path)
	assert.Error(t, err)
}

func TestLoadSlackWebhookRejected(t *testing.T) {
	// Webhook URLはシークレットのため設定ファイルでは受け付けない
	dir := t.TempDir()
	path := filepath.Join(dir, ".secretlens.yml")
	require.NoError(t, os.WriteFile(path, []byte("slack_webhook: https://hooks.slack.com/x\n"), 0o644))

	_, err := Load(path)
	assert.Error(t, err)
}

func TestLoadFromDir(t *testing.T) {
	dir := t.TempDir()

	// 設定ファイルが無い場合は (nil, nil)
	cfg, err := LoadFromDir(dir)
	require.NoError(t, err)
	assert.Nil(t, cfg)

	// .secretlens.yaml も探索対象
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".secretlens.yaml"), []byte("format: json\n"), 0o644))
	cfg, err = LoadFromDir(dir)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "json", cfg.Format)
}

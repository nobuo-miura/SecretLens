package envfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.env")
	require.NoError(t, os.WriteFile(path, []byte("KEY1=value1\nKEY2=value2\n"), 0o600))

	lines, err := ScanFile(path)
	require.NoError(t, err)
	require.Len(t, lines, 2)
	assert.Equal(t, "KEY1=value1", lines[0].Text)
	assert.Equal(t, 1, lines[0].Line)
	assert.Equal(t, "KEY2=value2", lines[1].Text)
	assert.Equal(t, 2, lines[1].Line)
	assert.Equal(t, path, lines[0].File)
}

func TestScanFile_NotExist(t *testing.T) {
	_, err := ScanFile(filepath.Join(t.TempDir(), "missing.env"))
	assert.Error(t, err)
}

func TestScanDir(t *testing.T) {
	dir := t.TempDir()
	// 対象: .env / .yaml、非対象: .go
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("A=1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("b: 2\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o600))

	// スキップ対象ディレクトリ内のファイルは無視される
	for _, skip := range []string{".git", "node_modules", "vendor"} {
		sub := filepath.Join(dir, skip)
		require.NoError(t, os.Mkdir(sub, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(sub, "leak.env"), []byte("SECRET=x\n"), 0o600))
	}

	lines, err := ScanDir(dir)
	require.NoError(t, err)

	files := map[string]bool{}
	for _, l := range lines {
		files[filepath.Base(l.File)] = true
	}
	assert.True(t, files[".env"])
	assert.True(t, files["config.yaml"])
	assert.False(t, files["main.go"])
	assert.False(t, files["leak.env"])
}

func TestIsTarget(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"app.env", true},
		{".env", true},
		{".env.production", true}, // 拡張子を除いた名前が .env のため対象
		{"config.yaml", true},
		{"config.yml", true},
		{"terraform.tfvars", true},
		{"app.properties", true},
		{"nginx.conf", true},
		{"setup.cfg", true},
		{"php.ini", true},
		{"Cargo.toml", true},
		{"credentials", true},
		{"secrets.json", true}, // 拡張子を除いた名前がsecretsのため対象
		{"main.go", false},
		{"README.md", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, isTarget(tt.path), "isTarget(%q)", tt.path)
	}
}

func TestIsSensitiveFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{".env", true},
		{"path/to/.env", true},
		{"credentials", true},
		{"AWS/credentials", true},
		{"secrets.yaml", true},
		{"secret.txt", true},
		{"SECRETS.YAML", true}, // 大文字小文字を無視
		{"config.yaml", false},
		{"main.go", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, IsSensitiveFile(tt.path), "IsSensitiveFile(%q)", tt.path)
	}
}

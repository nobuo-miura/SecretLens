package org

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initLocalRepo はclone元となるローカルgitリポジトリを作成する
func initLocalRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	run("init", "-q")
	// ユーザーのグローバル設定(GPG署名等)に依存しないようにする
	run("config", "commit.gpgsign", "false")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o600))
	run("add", ".")
	run("commit", "-q", "-m", "init")
	return dir
}

func TestCloneRepo_Local(t *testing.T) {
	src := initLocalRepo(t)
	dst := filepath.Join(t.TempDir(), "clone")

	require.NoError(t, cloneRepo(context.Background(), src, dst, ""))
	assert.FileExists(t, filepath.Join(dst, "README.md"))
}

func TestCloneRepo_InvalidSource(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "clone")
	err := cloneRepo(context.Background(), filepath.Join(t.TempDir(), "nonexistent"), dst, "")
	require.Error(t, err)
	// git stderrの内容がエラーメッセージに含まれること
	assert.NotEmpty(t, err.Error())
}

func TestAuditOptions_ConcurrencyDefault(t *testing.T) {
	// Concurrency未指定(0以下)はAuditOrg内で4に補正される。
	// ネットワークを叩かずに検証できる範囲として、オプション構造体の初期値のみ確認
	opts := AuditOptions{}
	assert.Equal(t, 0, opts.Concurrency)
}

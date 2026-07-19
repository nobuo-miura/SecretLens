package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initTestRepo はコミット1つ（a.txt）を持つ一時リポジトリを作る
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("first line\n"), 0o644))
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-m", "init")
	return dir
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	base := []string{"-C", dir,
		"-c", "user.email=test@example.com", "-c", "user.name=test",
		"-c", "commit.gpgsign=false",
	}
	cmd := exec.Command("git", append(base, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

func collectLines(t *testing.T, dir string, opts StreamOptions) []DiffLine {
	t.Helper()
	ch := make(chan DiffLine, 100)
	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		err = Stream(dir, opts, ch)
		close(ch)
	}()
	var lines []DiffLine
	for l := range ch {
		lines = append(lines, l)
	}
	<-done
	require.NoError(t, err)
	return lines
}

func TestStreamWorktree(t *testing.T) {
	dir := initTestRepo(t)
	// 未ステージの変更（ステージ済みも含めworktreeで検出される）
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("first line\nSECRET=worktree-value\n"), 0o644))

	lines := collectLines(t, dir, StreamOptions{Mode: "worktree"})
	require.Len(t, lines, 1)
	assert.Equal(t, "a.txt", lines[0].File)
	assert.Equal(t, 2, lines[0].Line)
	assert.Equal(t, "SECRET=worktree-value", lines[0].Text)
	assert.Empty(t, lines[0].Commit)
}

func TestStreamWorktreeBeforeFirstCommit(t *testing.T) {
	// 初回コミット前でもステージ済みの変更をworktreeスキャンで検出できる（空ツリーとの比較）
	dir := t.TempDir()
	run(t, dir, "init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.txt"), []byte("SECRET=pre-first-commit\n"), 0o644))
	run(t, dir, "add", "s.txt")

	lines := collectLines(t, dir, StreamOptions{Mode: "worktree"})
	require.Len(t, lines, 1)
	assert.Equal(t, "s.txt", lines[0].File)
	assert.Equal(t, "SECRET=pre-first-commit", lines[0].Text)
}

func TestStreamStaged(t *testing.T) {
	dir := initTestRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("SECRET=staged-value\n"), 0o644))
	run(t, dir, "add", "b.txt")
	// ステージしていない変更はstagedモードでは検出されない
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("first line\nSECRET=unstaged\n"), 0o644))

	lines := collectLines(t, dir, StreamOptions{Mode: "staged"})
	require.Len(t, lines, 1)
	assert.Equal(t, "b.txt", lines[0].File)
	assert.Equal(t, "SECRET=staged-value", lines[0].Text)
}

func TestStreamHistoryRange(t *testing.T) {
	dir := initTestRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("SECRET=second-commit\n"), 0o644))
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-m", "second")

	// 全履歴: 両コミットの追加行が見える
	all := collectLines(t, dir, StreamOptions{Mode: "history"})
	assert.Len(t, all, 2)

	// --since=HEAD~1: 2番目のコミットのみ
	since := collectLines(t, dir, StreamOptions{Mode: "history", Since: "HEAD~1"})
	require.Len(t, since, 1)
	assert.Equal(t, "c.txt", since[0].File)
	assert.NotEmpty(t, since[0].Commit)

	// --commit-range=HEAD~1..HEAD: 同じく2番目のみ
	ranged := collectLines(t, dir, StreamOptions{Mode: "history", CommitRange: "HEAD~1..HEAD"})
	require.Len(t, ranged, 1)
	assert.Equal(t, "c.txt", ranged[0].File)
}

func TestStreamInvalidRangeReturnsError(t *testing.T) {
	dir := initTestRepo(t)
	ch := make(chan DiffLine, 10)
	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		err = Stream(dir, StreamOptions{Mode: "history", CommitRange: "nonexistent..HEAD"}, ch)
		close(ch)
	}()
	for range ch { //nolint:revive // ドレインのみ
	}
	<-done
	assert.Error(t, err)
}

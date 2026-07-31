package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nobuo-miura/SecretLens/internal/baseline"
	"github.com/nobuo-miura/SecretLens/internal/detector/regex"
	"github.com/nobuo-miura/SecretLens/internal/finding"
)

// テスト用のダミーAWSアクセスキー（AWSドキュメントの公式サンプル値）
const fakeAWSKey = "AKIAIOSFODNN7EXAMPLE"

func testRule(t *testing.T) regex.Rule {
	t.Helper()
	r := regex.Rule{
		ID:       "test-aws-key",
		Name:     "Test AWS Access Key",
		Severity: "CRITICAL",
		Pattern:  `AKIA[0-9A-Z]{16}`,
	}
	require.NoError(t, r.Compile())
	return r
}

func TestRun_NoRules(t *testing.T) {
	_, err := Run(Options{})
	assert.Error(t, err)
}

func TestRun_Envfile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".env"),
		[]byte("AWS_ACCESS_KEY_ID="+fakeAWSKey+"\n# comment "+fakeAWSKey+"\n"),
		0o600))

	findings, err := Run(Options{
		Source:   "envfile",
		RepoPath: dir,
		Rules:    []regex.Rule{testRule(t)},
	})
	require.NoError(t, err)
	// コメント行はスキャン対象外なので1件のみ
	require.Len(t, findings, 1)
	f := findings[0]
	assert.Equal(t, "SL-0001", f.ID)
	assert.Equal(t, "test-aws-key", f.RuleID)
	assert.Equal(t, "envfile", f.Source)
	assert.Equal(t, ".env", f.File)
	assert.Equal(t, 1, f.Line)
	// マスク済みマッチにraw値全体が含まれないこと
	assert.NotEqual(t, fakeAWSKey, f.Match)
	assert.Contains(t, f.Match, "****")
}

func TestRun_BaselineSuppression(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".env"),
		[]byte("KEY="+fakeAWSKey+"\n"),
		0o600))

	// 一度スキャンしてfingerprintを取得し、ベースラインに登録
	first, err := Run(Options{Source: "envfile", RepoPath: dir, Rules: []regex.Rule{testRule(t)}})
	require.NoError(t, err)
	require.Len(t, first, 1)

	blPath := filepath.Join(dir, "baseline.json")
	bl, err := baseline.Load(blPath)
	require.NoError(t, err)
	bl.Add(first[0].Fingerprint)
	require.NoError(t, bl.Save())

	// ベースライン登録済みの検出は抑制される
	second, err := Run(Options{
		Source: "envfile", RepoPath: dir,
		Rules: []regex.Rule{testRule(t)}, BaselineFile: blPath,
	})
	require.NoError(t, err)
	assert.Empty(t, second)
}

func TestRun_ExcludePattern(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".env"),
		[]byte("KEY="+fakeAWSKey+"\n"),
		0o600))

	findings, err := Run(Options{
		Source: "envfile", RepoPath: dir,
		Rules:   []regex.Rule{testRule(t)},
		Exclude: []string{"*.env", ".env"},
	})
	require.NoError(t, err)
	assert.Empty(t, findings)
}

// initGitRepo はテスト用のgitリポジトリを作りシークレット入りファイルをコミットする
func initGitRepo(t *testing.T) string {
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
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "config.txt"),
		[]byte("aws_key = "+fakeAWSKey+"\n"),
		0o600))
	run("add", ".")
	run("commit", "-q", "-m", "add config")
	return dir
}

func TestRun_GitHistory(t *testing.T) {
	dir := initGitRepo(t)

	findings, err := Run(Options{
		Source: "git", RepoPath: dir,
		Rules: []regex.Rule{testRule(t)},
	})
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "git", findings[0].Source)
	assert.Equal(t, "config.txt", findings[0].File)
	assert.NotEmpty(t, findings[0].Commit)
}

func TestRun_AllDeduplicates(t *testing.T) {
	// git履歴とenvfileの両方で検出される同一シークレットは1件にまとまる
	dir := initGitRepo(t)
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".env"),
		[]byte("KEY="+fakeAWSKey+"\n"),
		0o600))

	findings, err := Run(Options{
		Source: "all", RepoPath: dir,
		Rules: []regex.Rule{testRule(t)},
	})
	require.NoError(t, err)

	// config.txt(git) と .env(envfile) でファイルが異なるため2件だが、
	// fingerprintベースの重複除去でIDは連番になる
	require.Len(t, findings, 2)
	assert.Equal(t, "SL-0001", findings[0].ID)
	assert.Equal(t, "SL-0002", findings[1].ID)
}

func TestScoreAndBuild(t *testing.T) {
	rule := testRule(t) // CRITICAL: ベース60点

	t.Run("通常ファイル", func(t *testing.T) {
		f := scoreAndBuild(rule, "git", "config.txt", 10, fakeAWSKey, "abc123")
		assert.Equal(t, finding.SeverityCritical, f.Severity)
		assert.GreaterOrEqual(t, f.Score, 60)
		assert.Equal(t, fakeAWSKey, f.Secret)
		assert.NotEmpty(t, f.Fingerprint)
	})

	t.Run("センシティブファイルは加点", func(t *testing.T) {
		normal := scoreAndBuild(rule, "envfile", "config.txt", 1, fakeAWSKey, "")
		sensitive := scoreAndBuild(rule, "envfile", ".env", 1, fakeAWSKey, "")
		assert.Equal(t, finding.ScoreSensitiveFile, sensitive.Score-normal.Score)
	})

	t.Run("テストコードは減点", func(t *testing.T) {
		normal := scoreAndBuild(rule, "git", "main.go", 1, fakeAWSKey, "")
		test := scoreAndBuild(rule, "git", "main_test.go", 1, fakeAWSKey, "")
		assert.Equal(t, finding.ScoreTestCode, test.Score-normal.Score)
	})

	t.Run("severityがスコアに反映される", func(t *testing.T) {
		low := rule
		low.Severity = "LOW"
		fLow := scoreAndBuild(low, "git", "config.txt", 1, fakeAWSKey, "")
		fCrit := scoreAndBuild(rule, "git", "config.txt", 1, fakeAWSKey, "")
		assert.Equal(t, 60, fCrit.Score-fLow.Score)
	})
}

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSource(t *testing.T) {
	// デフォルト（何も指定なし）
	src, err := resolveSource(sourceInputs{})
	require.NoError(t, err)
	assert.Equal(t, "", src)

	// --staged は設定ファイルの source を上書きする（エラーにしない）
	src, err = resolveSource(sourceInputs{staged: true, cfgSource: "all"})
	require.NoError(t, err)
	assert.Equal(t, "staged", src)

	// --staged と CLI明示の --source は競合エラー（worktree含む）
	_, err = resolveSource(sourceInputs{staged: true, cliSource: "docker"})
	assert.Error(t, err)
	_, err = resolveSource(sourceInputs{staged: true, cliSource: "worktree"})
	assert.Error(t, err)

	// --source=staged の明示は --staged と矛盾しないので許容
	src, err = resolveSource(sourceInputs{staged: true, cliSource: "staged"})
	require.NoError(t, err)
	assert.Equal(t, "staged", src)

	// --staged と --all は競合エラー
	_, err = resolveSource(sourceInputs{staged: true, all: true})
	assert.Error(t, err)

	// --since / --commit-range 指定時、source省略ならgit（envfileを巻き込まない）
	src, err = resolveSource(sourceInputs{since: "HEAD~5"})
	require.NoError(t, err)
	assert.Equal(t, "git", src)

	src, err = resolveSource(sourceInputs{commitRange: "main..HEAD"})
	require.NoError(t, err)
	assert.Equal(t, "git", src)

	// CLIで明示的に all を指定していれば範囲指定と併用できる
	src, err = resolveSource(sourceInputs{cliSource: "all", since: "HEAD~5"})
	require.NoError(t, err)
	assert.Equal(t, "all", src)

	src, err = resolveSource(sourceInputs{all: true, since: "HEAD~5"})
	require.NoError(t, err)
	assert.Equal(t, "all", src)

	// 設定ファイル由来の source は、CLI明示がなければ範囲指定時に git へ切り替わる
	for _, cfgSrc := range []string{"all", "worktree", "staged", "envfile", "docker"} {
		src, err = resolveSource(sourceInputs{cfgSource: cfgSrc, commitRange: "main..HEAD"})
		require.NoError(t, err, "cfgSource=%s", cfgSrc)
		assert.Equal(t, "git", src, "cfgSource=%s", cfgSrc)
	}

	// --staged と範囲指定は競合エラー（黙ってgitに切り替えない）
	_, err = resolveSource(sourceInputs{staged: true, since: "HEAD~1"})
	assert.Error(t, err)

	// 範囲指定と履歴以外のソースは競合エラー
	_, err = resolveSource(sourceInputs{cliSource: "docker", since: "HEAD~5"})
	assert.Error(t, err)

	// --since と --commit-range の同時指定はエラー
	_, err = resolveSource(sourceInputs{since: "a", commitRange: "b..c"})
	assert.Error(t, err)

	// 不正なsourceはエラー
	_, err = resolveSource(sourceInputs{cliSource: "bogus"})
	assert.Error(t, err)
	_, err = resolveSource(sourceInputs{cfgSource: "bogus"})
	assert.Error(t, err)
}

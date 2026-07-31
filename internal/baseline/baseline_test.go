package baseline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_FileNotExist(t *testing.T) {
	// 存在しないファイルは空のベースラインを返す（エラーにしない）
	b, err := Load(filepath.Join(t.TempDir(), "nonexistent.json"))
	require.NoError(t, err)
	assert.Empty(t, b.List())
	assert.False(t, b.Contains("deadbeef"))
}

func TestLoad_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.json")
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))

	_, err := Load(path)
	assert.Error(t, err)
}

func TestAddContainsList(t *testing.T) {
	b, err := Load(filepath.Join(t.TempDir(), "bl.json"))
	require.NoError(t, err)

	b.Add("fp1")
	b.Add("fp2")
	b.Add("fp1") // 重複追加は冪等

	assert.True(t, b.Contains("fp1"))
	assert.True(t, b.Contains("fp2"))
	assert.False(t, b.Contains("fp3"))
	assert.Len(t, b.List(), 2)
}

func TestSaveAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bl.json")
	b, err := Load(path)
	require.NoError(t, err)

	b.Add("fingerprint-abc")
	require.NoError(t, b.Save())

	// 保存したファイルを再読込して内容が一致すること
	reloaded, err := Load(path)
	require.NoError(t, err)
	assert.True(t, reloaded.Contains("fingerprint-abc"))
	assert.Len(t, reloaded.List(), 1)

	// パーミッションが0600であること（シークレット情報を含みうるため）
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

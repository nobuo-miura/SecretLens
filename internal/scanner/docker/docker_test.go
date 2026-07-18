package docker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildTar はname→contentのマップからtarバイト列を作る
func buildTar(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg,
		}))
		_, err := tw.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	return buf.Bytes()
}

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, err := gw.Write(data)
	require.NoError(t, err)
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func collect(t *testing.T, outer []byte) ([]FileLine, error) {
	t.Helper()
	ch := make(chan FileLine, 100)
	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		err = parseTar("test-image", tar.NewReader(bytes.NewReader(outer)), ch)
		close(ch)
	}()
	var lines []FileLine
	for l := range ch {
		lines = append(lines, l)
	}
	<-done
	return lines, err
}

func TestParseTarLegacyLayout(t *testing.T) {
	// legacy形式: <id>/layer.tar（非圧縮）
	layer := buildTar(t, map[string][]byte{
		"app/.env": []byte("API_KEY=abc123\nDB_PASSWORD=hunter2\n"),
	})
	outer := buildTar(t, map[string][]byte{
		"manifest.json":          []byte(`[{"Layers":["abc123def456/layer.tar"]}]`),
		"abc123def456/layer.tar": layer,
		"abc123def456/json":      []byte(`{}`),
	})

	lines, err := collect(t, outer)
	require.NoError(t, err)
	require.Len(t, lines, 2)
	assert.Equal(t, "abc123def456", lines[0].Layer)
	assert.Equal(t, "app/.env", lines[0].File)
	assert.Equal(t, "API_KEY=abc123", lines[0].Text)
	assert.Equal(t, 2, lines[1].Line)
}

func TestParseTarOCILayoutGzip(t *testing.T) {
	// OCI形式: blobs/sha256/<digest>（拡張子なし・gzip圧縮）
	layer := buildTar(t, map[string][]byte{
		"etc/secrets.yaml": []byte("token: eyJabc\n"),
	})
	outer := buildTar(t, map[string][]byte{
		"oci-layout":        []byte(`{"imageLayoutVersion":"1.0.0"}`),
		"blobs/sha256/aaaa": gzipBytes(t, layer),
	})

	lines, err := collect(t, outer)
	require.NoError(t, err)
	require.Len(t, lines, 1)
	assert.Equal(t, "aaaa", lines[0].Layer)
	assert.Equal(t, "etc/secrets.yaml", lines[0].File)
	assert.Equal(t, "token: eyJabc", lines[0].Text)
}

func TestParseTarSkipsNonTextAndBinary(t *testing.T) {
	layer := buildTar(t, map[string][]byte{
		"bin/app":     []byte("binary\x00data"), // 拡張子対象外
		"app.conf":    {0x00, 0x01, 0x02},       // 対象拡張子だがバイナリ
		"config.toml": []byte("key = \"value\"\n"),
	})
	outer := buildTar(t, map[string][]byte{
		"x/layer.tar": layer,
	})

	lines, err := collect(t, outer)
	require.NoError(t, err)
	require.Len(t, lines, 1)
	assert.Equal(t, "config.toml", lines[0].File)
}

func TestParseTarBrokenLayerReturnsError(t *testing.T) {
	// tarマジックを持つが途中で壊れているレイヤーはエラーとして伝播する
	valid := buildTar(t, map[string][]byte{"a.env": bytes.Repeat([]byte("AAAA\n"), 400)})
	broken := append([]byte{}, valid[:700]...) // ファイルデータの途中で途切れる
	outer := buildTar(t, map[string][]byte{
		"broken/layer.tar": broken,
	})

	_, err := collect(t, outer)
	assert.Error(t, err)
}

func TestDetectLayerFormat(t *testing.T) {
	tarData := buildTar(t, map[string][]byte{"a.env": []byte("K=V\n")})

	gz, isTar := detectLayerFormat(bufio.NewReader(bytes.NewReader(tarData)))
	assert.False(t, gz)
	assert.True(t, isTar)

	gz, isTar = detectLayerFormat(bufio.NewReader(bytes.NewReader(gzipBytes(t, tarData))))
	assert.True(t, gz)
	assert.True(t, isTar)

	gz, isTar = detectLayerFormat(bufio.NewReader(bytes.NewReader([]byte(`{"json": true}`))))
	assert.False(t, gz)
	assert.False(t, isTar)
}

package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nobuo-miura/SecretLens/internal/detector/regex"
)

func TestBuiltinRulesLoadAndCompile(t *testing.T) {
	// 内蔵ルールが全件バリデーション・コンパイルを通ること
	loaded, err := regex.LoadRulesFromFS(FS)
	require.NoError(t, err)
	assert.NotEmpty(t, loaded)

	ids := map[string]bool{}
	for _, r := range loaded {
		assert.False(t, ids[r.ID], "ルールID %s が重複", r.ID)
		ids[r.ID] = true
	}
}

func TestBuiltinRulesDetectKnownSamples(t *testing.T) {
	loaded, err := regex.LoadRulesFromFS(FS)
	require.NoError(t, err)

	// 代表的なダミーシークレットがいずれかのルールにマッチすること
	// (すべてドキュメント用の公式サンプル値・無効な値)
	samples := []string{
		"AKIAIOSFODNN7EXAMPLE",                          // AWSドキュメントのサンプルアクセスキー
		"ghp_" + "0123456789abcdefghijklmnopqrstuvwxyz", // GitHub PAT形式のダミー
	}
	for _, sample := range samples {
		matched := false
		for _, r := range loaded {
			if len(r.Match(sample)) > 0 {
				matched = true
				break
			}
		}
		assert.True(t, matched, "サンプル %q がどのルールにもマッチしない", sample)
	}
}

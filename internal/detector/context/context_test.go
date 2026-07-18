package context

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchesExcludePattern(t *testing.T) {
	patterns := []string{"**/*_test.go", "**/testdata/**", "**/*.md"}

	// ルート直下のtestdataにもマッチする（`**`はゼロ個のセグメントも許容）
	assert.True(t, MatchesExcludePattern("testdata/sample.env", patterns))
	assert.True(t, MatchesExcludePattern("./testdata/sample.env", patterns))
	assert.True(t, MatchesExcludePattern("a/b/testdata/c/d.yaml", patterns))
	assert.True(t, MatchesExcludePattern("foo_test.go", patterns))
	assert.True(t, MatchesExcludePattern("pkg/deep/foo_test.go", patterns))
	assert.True(t, MatchesExcludePattern("README.md", patterns))

	assert.False(t, MatchesExcludePattern("main.go", patterns))
	assert.False(t, MatchesExcludePattern("config/app.yaml", patterns))
	assert.False(t, MatchesExcludePattern("mytestdata.env", patterns))
}

func TestIsTestFile(t *testing.T) {
	assert.True(t, IsTestFile("foo_test.go"))
	assert.True(t, IsTestFile("testdata/sample.env"))
	assert.False(t, IsTestFile("main.go"))
}

package context

import (
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

var commentPrefixes = []string{"//", "#", "/*", "*", "<!--"}

// IsTestFile はファイルパスがテストコードかどうかを判定する
func IsTestFile(path string) bool {
	base := filepath.Base(path)
	if strings.HasSuffix(base, "_test.go") {
		return true
	}
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, p := range parts {
		if p == "testdata" || p == "test" || p == "tests" {
			return true
		}
	}
	return false
}

// IsCommentLine は行がコメント行かどうかを判定する
func IsCommentLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	for _, prefix := range commentPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

// MatchesExcludePattern はファイルパスが除外パターンにマッチするかを判定する。
// doublestarにより `**` はゼロ個以上のパスセグメントにマッチする
// （例: `**/testdata/**` はルート直下の `testdata/sample.env` にもマッチ）
func MatchesExcludePattern(path string, patterns []string) bool {
	p := strings.TrimPrefix(filepath.ToSlash(path), "./")
	for _, pattern := range patterns {
		if matched, _ := doublestar.Match(pattern, p); matched {
			return true
		}
	}
	return false
}

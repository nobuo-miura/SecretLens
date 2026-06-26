package context

import (
	"path/filepath"
	"strings"
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

// MatchesExcludePattern はファイルパスが除外パターンにマッチするかを判定する
func MatchesExcludePattern(path string, patterns []string) bool {
	for _, pattern := range patterns {
		// **/ プレフィックスを除いたシンプルなglobマッチ
		clean := strings.TrimPrefix(pattern, "**/")
		if matched, _ := filepath.Match(clean, filepath.Base(path)); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}
	return false
}

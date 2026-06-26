package envfile

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Line struct {
	File string
	Line int
	Text string
}

var targetExtensions = map[string]bool{
	".env":        true,
	".yaml":       true,
	".yml":        true,
	".tfvars":     true,
	".properties": true,
	".conf":       true,
	".cfg":        true,
	".ini":        true,
	".toml":       true,
}

var sensitiveFileNames = map[string]bool{
	".env":         true,
	"credentials":  true,
	"secrets":      true,
	"secret":       true,
}

// ScanFile は単一ファイルを行単位でスキャンしてLineを返す
func ScanFile(path string) ([]Line, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("ファイルオープン失敗 %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	var lines []Line
	scanner := bufio.NewScanner(f)
	lineNum := 1
	for scanner.Scan() {
		lines = append(lines, Line{
			File: path,
			Line: lineNum,
			Text: scanner.Text(),
		})
		lineNum++
	}
	return lines, scanner.Err()
}

// ScanDir はディレクトリを再帰的にスキャンして対象ファイルのLineを返す
func ScanDir(dir string) ([]Line, error) {
	var result []Line
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !isTarget(path) {
			return nil
		}
		lines, err := ScanFile(path)
		if err != nil {
			return err
		}
		result = append(result, lines...)
		return nil
	})
	return result, err
}

func isTarget(path string) bool {
	base := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(base))
	nameNoExt := strings.TrimSuffix(strings.ToLower(base), ext)

	if targetExtensions[ext] {
		return true
	}
	if sensitiveFileNames[strings.ToLower(base)] || sensitiveFileNames[nameNoExt] {
		return true
	}
	return false
}

// IsSensitiveFile はファイル名がセンシティブなファイルかどうかを判定する
func IsSensitiveFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	ext := filepath.Ext(base)
	nameNoExt := strings.TrimSuffix(base, ext)
	return sensitiveFileNames[base] || sensitiveFileNames[nameNoExt]
}

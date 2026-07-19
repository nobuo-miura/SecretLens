// Package config は .secretlens.yml 設定ファイルの読み込みを提供する。
// 優先順位は CLIフラグ > 設定ファイル > デフォルト値。
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultFileNames は自動探索する設定ファイル名（スキャン対象ディレクトリ直下）
var DefaultFileNames = []string{".secretlens.yml", ".secretlens.yaml"}

// Config は .secretlens.yml の設定項目。
// Slack Webhook URLはそれ自体がシークレットのため設定ファイルでは扱わない
// （--slack-webhook フラグまたは SLACK_WEBHOOK_URL 環境変数のみ）
type Config struct {
	Source   string   `yaml:"source"`    // git | envfile | all | worktree | staged | cilog | docker
	Format   string   `yaml:"format"`    // text | json | sarif | html | github-pr
	Out      string   `yaml:"out"`       // 出力ファイルパス（明示的な--config指定時のみ適用）
	FailOn   string   `yaml:"fail_on"`   // CRITICAL | HIGH | MEDIUM | LOW
	RulesDir string   `yaml:"rules_dir"` // 追加・上書きルールディレクトリ
	Baseline string   `yaml:"baseline"`  // ベースラインファイルパス
	Exclude  []string `yaml:"exclude"`   // 全ルール共通の除外globパターン
}

// Load は指定パスの設定ファイルを読み込む。
// 相対パス項目（rules_dir / baseline / out）は設定ファイルのディレクトリ基準で解決する
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("設定ファイル読み込み失敗 %s: %w", path, err)
	}

	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // 未知の項目（typoなど）をエラーにする
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("設定ファイルパース失敗 %s: %w", path, err)
	}

	baseDir := filepath.Dir(path)
	cfg.RulesDir = resolveRelative(baseDir, cfg.RulesDir)
	cfg.Baseline = resolveRelative(baseDir, cfg.Baseline)
	cfg.Out = resolveRelative(baseDir, cfg.Out)
	return &cfg, nil
}

// resolveRelative は相対パスをbaseDir基準の絶対パスへ正規化する
func resolveRelative(baseDir, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(baseDir, p)
}

// LoadFromDir はディレクトリ直下のデフォルト設定ファイルを探して読み込む。
// 見つからない場合は (nil, nil) を返す
func LoadFromDir(dir string) (*Config, error) {
	for _, name := range DefaultFileNames {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return Load(path)
		}
	}
	return nil, nil
}

package regex

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type VerifyConfig struct {
	Type string `yaml:"type"`
}

type Rule struct {
	ID             string        `yaml:"id"`
	Name           string        `yaml:"name"`
	Severity       string        `yaml:"severity"`
	Pattern        string        `yaml:"pattern"`
	EntropyMin     float64       `yaml:"entropy_min"`
	ContextExclude []string      `yaml:"context_exclude"`
	Verify         *VerifyConfig `yaml:"verify"`
	Tags           []string      `yaml:"tags"`

	compiled *regexp.Regexp
}

var validSeverities = map[string]bool{
	"CRITICAL": true,
	"HIGH":     true,
	"MEDIUM":   true,
	"LOW":      true,
}

// Validate はルールの必須項目とSeverityを検証する
func (r *Rule) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("ルールに id がありません (name=%q)", r.Name)
	}
	if r.Name == "" {
		return fmt.Errorf("ルール %s に name がありません", r.ID)
	}
	if r.Pattern == "" {
		return fmt.Errorf("ルール %s に pattern がありません", r.ID)
	}
	if !validSeverities[strings.ToUpper(r.Severity)] {
		return fmt.Errorf("ルール %s のseverityが不正です: %q (CRITICAL|HIGH|MEDIUM|LOW)", r.ID, r.Severity)
	}
	return nil
}

func (r *Rule) Compile() error {
	re, err := regexp.Compile(r.Pattern)
	if err != nil {
		return fmt.Errorf("ルール %s のパターンコンパイル失敗: %w", r.ID, err)
	}
	r.compiled = re
	return nil
}

func (r *Rule) Match(s string) []string {
	if r.compiled == nil {
		return nil
	}
	return r.compiled.FindAllString(s, -1)
}

func (r *Rule) MatchWithIndex(s string) [][]int {
	if r.compiled == nil {
		return nil
	}
	return r.compiled.FindAllStringIndex(s, -1)
}

type RuleSet struct {
	Rules []Rule `yaml:"rules"`
}

// LoadRulesFromDir はディレクトリ内の全YAMLルールファイルを読み込む
func LoadRulesFromDir(dir string) ([]Rule, error) {
	rules, err := loadRulesFromFS(os.DirFS(dir), dir)
	if err != nil {
		return nil, err
	}
	return rules, nil
}

// LoadRulesFromFS はfs.FS直下の全YAMLルールファイルを読み込む（go:embed用）
func LoadRulesFromFS(fsys fs.FS) ([]Rule, error) {
	return loadRulesFromFS(fsys, "(embedded)")
}

func loadRulesFromFS(fsys fs.FS, label string) ([]Rule, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("ルールディレクトリ読み込み失敗 %s: %w", label, err)
	}

	var rules []Rule
	seen := map[string]string{} // id -> ファイル名
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		data, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("ルールファイル読み込み失敗 %s: %w", e.Name(), err)
		}
		rs, err := parseRules(data, e.Name())
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			if prev, dup := seen[r.ID]; dup {
				return nil, fmt.Errorf("ルールID %s が重複しています (%s と %s)", r.ID, prev, e.Name())
			}
			seen[r.ID] = e.Name()
		}
		rules = append(rules, rs...)
	}
	return rules, nil
}

// LoadRulesFromFile は単一YAMLファイルからルールを読み込む
func LoadRulesFromFile(path string) ([]Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ルールファイル読み込み失敗 %s: %w", path, err)
	}
	return parseRules(data, path)
}

func parseRules(data []byte, source string) ([]Rule, error) {
	var rs RuleSet
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // 未知のYAML項目（typoなど）をエラーにする
	if err := dec.Decode(&rs); err != nil && err != io.EOF {
		return nil, fmt.Errorf("ルールYAMLパース失敗 %s: %w", source, err)
	}

	for i := range rs.Rules {
		if err := rs.Rules[i].Validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", source, err)
		}
		if err := rs.Rules[i].Compile(); err != nil {
			return nil, fmt.Errorf("%s: %w", source, err)
		}
	}
	return rs.Rules, nil
}

// MergeRules はbaseにoverridesをマージする。同一IDはoverrides側で上書きする
func MergeRules(base, overrides []Rule) []Rule {
	byID := make(map[string]int, len(base))
	merged := make([]Rule, len(base))
	copy(merged, base)
	for i, r := range merged {
		byID[r.ID] = i
	}
	for _, r := range overrides {
		if i, ok := byID[r.ID]; ok {
			merged[i] = r
		} else {
			byID[r.ID] = len(merged)
			merged = append(merged, r)
		}
	}
	return merged
}

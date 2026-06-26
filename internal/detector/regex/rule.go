package regex

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

type VerifyConfig struct {
	Type string `yaml:"type"`
}

type Rule struct {
	ID             string       `yaml:"id"`
	Name           string       `yaml:"name"`
	Severity       string       `yaml:"severity"`
	Pattern        string       `yaml:"pattern"`
	EntropyMin     float64      `yaml:"entropy_min"`
	ContextExclude []string     `yaml:"context_exclude"`
	Verify         *VerifyConfig `yaml:"verify"`
	Tags           []string     `yaml:"tags"`

	compiled *regexp.Regexp
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
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("ルールディレクトリ読み込み失敗: %w", err)
	}

	var rules []Rule
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		rs, err := LoadRulesFromFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
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

	var rs RuleSet
	if err := yaml.Unmarshal(data, &rs); err != nil {
		return nil, fmt.Errorf("ルールYAMLパース失敗 %s: %w", path, err)
	}

	for i := range rs.Rules {
		if err := rs.Rules[i].Compile(); err != nil {
			return nil, err
		}
	}
	return rs.Rules, nil
}

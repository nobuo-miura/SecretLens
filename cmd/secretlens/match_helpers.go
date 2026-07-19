package main

import (
	"fmt"

	"strings"

	detctx "github.com/nobuo-miura/SecretLens/internal/detector/context"
	"github.com/nobuo-miura/SecretLens/internal/detector/entropy"
	"github.com/nobuo-miura/SecretLens/internal/detector/regex"
	"github.com/nobuo-miura/SecretLens/internal/finding"
	"github.com/nobuo-miura/SecretLens/internal/scanner/cilog"
	"github.com/nobuo-miura/SecretLens/internal/scanner/docker"
	"github.com/nobuo-miura/SecretLens/internal/scanner/envfile"
	embeddedrules "github.com/nobuo-miura/SecretLens/rules"
)

// loadRules はバイナリ内蔵の標準ルールを読み込み、rulesDir指定時は
// そのディレクトリのルールを追加・上書き（同一ID）としてマージする
func loadRules(rulesDir string) ([]regex.Rule, error) {
	base, err := regex.LoadRulesFromFS(embeddedrules.FS)
	if err != nil {
		return nil, fmt.Errorf("内蔵ルール読み込み失敗: %w", err)
	}
	if rulesDir == "" {
		return base, nil
	}
	overrides, err := regex.LoadRulesFromDir(rulesDir)
	if err != nil {
		return nil, fmt.Errorf("ルール読み込み失敗 (%s): %w", rulesDir, err)
	}
	return regex.MergeRules(base, overrides), nil
}

func matchCILog(ch <-chan cilog.LogLine, rules []regex.Rule) []finding.Finding {
	var findings []finding.Finding
	idCounter := 0
	for line := range ch {
		if detctx.IsCommentLine(line.Text) {
			continue
		}
		for _, rule := range rules {
			matches := rule.Match(line.Text)
			for _, m := range matches {
				idCounter++
				f := buildFinding(idCounter, rule, line.Source, line.Job, line.Line, m, "")
				findings = append(findings, f)
			}
		}
	}
	return findings
}

func matchDockerLines(ch <-chan docker.FileLine, rules []regex.Rule, exclude []string) []finding.Finding {
	var findings []finding.Finding
	idCounter := 0
	for line := range ch {
		if detctx.IsCommentLine(line.Text) {
			continue
		}
		if detctx.MatchesExcludePattern(line.File, exclude) {
			continue
		}
		for _, rule := range rules {
			if detctx.MatchesExcludePattern(line.File, rule.ContextExclude) {
				continue
			}
			matches := rule.Match(line.Text)
			for _, m := range matches {
				idCounter++
				fileRef := fmt.Sprintf("%s[layer:%s]/%s", line.Image, shortLayer(line.Layer), line.File)
				f := buildFinding(idCounter, rule, "docker", fileRef, line.Line, m, "")
				findings = append(findings, f)
			}
		}
	}
	return findings
}

func buildFinding(id int, rule regex.Rule, source, file string, line int, matched, commit string) finding.Finding {
	score := 0

	// git/envfileスキャン（scoreAndBuild）と同じエントロピー判定ロジック
	if rule.EntropyMin > 0 && entropy.Shannon(matched) >= rule.EntropyMin {
		score += finding.ScoreHighEntropy
	} else if entropy.Shannon(matched) > 4.5 {
		score += finding.ScoreHighEntropy
	}
	if envfile.IsSensitiveFile(file) {
		score += finding.ScoreSensitiveFile
	}
	if detctx.IsTestFile(file) {
		score += finding.ScoreTestCode
	}

	switch strings.ToUpper(rule.Severity) {
	case "CRITICAL":
		score += 60
	case "HIGH":
		score += 40
	case "MEDIUM":
		score += 20
	}

	masked := finding.MaskSecret(matched)
	fp := finding.ComputeFingerprint(rule.ID, file, matched)

	verifyType := ""
	if rule.Verify != nil {
		verifyType = rule.Verify.Type
	}

	f := finding.Finding{
		ID:          fmt.Sprintf("SL-%04d", id),
		RuleID:      rule.ID,
		Score:       score,
		Source:      source,
		File:        file,
		Line:        line,
		Match:       masked,
		Commit:      commit,
		Fingerprint: fp,
		Secret:      matched, // Live検証専用。出力前に必ずクリアされる
		VerifyType:  verifyType,
	}
	f.Severity = finding.ScoreToSeverity(score)
	return f
}

func shortLayer(layer string) string {
	if len(layer) > 12 {
		return layer[:12]
	}
	return layer
}

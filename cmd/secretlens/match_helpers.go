package main

import (
	"fmt"

	detctx "github.com/nobuo-miura/SecretLens/internal/detector/context"
	"github.com/nobuo-miura/SecretLens/internal/detector/entropy"
	"github.com/nobuo-miura/SecretLens/internal/detector/regex"
	"github.com/nobuo-miura/SecretLens/internal/finding"
	"github.com/nobuo-miura/SecretLens/internal/scanner/cilog"
	"github.com/nobuo-miura/SecretLens/internal/scanner/docker"
	"github.com/nobuo-miura/SecretLens/internal/scanner/envfile"
	"strings"
)

func loadRules(rulesDir string) ([]regex.Rule, error) {
	rules, err := regex.LoadRulesFromDir(rulesDir)
	if err != nil {
		return nil, fmt.Errorf("ルール読み込み失敗: %w", err)
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("有効なルールが見つかりません: %s", rulesDir)
	}
	return rules, nil
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

func matchDockerLines(ch <-chan docker.FileLine, rules []regex.Rule) []finding.Finding {
	var findings []finding.Finding
	idCounter := 0
	for line := range ch {
		if detctx.IsCommentLine(line.Text) {
			continue
		}
		for _, rule := range rules {
			if detctx.MatchesExcludePattern(line.File, rule.ContextExclude) {
				continue
			}
			matches := rule.Match(line.Text)
			for _, m := range matches {
				idCounter++
				fileRef := fmt.Sprintf("%s[layer:%s]/%s", line.Image, line.Layer[:8], line.File)
				f := buildFinding(idCounter, rule, "docker", fileRef, line.Line, m, "")
				findings = append(findings, f)
			}
		}
	}
	return findings
}

func buildFinding(id int, rule regex.Rule, source, file string, line int, matched, commit string) finding.Finding {
	score := 0

	if entropy.Shannon(matched) > 4.5 {
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
	fp := finding.ComputeFingerprint(rule.ID, file, line, matched)

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
	}
	f.Severity = finding.ScoreToSeverity(score)
	return f
}

package scanner

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nobuo-miura/SecretLens/internal/baseline"
	detctx "github.com/nobuo-miura/SecretLens/internal/detector/context"
	"github.com/nobuo-miura/SecretLens/internal/detector/entropy"
	"github.com/nobuo-miura/SecretLens/internal/detector/regex"
	"github.com/nobuo-miura/SecretLens/internal/finding"
	"github.com/nobuo-miura/SecretLens/internal/scanner/envfile"
	gitscanner "github.com/nobuo-miura/SecretLens/internal/scanner/git"
)

type Options struct {
	Source       string // "git" | "envfile" | "all"（空は"all"扱い）
	RepoPath     string
	Rules        []regex.Rule
	BaselineFile string
	Format       string // "sarif" | "json" | "text"
	FailOn       string // "CRITICAL" | "HIGH" | "MEDIUM" | "LOW"
}

// Run はスキャンを実行してFinding一覧を返す
func Run(opts Options) ([]finding.Finding, error) {
	rules := opts.Rules
	if len(rules) == 0 {
		return nil, fmt.Errorf("有効なルールが指定されていません")
	}

	bl, err := baseline.Load(opts.BaselineFile)
	if err != nil {
		return nil, err
	}

	var findings []finding.Finding
	seen := map[string]bool{} // git履歴とenvfileで同じ検出が重複するのを防ぐ
	idCounter := 0

	addFinding := func(f finding.Finding) {
		if bl.Contains(f.Fingerprint) || seen[f.Fingerprint] {
			return
		}
		seen[f.Fingerprint] = true
		idCounter++
		f.ID = fmt.Sprintf("SL-%04d", idCounter)
		findings = append(findings, f)
	}

	source := opts.Source
	if source == "" {
		source = "all"
	}
	if source == "git" || source == "all" {
		gitFindings, err := scanGit(opts.RepoPath, rules)
		if err != nil {
			return nil, err
		}
		for _, f := range gitFindings {
			addFinding(f)
		}
	}
	if source == "envfile" || source == "all" {
		envFindings, err := scanEnvfile(opts.RepoPath, rules)
		if err != nil {
			return nil, err
		}
		for _, f := range envFindings {
			addFinding(f)
		}
	}

	return findings, nil
}

func scoreAndBuild(rule regex.Rule, source, file string, line int, matched, commit string) finding.Finding {
	score := 0

	// エントロピーチェック
	if rule.EntropyMin > 0 && entropy.Shannon(matched) >= rule.EntropyMin {
		score += finding.ScoreHighEntropy
	} else if entropy.Shannon(matched) > 4.5 {
		score += finding.ScoreHighEntropy
	}

	// センシティブファイル名チェック
	if envfile.IsSensitiveFile(file) {
		score += finding.ScoreSensitiveFile
	}

	// テストコードチェック
	if detctx.IsTestFile(file) {
		score += finding.ScoreTestCode
	}

	// ルールのSeverityをベーススコアに変換
	switch strings.ToUpper(rule.Severity) {
	case "CRITICAL":
		score += 60
	case "HIGH":
		score += 40
	case "MEDIUM":
		score += 20
	case "LOW":
		score += 0
	}

	masked := finding.MaskSecret(matched)
	fp := finding.ComputeFingerprint(rule.ID, file, matched)

	verifyType := ""
	if rule.Verify != nil {
		verifyType = rule.Verify.Type
	}

	f := finding.Finding{
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

func scanGit(repoPath string, rules []regex.Rule) ([]finding.Finding, error) {
	ch := make(chan gitscanner.DiffLine, 100)
	var scanErr error
	go func() {
		defer close(ch)
		scanErr = gitscanner.Stream(repoPath, ch)
	}()

	var findings []finding.Finding
	for dl := range ch {
		if detctx.IsCommentLine(dl.Text) {
			continue
		}
		for _, rule := range rules {
			if detctx.MatchesExcludePattern(dl.File, rule.ContextExclude) {
				continue
			}
			matches := rule.Match(dl.Text)
			for _, m := range matches {
				f := scoreAndBuild(rule, "git", dl.File, dl.Line, m, dl.Commit)
				findings = append(findings, f)
			}
		}
	}
	return findings, scanErr
}

func scanEnvfile(repoPath string, rules []regex.Rule) ([]finding.Finding, error) {
	lines, err := envfile.ScanDir(repoPath)
	if err != nil {
		return nil, err
	}

	var findings []finding.Finding
	for _, l := range lines {
		if detctx.IsCommentLine(l.Text) {
			continue
		}
		relPath := l.File
		if abs, err := filepath.Rel(repoPath, l.File); err == nil {
			relPath = abs
		}
		for _, rule := range rules {
			if detctx.MatchesExcludePattern(relPath, rule.ContextExclude) {
				continue
			}
			matches := rule.Match(l.Text)
			for _, m := range matches {
				f := scoreAndBuild(rule, "envfile", relPath, l.Line, m, "")
				findings = append(findings, f)
			}
		}
	}
	return findings, nil
}

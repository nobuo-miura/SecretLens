package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nobuo-miura/SecretLens/internal/baseline"
	"github.com/nobuo-miura/SecretLens/internal/config"
	"github.com/nobuo-miura/SecretLens/internal/detector/regex"
	"github.com/nobuo-miura/SecretLens/internal/detector/verifier"
	"github.com/nobuo-miura/SecretLens/internal/finding"
	reportgithub "github.com/nobuo-miura/SecretLens/internal/reporter/github"
	reporthtml "github.com/nobuo-miura/SecretLens/internal/reporter/html"
	"github.com/nobuo-miura/SecretLens/internal/reporter/sarif"
	"github.com/nobuo-miura/SecretLens/internal/reporter/slack"
	"github.com/nobuo-miura/SecretLens/internal/scanner"
	"github.com/nobuo-miura/SecretLens/internal/scanner/cilog"
	"github.com/nobuo-miura/SecretLens/internal/scanner/docker"
)

var (
	flagAll      bool
	flagSource   string
	flagFormat   string
	flagOut      string
	flagFailOn   string
	flagRulesDir string
	flagBaseline string
	flagConfig   string

	// 差分・増分スキャン
	flagStaged      bool
	flagSince       string
	flagCommitRange string

	// CIログスキャン
	flagRepo      string // owner/repo
	flagGitLabURL string
	flagProjectID string

	// Docker
	flagImage string

	// 出力先
	flagGitHubToken  string
	flagGitHubPR     int
	flagGitHubSHA    string
	flagSlackWebhook string

	// verifier
	flagVerify bool
)

var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "シークレットをスキャンする",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runScan,
}

func init() {
	scanCmd.Flags().BoolVar(&flagAll, "all", false, "Git履歴 + 環境変数ファイルをスキャン")
	scanCmd.Flags().StringVar(&flagSource, "source", "", "スキャンソース (git|envfile|all|worktree|staged|cilog|docker)")
	scanCmd.Flags().BoolVar(&flagStaged, "staged", false, "ステージ済み差分のみをスキャン (--source=staged の別名、pre-commit用)")
	scanCmd.Flags().StringVar(&flagSince, "since", "", "指定コミット以降の履歴のみスキャン (<commit>..HEAD)")
	scanCmd.Flags().StringVar(&flagCommitRange, "commit-range", "", "指定範囲の履歴のみスキャン (base..head 形式)")
	scanCmd.Flags().StringVar(&flagConfig, "config", "", "設定ファイルパス（省略時はスキャン対象直下の .secretlens.yml を自動探索）")
	scanCmd.Flags().StringVar(&flagFormat, "format", "text", "出力フォーマット (text|json|sarif|html|github-pr)")
	scanCmd.Flags().StringVar(&flagOut, "out", "", "出力ファイルパス（省略時はstdout）")
	scanCmd.Flags().StringVar(&flagFailOn, "fail-on", "", "指定したSeverity以上で終了コード1 (CRITICAL|HIGH|MEDIUM|LOW)")
	scanCmd.Flags().StringVar(&flagRulesDir, "rules-dir", "", "追加・上書きYAMLルールディレクトリ（省略時は内蔵ルールのみ）")
	scanCmd.Flags().StringVar(&flagBaseline, "baseline", baseline.DefaultFile, "ベースラインファイルパス")

	// CIログ
	scanCmd.Flags().StringVar(&flagRepo, "repo", "", "GitHub リポジトリ (owner/repo) — cilog ソース用")
	scanCmd.Flags().StringVar(&flagGitLabURL, "gitlab-url", "", "GitLab インスタンスURL (cilog ソース用)")
	scanCmd.Flags().StringVar(&flagProjectID, "project-id", "", "GitLab プロジェクトID (cilog ソース用)")

	// Docker
	scanCmd.Flags().StringVar(&flagImage, "image", "", "DockerイメージタグまたはID (docker ソース用)")

	// GitHub出力
	scanCmd.Flags().StringVar(&flagGitHubToken, "github-token", "", "GitHub APIトークン (GITHUB_TOKEN 環境変数も可)")
	scanCmd.Flags().IntVar(&flagGitHubPR, "pr", 0, "PRコメントを投稿するプルリクエスト番号")
	scanCmd.Flags().StringVar(&flagGitHubSHA, "sha", "", "Check Runを作成するコミットSHA")
	scanCmd.Flags().StringVar(&flagSlackWebhook, "slack-webhook", "", "Slack Webhook URL (SLACK_WEBHOOK_URL 環境変数も可)")

	// Live検証
	scanCmd.Flags().BoolVar(&flagVerify, "verify", false, "検出したシークレットのLive API検証を実行 (opt-in)")

	rootCmd.AddCommand(scanCmd)
}

var validSources = map[string]bool{
	"": true, "git": true, "envfile": true, "all": true,
	"worktree": true, "staged": true, "cilog": true, "docker": true,
}

var validFormats = map[string]bool{
	"text": true, "json": true, "sarif": true, "html": true, "github-pr": true,
}

var validFailOn = map[string]bool{
	"": true, "CRITICAL": true, "HIGH": true, "MEDIUM": true, "LOW": true,
}

func runScan(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	repoPath := "."
	if len(args) > 0 {
		repoPath = args[0]
	}

	cfg, cfgExplicit, err := loadScanConfig(repoPath)
	if err != nil {
		return err
	}
	applyConfig(cmd, cfg, cfgExplicit)

	cliSource := ""
	if cmd.Flags().Changed("source") {
		cliSource = flagSource
	}
	cfgSource := ""
	if cfg != nil {
		cfgSource = cfg.Source
	}
	source, err := resolveSource(sourceInputs{
		cliSource:   cliSource,
		cfgSource:   cfgSource,
		all:         flagAll,
		staged:      flagStaged,
		since:       flagSince,
		commitRange: flagCommitRange,
	})
	if err != nil {
		return err
	}
	if !validFormats[flagFormat] {
		return fmt.Errorf("不正な --format です: %q (text|json|sarif|html|github-pr)", flagFormat)
	}
	if !validFailOn[strings.ToUpper(flagFailOn)] {
		return fmt.Errorf("不正な --fail-on です: %q (CRITICAL|HIGH|MEDIUM|LOW)", flagFailOn)
	}

	rules, err := loadRules(flagRulesDir)
	if err != nil {
		return err
	}

	var exclude []string
	if cfg != nil {
		exclude = cfg.Exclude
	}

	var findings []finding.Finding

	switch source {
	case "cilog":
		findings, err = scanCILog(ctx, rules)
	case "docker":
		findings, err = scanDockerImage(rules, exclude)
	default:
		opts := scanner.Options{
			Source:       source,
			RepoPath:     repoPath,
			Rules:        rules,
			BaselineFile: flagBaseline,
			Format:       flagFormat,
			FailOn:       flagFailOn,
			Since:        flagSince,
			CommitRange:  flagCommitRange,
			Exclude:      exclude,
		}
		findings, err = scanner.Run(opts)
	}
	if err != nil {
		return err
	}

	// Live検証 (opt-in)
	if flagVerify {
		findings = runVerification(ctx, findings)
	}

	// raw値は検証以外の用途に使わせない。出力前に必ずクリアする
	for i := range findings {
		findings[i].Secret = ""
	}

	if err := outputFindings(ctx, findings, repoPath); err != nil {
		return err
	}

	if flagFailOn != "" {
		threshold := finding.Severity(strings.ToUpper(flagFailOn))
		for _, f := range findings {
			if severityGTE(f.Severity, threshold) {
				os.Exit(1)
			}
		}
	}
	return nil
}

// loadScanConfig は --config 指定ファイル、なければスキャン対象直下の
// .secretlens.yml を読み込む。戻り値のboolは明示的な--config指定かどうか
func loadScanConfig(repoPath string) (*config.Config, bool, error) {
	if flagConfig != "" {
		cfg, err := config.Load(flagConfig)
		return cfg, true, err
	}
	cfg, err := config.LoadFromDir(repoPath)
	return cfg, false, err
}

// applyConfig は設定ファイルの値を反映する。CLIで明示指定されたフラグが常に優先される。
// outは出力先を握られる危険があるため、明示的な--config指定時のみ適用する
// （自動検出した設定＝スキャン対象リポジトリ由来の可能性があるため）
func applyConfig(cmd *cobra.Command, cfg *config.Config, explicit bool) {
	if cfg == nil {
		return
	}
	fl := cmd.Flags()
	if !fl.Changed("format") && cfg.Format != "" {
		flagFormat = cfg.Format
	}
	if explicit && !fl.Changed("out") && cfg.Out != "" {
		flagOut = cfg.Out
	}
	if !fl.Changed("fail-on") && cfg.FailOn != "" {
		flagFailOn = cfg.FailOn
	}
	if !fl.Changed("rules-dir") && cfg.RulesDir != "" {
		flagRulesDir = cfg.RulesDir
	}
	if !fl.Changed("baseline") && cfg.Baseline != "" {
		flagBaseline = cfg.Baseline
	}
}

// sourceInputs はスキャンソース決定に関わる入力
type sourceInputs struct {
	cliSource   string // CLIで明示された--source（未指定は空）
	cfgSource   string // 設定ファイルのsource
	all         bool   // --all
	staged      bool   // --staged
	since       string // --since
	commitRange string // --commit-range
}

// resolveSource はCLIフラグと設定ファイルからスキャンソースを決定する。
//   - 優先順位: --staged / --all / --source（CLI） > 設定ファイル > デフォルト
//   - --stagedはCLIの--source/--allとだけ競合チェックし、設定ファイルのsourceは上書きする
//   - --since / --commit-range 指定時のデフォルトは git（envfileを巻き込まない）
func resolveSource(in sourceInputs) (string, error) {
	source := in.cliSource
	if source == "" {
		source = in.cfgSource
	}
	if in.all {
		source = "all"
	}
	if in.staged {
		if in.all {
			return "", fmt.Errorf("--staged と --all は同時に指定できません")
		}
		// CLIで異なるスキャン対象が明示されている場合は黙って上書きせず競合エラーにする
		if in.cliSource != "" && in.cliSource != "staged" {
			return "", fmt.Errorf("--staged は --source=%s と併用できません", in.cliSource)
		}
		source = "staged"
	}
	if !validSources[source] {
		return "", fmt.Errorf("不正な --source です: %q (git|envfile|all|worktree|staged|cilog|docker)", source)
	}
	if in.since != "" && in.commitRange != "" {
		return "", fmt.Errorf("--since と --commit-range は同時に指定できません")
	}
	if in.since != "" || in.commitRange != "" {
		// CLIでソースを明示していなければ、設定ファイルのsourceにかかわらずgitを使う
		// （範囲指定の意図は履歴スキャンであり、envfile等を巻き込まない）。
		// CLI明示時（--source / --all / --staged）のみ競合判定を行う
		cliExplicit := in.all || in.staged || in.cliSource != ""
		if !cliExplicit {
			source = "git"
		} else if source != "git" && source != "all" {
			return "", fmt.Errorf("--since / --commit-range は履歴スキャン (git|all) 専用です (--source=%s)", source)
		}
	}
	return source, nil
}

func scanCILog(ctx context.Context, rules []regex.Rule) ([]finding.Finding, error) {
	token := flagGitHubToken
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	ch := make(chan cilog.LogLine, 200)
	var scanErr error
	go func() {
		defer close(ch)
		if flagRepo != "" {
			parts := strings.SplitN(flagRepo, "/", 2)
			if len(parts) != 2 {
				scanErr = fmt.Errorf("--repo は owner/repo 形式で指定してください")
				return
			}
			s := cilog.NewGitHubActionsScanner(token, parts[0], parts[1])
			scanErr = s.StreamLogs(ctx, ch)
		} else if flagProjectID != "" {
			s := cilog.NewGitLabCIScanner(flagGitLabURL, token, flagProjectID)
			scanErr = s.StreamLogs(ctx, ch)
		} else {
			scanErr = fmt.Errorf("cilogスキャンには --repo (GitHub) または --project-id (GitLab) が必要です")
		}
	}()

	return matchCILog(ch, rules), scanErr
}

func scanDockerImage(rules []regex.Rule, exclude []string) ([]finding.Finding, error) {
	if flagImage == "" {
		return nil, fmt.Errorf("dockerスキャンには --image が必要です")
	}

	ch := make(chan docker.FileLine, 200)
	var scanErr error
	go func() {
		defer close(ch)
		scanErr = docker.StreamLayers(flagImage, ch)
	}()

	findings := matchDockerLines(ch, rules, exclude)
	if scanErr != nil {
		return nil, fmt.Errorf("dockerイメージスキャン失敗: %w", scanErr)
	}
	return findings, nil
}

func outputFindings(ctx context.Context, findings []finding.Finding, repoPath string) error {
	repoName := filepath.Base(repoPath)
	if flagRepo != "" {
		repoName = flagRepo
	}

	// GitHub PRコメント
	if flagFormat == "github-pr" || flagGitHubPR > 0 {
		token := flagGitHubToken
		if token == "" {
			token = os.Getenv("GITHUB_TOKEN")
		}
		if flagRepo == "" {
			return fmt.Errorf("github-pr フォーマットには --repo が必要です")
		}
		parts := strings.SplitN(flagRepo, "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("--repo は owner/repo 形式で指定してください")
		}
		r := reportgithub.New(token, parts[0], parts[1])
		if flagGitHubPR > 0 {
			if err := r.PostPRComment(ctx, flagGitHubPR, findings); err != nil {
				return err
			}
		}
		if flagGitHubSHA != "" {
			if err := r.CreateCheckRun(ctx, flagGitHubSHA, findings); err != nil {
				return err
			}
		}
	}

	// Slack通知
	webhookURL := flagSlackWebhook
	if webhookURL == "" {
		webhookURL = os.Getenv("SLACK_WEBHOOK_URL")
	}
	if webhookURL != "" {
		if err := slack.Notify(webhookURL, findings, repoName); err != nil {
			fmt.Fprintf(os.Stderr, "Slack通知失敗: %v\n", err)
		}
	}

	// ファイル出力またはstdout
	out := os.Stdout
	if flagOut != "" {
		f, err := os.Create(flagOut)
		if err != nil {
			return fmt.Errorf("出力ファイル作成失敗: %w", err)
		}
		defer func() { _ = f.Close() }()
		out = f
	}

	switch flagFormat {
	case "sarif":
		return sarif.Write(out, findings)
	case "json":
		if findings == nil {
			findings = []finding.Finding{} // 検出0件でも null ではなく [] を出力する
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(findings)
	case "html":
		return reporthtml.Write(out, findings, repoName)
	case "github-pr":
		// 既に上で処理済み
	default:
		printText(out, findings)
	}
	return nil
}

func printText(out io.Writer, findings []finding.Finding) {
	if len(findings) == 0 {
		_, _ = fmt.Fprintln(out, "シークレットは検出されませんでした。")
		return
	}
	for _, f := range findings {
		verified := ""
		if f.Verified {
			verified = " [VERIFIED]"
		}
		_, _ = fmt.Fprintf(out, "[%s]%s %s  %s:%d  rule=%s  score=%d  fingerprint=%s\n",
			f.Severity, verified, f.Source, f.File, f.Line, f.RuleID, f.Score, f.Fingerprint[:16])
		_, _ = fmt.Fprintf(out, "  match: %s\n", f.Match)
		if f.Commit != "" {
			_, _ = fmt.Fprintf(out, "  commit: %s\n", f.Commit)
		}
	}
	_, _ = fmt.Fprintf(out, "\n合計: %d件\n", len(findings))
}

func severityGTE(a, b finding.Severity) bool {
	order := map[finding.Severity]int{
		finding.SeverityLow:      0,
		finding.SeverityMedium:   1,
		finding.SeverityHigh:     2,
		finding.SeverityCritical: 3,
	}
	return order[a] >= order[b]
}

func runVerification(ctx context.Context, findings []finding.Finding) []finding.Finding {
	result := make([]finding.Finding, len(findings))
	copy(result, findings)
	for i, f := range result {
		// 検証先はルールYAMLのverify.typeで明示されたもののみ。raw値を渡す
		if f.VerifyType == "" || f.Secret == "" {
			continue
		}
		r := verifier.Verify(ctx, f.VerifyType, f.Secret)
		if r.Valid {
			result[i].Verified = true
			result[i].Score += finding.ScoreVerified
			result[i].Severity = finding.ScoreToSeverity(result[i].Score)
		}
	}
	return result
}

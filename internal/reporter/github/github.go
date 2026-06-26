package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	gogithub "github.com/google/go-github/v72/github"
	"golang.org/x/oauth2"

	"github.com/nobuo-miura/SecretLens/internal/finding"
)

// Reporter はGitHub PRコメントとCheck Runを管理する
type Reporter struct {
	client *gogithub.Client
	Owner  string
	Repo   string
}

// New はGitHub Reporterを生成する
func New(token, owner, repo string) *Reporter {
	var hc *http.Client
	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		hc = oauth2.NewClient(context.Background(), ts)
	}
	return &Reporter{
		client: gogithub.NewClient(hc),
		Owner:  owner,
		Repo:   repo,
	}
}

// PostPRComment はプルリクエストにスキャン結果をコメントとして投稿する
func (r *Reporter) PostPRComment(ctx context.Context, prNumber int, findings []finding.Finding) error {
	body := formatPRComment(findings)
	comment := &gogithub.IssueComment{Body: gogithub.Ptr(body)}
	_, _, err := r.client.Issues.CreateComment(ctx, r.Owner, r.Repo, prNumber, comment)
	if err != nil {
		return fmt.Errorf("PRコメント投稿失敗: %w", err)
	}
	return nil
}

// CreateCheckRun はGitHub Check Runを作成してスキャン結果を報告する
func (r *Reporter) CreateCheckRun(ctx context.Context, sha string, findings []finding.Finding) error {
	status := "completed"
	conclusion := "success"
	if hasHigherThan(findings, finding.SeverityLow) {
		conclusion = "failure"
	}

	summary := fmt.Sprintf("SecretLens が %d 件の問題を検出しました。", len(findings))
	text := formatCheckRunText(findings)

	opts := gogithub.CreateCheckRunOptions{
		Name:       "SecretLens Secret Scan",
		HeadSHA:    sha,
		Status:     gogithub.Ptr(status),
		Conclusion: gogithub.Ptr(conclusion),
		CompletedAt: &gogithub.Timestamp{Time: time.Now()},
		Output: &gogithub.CheckRunOutput{
			Title:   gogithub.Ptr("SecretLens Scan Results"),
			Summary: gogithub.Ptr(summary),
			Text:    gogithub.Ptr(text),
		},
	}

	_, _, err := r.client.Checks.CreateCheckRun(ctx, r.Owner, r.Repo, opts)
	if err != nil {
		return fmt.Errorf("check run作成失敗: %w", err)
	}
	return nil
}

func formatPRComment(findings []finding.Finding) string {
	if len(findings) == 0 {
		return "## SecretLens Scan\n\n✅ シークレットは検出されませんでした。"
	}

	var sb strings.Builder
	sb.WriteString("## SecretLens Scan Results\n\n")
	fmt.Fprintf(&sb, "🔍 **%d 件の問題を検出しました。**\n\n", len(findings))
	sb.WriteString("| Severity | Rule | File | Line | Match |\n")
	sb.WriteString("|----------|------|------|------|-------|\n")
	for _, f := range findings {
		icon := severityIcon(f.Severity)
		fmt.Fprintf(&sb, "| %s %s | `%s` | `%s` | %d | `%s` |\n",
			icon, f.Severity, f.RuleID, f.File, f.Line, f.Match)
	}
	sb.WriteString("\n<details><summary>詳細情報</summary>\n\n")
	sb.WriteString("SecretLens によって自動検出されました。誤検知の場合は `.secretlens.baseline.json` に追加してください。\n</details>")
	return sb.String()
}

func formatCheckRunText(findings []finding.Finding) string {
	if len(findings) == 0 {
		return "問題は検出されませんでした。"
	}
	var sb strings.Builder
	for _, f := range findings {
		fmt.Fprintf(&sb, "- [%s] %s: %s:%d (%s)\n",
			f.Severity, f.RuleID, f.File, f.Line, f.Match)
	}
	return sb.String()
}

func severityIcon(s finding.Severity) string {
	switch s {
	case finding.SeverityCritical:
		return "🔴"
	case finding.SeverityHigh:
		return "🟠"
	case finding.SeverityMedium:
		return "🟡"
	default:
		return "🔵"
	}
}

func hasHigherThan(findings []finding.Finding, threshold finding.Severity) bool {
	order := map[finding.Severity]int{
		finding.SeverityLow:      0,
		finding.SeverityMedium:   1,
		finding.SeverityHigh:     2,
		finding.SeverityCritical: 3,
	}
	for _, f := range findings {
		if order[f.Severity] > order[threshold] {
			return true
		}
	}
	return false
}

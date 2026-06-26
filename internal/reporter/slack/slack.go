package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nobuo-miura/SecretLens/internal/finding"
)

type block struct {
	Type string    `json:"type"`
	Text *textObj  `json:"text,omitempty"`
}

type textObj struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type payload struct {
	Blocks []block `json:"blocks"`
}

// Notify はSlack Webhookにスキャン結果を通知する
func Notify(webhookURL string, findings []finding.Finding, repoName string) error {
	blocks := buildBlocks(findings, repoName)
	p := payload{Blocks: blocks}

	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("slackペイロード生成失敗: %w", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(data)) //nolint:noctx
	if err != nil {
		return fmt.Errorf("slack通知送信失敗: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook エラー: %s", resp.Status)
	}
	return nil
}

func buildBlocks(findings []finding.Finding, repoName string) []block {
	blocks := []block{
		{
			Type: "header",
			Text: &textObj{Type: "plain_text", Text: "🔍 SecretLens スキャン結果"},
		},
		{
			Type: "section",
			Text: &textObj{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*リポジトリ:* `%s`\n*検出件数:* %d 件", repoName, len(findings)),
			},
		},
	}

	if len(findings) == 0 {
		blocks = append(blocks, block{
			Type: "section",
			Text: &textObj{Type: "mrkdwn", Text: "✅ シークレットは検出されませんでした。"},
		})
		return blocks
	}

	// 上位10件まで表示
	limit := len(findings)
	if limit > 10 {
		limit = 10
	}

	var lines string
	for _, f := range findings[:limit] {
		icon := severityIcon(f.Severity)
		lines += fmt.Sprintf("%s *[%s]* `%s` — `%s:%d`\n", icon, f.Severity, f.RuleID, f.File, f.Line)
	}
	if len(findings) > 10 {
		lines += fmt.Sprintf("...他 %d 件\n", len(findings)-10)
	}

	blocks = append(blocks, block{
		Type: "section",
		Text: &textObj{Type: "mrkdwn", Text: lines},
	})
	return blocks
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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/nobuo-miura/SecretLens/internal/finding"
	"github.com/nobuo-miura/SecretLens/internal/org"
	reporthtml "github.com/nobuo-miura/SecretLens/internal/reporter/html"
)

var (
	flagOrgName     string
	flagOrgToken    string
	flagConcurrency int
	flagOrgFormat   string
	flagOrgOut      string
)

var orgCmd = &cobra.Command{
	Use:   "org",
	Short: "Org監査モード — GitHub Org全リポジトリを横断スキャン",
	RunE:  runOrgAudit,
}

func init() {
	orgCmd.Flags().StringVar(&flagOrgName, "org", "", "GitHub Organization名（必須）")
	orgCmd.Flags().StringVar(&flagOrgToken, "token", "", "GitHub APIトークン (GITHUB_TOKEN 環境変数も可)")
	orgCmd.Flags().IntVar(&flagConcurrency, "concurrency", 4, "並列スキャン数")
	orgCmd.Flags().StringVar(&flagOrgFormat, "format", "text", "出力フォーマット (text|json|html)")
	orgCmd.Flags().StringVar(&flagOrgOut, "out", "", "出力ファイルパス（省略時はstdout）")
	orgCmd.Flags().StringVar(&flagRulesDir, "rules-dir", "", "YAMLルールディレクトリ")
	_ = orgCmd.MarkFlagRequired("org")
	rootCmd.AddCommand(orgCmd)
}

func runOrgAudit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	token := flagOrgToken
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		return fmt.Errorf("GitHub APIトークンが必要です（--token または GITHUB_TOKEN 環境変数）")
	}

	rulesDir := flagRulesDir
	if rulesDir == "" {
		rulesDir = resolveRulesDir(".")
	}

	opts := org.AuditOptions{
		Token:       token,
		Org:         flagOrgName,
		RulesDir:    rulesDir,
		Concurrency: flagConcurrency,
	}

	results, err := org.AuditOrg(ctx, opts)
	if err != nil {
		return err
	}

	return outputOrgResults(results, flagOrgName)
}

func outputOrgResults(results []org.RepoResult, orgName string) error {
	out := os.Stdout
	if flagOrgOut != "" {
		f, err := os.Create(flagOrgOut)
		if err != nil {
			return fmt.Errorf("出力ファイル作成失敗: %w", err)
		}
		defer func() { _ = f.Close() }()
		out = f
	}

	switch flagOrgFormat {
	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	case "html":
		// 全リポジトリのfindingsを結合
		var allFindings []finding.Finding
		for _, r := range results {
			allFindings = append(allFindings, r.Findings...)
		}
		return reporthtml.Write(out, allFindings, orgName)
	default:
		printOrgText(results)
	}
	return nil
}

func printOrgText(results []org.RepoResult) {
	total := 0
	for _, r := range results {
		if r.Err != nil {
			fmt.Printf("  [ERROR] %s: %v\n", r.Repo, r.Err)
			continue
		}
		if len(r.Findings) == 0 {
			continue
		}
		fmt.Printf("\n=== %s (%d件) ===\n", r.Repo, len(r.Findings))
		for _, f := range r.Findings {
			fmt.Printf("  [%s] %s:%d  rule=%s\n", f.Severity, f.File, f.Line, f.RuleID)
		}
		total += len(r.Findings)
	}
	fmt.Printf("\n合計: %d件 (%d リポジトリ)\n", total, len(results))
}

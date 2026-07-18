package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nobuo-miura/SecretLens/internal/baseline"
	"github.com/nobuo-miura/SecretLens/internal/scanner"
)

var (
	flagBLFile     string
	flagBLRulesDir string
)

var baselineCmd = &cobra.Command{
	Use:   "baseline",
	Short: "ベースライン管理",
}

var baselineListCmd = &cobra.Command{
	Use:   "list",
	Short: "登録済みfingerprintを一覧表示",
	RunE: func(cmd *cobra.Command, args []string) error {
		bl, err := baseline.Load(flagBLFile)
		if err != nil {
			return err
		}
		fps := bl.List()
		if len(fps) == 0 {
			fmt.Println("ベースラインにエントリがありません。")
			return nil
		}
		for _, fp := range fps {
			fmt.Println(fp)
		}
		return nil
	},
}

var baselineUpdateCmd = &cobra.Command{
	Use:   "update [path]",
	Short: "現在のスキャン結果をベースラインに追加",
	Long:  "指定パス（省略時はカレントディレクトリ）をスキャンし、検出された全fingerprintをベースラインに追加して保存します。",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := "."
		if len(args) > 0 {
			repoPath = args[0]
		}

		rules, err := loadRules(flagBLRulesDir)
		if err != nil {
			return err
		}

		bl, err := baseline.Load(flagBLFile)
		if err != nil {
			return err
		}

		// 既存baselineでのフィルタは有効のまま実行し、新規検出分だけを追加する
		findings, err := scanner.Run(scanner.Options{
			Source:       "all",
			RepoPath:     repoPath,
			Rules:        rules,
			BaselineFile: flagBLFile,
		})
		if err != nil {
			return err
		}

		for _, f := range findings {
			bl.Add(f.Fingerprint)
		}
		if err := bl.Save(); err != nil {
			return err
		}
		fmt.Printf("ベースラインを更新しました: %d件追加 (合計%d件) → %s\n", len(findings), len(bl.List()), flagBLFile)
		return nil
	},
}

var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "ルール管理",
}

var rulesListCmd = &cobra.Command{
	Use:   "list",
	Short: "有効ルール一覧を表示",
	RunE: func(cmd *cobra.Command, args []string) error {
		rulesDir, _ := cmd.Flags().GetString("rules-dir")
		rules, err := loadRules(rulesDir)
		if err != nil {
			return err
		}
		fmt.Printf("%-30s %-10s %s\n", "ID", "SEVERITY", "NAME")
		fmt.Printf("%-30s %-10s %s\n", "---", "--------", "----")
		for _, r := range rules {
			fmt.Printf("%-30s %-10s %s\n", r.ID, r.Severity, r.Name)
		}
		fmt.Printf("\n合計: %d ルール\n", len(rules))
		return nil
	},
}

func init() {
	baselineCmd.PersistentFlags().StringVar(&flagBLFile, "baseline", baseline.DefaultFile, "ベースラインファイルパス")
	baselineUpdateCmd.Flags().StringVar(&flagBLRulesDir, "rules-dir", "", "追加・上書きYAMLルールディレクトリ（省略時は内蔵ルールのみ）")
	baselineCmd.AddCommand(baselineListCmd)
	baselineCmd.AddCommand(baselineUpdateCmd)
	rootCmd.AddCommand(baselineCmd)

	rulesListCmd.Flags().String("rules-dir", "", "追加・上書きYAMLルールディレクトリ（省略時は内蔵ルールのみ）")
	rulesCmd.AddCommand(rulesListCmd)
	rootCmd.AddCommand(rulesCmd)
}

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nobuo-miura/SecretLens/internal/baseline"
	"github.com/nobuo-miura/SecretLens/internal/detector/regex"
)

var baselineCmd = &cobra.Command{
	Use:   "baseline",
	Short: "ベースライン管理",
}

var baselineListCmd = &cobra.Command{
	Use:   "list",
	Short: "登録済みfingerprintを一覧表示",
	RunE: func(cmd *cobra.Command, args []string) error {
		bl, err := baseline.Load(baseline.DefaultFile)
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
	Use:   "update",
	Short: "現在のスキャン結果をベースラインに追加",
	Long:  "scan コマンドの出力をベースラインに追加します。先に scan --format=json を実行してください。",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("使用方法: secretlens scan --format=json | jq -r '.[].fingerprint' でfingerprintを取得し、baseline updateで登録してください。")
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
		if rulesDir == "" {
			exe, err := os.Executable()
			if err != nil {
				exe = "."
			}
			rulesDir = filepath.Join(filepath.Dir(exe), "rules")
			if _, err := os.Stat(rulesDir); os.IsNotExist(err) {
				rulesDir = "rules"
			}
		}

		rules, err := regex.LoadRulesFromDir(rulesDir)
		if err != nil {
			return err
		}
		if len(rules) == 0 {
			fmt.Printf("ルールが見つかりません: %s\n", rulesDir)
			return nil
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
	baselineCmd.AddCommand(baselineListCmd)
	baselineCmd.AddCommand(baselineUpdateCmd)
	rootCmd.AddCommand(baselineCmd)

	rulesListCmd.Flags().String("rules-dir", "", "YAMLルールディレクトリ（デフォルト: 実行ファイル隣のrules/）")
	rulesCmd.AddCommand(rulesListCmd)
	rootCmd.AddCommand(rulesCmd)
}

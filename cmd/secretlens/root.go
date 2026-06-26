package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "secretlens",
	Short: "SecretLens — シークレット検出CLIツール",
	Long:  `SecretLens は git履歴・環境変数ファイル・CIログ・Dockerレイヤーを対象に、優先度付きアラートを出力するシークレット検出ツールです。`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

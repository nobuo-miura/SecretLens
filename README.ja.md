# SecretLens

[English](README.md) | [日本語](README.ja.md)

> **Git履歴・CIログ・環境変数ファイル・Dockerレイヤー**を対象に、優先度付きアラートを出力するGo製OSSシークレット検出CLI。

[![CI](https://github.com/nobuo-miura/SecretLens/actions/workflows/ci.yml/badge.svg)](https://github.com/nobuo-miura/SecretLens/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.26-blue)](go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/nobuo-miura/SecretLens)](https://goreportcard.com/report/github.com/nobuo-miura/SecretLens)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE.md)

---

## インストール

### go install

```bash
go install github.com/nobuo-miura/SecretLens/cmd/secretlens@latest
```

### ソースからビルド

```bash
git clone https://github.com/nobuo-miura/SecretLens.git
cd SecretLens
make build        # → bin/secretlens
make install      # → $GOPATH/bin/secretlens
```

---

## クイックスタート

```bash
# Git履歴 + 環境変数ファイルをスキャン（デフォルト）
secretlens scan .

# Git履歴 + 環境変数ファイルを明示的にスキャン
secretlens scan --all .

# CIログをスキャン（GitHub Actions）
secretlens scan --source=cilog --repo=owner/repo

# Dockerレイヤーをスキャン
secretlens scan --source=docker --image=myapp:latest

# SARIF形式で出力
secretlens scan --format=sarif --out=results.sarif .

# HTMLレポートを生成
secretlens scan --format=html --out=report.html .

# 検出時にCI終了コード1を返す
secretlens scan --fail-on=HIGH .

# ステージ済み差分のみスキャン（pre-commit用）
secretlens scan --staged .

# 作業ツリーの未コミット変更（追跡済みファイル）をスキャン
secretlens scan --source=worktree .

# ブランチで新しく追加されたコミットだけスキャン（CIのPRスキャン高速化）
secretlens scan --commit-range=origin/main..HEAD .
```

---

## コマンドリファレンス

### `secretlens scan`

| フラグ | 説明 | デフォルト |
|--------|------|-----------|
| `--all` | Git履歴 + 環境変数ファイルをスキャン | false |
| `--source` | スキャンソース: `git` `envfile` `all` `worktree` `staged` `cilog` `docker` | git+envfile |
| `--staged` | ステージ済み差分のみスキャン（`--source=staged`の別名、pre-commit用） | false |
| `--since` | 指定コミット以降の履歴のみスキャン（`<commit>..HEAD`） | — |
| `--commit-range` | 指定範囲の履歴のみスキャン（`base..head`形式） | — |
| `--config` | 設定ファイルパス（省略時はスキャン対象直下の`.secretlens.yml`を自動探索） | — |
| `--format` | 出力形式: `text` `json` `sarif` `html` `github-pr` | text |
| `--out` | 出力ファイルパス（省略時: stdout） | — |
| `--fail-on` | 指定Severity以上で exit 1: `CRITICAL` `HIGH` `MEDIUM` `LOW` | — |
| `--rules-dir` | 追加・上書きYAMLルールディレクトリ | なし（内蔵ルールのみ） |
| `--baseline` | ベースラインファイルパス | `.secretlens.baseline.json` |
| `--repo` | GitHub リポジトリ `owner/repo`（cilog用） | — |
| `--gitlab-url` | GitLabインスタンスURL（cilog用） | — |
| `--project-id` | GitLabプロジェクトID（cilog用） | — |
| `--image` | Dockerイメージ名（docker用） | — |
| `--pr` | PRコメントを投稿するPR番号 | — |
| `--sha` | Check Runを作成するコミットSHA | — |
| `--github-token` | GitHub APIトークン（`GITHUB_TOKEN`環境変数も可） | — |
| `--slack-webhook` | Slack Webhook URL（`SLACK_WEBHOOK_URL`環境変数も可） | — |
| `--verify` | 対応ルールのLive API検証を実行（opt-in、現在はGitHubトークンのみ） | false |

### `secretlens org`

GitHub Organization全リポジトリを並列横断スキャンする。

```bash
secretlens org --org=my-company --format=html --out=audit.html
```

| フラグ | 説明 | デフォルト |
|--------|------|-----------|
| `--org` | GitHub Organization名（必須） | — |
| `--token` | GitHub APIトークン（`GITHUB_TOKEN`環境変数も可） | — |
| `--concurrency` | 並列スキャン数 | 4 |
| `--format` | 出力形式: `text` `json` `html` | text |
| `--out` | 出力ファイルパス | — |
| `--rules-dir` | 追加・上書きYAMLルールディレクトリ | なし（内蔵ルールのみ） |

### `secretlens baseline`

```bash
secretlens baseline update .        # スキャンして現在の検出結果をベースラインに追加
secretlens baseline list            # 登録済みfingerprint一覧
```

### `secretlens rules list`

```bash
secretlens rules list               # 有効ルール一覧（ID / Severity / 名前）
```

---

## スコアリングと優先度

検出された各シークレットにはスコアが付与され、Severityに変換されます。

| 条件 | スコア |
|------|--------|
| ベースルール: CRITICAL | +60 |
| ベースルール: HIGH | +40 |
| ベースルール: MEDIUM | +20 |
| Live検証通過 | +50 |
| エントロピーがルールの`entropy_min`以上（未設定時は > 4.5） | +20 |
| センシティブファイル名 (`.env`, `credentials` 等) | +15 |
| テストコード内 | -30 |

コメント行はスキャン対象外のため、検出自体が行われません。

| スコア | Severity |
|--------|----------|
| 60以上 | 🔴 CRITICAL |
| 40〜59 | 🟠 HIGH |
| 20〜39 | 🟡 MEDIUM |
| 未満 | 🔵 LOW |

---

## カスタムルール

標準ルールはバイナリに内蔵されています。任意のディレクトリにYAMLファイルを置き `--rules-dir` で指定すると、ルールを追加できます（同一IDは上書き）。

```yaml
# rules/my-company.yaml
rules:
  - id: myco-internal-token
    name: MyCompany Internal Token
    severity: CRITICAL
    pattern: 'MYCO_[A-Za-z0-9]{32}'
    entropy_min: 4.0
    context_exclude:
      - "**/*_test.go"
      - "**/testdata/**"
    tags:
      - myco
      - internal
```

### ルールスキーマ

| フィールド | 型 | 説明 |
|-----------|-----|------|
| `id` | string | ユニークID（必須） |
| `name` | string | 表示名（必須） |
| `severity` | string | `CRITICAL` `HIGH` `MEDIUM` `LOW`（必須） |
| `pattern` | string | Go regexp 構文の正規表現（必須） |
| `entropy_min` | float | 最低エントロピー閾値（省略可） |
| `context_exclude` | []string | 除外globパターン（省略可） |
| `tags` | []string | タグ（省略可） |

---

## 設定ファイル

スキャン対象ディレクトリ直下に`.secretlens.yml`（または`.secretlens.yaml`）を置くか`--config`で指定すると、CIとローカルで同じ設定を共有できます。CLIで明示指定したフラグが常に設定ファイルより優先されます。

```yaml
# .secretlens.yml
source: all
format: sarif
out: results.sarif
fail_on: HIGH
rules_dir: ./custom-rules
baseline: .secretlens.baseline.json
exclude:                 # 全ルール共通の除外glob（git / worktree / staged / envfile / docker スキャンに適用）
  - "**/vendor/**"
  - "**/testdata/**"
```

注意点:

- 相対パス（`rules_dir` / `baseline` / `out`）は設定ファイルのディレクトリ基準で解決されます。
- `out`は`--config`で明示指定した場合のみ適用されます。自動検出した設定はスキャン対象リポジトリ由来のため、出力先を握らせないための制限です。
- Slack Webhook URLはそれ自体がシークレットのため、設定ファイルでは意図的に**非対応**です。`--slack-webhook`フラグまたは`SLACK_WEBHOOK_URL`環境変数を使ってください。

### pre-commit hook

```bash
#!/bin/sh
# .git/hooks/pre-commit
exec secretlens scan --staged --fail-on=HIGH .
```

---

## ベースライン管理

誤検知をベースラインに登録して以降のスキャンで除外できます。

```bash
# スキャンして現在の検出結果をすべてベースラインに追加
secretlens baseline update .

# 登録済みfingerprintを一覧表示
secretlens baseline list

# 現在のスキャンのfingerprintを確認
secretlens scan --format=json . | jq -r '.[].fingerprint'

# .secretlens.baseline.json を手動編集して追加
{
  "fingerprints": {
    "abc123...": true
  }
}
```

---

## GitHub Actions 連携

```yaml
# .github/workflows/secretlens.yml
name: Secret Scan

on: [push, pull_request]

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0  # Git履歴を全取得

      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - name: Install SecretLens
        run: go install github.com/nobuo-miura/SecretLens/cmd/secretlens@latest

      - name: Scan
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          secretlens scan \
            --all \
            --format=sarif \
            --out=results.sarif \
            --fail-on=HIGH \
            .

      - name: Upload SARIF
        uses: github/codeql-action/upload-sarif@v4
        if: always()
        with:
          sarif_file: results.sarif

      - name: PR Comment
        if: github.event_name == 'pull_request'
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          secretlens scan \
            --all \
            --format=github-pr \
            --repo=${{ github.repository }} \
            --pr=${{ github.event.pull_request.number }} \
            --sha=${{ github.sha }} \
            .
```

---

## ビルトインルール

| ファイル | カバー範囲 |
|---------|-----------|
| `rules/aws.yaml` | AWS Access Key ID / Secret Access Key |
| `rules/gcp.yaml` | GCP Service Account Key / API Key |
| `rules/azure.yaml` | Azure Storage Account Key / Connection String |
| `rules/github.yaml` | GitHub Token |
| `rules/jwt.yaml` | JWT Token |
| `rules/generic.yaml` | API Key / Password / Token / Private Key / Connection String |

---

## 開発

```bash
make test           # テスト実行
make check          # vet + test + lint
make run-scan       # サンプルスキャン
make report-html    # HTMLレポートを生成してブラウザで開く
make release        # 主要プラットフォーム向けクロスコンパイル
make help           # コマンド一覧
```

### ディレクトリ構成

```
secretlens/
├── cmd/secretlens/          # CLIエントリポイント (cobra)
├── internal/
│   ├── scanner/
│   │   ├── git/             # git log --all -p ストリーミング解析
│   │   ├── cilog/           # GitHub Actions / GitLab CI ログAPI
│   │   ├── envfile/         # .env / .tfvars / *.yaml スキャン
│   │   └── docker/          # Dockerイメージレイヤー展開スキャン
│   ├── detector/
│   │   ├── regex/           # YAMLルール読み込み + 正規表現マッチ
│   │   ├── entropy/         # Shannon entropy 計算
│   │   ├── context/         # テストコード / コメント除外
│   │   └── verifier/        # Live API検証（現在はGitHubトークンのみ）
│   ├── finding/             # Finding構造体 + スコアリング
│   ├── reporter/
│   │   ├── sarif/           # SARIF v2.1.0 出力
│   │   ├── github/          # PRコメント + Check Run API
│   │   ├── slack/           # Webhook通知 (Block Kit)
│   │   └── html/            # インタラクティブHTMLレポート
│   ├── baseline/            # .secretlens.baseline.json 管理
│   └── org/                 # GitHub Org横断監査モード
├── rules/                   # ビルトインYAMLルール
└── testdata/                # テスト用サンプルファイル
```

---

## ライセンス

MIT License

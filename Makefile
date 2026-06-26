BINARY     := secretlens
MODULE     := github.com/nobuo-miura/SecretLens
CMD        := ./cmd/secretlens
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    := -ldflags "-X main.version=$(VERSION) -s -w"

GOBIN      ?= $(shell go env GOPATH)/bin
GOLANGCI   := $(GOBIN)/golangci-lint

.PHONY: all build test lint vet fmt clean install release help

all: build ## ビルド（デフォルト）

## ─── ビルド ───────────────────────────────────────────────────────────────────

build: ## バイナリをビルドして ./bin/ に出力
	@mkdir -p bin
	go build $(LDFLAGS) -o bin/$(BINARY) $(CMD)
	@echo "ビルド完了: bin/$(BINARY)  (version=$(VERSION))"

install: ## $GOBIN にインストール
	go install $(LDFLAGS) $(CMD)
	@echo "インストール完了: $(GOBIN)/$(BINARY)"

release: ## 主要プラットフォーム向けクロスコンパイル
	@mkdir -p dist
	GOOS=linux  GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64   $(CMD)
	GOOS=linux  GOARCH=arm64  go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64   $(CMD)
	GOOS=darwin GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64  $(CMD)
	GOOS=darwin GOARCH=arm64  go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64  $(CMD)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe $(CMD)
	@echo "クロスコンパイル完了: dist/"
	@ls -lh dist/

## ─── 品質チェック ──────────────────────────────────────────────────────────────

test: ## テストを実行
	go test -race -count=1 ./...

test-coverage: ## カバレッジレポートを生成してブラウザで開く
	go test -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "カバレッジレポート: coverage.html"

vet: ## go vet を実行
	go vet ./...

fmt: ## gofmt でコードをフォーマット
	gofmt -l -w .

lint: $(GOLANGCI) ## golangci-lint を実行
	$(GOLANGCI) run ./...

$(GOLANGCI):
	@echo "golangci-lint をインストール中..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

check: vet test lint ## vet + test + lint をまとめて実行

## ─── 開発支援 ──────────────────────────────────────────────────────────────────

run-scan: build ## サンプルスキャン（envfile ソース）
	./bin/$(BINARY) scan --source=envfile --rules-dir=rules .

run-rules: build ## ルール一覧表示
	./bin/$(BINARY) rules list --rules-dir=rules

report-html: build ## HTMLレポートを生成して開く
	./bin/$(BINARY) scan --source=envfile --rules-dir=rules --format=html --out=/tmp/secretlens_report.html .
	@open /tmp/secretlens_report.html 2>/dev/null || xdg-open /tmp/secretlens_report.html 2>/dev/null || echo "レポート生成完了: /tmp/secretlens_report.html"

## ─── クリーンアップ ────────────────────────────────────────────────────────────

clean: ## ビルド成果物を削除
	rm -rf bin/ dist/ coverage.out coverage.html

tidy: ## go mod tidy を実行
	go mod tidy

## ─── ヘルプ ────────────────────────────────────────────────────────────────────

help: ## このヘルプを表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

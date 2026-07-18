package org

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	gogithub "github.com/google/go-github/v72/github"
	"golang.org/x/oauth2"

	"github.com/nobuo-miura/SecretLens/internal/detector/regex"
	"github.com/nobuo-miura/SecretLens/internal/finding"
	"github.com/nobuo-miura/SecretLens/internal/scanner"
)

// AuditOptions はOrg監査モードのオプション
type AuditOptions struct {
	Token       string
	Org         string
	Rules       []regex.Rule
	WorkDir     string // リポジトリをクローンする作業ディレクトリ
	Concurrency int
	FailOn      string
}

// RepoResult は単一リポジトリのスキャン結果
type RepoResult struct {
	Repo     string
	Findings []finding.Finding
	Err      error
}

// AuditOrg はGitHub Orgの全リポジトリをスキャンする
func AuditOrg(ctx context.Context, opts AuditOptions) ([]RepoResult, error) {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}

	var hc *http.Client
	if opts.Token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: opts.Token})
		hc = oauth2.NewClient(ctx, ts)
	}
	client := gogithub.NewClient(hc)

	repos, err := listOrgRepos(ctx, client, opts.Org)
	if err != nil {
		return nil, err
	}
	// 進捗はstderrへ出す（--format=jsonのstdout出力を汚染しないため）
	fmt.Fprintf(os.Stderr, "Org %s: %d リポジトリをスキャンします\n", opts.Org, len(repos))

	workDir := opts.WorkDir
	if workDir == "" {
		tmp, err := os.MkdirTemp("", "secretlens-org-*")
		if err != nil {
			return nil, fmt.Errorf("作業ディレクトリ作成失敗: %w", err)
		}
		defer func() { _ = os.RemoveAll(tmp) }()
		workDir = tmp
	}

	sem := make(chan struct{}, opts.Concurrency)
	resultCh := make(chan RepoResult, len(repos))
	var wg sync.WaitGroup

	for _, repo := range repos {
		wg.Add(1)
		go func(repoName string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			findings, err := scanRepo(ctx, opts, workDir, repoName)
			resultCh <- RepoResult{Repo: repoName, Findings: findings, Err: err}
		}(repo)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var results []RepoResult
	for r := range resultCh {
		results = append(results, r)
	}
	return results, nil
}

func listOrgRepos(ctx context.Context, client *gogithub.Client, org string) ([]string, error) {
	var allRepos []string
	opts := &gogithub.RepositoryListByOrgOptions{
		Type:        "all",
		ListOptions: gogithub.ListOptions{PerPage: 100},
	}
	for {
		repos, resp, err := client.Repositories.ListByOrg(ctx, org, opts)
		if err != nil {
			return nil, fmt.Errorf("orgリポジトリ一覧取得失敗: %w", err)
		}
		for _, r := range repos {
			if !r.GetArchived() {
				allRepos = append(allRepos, r.GetName())
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allRepos, nil
}

func scanRepo(ctx context.Context, opts AuditOptions, workDir, repoName string) ([]finding.Finding, error) {
	repoDir := filepath.Join(workDir, repoName)
	cloneURL := fmt.Sprintf("https://github.com/%s/%s.git", opts.Org, repoName)

	if err := cloneRepo(ctx, cloneURL, repoDir, opts.Token); err != nil {
		return nil, fmt.Errorf("リポジトリクローン失敗 %s: %w", repoName, err)
	}
	defer func() { _ = os.RemoveAll(repoDir) }()

	scanOpts := scanner.Options{
		Source:       "all",
		RepoPath:     repoDir,
		Rules:        opts.Rules,
		BaselineFile: filepath.Join(repoDir, ".secretlens.baseline.json"),
		Format:       "json",
		FailOn:       opts.FailOn,
	}
	return scanner.Run(scanOpts)
}

// cloneRepo は全履歴をクローンする（履歴スキャンのためシャロークローンにしない）。
// トークンはURLやプロセス引数に載せず、環境変数経由のhttp.extraheaderで渡す
func cloneRepo(ctx context.Context, cloneURL, dir, token string) error {
	cmd := exec.CommandContext(ctx, "git", "clone", "--quiet", cloneURL, dir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if token != "" {
		basic := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
		cmd.Env = append(cmd.Env,
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=http.https://github.com/.extraheader",
			"GIT_CONFIG_VALUE_0=Authorization: Basic "+basic,
		)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

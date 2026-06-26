package org

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	gogithub "github.com/google/go-github/v72/github"
	"golang.org/x/oauth2"

	"github.com/nobuo-miura/SecretLens/internal/finding"
	"github.com/nobuo-miura/SecretLens/internal/scanner"
)

// AuditOptions はOrg監査モードのオプション
type AuditOptions struct {
	Token       string
	Org         string
	RulesDir    string
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
	fmt.Printf("Org %s: %d リポジトリをスキャンします\n", opts.Org, len(repos))

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
	cloneURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.git", opts.Token, opts.Org, repoName)

	if err := cloneRepoShallow(ctx, cloneURL, repoDir); err != nil {
		return nil, fmt.Errorf("リポジトリクローン失敗 %s: %w", repoName, err)
	}
	defer func() { _ = os.RemoveAll(repoDir) }()

	scanOpts := scanner.Options{
		Source:       "all",
		RepoPath:     repoDir,
		RulesDir:     opts.RulesDir,
		BaselineFile: filepath.Join(repoDir, ".secretlens.baseline.json"),
		Format:       "json",
		FailOn:       opts.FailOn,
	}
	return scanner.Run(scanOpts)
}

// cloneRepoShallow はgit clone --depth=1でシャロークローンを実行する
func cloneRepoShallow(ctx context.Context, cloneURL, dir string) error {
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", cloneURL, dir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd.Run()
}

package cilog

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	gogithub "github.com/google/go-github/v72/github"
	"golang.org/x/oauth2"
)

type GitHubActionsScanner struct {
	client *gogithub.Client
	Owner  string
	Repo   string
}

// NewGitHubActionsScanner はGitHub Actionsログスキャナーを生成する
func NewGitHubActionsScanner(token, owner, repo string) *GitHubActionsScanner {
	var hc *http.Client
	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		hc = oauth2.NewClient(context.Background(), ts)
	}
	return &GitHubActionsScanner{
		client: gogithub.NewClient(hc),
		Owner:  owner,
		Repo:   repo,
	}
}

// LogLine はCIログの1行を表す
type LogLine struct {
	Source string // "github-actions" | "gitlab-ci"
	Job    string
	Step   string
	Line   int
	Text   string
}

// StreamLogs は最新ワークフローランのログをストリーミングで返す。
// 一部のランで取得に失敗した場合も残りを処理し、失敗はまとめてエラーとして返す
func (s *GitHubActionsScanner) StreamLogs(ctx context.Context, out chan<- LogLine) error {
	// リポジトリ全体のワークフローラン一覧を取得する
	runs, _, err := s.client.Actions.ListRepositoryWorkflowRuns(
		ctx, s.Owner, s.Repo,
		&gogithub.ListWorkflowRunsOptions{
			ListOptions: gogithub.ListOptions{PerPage: 10},
		},
	)
	if err != nil {
		return fmt.Errorf("ワークフローラン取得失敗: %w", err)
	}

	var runErrs []error
	for _, run := range runs.WorkflowRuns {
		if err := s.streamRunLogs(ctx, run.GetID(), out); err != nil {
			runErrs = append(runErrs, fmt.Errorf("run %d (%s) のログ取得失敗: %w", run.GetID(), run.GetName(), err))
		}
	}
	return errors.Join(runErrs...)
}

func (s *GitHubActionsScanner) streamRunLogs(ctx context.Context, runID int64, out chan<- LogLine) error {
	jobs, _, err := s.client.Actions.ListWorkflowJobs(
		ctx, s.Owner, s.Repo, runID,
		&gogithub.ListWorkflowJobsOptions{Filter: "latest"},
	)
	if err != nil {
		return err
	}

	var jobErrs []error
	for _, job := range jobs.Jobs {
		if err := s.streamJobLogs(ctx, job, out); err != nil {
			jobErrs = append(jobErrs, fmt.Errorf("job %s: %w", job.GetName(), err))
		}
	}
	return errors.Join(jobErrs...)
}

func (s *GitHubActionsScanner) streamJobLogs(ctx context.Context, job *gogithub.WorkflowJob, out chan<- LogLine) error {
	url, _, err := s.client.Actions.GetWorkflowJobLogs(ctx, s.Owner, s.Repo, job.GetID(), 3)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ログダウンロード失敗 (HTTP %d)", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		text := scanner.Text()
		// タイムスタンププレフィックスを除去 (2024-01-01T00:00:00.0000000Z )
		if idx := strings.Index(text, " "); idx > 0 && len(text) > idx {
			text = text[idx+1:]
		}
		out <- LogLine{
			Source: "github-actions",
			Job:    job.GetName(),
			Line:   lineNum,
			Text:   text,
		}
	}
	return scanner.Err()
}

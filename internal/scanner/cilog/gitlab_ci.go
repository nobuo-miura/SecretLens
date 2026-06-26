package cilog

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type GitLabCIScanner struct {
	client    *http.Client
	BaseURL   string
	Token     string
	ProjectID string
}

// NewGitLabCIScanner はGitLab CIログスキャナーを生成する
func NewGitLabCIScanner(baseURL, token, projectID string) *GitLabCIScanner {
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}
	return &GitLabCIScanner{
		client:    &http.Client{},
		BaseURL:   strings.TrimRight(baseURL, "/"),
		Token:     token,
		ProjectID: url.PathEscape(projectID),
	}
}

type gitlabJob struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// StreamLogs はGitLab CIジョブログをストリーミングで返す
func (s *GitLabCIScanner) StreamLogs(ctx context.Context, out chan<- LogLine) error {
	jobs, err := s.listJobs(ctx)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := s.streamJobLog(ctx, job, out); err != nil {
			continue
		}
	}
	return nil
}

func (s *GitLabCIScanner) listJobs(ctx context.Context) ([]gitlabJob, error) {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/jobs?per_page=20", s.BaseURL, s.ProjectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", s.Token)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitLab CIジョブ一覧取得失敗: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitLab API エラー: %s", resp.Status)
	}

	var jobs []gitlabJob
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (s *GitLabCIScanner) streamJobLog(ctx context.Context, job gitlabJob, out chan<- LogLine) error {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/jobs/%d/trace", s.BaseURL, s.ProjectID, job.ID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", s.Token)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		out <- LogLine{
			Source: "gitlab-ci",
			Job:    job.Name,
			Line:   lineNum,
			Text:   scanner.Text(),
		}
	}
	return scanner.Err()
}

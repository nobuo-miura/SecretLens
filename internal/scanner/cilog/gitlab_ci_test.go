package cilog

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGitLabCIScanner_Defaults(t *testing.T) {
	s := NewGitLabCIScanner("", "tok", "group/project")
	assert.Equal(t, "https://gitlab.com", s.BaseURL)
	// プロジェクトIDはURLエスケープされる（group/project → group%2Fproject）
	assert.Equal(t, "group%2Fproject", s.ProjectID)

	s2 := NewGitLabCIScanner("https://gitlab.example.com/", "tok", "123")
	assert.Equal(t, "https://gitlab.example.com", s2.BaseURL)
	assert.Equal(t, "123", s2.ProjectID)
}

func TestGitLabCIScanner_StreamLogs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/123/jobs", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-token", r.Header.Get("PRIVATE-TOKEN"))
		_, _ = fmt.Fprint(w, `[{"id": 1, "name": "build"}, {"id": 2, "name": "test"}]`)
	})
	mux.HandleFunc("/api/v4/projects/123/jobs/1/trace", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "line one\nline two\n")
	})
	mux.HandleFunc("/api/v4/projects/123/jobs/2/trace", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "test output\n")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s := NewGitLabCIScanner(srv.URL, "test-token", "123")

	ch := make(chan LogLine, 100)
	go func() {
		defer close(ch)
		require.NoError(t, s.StreamLogs(context.Background(), ch))
	}()

	var lines []LogLine
	for l := range ch {
		lines = append(lines, l)
	}
	require.Len(t, lines, 3)
	assert.Equal(t, "gitlab-ci", lines[0].Source)
	assert.Equal(t, "build", lines[0].Job)
	assert.Equal(t, 1, lines[0].Line)
	assert.Equal(t, "line one", lines[0].Text)
	assert.Equal(t, "test", lines[2].Job)
	assert.Equal(t, "test output", lines[2].Text)
}

func TestGitLabCIScanner_StreamLogs_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	s := NewGitLabCIScanner(srv.URL, "bad-token", "123")
	ch := make(chan LogLine, 1)
	err := s.StreamLogs(context.Background(), ch)
	assert.Error(t, err)
}

func TestNewGitHubActionsScanner(t *testing.T) {
	s, err := NewGitHubActionsScanner("", "owner", "repo")
	require.NoError(t, err)
	assert.Equal(t, "owner", s.Owner)
	assert.Equal(t, "repo", s.Repo)
}

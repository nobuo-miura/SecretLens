package slack

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nobuo-miura/SecretLens/internal/finding"
)

func TestBuildBlocks_Empty(t *testing.T) {
	blocks := buildBlocks(nil, "owner/repo")
	require.Len(t, blocks, 3)
	assert.Equal(t, "header", blocks[0].Type)
	assert.Contains(t, blocks[1].Text.Text, "owner/repo")
	assert.Contains(t, blocks[2].Text.Text, "検出されませんでした")
}

func TestBuildBlocks_WithFindings(t *testing.T) {
	findings := []finding.Finding{
		{RuleID: "aws-key", Severity: finding.SeverityCritical, File: "a.env", Line: 1},
		{RuleID: "gh-token", Severity: finding.SeverityLow, File: "b.yml", Line: 2},
	}
	blocks := buildBlocks(findings, "owner/repo")
	require.Len(t, blocks, 3)
	assert.Contains(t, blocks[1].Text.Text, "2 件")
	assert.Contains(t, blocks[2].Text.Text, "aws-key")
	assert.Contains(t, blocks[2].Text.Text, "a.env:1")
}

func TestBuildBlocks_LimitsToTen(t *testing.T) {
	var findings []finding.Finding
	for i := 0; i < 15; i++ {
		findings = append(findings, finding.Finding{
			RuleID: fmt.Sprintf("rule-%02d", i), Severity: finding.SeverityHigh,
			File: "f.env", Line: i + 1,
		})
	}
	blocks := buildBlocks(findings, "repo")
	require.Len(t, blocks, 3)
	text := blocks[2].Text.Text
	assert.Contains(t, text, "rule-09")
	assert.NotContains(t, text, "rule-10") // 11件目以降は省略
	assert.Contains(t, text, "他 5 件")
}

func TestNotify(t *testing.T) {
	var received payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &received))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := Notify(srv.URL, []finding.Finding{
		{RuleID: "aws-key", Severity: finding.SeverityCritical, File: "a.env", Line: 1},
	}, "owner/repo")
	require.NoError(t, err)
	assert.NotEmpty(t, received.Blocks)
}

func TestNotify_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := Notify(srv.URL, nil, "repo")
	assert.Error(t, err)
}

func TestSeverityIcon(t *testing.T) {
	assert.Equal(t, "🔴", severityIcon(finding.SeverityCritical))
	assert.Equal(t, "🟠", severityIcon(finding.SeverityHigh))
	assert.Equal(t, "🟡", severityIcon(finding.SeverityMedium))
	assert.Equal(t, "🔵", severityIcon(finding.SeverityLow))
}

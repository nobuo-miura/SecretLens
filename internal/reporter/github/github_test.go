package github

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nobuo-miura/SecretLens/internal/finding"
)

func TestFormatPRComment_Empty(t *testing.T) {
	body := formatPRComment(nil)
	assert.Contains(t, body, "検出されませんでした")
}

func TestFormatPRComment_WithFindings(t *testing.T) {
	findings := []finding.Finding{
		{RuleID: "aws-key", Severity: finding.SeverityCritical, File: "a.env", Line: 3, Match: "AKIA****MPLE"},
	}
	body := formatPRComment(findings)
	assert.Contains(t, body, "1 件")
	assert.Contains(t, body, "aws-key")
	assert.Contains(t, body, "a.env")
	assert.Contains(t, body, "AKIA****MPLE")
	assert.Contains(t, body, ".secretlens.baseline.json")
}

func TestFormatCheckRunText(t *testing.T) {
	assert.Contains(t, formatCheckRunText(nil), "検出されませんでした")

	text := formatCheckRunText([]finding.Finding{
		{RuleID: "gh-token", Severity: finding.SeverityHigh, File: "ci.yml", Line: 5, Match: "ghp_****abcd"},
	})
	assert.Contains(t, text, "gh-token")
	assert.Contains(t, text, "ci.yml:5")
}

func TestHasHigherThan(t *testing.T) {
	low := []finding.Finding{{Severity: finding.SeverityLow}}
	high := []finding.Finding{{Severity: finding.SeverityLow}, {Severity: finding.SeverityHigh}}

	assert.False(t, hasHigherThan(nil, finding.SeverityLow))
	assert.False(t, hasHigherThan(low, finding.SeverityLow))
	assert.True(t, hasHigherThan(high, finding.SeverityLow))
	assert.False(t, hasHigherThan(high, finding.SeverityHigh))
	assert.False(t, hasHigherThan(high, finding.SeverityCritical))
}

func TestSeverityIcon(t *testing.T) {
	assert.Equal(t, "🔴", severityIcon(finding.SeverityCritical))
	assert.Equal(t, "🟠", severityIcon(finding.SeverityHigh))
	assert.Equal(t, "🟡", severityIcon(finding.SeverityMedium))
	assert.Equal(t, "🔵", severityIcon(finding.SeverityLow))
}

func TestNew(t *testing.T) {
	r, err := New("", "owner", "repo")
	assert.NoError(t, err)
	assert.Equal(t, "owner", r.Owner)
	assert.Equal(t, "repo", r.Repo)
	assert.NotNil(t, r.client)
}

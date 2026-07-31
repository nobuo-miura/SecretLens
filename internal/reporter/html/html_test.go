package html

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nobuo-miura/SecretLens/internal/finding"
)

func sampleFindings() []finding.Finding {
	return []finding.Finding{
		{ID: "SL-0001", RuleID: "low-rule", Severity: finding.SeverityLow, File: "a.txt", Line: 1},
		{ID: "SL-0002", RuleID: "crit-rule", Severity: finding.SeverityCritical, File: "b.env", Line: 2},
		{ID: "SL-0003", RuleID: "med-rule", Severity: finding.SeverityMedium, File: "c.yml", Line: 3},
		{ID: "SL-0004", RuleID: "high-rule", Severity: finding.SeverityHigh, File: "d.cfg", Line: 4},
	}
}

func TestBuildTemplateData(t *testing.T) {
	data := buildTemplateData(sampleFindings(), "owner/repo")

	assert.Equal(t, "owner/repo", data.RepoName)
	assert.Equal(t, 4, data.Summary.Total)
	assert.Equal(t, 1, data.Summary.Critical)
	assert.Equal(t, 1, data.Summary.High)
	assert.Equal(t, 1, data.Summary.Medium)
	assert.Equal(t, 1, data.Summary.Low)

	// Severity降順にソートされること
	require.Len(t, data.Findings, 4)
	assert.Equal(t, "CRITICAL", data.Findings[0].Severity)
	assert.Equal(t, "HIGH", data.Findings[1].Severity)
	assert.Equal(t, "MEDIUM", data.Findings[2].Severity)
	assert.Equal(t, "LOW", data.Findings[3].Severity)
}

func TestWrite(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, sampleFindings(), "owner/repo"))

	out := buf.String()
	assert.True(t, strings.HasPrefix(out, "<!DOCTYPE html>"))
	assert.Contains(t, out, "owner/repo")
	assert.Contains(t, out, "crit-rule")
}

func TestWrite_EscapesRepoName(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, nil, `<script>alert("xss")</script>`))

	// リポジトリ名はHTMLエスケープされ、生の<script>タグとして出力されないこと
	assert.NotContains(t, buf.String(), `<script>alert("xss")</script>`)
}

func TestSeverityColor(t *testing.T) {
	assert.Equal(t, "#dc2626", severityColor("CRITICAL"))
	assert.Equal(t, "#ea580c", severityColor("HIGH"))
	assert.Equal(t, "#d97706", severityColor("MEDIUM"))
	assert.Equal(t, "#2563eb", severityColor("LOW"))
	assert.Equal(t, "#2563eb", severityColor("UNKNOWN"))
}

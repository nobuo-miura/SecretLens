package sarif

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nobuo-miura/SecretLens/internal/finding"
)

func TestWrite(t *testing.T) {
	findings := []finding.Finding{
		{RuleID: "aws-key", Severity: finding.SeverityCritical, Score: 80, File: "config.env", Line: 3},
		{RuleID: "gh-token", Severity: finding.SeverityMedium, Score: 25, File: "ci.yml", Line: 10},
	}

	var buf bytes.Buffer
	require.NoError(t, Write(&buf, findings))

	// 出力が有効なSARIF JSONであること
	var log Log
	require.NoError(t, json.Unmarshal(buf.Bytes(), &log))
	assert.Equal(t, "2.1.0", log.Version)
	require.Len(t, log.Runs, 1)
	assert.Equal(t, "SecretLens", log.Runs[0].Tool.Driver.Name)

	results := log.Runs[0].Results
	require.Len(t, results, 2)
	assert.Equal(t, "aws-key", results[0].RuleID)
	assert.Equal(t, "error", results[0].Level)
	assert.Equal(t, "config.env", results[0].Locations[0].PhysicalLocation.ArtifactLocation.URI)
	assert.Equal(t, 3, results[0].Locations[0].PhysicalLocation.Region.StartLine)
	assert.Equal(t, "warning", results[1].Level)
}

func TestWrite_Empty(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, nil))

	var log Log
	require.NoError(t, json.Unmarshal(buf.Bytes(), &log))
	require.Len(t, log.Runs, 1)
	// findingsゼロ件でも results は null ではなく空配列になる
	assert.NotNil(t, log.Runs[0].Results)
	assert.Empty(t, log.Runs[0].Results)
}

func TestSeverityToLevel(t *testing.T) {
	assert.Equal(t, "error", severityToLevel(finding.SeverityCritical))
	assert.Equal(t, "error", severityToLevel(finding.SeverityHigh))
	assert.Equal(t, "warning", severityToLevel(finding.SeverityMedium))
	assert.Equal(t, "note", severityToLevel(finding.SeverityLow))
}

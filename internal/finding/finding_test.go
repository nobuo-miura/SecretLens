package finding

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScoreToSeverity(t *testing.T) {
	assert.Equal(t, SeverityCritical, ScoreToSeverity(60))
	assert.Equal(t, SeverityCritical, ScoreToSeverity(80))
	assert.Equal(t, SeverityHigh, ScoreToSeverity(40))
	assert.Equal(t, SeverityHigh, ScoreToSeverity(59))
	assert.Equal(t, SeverityMedium, ScoreToSeverity(20))
	assert.Equal(t, SeverityMedium, ScoreToSeverity(39))
	assert.Equal(t, SeverityLow, ScoreToSeverity(0))
	assert.Equal(t, SeverityLow, ScoreToSeverity(19))
}

func TestMaskSecret(t *testing.T) {
	assert.Equal(t, "****", MaskSecret("abc"))
	assert.Equal(t, "abcd****wxyz", MaskSecret("abcdefghijklmnopwxyz"))
}

func TestComputeFingerprint(t *testing.T) {
	fp1 := ComputeFingerprint("rule1", "file.go", "secret")
	fp2 := ComputeFingerprint("rule1", "file.go", "secret")
	fp3 := ComputeFingerprint("rule2", "file.go", "secret")
	assert.Equal(t, fp1, fp2)
	assert.NotEqual(t, fp1, fp3)
}

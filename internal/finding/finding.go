package finding

import (
	"crypto/sha256"
	"fmt"
)

type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
)

type Finding struct {
	ID          string
	RuleID      string
	Severity    Severity
	Score       int
	Source      string // "git" | "cilog" | "envfile" | "docker" | "fs"
	File        string
	Line        int
	Match       string // マスク済み
	Commit      string
	Verified    bool
	Fingerprint string // SHA256(RuleID+File+Line+Match)
}

// スコアリングパラメータ
const (
	ScoreVerified      = 50
	ScoreHighEntropy   = 20
	ScoreSensitiveFile = 15
	ScoreMainBranch    = 10
	ScoreTestCode      = -30
	ScoreComment       = -20
)

// ScoreToSeverity はスコアをSeverityに変換する
func ScoreToSeverity(score int) Severity {
	switch {
	case score >= 60:
		return SeverityCritical
	case score >= 40:
		return SeverityHigh
	case score >= 20:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

// ComputeFingerprint はFindingのフィンガープリントを計算する
func ComputeFingerprint(ruleID, file string, line int, match string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d:%s", ruleID, file, line, match)))
	return fmt.Sprintf("%x", h)
}

// MaskSecret はシークレット値を部分マスクする
func MaskSecret(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

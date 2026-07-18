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
	Fingerprint string // SHA256(RuleID+File+Match)

	// Secret は検出したraw値。Live検証専用で、検証後は必ずクリアされ、
	// いかなる出力にも含めてはならない
	Secret string `json:"-"`
	// VerifyType はルールYAMLの verify.type（Live検証先の明示指定）
	VerifyType string `json:"-"`
}

// スコアリングパラメータ（コメント行はスコア減点ではなくスキャン対象外）
const (
	ScoreVerified      = 50
	ScoreHighEntropy   = 20
	ScoreSensitiveFile = 15
	ScoreMainBranch    = 10
	ScoreTestCode      = -30
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

// ComputeFingerprint はFindingのフィンガープリントを計算する。
// 行番号は含めない（前の行の増減だけでbaselineが無効化されるのを防ぐ）
func ComputeFingerprint(ruleID, file, match string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", ruleID, file, match)))
	return fmt.Sprintf("%x", h)
}

// MaskSecret はシークレット値を部分マスクする
func MaskSecret(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

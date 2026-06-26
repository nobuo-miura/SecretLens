package sarif

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/nobuo-miura/SecretLens/internal/finding"
)

// SARIF v2.1.0 最小構造体

type Location struct {
	PhysicalLocation PhysicalLocation `json:"physicalLocation"`
}

type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Region           Region           `json:"region"`
}

type ArtifactLocation struct {
	URI string `json:"uri"`
}

type Region struct {
	StartLine int `json:"startLine"`
}

type Message struct {
	Text string `json:"text"`
}

type Result struct {
	RuleID    string     `json:"ruleId"`
	Level     string     `json:"level"`
	Message   Message    `json:"message"`
	Locations []Location `json:"locations"`
}

type Tool struct {
	Driver Driver `json:"driver"`
}

type Driver struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	InformationURI string `json:"informationUri"`
}

type Run struct {
	Tool    Tool     `json:"tool"`
	Results []Result `json:"results"`
}

type Log struct {
	Version string `json:"version"`
	Schema  string `json:"$schema"`
	Runs    []Run  `json:"runs"`
}

func severityToLevel(s finding.Severity) string {
	switch s {
	case finding.SeverityCritical, finding.SeverityHigh:
		return "error"
	case finding.SeverityMedium:
		return "warning"
	default:
		return "note"
	}
}

// Write はfindings をSARIF v2.1.0形式でwに出力する
func Write(w io.Writer, findings []finding.Finding) error {
	results := make([]Result, 0, len(findings))
	for _, f := range findings {
		results = append(results, Result{
			RuleID: f.RuleID,
			Level:  severityToLevel(f.Severity),
			Message: Message{
				Text: fmt.Sprintf("[%s] %s (score: %d)", f.Severity, f.RuleID, f.Score),
			},
			Locations: []Location{
				{
					PhysicalLocation: PhysicalLocation{
						ArtifactLocation: ArtifactLocation{URI: f.File},
						Region:           Region{StartLine: f.Line},
					},
				},
			},
		})
	}

	log := Log{
		Version: "2.1.0",
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Runs: []Run{
			{
				Tool: Tool{
					Driver: Driver{
						Name:           "SecretLens",
						Version:        "0.1.0",
						InformationURI: "https://github.com/nobuo-miura/SecretLens",
					},
				},
				Results: results,
			},
		},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(log)
}

package html

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"sort"
	"time"

	"github.com/nobuo-miura/SecretLens/internal/finding"
)

//nolint:unused

// Write はfindings をインタラクティブHTMLレポートとしてwに出力する
func Write(w io.Writer, findings []finding.Finding, repoName string) error {
	data := buildTemplateData(findings, repoName)
	jsonBytes, err := json.Marshal(data.Findings)
	if err != nil {
		return fmt.Errorf("JSONシリアライズ失敗: %w", err)
	}

	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"severityColor": severityColor,
	}).Parse(reportTemplate)
	if err != nil {
		return fmt.Errorf("テンプレートパース失敗: %w", err)
	}

	return tmpl.Execute(w, map[string]interface{}{
		"RepoName":     data.RepoName,
		"GeneratedAt":  data.GeneratedAt,
		"Summary":      data.Summary,
		"FindingsJSON": template.JS(jsonBytes), //nolint:gosec
	})
}

type templateData struct {
	RepoName    string
	GeneratedAt string
	Summary     summary
	Findings    []findingJSON
}

type summary struct {
	Total    int
	Critical int
	High     int
	Medium   int
	Low      int
}

type findingJSON struct {
	ID          string `json:"id"`
	RuleID      string `json:"ruleId"`
	Severity    string `json:"severity"`
	Score       int    `json:"score"`
	Source      string `json:"source"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Match       string `json:"match"`
	Commit      string `json:"commit"`
	Fingerprint string `json:"fingerprint"`
}

func buildTemplateData(findings []finding.Finding, repoName string) templateData {
	// Severity降順でソート
	sorted := make([]finding.Finding, len(findings))
	copy(sorted, findings)
	order := map[finding.Severity]int{
		finding.SeverityCritical: 3,
		finding.SeverityHigh:     2,
		finding.SeverityMedium:   1,
		finding.SeverityLow:      0,
	}
	sort.Slice(sorted, func(i, j int) bool {
		return order[sorted[i].Severity] > order[sorted[j].Severity]
	})

	s := summary{Total: len(findings)}
	fjs := make([]findingJSON, 0, len(sorted))
	for _, f := range sorted {
		switch f.Severity {
		case finding.SeverityCritical:
			s.Critical++
		case finding.SeverityHigh:
			s.High++
		case finding.SeverityMedium:
			s.Medium++
		case finding.SeverityLow:
			s.Low++
		}
		fjs = append(fjs, findingJSON{
			ID: f.ID, RuleID: f.RuleID, Severity: string(f.Severity),
			Score: f.Score, Source: f.Source, File: f.File,
			Line: f.Line, Match: f.Match, Commit: f.Commit,
			Fingerprint: f.Fingerprint,
		})
	}

	return templateData{
		RepoName:    repoName,
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05 MST"),
		Summary:     s,
		Findings:    fjs,
	}
}

func severityColor(s string) string {
	switch s {
	case "CRITICAL":
		return "#dc2626"
	case "HIGH":
		return "#ea580c"
	case "MEDIUM":
		return "#d97706"
	default:
		return "#2563eb"
	}
}

const reportTemplate = `<!DOCTYPE html>
<html lang="ja">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>SecretLens Report — {{.RepoName}}</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #0f172a; color: #e2e8f0; }
  header { background: #1e293b; border-bottom: 1px solid #334155; padding: 1.5rem 2rem; }
  header h1 { font-size: 1.5rem; font-weight: 700; color: #f1f5f9; }
  header p { color: #94a3b8; font-size: 0.875rem; margin-top: 0.25rem; }
  .summary { display: flex; gap: 1rem; padding: 1.5rem 2rem; flex-wrap: wrap; }
  .stat { background: #1e293b; border: 1px solid #334155; border-radius: 0.5rem; padding: 1rem 1.5rem; text-align: center; min-width: 120px; }
  .stat .num { font-size: 2rem; font-weight: 700; }
  .stat .label { font-size: 0.75rem; color: #94a3b8; margin-top: 0.25rem; }
  .stat.critical .num { color: #dc2626; }
  .stat.high .num { color: #ea580c; }
  .stat.medium .num { color: #d97706; }
  .stat.low .num { color: #2563eb; }
  .filters { padding: 0 2rem 1rem; display: flex; gap: 0.75rem; flex-wrap: wrap; }
  .filter-btn { padding: 0.375rem 0.875rem; border-radius: 9999px; border: 1px solid #475569; background: transparent; color: #94a3b8; cursor: pointer; font-size: 0.875rem; transition: all 0.15s; }
  .filter-btn.active, .filter-btn:hover { background: #334155; color: #f1f5f9; border-color: #64748b; }
  .search { margin-left: auto; }
  .search input { background: #1e293b; border: 1px solid #334155; color: #e2e8f0; padding: 0.375rem 0.75rem; border-radius: 0.375rem; font-size: 0.875rem; width: 250px; }
  .findings { padding: 0 2rem 2rem; }
  table { width: 100%; border-collapse: collapse; background: #1e293b; border-radius: 0.5rem; overflow: hidden; }
  thead th { background: #0f172a; padding: 0.75rem 1rem; text-align: left; font-size: 0.75rem; font-weight: 600; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; border-bottom: 1px solid #334155; }
  tbody tr { border-bottom: 1px solid #1e293b; transition: background 0.1s; }
  tbody tr:hover { background: #243044; }
  td { padding: 0.75rem 1rem; font-size: 0.875rem; vertical-align: top; }
  .badge { display: inline-block; padding: 0.2rem 0.6rem; border-radius: 9999px; font-size: 0.75rem; font-weight: 600; }
  .badge.CRITICAL { background: #450a0a; color: #fca5a5; }
  .badge.HIGH { background: #431407; color: #fdba74; }
  .badge.MEDIUM { background: #451a03; color: #fcd34d; }
  .badge.LOW { background: #172554; color: #93c5fd; }
  .source-badge { background: #1e3a5f; color: #7dd3fc; padding: 0.2rem 0.5rem; border-radius: 0.25rem; font-size: 0.75rem; }
  code { background: #0f172a; padding: 0.15rem 0.4rem; border-radius: 0.25rem; font-family: 'JetBrains Mono', 'Fira Code', monospace; font-size: 0.8rem; color: #a5f3fc; }
  .commit { color: #6366f1; font-family: monospace; font-size: 0.8rem; }
  .score { color: #94a3b8; font-size: 0.8rem; }
  .no-results { text-align: center; padding: 3rem; color: #475569; }
  .count { color: #64748b; font-size: 0.875rem; padding: 0 2rem 0.5rem; }
</style>
</head>
<body>
<header>
  <h1>🔍 SecretLens — シークレットスキャンレポート</h1>
  <p>リポジトリ: <strong>{{.RepoName}}</strong> &nbsp;|&nbsp; 生成日時: {{.GeneratedAt}}</p>
</header>

<div class="summary">
  <div class="stat"><div class="num">{{.Summary.Total}}</div><div class="label">TOTAL</div></div>
  <div class="stat critical"><div class="num">{{.Summary.Critical}}</div><div class="label">CRITICAL</div></div>
  <div class="stat high"><div class="num">{{.Summary.High}}</div><div class="label">HIGH</div></div>
  <div class="stat medium"><div class="num">{{.Summary.Medium}}</div><div class="label">MEDIUM</div></div>
  <div class="stat low"><div class="num">{{.Summary.Low}}</div><div class="label">LOW</div></div>
</div>

<div class="filters">
  <button class="filter-btn active" onclick="filterBy('ALL')">ALL</button>
  <button class="filter-btn" onclick="filterBy('CRITICAL')">CRITICAL</button>
  <button class="filter-btn" onclick="filterBy('HIGH')">HIGH</button>
  <button class="filter-btn" onclick="filterBy('MEDIUM')">MEDIUM</button>
  <button class="filter-btn" onclick="filterBy('LOW')">LOW</button>
  <div class="search"><input type="text" id="search" placeholder="ファイル名・ルールIDで検索..." oninput="applyFilters()"></div>
</div>

<div class="count" id="count"></div>

<div class="findings">
  <table id="findings-table">
    <thead>
      <tr>
        <th>Severity</th>
        <th>Rule</th>
        <th>Source</th>
        <th>File / Line</th>
        <th>Match</th>
        <th>Commit</th>
        <th>Score</th>
      </tr>
    </thead>
    <tbody id="findings-body"></tbody>
  </table>
  <div class="no-results" id="no-results" style="display:none">検出結果がありません</div>
</div>

<script>
const ALL_FINDINGS = {{.FindingsJSON}};
let currentSeverity = 'ALL';

function filterBy(sev) {
  currentSeverity = sev;
  document.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
  event.target.classList.add('active');
  applyFilters();
}

function applyFilters() {
  const q = document.getElementById('search').value.toLowerCase();
  const filtered = ALL_FINDINGS.filter(f => {
    const sevMatch = currentSeverity === 'ALL' || f.severity === currentSeverity;
    const textMatch = !q || f.file.toLowerCase().includes(q) || f.ruleId.toLowerCase().includes(q) || f.match.toLowerCase().includes(q);
    return sevMatch && textMatch;
  });
  render(filtered);
}

function render(findings) {
  const tbody = document.getElementById('findings-body');
  const noResults = document.getElementById('no-results');
  const count = document.getElementById('count');
  count.textContent = findings.length + ' 件表示中（全 ' + ALL_FINDINGS.length + ' 件）';
  if (findings.length === 0) {
    tbody.innerHTML = '';
    noResults.style.display = 'block';
    return;
  }
  noResults.style.display = 'none';
  tbody.innerHTML = findings.map(f => ` + "`" + `
    <tr>
      <td><span class="badge ${f.severity}">${f.severity}</span></td>
      <td><code>${esc(f.ruleId)}</code></td>
      <td><span class="source-badge">${esc(f.source)}</span></td>
      <td><code>${esc(f.file)}</code><br><span style="color:#64748b;font-size:0.75rem">L${f.line}</span></td>
      <td><code>${esc(f.match)}</code></td>
      <td>${f.commit ? '<span class="commit">' + esc(f.commit.slice(0,8)) + '</span>' : '—'}</td>
      <td><span class="score">${f.score}</span></td>
    </tr>` + "`" + `).join('');
}

function esc(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

render(ALL_FINDINGS);
</script>
</body>
</html>`

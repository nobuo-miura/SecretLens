package git

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type DiffLine struct {
	Commit string
	File   string
	Line   int
	Text   string
}

// StreamOptions はgit差分ストリームの取得モードを指定する
type StreamOptions struct {
	Mode        string // "history"（デフォルト） | "worktree" | "staged"
	Since       string // 履歴スキャンの開始コミット（<since>..HEAD をスキャン）
	CommitRange string // base..head 形式のコミット範囲
}

// Stream はgitの差分出力をストリーミング解析してDiffLineを返す。
//   - history:  git log -p（--since/--commit-range指定時はその範囲、未指定時は--all）
//   - worktree: git diff HEAD（未コミットの変更すべて。ステージ済み含む）
//   - staged:   git diff --cached（ステージ済みの変更のみ。pre-commit用）
func Stream(repoPath string, opts StreamOptions, out chan<- DiffLine) error {
	args := []string{"-C", repoPath}
	switch opts.Mode {
	case "worktree":
		args = append(args, "diff", "--unified=0", "--no-color")
		if hasHEAD(repoPath) {
			args = append(args, "HEAD")
		} else if empty := emptyTreeHash(repoPath); empty != "" {
			// 初回コミット前は空ツリーと比較する。引数なしのgit diffだと
			// ステージ済みの変更が差分に現れず見落とすため
			args = append(args, empty)
		}
	case "staged":
		args = append(args, "diff", "--cached", "--unified=0", "--no-color")
	default:
		args = append(args, "log", "-p", "--unified=0", "--no-color")
		switch {
		case opts.CommitRange != "":
			args = append(args, opts.CommitRange)
		case opts.Since != "":
			args = append(args, opts.Since+"..HEAD")
		default:
			args = append(args, "--all")
		}
	}

	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("git パイプ作成失敗: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("git 起動失敗: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var currentCommit, currentFile string
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "commit "):
			currentCommit = strings.TrimPrefix(line, "commit ")
			if len(currentCommit) > 40 {
				currentCommit = currentCommit[:40]
			}
		case strings.HasPrefix(line, "+++ b/"):
			currentFile = strings.TrimPrefix(line, "+++ b/")
			lineNum = 0
		case strings.HasPrefix(line, "@@ "):
			// @@ -old +new,count @@ のnew行番号を取得
			lineNum = parseNewLineNum(line)
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			out <- DiffLine{
				Commit: currentCommit,
				File:   currentFile,
				Line:   lineNum,
				Text:   line[1:],
			}
			lineNum++
		case !strings.HasPrefix(line, "-"):
			lineNum++
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("git 終了エラー: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return scanner.Err()
}

// hasHEAD はHEADが解決できるか（=コミットが1つ以上あるか）を返す。
// コミットゼロのリポジトリでは `git diff HEAD` が失敗するため、空ツリーとの比較に落とす
func hasHEAD(repoPath string) bool {
	return exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "--quiet", "HEAD").Run() == nil
}

// emptyTreeHash は空ツリーのオブジェクトIDを返す（SHA-1/SHA-256両リポジトリ対応）
func emptyTreeHash(repoPath string) string {
	out, err := exec.Command("git", "-C", repoPath, "hash-object", "-t", "tree", "--stdin").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func parseNewLineNum(hunk string) int {
	// @@ -old +new[,count] @@
	start := strings.Index(hunk, " +")
	if start < 0 {
		return 0
	}
	rest := hunk[start+2:]
	end := strings.IndexAny(rest, ", @")
	if end < 0 {
		end = len(rest)
	}
	var n int
	_, _ = fmt.Sscanf(rest[:end], "%d", &n)
	return n
}

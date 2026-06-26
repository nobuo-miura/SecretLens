package git

import (
	"bufio"
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

// Stream はgit log --all -pの出力をストリーミング解析してDiffLineを返す
func Stream(repoPath string, out chan<- DiffLine) error {
	cmd := exec.Command("git", "-C", repoPath, "log", "--all", "-p", "--unified=0", "--no-color")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("git log パイプ作成失敗: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("git log 起動失敗: %w", err)
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
		return fmt.Errorf("git log 終了エラー: %w", err)
	}
	return scanner.Err()
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

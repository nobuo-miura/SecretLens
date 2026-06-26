package docker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

type FileLine struct {
	Image  string
	Layer  string
	File   string
	Line   int
	Text   string
}

// StreamLayers は `docker save` でイメージを展開してファイル内容を行単位で返す
func StreamLayers(image string, out chan<- FileLine) error {
	cmd := exec.Command("docker", "save", image)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("docker save パイプ作成失敗: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("docker save 起動失敗（Dockerが起動していますか？）: %w", err)
	}
	defer func() { _ = cmd.Wait() }()

	return parseTar(image, tar.NewReader(stdout), out)
}

func parseTar(image string, tr *tar.Reader, out chan<- FileLine) error {
	// 全エントリをメモリに読み込んでマニフェストを先に処理する
	type entry struct {
		header *tar.Header
		data   []byte
	}
	var entries []entry

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar読み込みエラー: %w", err)
		}
		data, err := io.ReadAll(io.LimitReader(tr, 50*1024*1024)) // 50MB上限/ファイル
		if err != nil {
			return err
		}
		entries = append(entries, entry{hdr, data})
	}

	// layer.tar / layer.tar.gz を解析
	for _, e := range entries {
		name := filepath.ToSlash(e.header.Name)
		if !strings.HasSuffix(name, "/layer.tar") && !strings.HasSuffix(name, ".tar") {
			continue
		}
		layerName := filepath.Base(filepath.Dir(name))
		if err := parseLayerTar(image, layerName, bytes.NewReader(e.data), out); err != nil {
			continue
		}
	}
	return nil
}

func parseLayerTar(image, layer string, r io.Reader, out chan<- FileLine) error {
	// gzip圧縮の場合は展開
	reader := r
	peek := make([]byte, 2)
	buf := &bytes.Buffer{}
	tee := io.TeeReader(r, buf)
	if n, _ := tee.Read(peek); n == 2 && peek[0] == 0x1f && peek[1] == 0x8b {
		gz, err := gzip.NewReader(io.MultiReader(bytes.NewReader(peek), buf, r))
		if err == nil {
			reader = gz
			defer func() { _ = gz.Close() }()
		}
	} else {
		reader = io.MultiReader(bytes.NewReader(peek), buf, r)
	}

	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if !isTextTarget(hdr.Name) {
			continue
		}
		if err := scanFileLines(image, layer, hdr.Name, tr, out); err != nil {
			continue
		}
	}
	return nil
}

func scanFileLines(image, layer, filePath string, r io.Reader, out chan<- FileLine) error {
	data, err := io.ReadAll(io.LimitReader(r, 1024*1024)) // 1MB上限/ファイル
	if err != nil {
		return err
	}
	// バイナリ判定（NULLバイトが含まれる場合はスキップ）
	if bytes.ContainsRune(data, 0) {
		return nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		out <- FileLine{
			Image: image,
			Layer: layer,
			File:  filePath,
			Line:  lineNum,
			Text:  scanner.Text(),
		}
	}
	return scanner.Err()
}

// スキャン対象ファイル拡張子・パターン
var targetPaths = []string{
	".env", ".yaml", ".yml", ".json", ".conf", ".cfg",
	".ini", ".toml", ".properties", ".tfvars", ".xml",
	"credentials", "secret", "secrets",
}

func isTextTarget(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(base))
	for _, t := range targetPaths {
		if ext == t || base == t || strings.Contains(base, t) {
			return true
		}
	}
	return false
}


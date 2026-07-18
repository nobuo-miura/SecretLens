package docker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

type FileLine struct {
	Image string
	Layer string
	File  string
	Line  int
	Text  string
}

// maxFileSize はレイヤー内の1ファイルあたりのスキャン上限。超過ファイルは
// 途中切り捨てだと壊れた行を誤検出し得るため、まるごとスキップする
const maxFileSize = 1024 * 1024

// StreamLayers は `docker save` の出力をストリーミング解析してファイル内容を行単位で返す
func StreamLayers(image string, out chan<- FileLine) error {
	cmd := exec.Command("docker", "save", image)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("docker save パイプ作成失敗: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("docker save 起動失敗（Dockerが起動していますか？）: %w", err)
	}

	parseErr := parseTar(image, tar.NewReader(stdout), out)
	// 残りを読み捨ててからWaitしないとdocker saveがパイプ書き込みでブロックする
	_, _ = io.Copy(io.Discard, stdout)
	if err := cmd.Wait(); err != nil {
		return errors.Join(fmt.Errorf("docker save 失敗: %w: %s", err, strings.TrimSpace(stderr.String())), parseErr)
	}
	return parseErr
}

// parseTar はdocker saveのtarをストリーミングで走査し、レイヤーtarを順次解析する。
// legacy形式（<id>/layer.tar）とOCI形式（blobs/sha256/<digest>）の両方に対応するため、
// エントリ名ではなく先頭バイトのマジックナンバーでレイヤーtarを判別する
func parseTar(image string, tr *tar.Reader, out chan<- FileLine) error {
	var layerErrs []error
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			layerErrs = append(layerErrs, fmt.Errorf("tar読み込みエラー: %w", err))
			break
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		br := bufio.NewReader(tr)
		gzipped, isTar := detectLayerFormat(br)
		if !isTar {
			continue // manifest.json / config json など
		}
		if err := parseLayerTar(image, layerDisplayName(hdr.Name), br, gzipped, out); err != nil {
			layerErrs = append(layerErrs, fmt.Errorf("レイヤー %s 解析失敗: %w", hdr.Name, err))
		}
	}
	return errors.Join(layerErrs...)
}

// detectLayerFormat は先頭バイトからgzip/tarを判別する。読み込み位置は進めない
func detectLayerFormat(br *bufio.Reader) (gzipped, isTar bool) {
	magic, err := br.Peek(2)
	if err == nil && magic[0] == 0x1f && magic[1] == 0x8b {
		return true, true // gzipはレイヤーtar.gzとみなす（中身は展開後にtarとして検証される）
	}
	// tarマジック: オフセット257から "ustar"
	head, err := br.Peek(262)
	if err != nil {
		return false, false
	}
	return false, bytes.Equal(head[257:262], []byte("ustar"))
}

func layerDisplayName(entryName string) string {
	name := filepath.ToSlash(entryName)
	if strings.HasSuffix(name, "/layer.tar") {
		return filepath.Base(filepath.Dir(name)) // legacy: <id>/layer.tar
	}
	return filepath.Base(name) // OCI: blobs/sha256/<digest>
}

func parseLayerTar(image, layer string, r io.Reader, gzipped bool, out chan<- FileLine) error {
	if gzipped {
		gz, err := gzip.NewReader(r)
		if err != nil {
			return fmt.Errorf("gzip展開失敗: %w", err)
		}
		defer func() { _ = gz.Close() }()
		r = gz
	}

	var fileErrs []error
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fileErrs = append(fileErrs, fmt.Errorf("レイヤーtar読み込みエラー: %w", err))
			break
		}
		if hdr.Typeflag != tar.TypeReg || !isTextTarget(hdr.Name) {
			continue
		}
		if hdr.Size > maxFileSize {
			continue
		}
		if err := scanFileLines(image, layer, hdr.Name, tr, out); err != nil {
			fileErrs = append(fileErrs, fmt.Errorf("%s スキャン失敗: %w", hdr.Name, err))
		}
	}
	return errors.Join(fileErrs...)
}

func scanFileLines(image, layer, filePath string, r io.Reader, out chan<- FileLine) error {
	data, err := io.ReadAll(r)
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

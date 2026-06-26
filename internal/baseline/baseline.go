package baseline

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

const DefaultFile = ".secretlens.baseline.json"

type Baseline struct {
	Fingerprints map[string]bool `json:"fingerprints"`
	path         string
}

// Load はベースラインファイルを読み込む。ファイルが存在しない場合は空のベースラインを返す
func Load(path string) (*Baseline, error) {
	b := &Baseline{
		Fingerprints: make(map[string]bool),
		path:         path,
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return b, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ベースライン読み込み失敗: %w", err)
	}
	if err := json.Unmarshal(data, b); err != nil {
		return nil, fmt.Errorf("ベースラインパース失敗: %w", err)
	}
	b.path = path
	return b, nil
}

// Contains はフィンガープリントがベースラインに含まれているかを返す
func (b *Baseline) Contains(fingerprint string) bool {
	return b.Fingerprints[fingerprint]
}

// Add はフィンガープリントをベースラインに追加する
func (b *Baseline) Add(fingerprint string) {
	b.Fingerprints[fingerprint] = true
}

// Save はベースラインをファイルに保存する
func (b *Baseline) Save() error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("ベースライン直列化失敗: %w", err)
	}
	return os.WriteFile(b.path, data, 0o600)
}

// List は登録済みフィンガープリント一覧を返す
func (b *Baseline) List() []string {
	keys := make([]string, 0, len(b.Fingerprints))
	for k := range b.Fingerprints {
		keys = append(keys, k)
	}
	return keys
}

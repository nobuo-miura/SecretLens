// Package rules は標準ルールYAMLをバイナリに内蔵する
package rules

import "embed"

//go:embed *.yaml
var FS embed.FS

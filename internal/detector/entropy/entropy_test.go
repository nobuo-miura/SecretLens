package entropy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShannon(t *testing.T) {
	assert.Equal(t, 0.0, Shannon(""))
	assert.Equal(t, 0.0, Shannon("aaaa"))
	assert.InDelta(t, 1.0, Shannon("aabb"), 0.01)

	// ランダムな文字列は高エントロピー
	high := Shannon("aB3$xKp9mNqR2vLz")
	assert.Greater(t, high, 3.0)
}

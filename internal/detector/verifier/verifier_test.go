package verifier

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerify_Dispatch(t *testing.T) {
	ctx := context.Background()

	t.Run("aws はペアが必要なため単体では検証不可", func(t *testing.T) {
		r := Verify(ctx, "aws", "AKIAIOSFODNN7EXAMPLE")
		assert.False(t, r.Valid)
		assert.Contains(t, r.Message, "両方が必要")
	})

	t.Run("空typeは未対応メッセージ", func(t *testing.T) {
		r := Verify(ctx, "", "value")
		assert.False(t, r.Valid)
		assert.Contains(t, r.Message, "対応していません")
	})

	t.Run("未知のtypeはエラーメッセージ", func(t *testing.T) {
		r := Verify(ctx, "unknown-service", "value")
		assert.False(t, r.Valid)
		assert.Contains(t, r.Message, "unknown-service")
	})
}

func TestSha256Hex(t *testing.T) {
	// SHA256("") の既知ベクトル
	assert.Equal(t,
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		sha256Hex(""))
	assert.Equal(t,
		"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		sha256Hex("hello"))
}

func TestHmacSHA256(t *testing.T) {
	// RFC 4231 Test Case 2: key="Jefe", data="what do ya want for nothing?"
	mac := hmacSHA256([]byte("Jefe"), "what do ya want for nothing?")
	assert.Equal(t,
		"5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843",
		hex.EncodeToString(mac))
}

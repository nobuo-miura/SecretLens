package verifier

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Result はLive検証結果
type Result struct {
	Valid   bool
	Message string
}

// VerifyAWSAccessKey はAWSアクセスキー+シークレットキーをSTS GetCallerIdentityで検証する
func VerifyAWSAccessKey(ctx context.Context, accessKeyID, secretAccessKey string) Result {
	// AWS Signature Version 4でSTS GetCallerIdentityを呼び出す
	const service = "sts"
	const region = "us-east-1"
	const host = "sts.amazonaws.com"
	const endpoint = "https://sts.amazonaws.com/"
	const body = "Action=GetCallerIdentity&Version=2011-06-15"

	now := time.Now().UTC()
	dateShort := now.Format("20060102")
	dateISO := now.Format("20060102T150405Z")

	// Canonical request
	canonicalHeaders := fmt.Sprintf("content-type:application/x-www-form-urlencoded\nhost:%s\nx-amz-date:%s\n", host, dateISO)
	signedHeaders := "content-type;host;x-amz-date"
	bodyHash := sha256Hex(body)
	canonicalRequest := strings.Join([]string{
		"POST", "/", "",
		canonicalHeaders, signedHeaders, bodyHash,
	}, "\n")

	// String to sign
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateShort, region, service)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s", dateISO, credentialScope, sha256Hex(canonicalRequest))

	// Signing key
	signingKey := hmacSHA256(
		hmacSHA256(
			hmacSHA256(
				hmacSHA256([]byte("AWS4"+secretAccessKey), dateShort),
				region),
			service),
		"aws4_request")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	authHeader := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKeyID, credentialScope, signedHeaders, signature,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return Result{Message: fmt.Sprintf("リクエスト作成失敗: %v", err)}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Host", host)
	req.Header.Set("X-Amz-Date", dateISO)
	req.Header.Set("Authorization", authHeader)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Result{Message: fmt.Sprintf("AWS STS接続失敗: %v", err)}
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusOK {
		return Result{Valid: true, Message: "AWS認証情報が有効です"}
	}
	return Result{Message: fmt.Sprintf("AWS認証情報が無効です (HTTP %d)", resp.StatusCode)}
}

// VerifyGitHubToken はGitHubトークンをUsers APIで検証する
func VerifyGitHubToken(ctx context.Context, token string) Result {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return Result{Message: fmt.Sprintf("リクエスト作成失敗: %v", err)}
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Result{Message: fmt.Sprintf("GitHub API接続失敗: %v", err)}
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusOK {
		return Result{Valid: true, Message: "GitHubトークンが有効です"}
	}
	return Result{Message: fmt.Sprintf("GitHubトークンが無効です (HTTP %d)", resp.StatusCode)}
}

// Verify はルールYAMLで明示されたverify.typeとraw値を受け取り、対応するLive検証を実行する。
// 検証先はルール側の宣言のみで決まり、ルールIDからの推測は行わない
func Verify(ctx context.Context, verifyType, secret string) Result {
	switch verifyType {
	case "aws":
		// AWS検証はアクセスキーIDとシークレットキーのペアが必要で、単一マッチでは実行できない
		return Result{Message: "AWS検証はアクセスキーIDとシークレットキーの両方が必要です"}
	case "github-token":
		return VerifyGitHubToken(ctx, secret)
	case "":
		return Result{Message: "このルールはLive検証に対応していません"}
	default:
		return Result{Message: fmt.Sprintf("未対応のverify.typeです: %s", verifyType)}
	}
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

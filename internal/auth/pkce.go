package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// verifierEntropyBytes は code_verifier の元となる乱数のバイト数。
// 32 バイトを base64url で符号化すると 43 文字となり、RFC 7636 の下限を満たす。
const verifierEntropyBytes = 32

// newCodeVerifier は PKCE の code_verifier を生成する。
func newCodeVerifier() (string, error) {
	raw := make([]byte, verifierEntropyBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("auth: code_verifier の生成に失敗: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// codeChallenge は code_verifier から S256 方式の code_challenge を導出する。
func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// Package session は BFF が発行するセッションの封緘・開封を扱う。
//
// セッションの内容は AES-256-GCM で暗号化してクッキーに封入する（封緘クッキー方式）。
// サーバー側のセッションストアは持たない。Supabase のアクセストークン /
// リフレッシュトークンを内包するため、署名のみでは中身がブラウザから読めてしまう。
// 暗号化することで、状態をクッキーに載せたままブラウザには不透明に保つ。
//
// 詳細と трейド-オフは docs/adr/0002-session-strategy.md を参照。
package session

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// KeySize は封緘に用いる鍵のバイト数。AES-256 を使うため 32 バイト固定。
const KeySize = 32

var (
	// ErrInvalidKeySize は鍵長が KeySize と異なる場合に返る。
	ErrInvalidKeySize = errors.New("session: 鍵は 32 バイトである必要がある")
	// ErrInvalidToken は封緘トークンが不正・改竄・別の鍵で封緘されている場合に返る。
	// 原因を区別せず単一のエラーとするのは、呼び出し側に区別する必要がなく、
	// 詳細を返すこと自体が攻撃者への手がかりになるため。
	ErrInvalidToken = errors.New("session: 封緘トークンが不正")
)

// Session は BFF がフロントエンドへ発行するセッションの中身。
type Session struct {
	// UserID は Supabase 上のユーザー識別子。
	UserID string `json:"user_id"`
	// Email はユーザーのメールアドレス。
	Email string `json:"email"`
	// AccessToken は Supabase のアクセストークン。BFF が外部 API を呼ぶ際に用いる。
	AccessToken string `json:"access_token"`
	// RefreshToken は Supabase のリフレッシュトークン。フロントエンドへは渡さない。
	RefreshToken string `json:"refresh_token"`
	// ExpiresAt は AccessToken の有効期限。
	ExpiresAt time.Time `json:"expires_at"`
	// IssuedAt はこのセッションを発行した時刻。
	IssuedAt time.Time `json:"issued_at"`
}

// AccessTokenExpired は now 時点でアクセストークンが失効しているかを返す。
// 期限ちょうどの場合も失効として扱う（直後の外部 API 呼び出しが失敗するのを避けるため）。
func (s Session) AccessTokenExpired(now time.Time) bool {
	return !now.Before(s.ExpiresAt)
}

// Sealer はセッションの封緘・開封を行う。
type Sealer struct {
	aead cipher.AEAD
}

// NewSealer は 32 バイトの鍵から Sealer を生成する。
func NewSealer(key []byte) (*Sealer, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("%w: %d バイトが渡された", ErrInvalidKeySize, len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("session: AES 暗号の生成に失敗: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("session: GCM の生成に失敗: %w", err)
	}

	return &Sealer{aead: aead}, nil
}

// Seal はセッションを封緘し、クッキーへ格納可能な文字列を返す。
// 出力は base64url(nonce || 暗号文) の形式。
func (s *Sealer) Seal(sess Session) (string, error) {
	plaintext, err := json.Marshal(sess)
	if err != nil {
		return "", fmt.Errorf("session: セッションの符号化に失敗: %w", err)
	}

	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("session: nonce の生成に失敗: %w", err)
	}

	sealed := s.aead.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Open は封緘トークンを開封してセッションを復元する。
// 改竄されている場合や別の鍵で封緘された場合は ErrInvalidToken を返す。
func (s *Sealer) Open(token string) (Session, error) {
	sealed, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return Session{}, ErrInvalidToken
	}

	nonceSize := s.aead.NonceSize()
	if len(sealed) < nonceSize {
		return Session{}, ErrInvalidToken
	}

	nonce, ciphertext := sealed[:nonceSize], sealed[nonceSize:]
	plaintext, err := s.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return Session{}, ErrInvalidToken
	}

	var sess Session
	if err := json.Unmarshal(plaintext, &sess); err != nil {
		return Session{}, ErrInvalidToken
	}

	return sess, nil
}

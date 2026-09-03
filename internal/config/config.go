// Package config はアプリケーションの設定読み込みを扱う。
package config

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/spf13/viper"

	"github.com/charlie-dev-ios/ganana-bff/internal/session"
)

// Config はアプリケーションの設定値。
type Config struct {
	// Port は HTTP サーバの待ち受けポート。
	Port string

	// SupabaseURL は Supabase プロジェクトの URL（例: https://xxxx.supabase.co）。
	SupabaseURL string
	// SupabaseAnonKey は Supabase の publishable（anon）キー。
	SupabaseAnonKey string

	// SessionKey はセッションの封緘に用いる 32 バイトの鍵。
	SessionKey []byte

	// AuthCallbackURL は Supabase が認可後にブラウザを戻す先。
	// この BFF の /auth/callback を指す公開 URL であり、
	// Supabase プロジェクトの Redirect URL 許可リストに登録する必要がある。
	AuthCallbackURL string
	// PostLoginRedirect はログイン完了後にブラウザを送る先（web フロントエンドの URL）。
	PostLoginRedirect string

	// CookieDomain はセッションクッキーの Domain 属性。空の場合はホスト限定クッキーになる。
	CookieDomain string
	// CookieSecure はセッションクッキーに Secure 属性を付けるか。既定は true。
	CookieSecure bool

	// AllowedOrigins は CORS で許可するオリジン。空の場合は CORS を無効にする。
	AllowedOrigins []string
}

// Load は環境変数（GANANA_ プレフィックス）から設定を読み込む。
// 必須項目が欠けている場合や値が不正な場合はエラーを返す。
func Load() (Config, error) {
	v := viper.New()
	v.SetEnvPrefix("GANANA")
	v.AutomaticEnv()
	v.SetDefault("port", "8080")
	v.SetDefault("cookie_secure", true)

	cfg := Config{
		Port:              v.GetString("port"),
		SupabaseURL:       v.GetString("supabase_url"),
		SupabaseAnonKey:   v.GetString("supabase_anon_key"),
		AuthCallbackURL:   v.GetString("auth_callback_url"),
		PostLoginRedirect: v.GetString("post_login_redirect"),
		CookieDomain:      v.GetString("cookie_domain"),
		CookieSecure:      v.GetBool("cookie_secure"),
		AllowedOrigins:    splitAndTrim(v.GetString("allowed_origins")),
	}

	var problems []string

	for _, required := range []struct {
		env   string
		value string
	}{
		{"GANANA_SUPABASE_URL", cfg.SupabaseURL},
		{"GANANA_SUPABASE_ANON_KEY", cfg.SupabaseAnonKey},
		{"GANANA_AUTH_CALLBACK_URL", cfg.AuthCallbackURL},
		{"GANANA_POST_LOGIN_REDIRECT", cfg.PostLoginRedirect},
	} {
		if required.value == "" {
			problems = append(problems, required.env+" が未設定")
		}
	}

	sessionKey, problem := decodeSessionKey(v.GetString("session_key"))
	if problem != "" {
		problems = append(problems, problem)
	}
	cfg.SessionKey = sessionKey

	if len(problems) > 0 {
		return Config{}, fmt.Errorf("config: 設定が不正です: %s", strings.Join(problems, " / "))
	}

	return cfg, nil
}

// decodeSessionKey は base64 の鍵を復号し、問題があればその説明を返す。
func decodeSessionKey(raw string) ([]byte, string) {
	if raw == "" {
		return nil, "GANANA_SESSION_KEY が未設定（`openssl rand -base64 32` で生成する）"
	}

	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, "GANANA_SESSION_KEY が base64 として解釈できない"
	}

	if len(key) != session.KeySize {
		return nil, fmt.Sprintf("GANANA_SESSION_KEY は %d バイトである必要がある（%d バイトが指定された）", session.KeySize, len(key))
	}

	return key, ""
}

// splitAndTrim はカンマ区切りの文字列を分割し、前後の空白を除いて返す。
// 空文字の場合は nil を返す。
func splitAndTrim(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

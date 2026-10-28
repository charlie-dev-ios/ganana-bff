package config

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validSessionKey は 32 バイト鍵の base64 表現。
var validSessionKey = base64.StdEncoding.EncodeToString(make([]byte, 32))

// setRequiredEnv は必須の環境変数を設定する。
func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GANANA_SUPABASE_URL", "https://proj.supabase.co")
	t.Setenv("GANANA_SUPABASE_ANON_KEY", "anon-key")
	t.Setenv("GANANA_SESSION_KEY", validSessionKey)
	t.Setenv("GANANA_AUTH_CALLBACK_URL", "https://api.ganana.jp/auth/callback")
	t.Setenv("GANANA_POST_LOGIN_REDIRECT", "https://app.ganana.jp/")
}

func TestLoad_Defaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "8080", cfg.Port)
	assert.True(t, cfg.CookieSecure, "既定では Secure 属性を付ける")
	assert.Empty(t, cfg.CookieDomain)
	assert.Empty(t, cfg.AllowedOrigins)
	assert.Len(t, cfg.SessionKey, 32)
}

func TestLoad_EnvOverride(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GANANA_PORT", "9090")
	t.Setenv("GANANA_COOKIE_DOMAIN", ".ganana.jp")
	t.Setenv("GANANA_COOKIE_SECURE", "false")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, ".ganana.jp", cfg.CookieDomain)
	assert.False(t, cfg.CookieSecure)
}

func TestLoad_AllowedOriginsIsCommaSeparated(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GANANA_ALLOWED_ORIGINS", "https://app.ganana.jp, https://stg.ganana.jp")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, []string{"https://app.ganana.jp", "https://stg.ganana.jp"}, cfg.AllowedOrigins)
}

func TestLoad_RequiresSupabaseSettings(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GANANA_SUPABASE_URL", "")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GANANA_SUPABASE_URL")
}

func TestLoad_ReportsAllMissingSettingsAtOnce(t *testing.T) {
	t.Setenv("GANANA_SUPABASE_URL", "")
	t.Setenv("GANANA_SUPABASE_ANON_KEY", "")
	t.Setenv("GANANA_SESSION_KEY", "")
	t.Setenv("GANANA_AUTH_CALLBACK_URL", "")
	t.Setenv("GANANA_POST_LOGIN_REDIRECT", "")

	_, err := Load()

	require.Error(t, err)
	for _, name := range []string{
		"GANANA_SUPABASE_URL",
		"GANANA_SUPABASE_ANON_KEY",
		"GANANA_SESSION_KEY",
		"GANANA_AUTH_CALLBACK_URL",
		"GANANA_POST_LOGIN_REDIRECT",
	} {
		assert.Contains(t, err.Error(), name)
	}
}

func TestLoad_RejectsSessionKeyThatIsNotBase64(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GANANA_SESSION_KEY", "not-base64!!")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GANANA_SESSION_KEY")
}

func TestLoad_RejectsSessionKeyOfWrongLength(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GANANA_SESSION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 16)))

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "32")
}

package session

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, KeySize)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func testSession() Session {
	return Session{
		UserID:       "6f1d2c3b-0000-4000-8000-000000000001",
		Email:        "user@example.com",
		AccessToken:  "supabase-access-token",
		RefreshToken: "supabase-refresh-token",
		ExpiresAt:    time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC),
		IssuedAt:     time.Date(2026, 9, 3, 11, 0, 0, 0, time.UTC),
	}
}

func TestNewSealer_RejectsWrongKeySize(t *testing.T) {
	_, err := NewSealer(make([]byte, 16))
	assert.ErrorIs(t, err, ErrInvalidKeySize)
}

func TestSealOpen_RoundTrip(t *testing.T) {
	sealer, err := NewSealer(testKey(t))
	require.NoError(t, err)

	want := testSession()
	token, err := sealer.Seal(want)
	require.NoError(t, err)

	got, err := sealer.Open(token)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// 封緘されたトークンから中身が読み取れないことを確認する。
// ブラウザに渡るのはこの文字列であり、リフレッシュトークンが露出してはならない。
func TestSeal_DoesNotLeakPlaintext(t *testing.T) {
	sealer, err := NewSealer(testKey(t))
	require.NoError(t, err)

	token, err := sealer.Seal(testSession())
	require.NoError(t, err)

	assert.NotContains(t, token, "supabase-refresh-token")
	assert.NotContains(t, token, "user@example.com")
}

// 同じ内容を封緘しても毎回異なるトークンになる（nonce がランダムであること）。
func TestSeal_UsesFreshNonce(t *testing.T) {
	sealer, err := NewSealer(testKey(t))
	require.NoError(t, err)

	first, err := sealer.Seal(testSession())
	require.NoError(t, err)
	second, err := sealer.Seal(testSession())
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
}

func TestOpen_RejectsTamperedToken(t *testing.T) {
	sealer, err := NewSealer(testKey(t))
	require.NoError(t, err)

	token, err := sealer.Seal(testSession())
	require.NoError(t, err)

	tampered := token[:len(token)-1] + flipLastChar(token)
	_, err = sealer.Open(tampered)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestOpen_RejectsTokenFromAnotherKey(t *testing.T) {
	sealer, err := NewSealer(testKey(t))
	require.NoError(t, err)
	token, err := sealer.Seal(testSession())
	require.NoError(t, err)

	otherKey := testKey(t)
	otherKey[0] ^= 0xff
	other, err := NewSealer(otherKey)
	require.NoError(t, err)

	_, err = other.Open(token)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestOpen_RejectsMalformedToken(t *testing.T) {
	sealer, err := NewSealer(testKey(t))
	require.NoError(t, err)

	for _, token := range []string{"", "not-base64!!", strings.Repeat("A", 8)} {
		_, err := sealer.Open(token)
		assert.ErrorIs(t, err, ErrInvalidToken, "token=%q", token)
	}
}

func TestSession_AccessTokenExpired(t *testing.T) {
	sess := testSession()

	assert.False(t, sess.AccessTokenExpired(sess.ExpiresAt.Add(-time.Minute)))
	assert.True(t, sess.AccessTokenExpired(sess.ExpiresAt.Add(time.Minute)))
}

// 期限ちょうどでは、直後の外部 API 呼び出しが失効する余地があるため失効扱いとする。
func TestSession_AccessTokenExpired_TreatsLeewayAsExpired(t *testing.T) {
	sess := testSession()
	assert.True(t, sess.AccessTokenExpired(sess.ExpiresAt))
}

func flipLastChar(token string) string {
	last := token[len(token)-1]
	if last == 'A' {
		return "B"
	}
	return "A"
}

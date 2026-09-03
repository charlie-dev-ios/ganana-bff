package supabase

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAPIKey = "test-publishable-key"

func TestAuthorizeURL(t *testing.T) {
	c := NewClient("https://proj.supabase.co", testAPIKey, http.DefaultClient)

	raw := c.AuthorizeURL("google", "https://api.ganana.jp/auth/callback", "challenge-value")

	u, err := url.Parse(raw)
	require.NoError(t, err)
	assert.Equal(t, "https", u.Scheme)
	assert.Equal(t, "proj.supabase.co", u.Host)
	assert.Equal(t, "/auth/v1/authorize", u.Path)

	q := u.Query()
	assert.Equal(t, "google", q.Get("provider"))
	assert.Equal(t, "https://api.ganana.jp/auth/callback", q.Get("redirect_to"))
	assert.Equal(t, "challenge-value", q.Get("code_challenge"))
	assert.Equal(t, "s256", q.Get("code_challenge_method"))
}

// baseURL 末尾のスラッシュがあってもパスが二重にならないこと。
func TestAuthorizeURL_TrimsTrailingSlash(t *testing.T) {
	c := NewClient("https://proj.supabase.co/", testAPIKey, http.DefaultClient)

	raw := c.AuthorizeURL("google", "https://api.ganana.jp/auth/callback", "challenge-value")

	u, err := url.Parse(raw)
	require.NoError(t, err)
	assert.Equal(t, "/auth/v1/authorize", u.Path)
}

func TestExchangeCode(t *testing.T) {
	var gotPath, gotGrantType, gotAPIKey, gotContentType string
	var gotBody map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotGrantType = r.URL.Query().Get("grant_type")
		gotAPIKey = r.Header.Get("apikey")
		gotContentType = r.Header.Get("Content-Type")

		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &gotBody))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token": "access-abc",
			"refresh_token": "refresh-xyz",
			"token_type": "bearer",
			"expires_in": 3600,
			"user": {"id": "user-1", "email": "user@example.com"}
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, testAPIKey, srv.Client())
	res, err := c.ExchangeCode(context.Background(), "auth-code-1", "verifier-1")
	require.NoError(t, err)

	assert.Equal(t, "/auth/v1/token", gotPath)
	assert.Equal(t, "pkce", gotGrantType)
	assert.Equal(t, testAPIKey, gotAPIKey)
	assert.Equal(t, "application/json", gotContentType)
	assert.Equal(t, map[string]string{"auth_code": "auth-code-1", "code_verifier": "verifier-1"}, gotBody)

	assert.Equal(t, "access-abc", res.AccessToken)
	assert.Equal(t, "refresh-xyz", res.RefreshToken)
	assert.Equal(t, 3600, res.ExpiresIn)
	assert.Equal(t, "user-1", res.User.ID)
	assert.Equal(t, "user@example.com", res.User.Email)
}

func TestExchangeCode_ReturnsErrorOnFailureStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"code verifier mismatch"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, testAPIKey, srv.Client())
	_, err := c.ExchangeCode(context.Background(), "auth-code-1", "verifier-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "invalid_grant")
}

func TestRefresh(t *testing.T) {
	var gotGrantType string
	var gotBody map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotGrantType = r.URL.Query().Get("grant_type")
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &gotBody))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token": "access-new",
			"refresh_token": "refresh-new",
			"expires_in": 3600,
			"user": {"id": "user-1", "email": "user@example.com"}
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, testAPIKey, srv.Client())
	res, err := c.Refresh(context.Background(), "refresh-old")
	require.NoError(t, err)

	assert.Equal(t, "refresh_token", gotGrantType)
	assert.Equal(t, map[string]string{"refresh_token": "refresh-old"}, gotBody)
	assert.Equal(t, "access-new", res.AccessToken)
	assert.Equal(t, "refresh-new", res.RefreshToken)
}

func TestLogout(t *testing.T) {
	var gotPath, gotAuth, gotAPIKey string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotAPIKey = r.Header.Get("apikey")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, testAPIKey, srv.Client())
	require.NoError(t, c.Logout(context.Background(), "access-abc"))

	assert.Equal(t, "/auth/v1/logout", gotPath)
	assert.Equal(t, "Bearer access-abc", gotAuth)
	assert.Equal(t, testAPIKey, gotAPIKey)
}

func TestLogout_ReturnsErrorOnFailureStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"msg":"invalid token"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, testAPIKey, srv.Client())
	err := c.Logout(context.Background(), "access-abc")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/charlie-dev-ios/ganana-bff/internal/session"
	"github.com/charlie-dev-ios/ganana-bff/internal/supabase"
)

var fixedNow = time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)

// fakeClient は Supabase Auth の呼び出しを記録するテストダブル。
type fakeClient struct {
	authorizeURL string

	gotRedirectTo   string
	gotChallenge    string
	gotAuthCode     string
	gotVerifier     string
	gotRefreshToken string
	gotAccessToken  string

	exchangeResult *supabase.TokenResponse
	exchangeErr    error
	refreshResult  *supabase.TokenResponse
	refreshErr     error
	logoutErr      error
	logoutCalls    int
}

func (f *fakeClient) AuthorizeURL(_, redirectTo, challenge string) string {
	f.gotRedirectTo = redirectTo
	f.gotChallenge = challenge
	return f.authorizeURL
}

func (f *fakeClient) ExchangeCode(_ context.Context, authCode, verifier string) (*supabase.TokenResponse, error) {
	f.gotAuthCode = authCode
	f.gotVerifier = verifier
	return f.exchangeResult, f.exchangeErr
}

func (f *fakeClient) Refresh(_ context.Context, refreshToken string) (*supabase.TokenResponse, error) {
	f.gotRefreshToken = refreshToken
	return f.refreshResult, f.refreshErr
}

func (f *fakeClient) Logout(_ context.Context, accessToken string) error {
	f.logoutCalls++
	f.gotAccessToken = accessToken
	return f.logoutErr
}

func tokenResponse() *supabase.TokenResponse {
	return &supabase.TokenResponse{
		AccessToken:  "access-abc",
		RefreshToken: "refresh-xyz",
		TokenType:    "bearer",
		ExpiresIn:    3600,
		User:         supabase.User{ID: "user-1", Email: "user@example.com"},
	}
}

func newTestHandler(t *testing.T, client Client) (*Handler, *session.Sealer) {
	t.Helper()

	key := make([]byte, session.KeySize)
	for i := range key {
		key[i] = byte(i + 1)
	}
	sealer, err := session.NewSealer(key)
	require.NoError(t, err)

	h := NewHandler(client, sealer, Config{
		CallbackURL:       "https://api.ganana.jp/auth/callback",
		PostLoginRedirect: "https://app.ganana.jp/home",
		CookieDomain:      ".ganana.jp",
		CookieSecure:      true,
	})
	h.now = func() time.Time { return fixedNow }

	return h, sealer
}

func newTestRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/auth/login", h.Login)
	r.GET("/auth/callback", h.Callback)
	r.POST("/auth/logout", h.Logout)

	protected := r.Group("/", h.RequireAuth())
	protected.GET("/auth/me", h.Me)

	return r
}

func cookieByName(res *http.Response, name string) *http.Cookie {
	for _, c := range res.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestLogin_RedirectsToAuthorizeURL(t *testing.T) {
	client := &fakeClient{authorizeURL: "https://proj.supabase.co/auth/v1/authorize?provider=google"}
	h, _ := newTestHandler(t, client)

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/login", nil))

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, client.authorizeURL, w.Header().Get("Location"))
	assert.Equal(t, "https://api.ganana.jp/auth/callback", client.gotRedirectTo)
}

// クッキーに保存した code_verifier と、Supabase に渡した code_challenge が対応していること。
func TestLogin_StoresVerifierMatchingTheChallenge(t *testing.T) {
	client := &fakeClient{authorizeURL: "https://proj.supabase.co/auth/v1/authorize"}
	h, _ := newTestHandler(t, client)

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/login", nil))

	verifier := cookieByName(w.Result(), verifierCookieName)
	require.NotNil(t, verifier, "code_verifier のクッキーが設定されていること")
	assert.Equal(t, codeChallenge(verifier.Value), client.gotChallenge)
}

func TestLogin_VerifierCookieIsProtected(t *testing.T) {
	client := &fakeClient{authorizeURL: "https://proj.supabase.co/auth/v1/authorize"}
	h, _ := newTestHandler(t, client)

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/login", nil))

	verifier := cookieByName(w.Result(), verifierCookieName)
	require.NotNil(t, verifier)
	assert.True(t, verifier.HttpOnly, "JavaScript から読めてはならない")
	assert.True(t, verifier.Secure)
	assert.Equal(t, http.SameSiteLaxMode, verifier.SameSite)
	assert.Positive(t, verifier.MaxAge, "認可の往復中しか有効でないこと")
}

// 認可コードの交換に、クッキーへ保存した code_verifier が用いられること。
// この結び付きが CSRF 対策（他人の認可コードを注入されても交換が成立しない）を担う。
func TestCallback_ExchangesCodeWithStoredVerifier(t *testing.T) {
	client := &fakeClient{exchangeResult: tokenResponse()}
	h, _ := newTestHandler(t, client)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=auth-code-1", nil)
	req.AddCookie(&http.Cookie{Name: verifierCookieName, Value: "stored-verifier"})

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "https://app.ganana.jp/home", w.Header().Get("Location"))
	assert.Equal(t, "auth-code-1", client.gotAuthCode)
	assert.Equal(t, "stored-verifier", client.gotVerifier)
}

func TestCallback_IssuesSessionCookie(t *testing.T) {
	client := &fakeClient{exchangeResult: tokenResponse()}
	h, sealer := newTestHandler(t, client)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=auth-code-1", nil)
	req.AddCookie(&http.Cookie{Name: verifierCookieName, Value: "stored-verifier"})

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	cookie := cookieByName(w.Result(), SessionCookieName)
	require.NotNil(t, cookie)
	assert.True(t, cookie.HttpOnly)
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	// Go の Cookie パーサは RFC 6265 に従い Domain 先頭のドットを除去する。
	assert.Equal(t, "ganana.jp", cookie.Domain)

	sess, err := sealer.Open(cookie.Value)
	require.NoError(t, err)
	assert.Equal(t, "user-1", sess.UserID)
	assert.Equal(t, "user@example.com", sess.Email)
	assert.Equal(t, "access-abc", sess.AccessToken)
	assert.Equal(t, "refresh-xyz", sess.RefreshToken)
	assert.Equal(t, fixedNow.Add(3600*time.Second), sess.ExpiresAt)
	assert.Equal(t, fixedNow, sess.IssuedAt)
}

// Supabase のトークンがブラウザから読める形で出てはならない。
func TestCallback_SessionCookieHidesSupabaseTokens(t *testing.T) {
	client := &fakeClient{exchangeResult: tokenResponse()}
	h, _ := newTestHandler(t, client)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=auth-code-1", nil)
	req.AddCookie(&http.Cookie{Name: verifierCookieName, Value: "stored-verifier"})

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	raw := w.Header().Get("Set-Cookie")
	assert.NotContains(t, raw, "refresh-xyz")
	assert.NotContains(t, raw, "access-abc")
}

func TestCallback_ClearsVerifierCookie(t *testing.T) {
	client := &fakeClient{exchangeResult: tokenResponse()}
	h, _ := newTestHandler(t, client)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=auth-code-1", nil)
	req.AddCookie(&http.Cookie{Name: verifierCookieName, Value: "stored-verifier"})

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	verifier := cookieByName(w.Result(), verifierCookieName)
	require.NotNil(t, verifier)
	assert.Empty(t, verifier.Value)
	assert.Negative(t, verifier.MaxAge, "使い終わった verifier は削除されること")
}

func TestCallback_RejectsRequestWithoutVerifierCookie(t *testing.T) {
	client := &fakeClient{exchangeResult: tokenResponse()}
	h, _ := newTestHandler(t, client)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=auth-code-1", nil)

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, client.gotAuthCode, "交換は試みないこと")
}

func TestCallback_RejectsRequestWithoutCode(t *testing.T) {
	client := &fakeClient{exchangeResult: tokenResponse()}
	h, _ := newTestHandler(t, client)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback", nil)
	req.AddCookie(&http.Cookie{Name: verifierCookieName, Value: "stored-verifier"})

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// 利用者が同意を拒否した場合など、Supabase が error を返してくるケース。
func TestCallback_RejectsProviderError(t *testing.T) {
	client := &fakeClient{exchangeResult: tokenResponse()}
	h, _ := newTestHandler(t, client)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?error=access_denied", nil)
	req.AddCookie(&http.Cookie{Name: verifierCookieName, Value: "stored-verifier"})

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, client.gotAuthCode)
}

func TestCallback_ReturnsBadGatewayWhenExchangeFails(t *testing.T) {
	client := &fakeClient{exchangeErr: errors.New("supabase: トークン要求が失敗（status=400）: invalid_grant")}
	h, _ := newTestHandler(t, client)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=auth-code-1", nil)
	req.AddCookie(&http.Cookie{Name: verifierCookieName, Value: "stored-verifier"})

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.NotContains(t, w.Body.String(), "invalid_grant", "外部サービスの詳細を利用者へ返さないこと")
}

func TestRequireAuth_RejectsRequestWithoutSession(t *testing.T) {
	h, _ := newTestHandler(t, &fakeClient{})

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/me", nil))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireAuth_RejectsTamperedSession(t *testing.T) {
	h, _ := newTestHandler(t, &fakeClient{})

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "not-a-valid-token"})

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMe_ReturnsCurrentUser(t *testing.T) {
	h, sealer := newTestHandler(t, &fakeClient{})

	token, err := sealer.Seal(session.Session{
		UserID:      "user-1",
		Email:       "user@example.com",
		AccessToken: "access-abc",
		ExpiresAt:   fixedNow.Add(time.Hour),
		IssuedAt:    fixedNow,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "user-1", body["id"])
	assert.Equal(t, "user@example.com", body["email"])
}

// アクセストークンが失効していれば、リフレッシュしてセッションを更新する。
func TestRequireAuth_RefreshesExpiredAccessToken(t *testing.T) {
	client := &fakeClient{refreshResult: &supabase.TokenResponse{
		AccessToken:  "access-new",
		RefreshToken: "refresh-new",
		ExpiresIn:    3600,
		User:         supabase.User{ID: "user-1", Email: "user@example.com"},
	}}
	h, sealer := newTestHandler(t, client)

	token, err := sealer.Seal(session.Session{
		UserID:       "user-1",
		Email:        "user@example.com",
		AccessToken:  "access-old",
		RefreshToken: "refresh-old",
		ExpiresAt:    fixedNow.Add(-time.Minute),
		IssuedAt:     fixedNow.Add(-time.Hour),
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "refresh-old", client.gotRefreshToken)

	cookie := cookieByName(w.Result(), SessionCookieName)
	require.NotNil(t, cookie, "更新されたセッションが再発行されること")

	updated, err := sealer.Open(cookie.Value)
	require.NoError(t, err)
	assert.Equal(t, "access-new", updated.AccessToken)
	assert.Equal(t, "refresh-new", updated.RefreshToken)
	assert.Equal(t, fixedNow.Add(time.Hour), updated.ExpiresAt)
}

func TestRequireAuth_RejectsAndClearsSessionWhenRefreshFails(t *testing.T) {
	client := &fakeClient{refreshErr: errors.New("supabase: リフレッシュに失敗")}
	h, sealer := newTestHandler(t, client)

	token, err := sealer.Seal(session.Session{
		UserID:       "user-1",
		RefreshToken: "refresh-old",
		ExpiresAt:    fixedNow.Add(-time.Minute),
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	cookie := cookieByName(w.Result(), SessionCookieName)
	require.NotNil(t, cookie)
	assert.Empty(t, cookie.Value, "使えないセッションは削除されること")
}

func TestLogout_RevokesSupabaseSessionAndClearsCookie(t *testing.T) {
	client := &fakeClient{}
	h, sealer := newTestHandler(t, client)

	token, err := sealer.Seal(session.Session{
		UserID:      "user-1",
		AccessToken: "access-abc",
		ExpiresAt:   fixedNow.Add(time.Hour),
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 1, client.logoutCalls)
	assert.Equal(t, "access-abc", client.gotAccessToken)

	cookie := cookieByName(w.Result(), SessionCookieName)
	require.NotNil(t, cookie)
	assert.Empty(t, cookie.Value)
	assert.Negative(t, cookie.MaxAge)
}

// セッションが無い状態のログアウトも成功として扱う（冪等）。
func TestLogout_SucceedsWithoutSession(t *testing.T) {
	client := &fakeClient{}
	h, _ := newTestHandler(t, client)

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/logout", nil))

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Zero(t, client.logoutCalls)
}

// Supabase 側の失効に失敗しても、BFF のセッションは確実に破棄する。
func TestLogout_ClearsCookieEvenIfSupabaseFails(t *testing.T) {
	client := &fakeClient{logoutErr: errors.New("supabase: ログアウトが失敗")}
	h, sealer := newTestHandler(t, client)

	token, err := sealer.Seal(session.Session{UserID: "user-1", AccessToken: "access-abc"})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	cookie := cookieByName(w.Result(), SessionCookieName)
	require.NotNil(t, cookie)
	assert.Empty(t, cookie.Value)
}

// CookieSecure=false（ローカル開発）でも他の保護属性は維持する。
func TestCookieAttributes_SecureCanBeDisabledForLocalDevelopment(t *testing.T) {
	client := &fakeClient{exchangeResult: tokenResponse()}
	h, _ := newTestHandler(t, client)
	h.cfg.CookieSecure = false

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=auth-code-1", nil)
	req.AddCookie(&http.Cookie{Name: verifierCookieName, Value: "stored-verifier"})

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	cookie := cookieByName(w.Result(), SessionCookieName)
	require.NotNil(t, cookie)
	assert.False(t, cookie.Secure)
	assert.True(t, cookie.HttpOnly)
}

// クッキーの MaxAge は盗まれた値には効かないため、発行時刻からの絶対寿命を
// サーバー側で検証する。アクセストークンが有効でも古すぎるセッションは拒否する。
func TestRequireAuth_RejectsSessionPastAbsoluteLifetime(t *testing.T) {
	client := &fakeClient{}
	h, sealer := newTestHandler(t, client)

	token, err := sealer.Seal(session.Session{
		UserID:       "user-1",
		AccessToken:  "access-abc",
		RefreshToken: "refresh-xyz",
		ExpiresAt:    fixedNow.Add(time.Hour),
		IssuedAt:     fixedNow.Add(-sessionMaxLifetime - time.Minute),
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Zero(t, client.gotRefreshToken, "寿命切れのセッションはリフレッシュしないこと")

	cookie := cookieByName(w.Result(), SessionCookieName)
	require.NotNil(t, cookie)
	assert.Empty(t, cookie.Value)
}

// 寿命内であれば、アクセストークンの失効はリフレッシュで回復する。
func TestRequireAuth_AcceptsSessionWithinAbsoluteLifetime(t *testing.T) {
	client := &fakeClient{}
	h, sealer := newTestHandler(t, client)

	token, err := sealer.Seal(session.Session{
		UserID:      "user-1",
		Email:       "user@example.com",
		AccessToken: "access-abc",
		ExpiresAt:   fixedNow.Add(time.Hour),
		IssuedAt:    fixedNow.Add(-sessionMaxLifetime + time.Hour),
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})

	w := httptest.NewRecorder()
	newTestRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

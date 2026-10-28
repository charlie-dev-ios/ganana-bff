package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newCORSRouter(allowed []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(allowed))
	r.GET("/auth/me", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r
}

func TestCORS_AllowsConfiguredOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Origin", "https://app.ganana.jp")

	w := httptest.NewRecorder()
	newCORSRouter([]string{"https://app.ganana.jp"}).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://app.ganana.jp", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	assert.Contains(t, w.Header().Values("Vary"), "Origin")
}

// 許可していないオリジンには許可ヘッダを返さない。
func TestCORS_DoesNotAllowUnknownOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Origin", "https://evil.example.com")

	w := httptest.NewRecorder()
	newCORSRouter([]string{"https://app.ganana.jp"}).ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

// ワイルドカードは資格情報付きリクエストでは使えないため、返してはならない。
func TestCORS_NeverReturnsWildcard(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Origin", "https://app.ganana.jp")

	w := httptest.NewRecorder()
	newCORSRouter([]string{"https://app.ganana.jp"}).ServeHTTP(w, req)

	assert.NotEqual(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_AnswersPreflight(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/auth/me", nil)
	req.Header.Set("Origin", "https://app.ganana.jp")
	req.Header.Set("Access-Control-Request-Method", "POST")

	w := httptest.NewRecorder()
	newCORSRouter([]string{"https://app.ganana.jp"}).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "https://app.ganana.jp", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
}

// 許可オリジンが未設定なら CORS ヘッダを一切付けない（同一オリジン運用）。
func TestCORS_DisabledWhenNoOriginsConfigured(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Origin", "https://app.ganana.jp")

	w := httptest.NewRecorder()
	newCORSRouter(nil).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

// 許可しないオリジンにも Vary を付ける（共有キャッシュ経由の混線を防ぐ）。
func TestCORS_SetsVaryEvenForDisallowedOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Origin", "https://evil.example.com")

	w := httptest.NewRecorder()
	newCORSRouter([]string{"https://app.ganana.jp"}).ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Values("Vary"), "Origin")
}

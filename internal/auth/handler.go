// Package auth は Supabase Auth を用いた BFF 認証パターンを実装する。
//
// BFF が認可コードをトークンと交換し、フロントエンドには BFF 自身のセッションを
// 発行する。Supabase のアクセストークン / リフレッシュトークンはフロントエンドへ
// 渡さず、封緘クッキー（internal/session）に格納する。
//
// 現時点では web 向けのクッキー配送のみを実装している。
// 設計の背景は docs/adr/0002-session-strategy.md を参照。
package auth

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/charlie-dev-ios/ganana-bff/internal/session"
	"github.com/charlie-dev-ios/ganana-bff/internal/supabase"
)

const (
	// SessionCookieName は BFF のセッションを格納するクッキー名。
	SessionCookieName = "ganana_session"
	// verifierCookieName は認可の往復中だけ PKCE の code_verifier を保持するクッキー名。
	verifierCookieName = "ganana_oauth_verifier"

	// verifierCookieMaxAge は認可の往復に許す時間。
	verifierCookieMaxAge = 10 * 60
	// sessionMaxLifetime はセッションの絶対寿命。これを過ぎると再ログインが必要になる。
	//
	// クッキーの MaxAge はクライアントへの指示にすぎず、盗まれたクッキーの値は
	// それを無視して送信できる。そのためサーバー側でも発行時刻から寿命を検証する。
	sessionMaxLifetime = 30 * 24 * time.Hour
	// sessionCookieMaxAge は sessionMaxLifetime に対応するクッキーの MaxAge（秒）。
	sessionCookieMaxAge = int(sessionMaxLifetime / time.Second)

	// providerGoogle は現在サポートしている唯一のログイン方式。
	providerGoogle = "google"

	// contextKeySession は gin のコンテキストにセッションを載せる際のキー。
	contextKeySession = "ganana.session"
)

// Client は auth が必要とする Supabase Auth の操作。
type Client interface {
	AuthorizeURL(provider, redirectTo, codeChallenge string) string
	ExchangeCode(ctx context.Context, authCode, codeVerifier string) (*supabase.TokenResponse, error)
	Refresh(ctx context.Context, refreshToken string) (*supabase.TokenResponse, error)
	Logout(ctx context.Context, accessToken string) error
}

// Config は認証ハンドラの設定。
type Config struct {
	// CallbackURL は Supabase が認可後にブラウザを戻す先（この BFF の /auth/callback）。
	CallbackURL string
	// PostLoginRedirect はログイン完了後にブラウザを送る先。
	PostLoginRedirect string
	// CookieDomain はクッキーの Domain 属性。空ならホスト限定クッキーになる。
	CookieDomain string
	// CookieSecure はクッキーに Secure 属性を付けるか。
	CookieSecure bool
}

// Handler は認証エンドポイントとミドルウェアを提供する。
type Handler struct {
	client Client
	sealer *session.Sealer
	cfg    Config
	// now は現在時刻を返す。テストで差し替える。
	now func() time.Time
}

// NewHandler は認証ハンドラを生成する。
func NewHandler(client Client, sealer *session.Sealer, cfg Config) *Handler {
	return &Handler{
		client: client,
		sealer: sealer,
		cfg:    cfg,
		now:    time.Now,
	}
}

// Login は PKCE の code_verifier を発行し、Supabase の認可エンドポイントへリダイレクトする。
func (h *Handler) Login(c *gin.Context) {
	verifier, err := newCodeVerifier()
	if err != nil {
		slog.Error("code_verifier の生成に失敗", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// code_verifier は HttpOnly クッキーに保持し、コールバック時に取り出す。
	// このクッキーと認可コードの結び付きが CSRF 対策を担う。攻撃者が自分の認可コードを
	// 被害者のブラウザへ注入しても、被害者のクッキーにある verifier とは対応しないため
	// 交換が成立しない。
	http.SetCookie(c.Writer, h.cookie(verifierCookieName, verifier, "/auth", verifierCookieMaxAge))

	c.Redirect(http.StatusFound, h.client.AuthorizeURL(providerGoogle, h.cfg.CallbackURL, codeChallenge(verifier)))
}

// Callback は認可コードをトークンと交換し、セッションクッキーを発行する。
func (h *Handler) Callback(c *gin.Context) {
	if providerErr := c.Query("error"); providerErr != "" {
		slog.Warn("認可が拒否された", slog.String("error", providerErr))
		h.clearCookie(c, verifierCookieName, "/auth")
		c.JSON(http.StatusBadRequest, gin.H{"error": "authorization_denied"})
		return
	}

	verifier, err := c.Cookie(verifierCookieName)
	if err != nil || verifier == "" {
		slog.Warn("code_verifier のクッキーが無い状態でコールバックされた")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}

	code := c.Query("code")
	if code == "" {
		h.clearCookie(c, verifierCookieName, "/auth")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}

	token, err := h.client.ExchangeCode(c.Request.Context(), code, verifier)
	if err != nil {
		// 失敗の詳細は外部サービスの応答を含むため、利用者へは返さずログに残す。
		slog.Error("認可コードの交換に失敗", slog.String("error", err.Error()))
		h.clearCookie(c, verifierCookieName, "/auth")
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream_error"})
		return
	}

	now := h.now()
	sess := session.Session{
		UserID:       token.User.ID,
		Email:        token.User.Email,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    now.Add(time.Duration(token.ExpiresIn) * time.Second),
		IssuedAt:     now,
	}

	if err := h.issueSession(c, sess); err != nil {
		slog.Error("セッションの封緘に失敗", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	h.clearCookie(c, verifierCookieName, "/auth")
	c.Redirect(http.StatusFound, h.cfg.PostLoginRedirect)
}

// Logout は Supabase 側のリフレッシュトークンを失効させ、セッションクッキーを削除する。
// セッションが無い場合も成功として扱う（冪等）。
func (h *Handler) Logout(c *gin.Context) {
	if sess, ok := h.readSession(c); ok && sess.AccessToken != "" {
		// Supabase 側の失効に失敗しても BFF のセッションは破棄する。
		// 失効できない場合でも、クッキーを消せばこのブラウザからは利用できなくなる。
		if err := h.client.Logout(c.Request.Context(), sess.AccessToken); err != nil {
			slog.Warn("Supabase 側のログアウトに失敗", slog.String("error", err.Error()))
		}
	}

	h.clearCookie(c, SessionCookieName, "/")
	c.Status(http.StatusNoContent)
}

// Me は現在のセッションのユーザーを返す。RequireAuth の内側で用いる。
func (h *Handler) Me(c *gin.Context) {
	sess, ok := UserFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": sess.UserID, "email": sess.Email})
}

// RequireAuth はセッションを検証し、コンテキストへ載せるミドルウェア。
// アクセストークンが失効している場合はリフレッシュを試みる。
func (h *Handler) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, ok := h.readSession(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		if h.now().Sub(sess.IssuedAt) >= sessionMaxLifetime {
			slog.Info("セッションが絶対寿命に達したため再ログインを求める", slog.String("user_id", sess.UserID))
			h.clearCookie(c, SessionCookieName, "/")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		if sess.AccessTokenExpired(h.now()) {
			refreshed, err := h.refresh(c, sess)
			if err != nil {
				slog.Info("セッションの更新に失敗したため再ログインを求める", slog.String("error", err.Error()))
				h.clearCookie(c, SessionCookieName, "/")
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
			sess = refreshed
		}

		c.Set(contextKeySession, sess)
		c.Next()
	}
}

// UserFrom は RequireAuth が載せたセッションを取り出す。
func UserFrom(c *gin.Context) (session.Session, bool) {
	value, exists := c.Get(contextKeySession)
	if !exists {
		return session.Session{}, false
	}

	sess, ok := value.(session.Session)
	return sess, ok
}

// refresh はリフレッシュトークンでセッションを更新し、クッキーを再発行する。
func (h *Handler) refresh(c *gin.Context, sess session.Session) (session.Session, error) {
	token, err := h.client.Refresh(c.Request.Context(), sess.RefreshToken)
	if err != nil {
		return session.Session{}, err
	}

	now := h.now()
	refreshed := session.Session{
		UserID:       sess.UserID,
		Email:        sess.Email,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    now.Add(time.Duration(token.ExpiresIn) * time.Second),
		IssuedAt:     sess.IssuedAt,
	}

	// リフレッシュ応答にユーザー情報が含まれる場合は最新の値を採用する。
	if token.User.ID != "" {
		refreshed.UserID = token.User.ID
		refreshed.Email = token.User.Email
	}

	if err := h.issueSession(c, refreshed); err != nil {
		return session.Session{}, err
	}

	return refreshed, nil
}

// readSession はクッキーからセッションを取り出す。
func (h *Handler) readSession(c *gin.Context) (session.Session, bool) {
	raw, err := c.Cookie(SessionCookieName)
	if err != nil || raw == "" {
		return session.Session{}, false
	}

	sess, err := h.sealer.Open(raw)
	if err != nil {
		return session.Session{}, false
	}

	return sess, true
}

// issueSession はセッションを封緘してクッキーに設定する。
func (h *Handler) issueSession(c *gin.Context, sess session.Session) error {
	sealed, err := h.sealer.Seal(sess)
	if err != nil {
		return err
	}

	http.SetCookie(c.Writer, h.cookie(SessionCookieName, sealed, "/", sessionCookieMaxAge))
	return nil
}

// clearCookie はクッキーを削除する。
func (h *Handler) clearCookie(c *gin.Context, name, path string) {
	http.SetCookie(c.Writer, h.cookie(name, "", path, -1))
}

// cookie は共通の保護属性を付けたクッキーを組み立てる。
//
// SameSite=Lax は、BFF と web フロントエンドが同一のレジストラブルドメインに
// 配置されていることを前提とする（docs/adr/0002-session-strategy.md）。
func (h *Handler) cookie(name, value, path string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		Domain:   h.cfg.CookieDomain,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

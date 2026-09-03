// Package supabase は Supabase Auth（GoTrue）の REST API クライアント。
//
// 用いるのは GoTrue の標準エンドポイントであり、Supabase の
// 「OAuth 2.1 server」機能（/auth/v1/oauth/*）ではない。後者は Supabase 自身が
// OAuth プロバイダとなりサードパーティ製アプリへアクセスを許可するための機能で、
// エンドユーザーのソーシャルログインには用いない。docs/adr/0002-session-strategy.md を参照。
package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// errorBodyLimit はエラー応答から読み取る最大バイト数。
// 外部サービスの応答をそのままログ・エラーに載せるため上限を設ける。
const errorBodyLimit = 1 << 12

// Client は Supabase Auth のクライアント。
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient は Supabase Auth のクライアントを生成する。
//
// baseURL は Supabase プロジェクトの URL（例: https://xxxx.supabase.co）。
// apiKey は publishable（anon）キー。公開前提のキーであり秘匿情報ではない。
func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

// User は Supabase 上のユーザー。BFF が必要とする項目のみを持つ。
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// TokenResponse はトークンエンドポイントの応答。
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	// ExpiresIn はアクセストークンの残り有効秒数。
	ExpiresIn int  `json:"expires_in"`
	User      User `json:"user"`
}

// AuthorizeURL は認可を開始するためのリダイレクト先 URL を組み立てる。
//
// redirectTo は認可後に Supabase がブラウザを戻す先（BFF のコールバック URL）で、
// Supabase プロジェクトの Redirect URL 許可リストに登録されている必要がある。
func (c *Client) AuthorizeURL(provider, redirectTo, codeChallenge string) string {
	q := url.Values{}
	q.Set("provider", provider)
	q.Set("redirect_to", redirectTo)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "s256")
	q.Set("apikey", c.apiKey)

	return c.baseURL + "/auth/v1/authorize?" + q.Encode()
}

// ExchangeCode は認可コードを PKCE フローでトークンと交換する。
func (c *Client) ExchangeCode(ctx context.Context, authCode, codeVerifier string) (*TokenResponse, error) {
	return c.token(ctx, "pkce", map[string]string{
		"auth_code":     authCode,
		"code_verifier": codeVerifier,
	})
}

// Refresh はリフレッシュトークンで新しいアクセストークンを取得する。
func (c *Client) Refresh(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	return c.token(ctx, "refresh_token", map[string]string{
		"refresh_token": refreshToken,
	})
}

// Logout は Supabase 側のリフレッシュトークンを失効させる。
func (c *Client) Logout(ctx context.Context, accessToken string) error {
	req, err := c.newRequest(ctx, c.baseURL+"/auth/v1/logout", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("supabase: ログアウト要求に失敗: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		return c.responseError("ログアウト", res)
	}
	return nil
}

func (c *Client) token(ctx context.Context, grantType string, body map[string]string) (*TokenResponse, error) {
	req, err := c.newRequest(ctx, c.baseURL+"/auth/v1/token?grant_type="+grantType, body)
	if err != nil {
		return nil, err
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("supabase: トークン要求に失敗（grant_type=%s）: %w", grantType, err)
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		return nil, c.responseError("トークン要求（grant_type="+grantType+"）", res)
	}

	var token TokenResponse
	if err := json.NewDecoder(res.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("supabase: トークン応答の解釈に失敗: %w", err)
	}
	return &token, nil
}

func (c *Client) newRequest(ctx context.Context, endpoint string, body map[string]string) (*http.Request, error) {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("supabase: リクエストの符号化に失敗: %w", err)
		}
		payload = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("supabase: リクエストの生成に失敗: %w", err)
	}
	req.Header.Set("apikey", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// responseError は失敗応答からエラーを組み立てる。
// 応答本文には Supabase 側の原因（invalid_grant 等）が含まれるため、
// 障害調査のために保持する。呼び出し側はこれをクライアントへ返してはならない。
func (c *Client) responseError(operation string, res *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(res.Body, errorBodyLimit))
	return fmt.Errorf("supabase: %sが失敗（status=%d）: %s", operation, res.StatusCode, strings.TrimSpace(string(raw)))
}

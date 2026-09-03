// Package middleware は BFF 共通の HTTP ミドルウェアを提供する。
package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

// allowedMethods は web フロントエンドから呼ばれるメソッド。
var allowedMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

// allowedHeaders は web フロントエンドが送るヘッダ。
var allowedHeaders = []string{"Content-Type"}

// CORS は許可オリジンからの資格情報付きリクエストを通すミドルウェア。
//
// セッションをクッキーで運ぶため Access-Control-Allow-Credentials を返す必要があり、
// その場合 Access-Control-Allow-Origin にワイルドカードは使えない。
// そのため許可リストと一致したオリジンのみをそのまま返す。
//
// allowedOrigins が空の場合は CORS ヘッダを一切付けない（同一オリジン運用）。
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if len(allowedOrigins) == 0 {
			c.Next()
			return
		}

		// 応答がオリジンによって変わることをキャッシュに伝える。許可しないオリジンにも
		// 付ける必要がある（付けないと、許可オリジン向けの応答が共有キャッシュ経由で
		// 別のオリジンへ返りうる）。
		c.Writer.Header().Add("Vary", "Origin")

		if origin == "" || !slices.Contains(allowedOrigins, origin) {
			c.Next()
			return
		}

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))
			c.Header("Access-Control-Allow-Headers", strings.Join(allowedHeaders, ", "))
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

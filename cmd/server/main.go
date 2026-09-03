package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/charlie-dev-ios/ganana-bff/internal/auth"
	"github.com/charlie-dev-ios/ganana-bff/internal/config"
	"github.com/charlie-dev-ios/ganana-bff/internal/handler"
	"github.com/charlie-dev-ios/ganana-bff/internal/middleware"
	"github.com/charlie-dev-ios/ganana-bff/internal/session"
	"github.com/charlie-dev-ios/ganana-bff/internal/supabase"
)

// supabaseTimeout は Supabase Auth への 1 回の呼び出しに許す時間。
const supabaseTimeout = 10 * time.Second

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("設定の読み込みに失敗", slog.String("error", err.Error()))
		os.Exit(1)
	}

	sealer, err := session.NewSealer(cfg.SessionKey)
	if err != nil {
		slog.Error("セッション鍵の初期化に失敗", slog.String("error", err.Error()))
		os.Exit(1)
	}

	supabaseClient := supabase.NewClient(
		cfg.SupabaseURL,
		cfg.SupabaseAnonKey,
		&http.Client{Timeout: supabaseTimeout},
	)

	authHandler := auth.NewHandler(supabaseClient, sealer, auth.Config{
		CallbackURL:       cfg.AuthCallbackURL,
		PostLoginRedirect: cfg.PostLoginRedirect,
		CookieDomain:      cfg.CookieDomain,
		CookieSecure:      cfg.CookieSecure,
	})

	r := gin.New()
	r.Use(gin.Recovery(), middleware.CORS(cfg.AllowedOrigins))

	r.GET("/health", handler.Health)
	r.GET("/auth/login", authHandler.Login)
	r.GET("/auth/callback", authHandler.Callback)
	r.POST("/auth/logout", authHandler.Logout)
	r.GET("/auth/me", authHandler.RequireAuth(), authHandler.Me)

	addr := ":" + cfg.Port
	slog.Info("starting BFF server", slog.String("addr", addr))
	if err := r.Run(addr); err != nil {
		slog.Error("server stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

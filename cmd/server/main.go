package main

import (
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/charlie-dev-ios/ganana-bff/internal/config"
	"github.com/charlie-dev-ios/ganana-bff/internal/handler"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg := config.Load()

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/health", handler.Health)

	addr := ":" + cfg.Port
	logger.Info("starting BFF server", slog.String("addr", addr))
	if err := r.Run(addr); err != nil {
		logger.Error("server stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

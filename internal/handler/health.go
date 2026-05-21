package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Health は BFF の稼働状態を返すヘルスチェックハンドラ。
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

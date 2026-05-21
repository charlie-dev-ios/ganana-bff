package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Default(t *testing.T) {
	cfg := Load()
	assert.Equal(t, "8080", cfg.Port)
}

func TestLoad_EnvOverride(t *testing.T) {
	t.Setenv("GANANA_PORT", "9090")
	cfg := Load()
	assert.Equal(t, "9090", cfg.Port)
}

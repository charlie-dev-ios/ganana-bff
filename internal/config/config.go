package config

import "github.com/spf13/viper"

// Config はアプリケーションの設定値。
type Config struct {
	// Port は HTTP サーバの待ち受けポート。
	Port string
}

// Load は環境変数（GANANA_ プレフィックス）から設定を読み込む。
// 未指定の項目はデフォルト値を用いる。
func Load() Config {
	v := viper.New()
	v.SetEnvPrefix("GANANA")
	v.AutomaticEnv()
	v.SetDefault("port", "8080")

	return Config{
		Port: v.GetString("port"),
	}
}

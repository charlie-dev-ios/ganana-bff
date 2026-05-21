# ganana-bff

iOS アプリ Ganana 向けの BFF（Backend For Frontend）。

コア（勘定系など重いドメイン）は持たず、エッジレイヤーとして薄く保つ。詳細は [docs/architecture.md](./docs/architecture.md) を参照。

## 必要なもの

- [mise](https://mise.jdx.dev/) — Go 本体のバージョン管理に使用

Go 本体のバージョンは [`mise.toml`](./mise.toml) で固定している。
ライブラリ（Gin / Viper / testify 等）のバージョンは [`go.mod`](./go.mod) で固定管理する。

## セットアップ

```sh
mise install      # mise.toml に固定された Go をインストール
mise run setup    # 依存ライブラリをダウンロード
```

## 開発

```sh
mise run dev      # 開発サーバを起動（デフォルト :8080）
mise run test     # テストを実行
mise run check    # gofmt / go vet によるチェック
```

`GANANA_PORT` 環境変数で待ち受けポートを変更できる（未指定時は `8080`）。

```sh
GANANA_PORT=9090 mise run dev
```

## 動作確認

```sh
curl localhost:8080/health
# => {"status":"ok"}
```

## ディレクトリ構成

```
cmd/server/        エントリポイント
internal/config/   設定読み込み（Viper）
internal/handler/  HTTP ハンドラ
docs/              設計・開発ドキュメント
```

## ドキュメント

- [docs/architecture.md](./docs/architecture.md) — アーキテクチャと技術選定
- [docs/development.md](./docs/development.md) — 開発原則・規約

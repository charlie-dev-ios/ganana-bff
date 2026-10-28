# ganana-bff

Ganana の BFF（Backend For Frontend）。web フロントエンド（ganana-web）と iOS アプリの双方を担う。

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

## 設定

環境変数（`GANANA_` プレフィックス）で設定する。必須項目が欠けている場合は起動時にエラーとなる。

| 環境変数 | 必須 | 既定値 | 説明 |
| --- | :---: | --- | --- |
| `GANANA_SUPABASE_URL` | ✓ | — | Supabase プロジェクトの URL（例: `https://xxxx.supabase.co`） |
| `GANANA_SUPABASE_ANON_KEY` | ✓ | — | Supabase の publishable（anon）キー |
| `GANANA_SESSION_KEY` | ✓ | — | セッション封緘用の 32 バイト鍵（base64）。`openssl rand -base64 32` で生成する |
| `GANANA_AUTH_CALLBACK_URL` | ✓ | — | この BFF の `/auth/callback` の公開 URL。Supabase の Redirect URL 許可リストに登録する |
| `GANANA_POST_LOGIN_REDIRECT` | ✓ | — | ログイン完了後にブラウザを送る先 |
| `GANANA_PORT` | | `8080` | HTTP サーバの待ち受けポート |
| `GANANA_COOKIE_DOMAIN` | | （空） | セッションクッキーの `Domain` 属性。空ならホスト限定クッキー |
| `GANANA_COOKIE_SECURE` | | `true` | セッションクッキーに `Secure` 属性を付けるか。ローカル HTTP 開発時のみ `false` |
| `GANANA_ALLOWED_ORIGINS` | | （空） | CORS で許可するオリジン（カンマ区切り）。空なら CORS を無効化 |

`GANANA_SESSION_KEY` は秘匿情報である。漏洩した場合、任意のセッションを偽造できる。

## 開発

```sh
mise run dev      # 開発サーバを起動（デフォルト :8080）
mise run test     # テストを実行
mise run check    # gofmt / go vet によるチェック
```

ローカルで起動する例。

```sh
export GANANA_SUPABASE_URL=https://xxxx.supabase.co
export GANANA_SUPABASE_ANON_KEY=...
export GANANA_SESSION_KEY=$(openssl rand -base64 32)
export GANANA_AUTH_CALLBACK_URL=http://localhost:8080/auth/callback
export GANANA_POST_LOGIN_REDIRECT=http://localhost:3000/
export GANANA_COOKIE_SECURE=false
export GANANA_ALLOWED_ORIGINS=http://localhost:3000

mise run dev
```

## エンドポイント

| メソッド | パス | 認証 | 説明 |
| --- | --- | :---: | --- |
| `GET` | `/health` | | ヘルスチェック |
| `GET` | `/auth/login` | | Google ログインを開始し、Supabase の認可エンドポイントへリダイレクトする |
| `GET` | `/auth/callback` | | 認可コードをトークンと交換し、セッションクッキーを発行する |
| `POST` | `/auth/logout` | | Supabase 側のセッションを失効させ、クッキーを削除する |
| `GET` | `/auth/me` | ✓ | 現在ログイン中のユーザーを返す |

## 動作確認

```sh
curl localhost:8080/health
# => {"status":"ok"}
```

ログインはブラウザで `http://localhost:8080/auth/login` を開いて確認する。
事前に Supabase プロジェクト側で Google プロバイダを有効化し、`GANANA_AUTH_CALLBACK_URL`
を Redirect URL 許可リストへ登録しておく必要がある。

## ディレクトリ構成

```
cmd/server/          エントリポイント
internal/auth/       認証エンドポイントと認証ミドルウェア
internal/config/     設定読み込み（Viper）
internal/handler/    HTTP ハンドラ
internal/middleware/ 共通ミドルウェア（CORS）
internal/session/    セッションの封緘・開封（AES-256-GCM）
internal/supabase/   Supabase Auth クライアント
docs/                設計・開発ドキュメント
```

## ドキュメント

- [docs/architecture.md](./docs/architecture.md) — アーキテクチャと技術選定
- [docs/development.md](./docs/development.md) — 開発原則・規約
- [docs/adr/](./docs/adr/) — アーキテクチャ決定記録（ADR）

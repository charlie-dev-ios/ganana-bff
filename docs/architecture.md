# Architecture

Ganana の BFF（Backend For Frontend）のアーキテクチャドキュメント。

## 役割

Ganana の BFF。コア（勘定系など重いドメイン）は持たず、エッジレイヤーとして薄く保つ。

**web フロントエンド（ganana-web）と iOS アプリの双方を担う。**

- BFF 専用エンドポイント / 画面最適化（画面単位の集約レスポンス・データ整形）
- 認証・セッション管理（トークン交換・認証中継）
- 複数 API / 外部サービスの集約・オーケストレーション

## フロントエンド

web と iOS を同一 BFF で扱う。認証処理の大半（state / PKCE 生成、認可コード交換、セッション封緘）は共通で、フロントエンドによって変わるのはセッションの配送方法のみ。

| | web | iOS |
| --- | --- | --- |
| セッション配送 | `HttpOnly` + `Secure` + `SameSite` クッキー | Bearer トークン（Keychain 保管） |
| CSRF 対策 | 必要（クッキーが自動送信されるため） | 不要 |
| CORS | 必要 | 不要 |
| コールバック先 | ブラウザへ 302 リダイレクト | Universal Link |

現時点では web 向けのクッキー配送のみを実装している。詳細は [ADR 0002](./adr/0002-session-strategy.md) を参照。

## 認証

Supabase Auth を用いた BFF 認証パターン。BFF が認可コードを交換し、フロントエンドには BFF 自身のセッションを発行する。Supabase のトークンはフロントエンドへ渡さない。

```
ブラウザ ──▶ BFF /auth/login ──▶ Supabase /authorize ──▶ Google
                                                            │
ブラウザ ◀── セッションクッキー ◀── BFF /auth/callback ◀── 認可コード
```

- 認可コードの交換は `POST /auth/v1/token?grant_type=pkce` で行う
- セッションは AES-256-GCM で封緘したクッキーとして発行する（サーバー側ストアなし）
- 認証実装は標準ライブラリのみで完結する

選定理由と経緯は [ADR 0001](./adr/0001-authentication-provider.md) および [ADR 0002](./adr/0002-session-strategy.md) を参照。

## 技術選定

| 項目 | 採用 | 選定理由 |
| --- | --- | --- |
| 言語 | Go | クラウドネイティブ / マイクロサービスの事実上の標準。軽量・高速起動でデプロイ先の選択肢が広い。並行処理（goroutine）が複数 API の並列呼び出しに適する。BFF はコアではなくエッジレイヤーであり、Java の重さを持ち込む必要がない |
| Web フレームワーク | Gin | Go の Web フレームワークで市場シェア最大。情報量・人材プールが最も厚く、保守性・採用面で有利 |
| 設定管理 | Viper | Go の設定ライブラリのデファクト |
| ロギング | `log/slog`（標準ライブラリ） | Go 1.21 以降の標準。構造化ログ |
| HTTP クライアント | 標準 `net/http` + `golang.org/x/sync/errgroup` | 複数 API の並列呼び出しとエラー集約 |
| テスト | 標準 `testing` + testify | アサーションライブラリのデファクト |
| 認証 | Supabase Auth + 標準ライブラリ | 資格情報の保管・検証を委譲。BFF 側は追加ライブラリなしで実装できる（[ADR 0001](./adr/0001-authentication-provider.md) / [ADR 0002](./adr/0002-session-strategy.md)） |

## デプロイ

（未定 — 言語選定上はサーバーレス / コンテナ常駐のいずれも可能）

## ADR

- [ADR 0001](./adr/0001-authentication-provider.md) — 認証基盤に Supabase Auth を採用する
- [ADR 0002](./adr/0002-session-strategy.md) — BFF セッションを封緘クッキー方式とし、web / iOS を同一 BFF で扱う

# ADR 0002: BFF セッションを封緘クッキー方式とし、web / iOS を同一 BFF で扱う

- ステータス: Accepted
- 日付: 2026-09-03
- 関連: [ADR 0001](./0001-authentication-provider.md)（未決事項の確定と、構成の一部訂正）

## コンテキスト

[ADR 0001](./0001-authentication-provider.md) で認証基盤に Supabase Auth を採用し、BFF 認証パターン（BFF が認可コードを交換し、フロントエンドには BFF 自身のセッションを発行する）を選んだ。ただし次の 2 点が未確定のまま残っていた。

1. **セッションの持ち方**: BFF が発行するセッションをステートレス署名トークンとするか、サーバー側セッションストアとするか
2. **リフレッシュトークンの取り扱い方針**

加えて、ADR 0001 は iOS アプリのみを想定して書かれていたが、**web フロントエンド（ganana-web）を先行して整備する**方針となった。両者を同一 BFF で扱うため、フロントエンド差の吸収方法を決める必要がある。

## 決定 1: Supabase Auth のエンドポイントを訂正する

ADR 0001 の「構成」節は Supabase の [OAuth 2.1 Flows](https://supabase.com/docs/guides/auth/oauth-server/oauth-flows) を参照していたが、**この機能は用途が異なる**。当該ドキュメントに次の記載がある。

> this is for third-party client applications, not end-user social login

これは Supabase 自身が OAuth プロバイダとなり、サードパーティ製アプリへユーザーデータへのアクセスを許可するための機能である。フロー中の「ユーザーは自前の認可ページで認証する」という前提からも、既に自前のログイン機構がある場合を想定していることが分かる。Ganana が必要とする「Google でエンドユーザーがログインする」用途には該当しない。

正しくは GoTrue（Supabase Auth 本体）の標準エンドポイントを用いる。

| 用途 | エンドポイント |
| --- | --- |
| 認可の開始 | `GET {SUPABASE_URL}/auth/v1/authorize?provider=google&redirect_to=...&code_challenge=...&code_challenge_method=s256` |
| コード交換 | `POST {SUPABASE_URL}/auth/v1/token?grant_type=pkce`<br>body: `{"auth_code": "...", "code_verifier": "..."}` |
| リフレッシュ | `POST {SUPABASE_URL}/auth/v1/token?grant_type=refresh_token`<br>body: `{"refresh_token": "..."}` |
| ログアウト | `POST {SUPABASE_URL}/auth/v1/logout`（Bearer 認証、リフレッシュトークンを失効） |

いずれも `apikey` に Supabase の publishable（anon）キーを指定する。

これに伴い ADR 0001 の記述を次のとおり訂正する。

- **BFF は Supabase に対する confidential client ではない**。この フローに client_secret は登場しない（Google の client secret は Supabase 側に設定され、BFF は保持しない）。フローの安全性は PKCE と、`redirect_to` が BFF を指すこと、`code_verifier` が BFF 外に出ないことで担保される
- **`go-oidc` による JWKS 署名検証は不要**。署名検証が必要になるのは「クライアントから提示されたトークンを検証する」場合である。本設計でブラウザが提示するのは BFF 自身のセッションクッキーであり、Supabase のトークンは BFF が TLS 越しの server-to-server 通信で直接受け取るため、発行元が自明である
- 結果として**認証実装に追加ライブラリを必要としない**（`net/http` / `crypto/aes` / `encoding/json` のみ）。ADR 0001 の「BFF 側の検証コストが小さいこと」という決定要因は、当初想定よりさらに小さい形で満たされる

Supabase を採用するという ADR 0001 の決定自体は変更しない。

## 決定 2: セッションは封緘クッキー（sealed cookie）とする

BFF が発行するセッションを、**セッション内容を AES-256-GCM で暗号化してクッキーに封入する方式**とする。サーバー側セッションストアは持たない。

クッキーに封入する内容は次のとおり。

- Supabase のアクセストークン / リフレッシュトークン
- ユーザー識別子とメールアドレス
- アクセストークンの有効期限、セッション自体の発行時刻

### 選択肢の比較

| 方式 | 評価 |
| --- | --- |
| サーバー側セッションストア（Redis / Postgres） | 最も堅牢で即時失効も可能だが、MVP 時点で運用対象が増える |
| 署名のみのステートレストークン（JWT 等） | **不可**。リフレッシュトークンを含める必要があり、署名だけでは中身がブラウザから読めてしまう |
| **封緘クッキー（採用）** | 暗号化により中身がブラウザから読めない。インフラ追加ゼロ。ADR 0001 の「インフラ最小のステートレス」という想定を、リフレッシュトークンを晒さずに満たせる |

ADR 0001 は「ステートレス署名トークン」を想定していたが、リフレッシュトークンを BFF が預かる以上、署名のみでは要件を満たせない。暗号化に切り替えることで、状態をクッキーに載せたままブラウザには不透明に保てる。

### トレードオフ

- **サーバー側からの即時失効ができない**。封緘クッキーの有効期限が切れるまで、そのクッキーは有効である
  - 緩和策: アクセストークンの寿命を短く保つ、ログアウト時に Supabase 側のリフレッシュトークンを失効させる（`POST /auth/v1/logout`）ことでセッション更新を断つ
  - 緩和策: セッションの絶対寿命（発行時刻から 30 日）をサーバー側で検証する。クッキーの `MaxAge` はクライアントへの指示にすぎず、盗まれたクッキーの値はそれを無視して送信できるため、封緘した発行時刻で判定する
  - 全ユーザーの一斉失効が必要になった場合は、暗号鍵をローテーションすることで達成できる
- 将来、即時失効やセッション一覧表示が要件になった場合はサーバー側ストアへ移行する。封緘・開封は `internal/session` に閉じており、移行時の影響範囲は限定される

## 決定 3: フロントエンド差はセッション配送層のみで吸収する

web と iOS を同一 BFF で扱う。認証処理の大半（state / PKCE 生成、認可コード交換、セッション封緘）は共通で、**フロントエンドによって変わるのはセッションの配送方法のみ**とする。

| | web | iOS |
| --- | --- | --- |
| セッション配送 | `HttpOnly` + `Secure` + `SameSite` クッキー | Bearer トークン（Keychain 保管） |
| CSRF 対策 | 必要（クッキーが自動送信されるため） | 不要 |
| CORS | 必要 | 不要 |
| コールバック先 | ブラウザへ 302 リダイレクト | Universal Link |

本 ADR の時点では **web 向けのクッキー配送のみを実装する**（YAGNI）。iOS 対応時に配送層を追加する。

### web 固有の前提

- **BFF と web フロントエンドは同一のレジストラブルドメインに配置する**（例: `app.ganana.jp` と `api.ganana.jp`）。これにより `SameSite=Lax` のまま XHR にクッキーが送信され、CSRF 耐性を保ったまま CORS の緩和を最小限にできる
- 別サイトに分かれる構成を採る場合は `SameSite=None; Secure` と CSRF トークンの併用が必要になる。その場合は別途 ADR を起こす

## 影響

### 利点

- 認証実装に追加の外部ライブラリを必要としない
- リフレッシュトークンがブラウザにも端末にも露出しない
- MVP 時点でデータストアを必要としない
- 認証コアが共通化され、iOS 対応は配送層の追加で済む

### 欠点・留意点

- サーバー側からの即時失効ができない（上記のとおり）
- 封緘クッキーのサイズはおよそ 1〜2KB になる。ブラウザの上限 4KB には収まるが、封入内容を無制限に増やせるわけではない
- セッション暗号鍵（32 バイト）の安全な配布・ローテーション運用が新たに必要になる

## 未決事項 / フォローアップ

- 暗号鍵のローテーション手順（旧鍵での復号を一定期間許容する仕組み）
- Supabase プロジェクトの環境分離（開発 / 本番）— ADR 0001 から継続
- iOS 向け配送層の設計（Universal Link とワンタイムコード交換）

## 参考

- [Supabase Auth (GoTrue) REST API](https://github.com/supabase/auth)
- [Supabase OAuth 2.1 Flows](https://supabase.com/docs/guides/auth/oauth-server/oauth-flows) — 本 ADR で用途が異なると判断したもの
- [OAuth 2.0 for Browser-Based Applications](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-browser-based-apps)

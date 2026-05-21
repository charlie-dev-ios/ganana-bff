# ADR 0001: 認証基盤に Supabase Auth を採用する

- ステータス: Accepted
- 日付: 2026-05-21

## コンテキスト

Ganana BFF にユーザー認証機能を追加する。要件は以下のとおり。

- **ミニマム**: まずは最小構成で始めたい
- **自前実装を避ける**: パスワード保管・MFA・暗号など、セキュリティ事故に直結する領域を自前で書きたくない
- **セキュリティ最優先**
- **ログイン方式**: 当面は Google ログインのみ

また、認証フローの起点について次の前提を置く。

- iOS アプリは IdP に直接つながず、**BFF を経由してログインする**
- したがって BFF が OAuth クライアント（confidential client）となり、Authorization Code + PKCE フローを実行し、アプリにはセッションを発行する（いわゆる「BFF 認証パターン」）

## 決定要因

- 認証そのもの（資格情報の保管・検証・ソーシャル連携）はマネージドサービスに完全委譲する
- BFF 側の検証コストが小さいこと。理想は OAuth2 / OIDC 標準に準拠し、汎用ライブラリ（`coreos/go-oidc` + `golang.org/x/oauth2`）だけで完結すること
- ベンダーロックインが BFF のコードに染み出さないこと（標準準拠なら IdP 乗り換え時も BFF はほぼ無変更）
- 将来のログイン方式追加（Sign in with Apple 等）に設定変更だけで対応できること

## 検討した選択肢

| 選択肢 | 評価 |
|---|---|
| 自前実装 | 却下。パスワード保管・暗号など事故リスクの高い領域を抱える。要件「自前実装を避ける」に反する |
| Firebase Auth | 実用的だが独自トークン方式で、モダンな代替を求める方針からは外れた |
| Auth0 | 標準準拠で高機能だが、無料枠を超えると課金が重い |
| AWS Cognito | デプロイ先が AWS 確定なら有力。現時点でデプロイ先は未定（[architecture.md](../architecture.md)）。設定・DX がやや重い |
| Clerk | iOS SDK の DX が最も優秀。ただし「アプリは BFF 経由でログイン」する前提では iOS SDK の優位性が効かず、独自トークンで BFF が縛られるため不採用 |
| Logto | 認証専用の OSS IdP。OIDC 完全準拠で BFF が綺麗。Supabase と最後まで競合 |
| Zitadel | Go 製の OSS、OIDC ネイティブ。B2B 寄りの機能が今回は過剰 |
| Vercel | 該当する認証サービスなし。「Sign in with Vercel」は開発者向け、「OIDC Federation」はマシン認証、「Auth.js / Better Auth」は JS 専用ライブラリ。Go BFF からエンドユーザー認証には使えない |
| **Supabase Auth** | **採用**（下記） |

## 決定

**認証基盤として Supabase Auth を採用する。**

- Google ログインに対応
- **OAuth 2.1 Authorization Code + PKCE** をサーバーサイドフローでサポートし、BFF が OAuth クライアントとなる本構成に合致する
- **JWKS エンドポイント**を公開しており、非対称署名（RS256 / ES256）に設定すれば Go BFF は汎用ライブラリでトークン検証できる。ベンダーロックインが BFF コードに染み出さない
- ユーザーアカウントの管理（テーブル・アカウント連携）は Supabase 側が担い、BFF は自前でユーザーテーブルを持たない
- 認証に加えて Postgres DB を内包する BaaS であり、BFF が今後データストアを必要とした場合に認証と DB を一本化できる

最終的に Logto と Supabase が残ったが、BFF 側が DB を必要とする可能性を踏まえ、認証と DB を一本化できる Supabase を選んだ。

## 構成

```
iOS アプリ ──▶ BFF /auth/login ──▶ Supabase Auth へリダイレクト（Google ログイン）
                                          │
iOS アプリ ◀── セッション ◀── BFF /auth/callback ◀── 認可コード返却
```

- BFF は confidential OAuth クライアントとして Authorization Code + PKCE フローを実行する
- 認可コードとトークンの交換はサーバー側で行い、Supabase のトークンはアプリに渡さず BFF が扱う
- BFF はアプリ向けに自前のセッションを発行する
- トークン検証は Supabase の JWKS に対する署名・`iss` / `aud` / `exp` 検証で行う（`go-oidc` が鍵キャッシュとローテーションを処理）

## 影響

### 利点

- セキュリティ事故に直結する領域（資格情報保管・MFA・暗号）を自前で実装しない
- BFF の認証コードは標準ライブラリ中心の小さなミドルウェアに収まる
- 認証と DB を 1 サービスに集約でき、運用対象が増えにくい
- 将来のログイン方式追加は Supabase 側の設定で対応できる

### 欠点・留意点

- Supabase の Go SDK は公式提供がなくコミュニティ製。ただしトークン検証は汎用 JWKS 検証で代替でき、SDK 依存を避けられる
- ID トークン検証には Supabase プロジェクトを非対称署名（RS256 / ES256）に設定する必要がある（デフォルトの HS256 のままでは ID トークン発行が失敗する）
- マネージドサービスへの外部依存が増える（可用性・課金体系の継続的な確認が必要）

## 未決事項 / フォローアップ

- **セッションの持ち方**: BFF がアプリへ発行するセッションを「ステートレス署名トークン」とするか「サーバー側セッションストア（Redis 等）」とするか。MVP ではインフラ最小のステートレス署名トークン（短命）を想定するが、別 ADR で確定する
- リフレッシュトークンの取り扱い方針
- Supabase プロジェクトの環境分離（開発 / 本番）

## 参考

- [Supabase OAuth 2.1 Flows](https://supabase.com/docs/guides/auth/oauth-server/oauth-flows)
- [Supabase Auth: SSO, Mobile, and Server-side support](https://supabase.com/blog/supabase-auth-sso-pkce)

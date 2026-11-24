# ADR 0003: BFF のデプロイ先を Cloud Run とする

- ステータス: Accepted
- 日付: 2026-11-24
- 関連: [ADR 0001](./0001-authentication-provider.md)（デプロイ先未定を理由に Cognito を見送った経緯）、[ADR 0002](./0002-session-strategy.md)（web と同一レジストラブルドメインに置く前提）

## コンテキスト

BFF の実装（ヘルスチェック・Google ログイン・セッション管理）が揃い、本番環境で疎通確認できる状態にしたい。[architecture.md](../architecture.md) ではデプロイ先を「未定（サーバーレス / コンテナ常駐のいずれも可能）」としていた。

前提は次のとおり。

- BFF は Go 製の単一バイナリで、状態を持たない（セッションは封緘クッキー。[ADR 0002](./0002-session-strategy.md)）
- web フロントエンドは Vercel の東京リージョン（`hnd1`）に置かれている
- 認証基盤は Supabase（AWS 上のマネージドサービス）であり、特定クラウドへの依存はない
- 運用対象は最小に保ちたい（MVP）

## 決定要因

- コンテナをそのまま動かせ、Go の高速起動を活かせること
- アクセスがない時間はゼロまでスケールし、コストがほぼかからないこと
- 東京リージョンで動かせること（web の `hnd1` と近接させる）
- GitHub Actions から長期の鍵を持たずにデプロイできること
- 秘匿情報（セッション鍵）をマネージドに保管できること

## 検討した選択肢

| 選択肢 | 評価 |
| --- | --- |
| **Cloud Run** | **採用**。コンテナをそのまま動かせ、ゼロスケールする。東京リージョン（`asia-northeast1`）あり。Workload Identity 連携で鍵なしデプロイ、Secret Manager を環境変数として注入できる |
| Vercel Functions（Go ランタイム） | web と同居できるが、Go ランタイムは関数単位の実行モデルで Gin のルータ構成と相性が悪い。BFF を web のデプロイ単位に縛ることにもなる |
| AWS（App Runner / ECS Fargate） | 有力だが、App Runner はゼロスケールせず最低料金が発生する。ECS は ALB や VPC など運用対象が増える |
| Fly.io | 東京リージョンあり・コンテナ常駐で手軽。秘匿情報管理や IAM が GCP / AWS ほど成熟しておらず、将来の拡張で見劣りする |

## 決定

BFF を **Cloud Run（`asia-northeast1`）** にデプロイする。

| 項目 | 方針 |
| --- | --- |
| コンテナ | マルチステージビルド。実行イメージは `gcr.io/distroless/static-debian12:nonroot`（シェルなし・非 root） |
| イメージ保管 | Artifact Registry（`asia-northeast1`） |
| デプロイ | GitHub Actions。`main` へのマージでテスト → ビルド → デプロイ |
| GCP 認証 | Workload Identity 連携（サービスアカウントキーは発行しない） |
| 秘匿情報 | `GANANA_SESSION_KEY` は Secret Manager に保管し、Cloud Run が環境変数として注入する |
| 非秘匿の設定 | GitHub Actions の Variables から `--set-env-vars` で渡す（ワークフローを設定の正とする） |
| 実行権限 | 実行専用サービスアカウント（Secret の読み取りのみ）。デプロイ用とは分ける |
| 公開範囲 | `--allow-unauthenticated`（BFF 自身が認証を担うため） |
| スケール | 最小 0 / 最大 2 インスタンス（MVP。必要に応じて引き上げる） |

### 段階的な公開

[ADR 0002](./0002-session-strategy.md) は「BFF と web を同一のレジストラブルドメインに配置する」ことを前提としている。Cloud Run の既定 URL（`*.run.app`）は web（Vercel）とドメインが異なるため、web からの結合には独自ドメインが必要となる。そこで次の 2 段階で進める。

1. **BFF 単体の疎通確認**（本 ADR の範囲）: `*.run.app` の URL のまま公開する。ログインフローは BFF のホスト内で完結させ（`GANANA_POST_LOGIN_REDIRECT` を BFF の `/auth/me` に向ける）、ヘルスチェック・ログイン・セッション確認までを検証する
2. **web との結合**（フォローアップ）: 独自ドメインを取得し、BFF をそのサブドメインに割り当てる（例: `api.ganana.jp`）。`GANANA_COOKIE_DOMAIN` / `GANANA_ALLOWED_ORIGINS` / `GANANA_POST_LOGIN_REDIRECT` を本番値に切り替える

## 影響

### 利点

- 常時稼働のサーバーを持たず、アクセスがなければコストがほぼかからない
- GitHub に長期の GCP 鍵を置かない
- セッション鍵がリポジトリにも GitHub にも残らない

### 欠点・留意点

- ゼロスケール時はコールドスタートが発生する（Go のため数百ミリ秒程度を想定）。問題になれば最小インスタンスを 1 にする
- インフラ（API 有効化・サービスアカウント・Workload Identity 連携）は初回に手動で作成する。手順は [README](../../README.md#cloud-run-のデプロイ) に記載する
- GCP と Supabase（AWS）のクラウド間通信になる。どちらも東京リージョンに置くことで遅延を抑える

## 未決事項 / フォローアップ

- 独自ドメインの取得と、BFF への割り当て方法（Cloud Run のドメインマッピング、または外部 HTTPS ロードバランサ）
- 環境分離（開発 / 本番）— ADR 0001 / 0002 から継続
- インフラのコード化（Terraform 等）。サービスが増えた時点で検討する

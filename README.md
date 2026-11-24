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

## Supabase のデプロイ

`supabase/` 配下の migration と設定（`config.toml`）は GitHub Actions（[`.github/workflows/supabase.yml`](./.github/workflows/supabase.yml)）で本番プロジェクトへ自動反映する。

| タイミング | 動作 |
| --- | --- |
| `main` 向け PR | `supabase db push --dry-run` で適用予定の migration を、`supabase config diff` で設定の差分を表示する（本番は変更しない） |
| `main` へのマージ | `supabase db push` で migration を適用し、`supabase config push` で設定を反映する |

いずれも `supabase/` 配下またはワークフロー自体に変更がある場合のみ実行される。

Auth の Redirect URL などの設定は、ダッシュボードではなく `config.toml` を編集して PR で変更する。
`config.toml` に書いていない項目（Google のクライアントシークレットなど）は反映時も本番の値が保たれる。
ダッシュボードで直接変更した場合は `supabase config pull` で `config.toml` に取り込むこと（取り込まないと次回の反映で上書きされる）。

リポジトリの Settings > Secrets and variables > Actions に以下を登録する。

| 種別 | 名前 | 内容 |
| --- | --- | --- |
| Secret | `SUPABASE_ACCESS_TOKEN` | Supabase の Personal Access Token（[作成ページ](https://supabase.com/dashboard/account/tokens)。対象プロジェクトに Read-write 権限） |
| Secret | `SUPABASE_DB_PASSWORD` | 本番プロジェクトの DB パスワード |
| Variable | `SUPABASE_PROJECT_ID` | 本番プロジェクトの Project ref（`uhitgoiltoqygdwdskll`） |

migration の追加は `supabase migration new <名前>` で行う。

## Cloud Run のデプロイ

BFF は Cloud Run（`asia-northeast1`）にデプロイする（[ADR 0003](./docs/adr/0003-deploy-target.md)）。
`main` へのマージで GitHub Actions（[`.github/workflows/deploy.yml`](./.github/workflows/deploy.yml)）がテスト → イメージのビルド → デプロイ → `/health` の確認までを行う。
Actions タブから手動実行（`workflow_dispatch`）もできる。

| 項目 | 値 |
| --- | --- |
| サービス名 | `ganana-bff` |
| URL | `https://ganana-bff-<プロジェクト番号>.asia-northeast1.run.app` |
| イメージ | `asia-northeast1-docker.pkg.dev/<プロジェクト ID>/ganana-bff/ganana-bff:<コミット SHA>` |
| 実行サービスアカウント | `ganana-bff-runtime`（Secret の読み取りのみ） |
| デプロイ用サービスアカウント | `ganana-bff-deployer`（GitHub Actions から Workload Identity 連携で利用） |

### 初回のみ: GCP 側の準備

以下を GCP プロジェクトのオーナー権限で一度だけ実行する。

```sh
PROJECT_ID=<GCP のプロジェクト ID>
PROJECT_NUMBER=$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')
REGION=asia-northeast1
REPO=charlie-dev-ios/ganana-bff
gcloud config set project "$PROJECT_ID"

# API の有効化
gcloud services enable run.googleapis.com artifactregistry.googleapis.com \
  secretmanager.googleapis.com iamcredentials.googleapis.com

# イメージの保管先
gcloud artifacts repositories create ganana-bff --repository-format=docker --location="$REGION"

# セッション鍵（値はどこにも保存しない）
openssl rand -base64 32 | tr -d '\n' | gcloud secrets create ganana-bff-session-key --data-file=-

# 実行用サービスアカウント: セッション鍵の読み取りのみ
gcloud iam service-accounts create ganana-bff-runtime
gcloud secrets add-iam-policy-binding ganana-bff-session-key \
  --member="serviceAccount:ganana-bff-runtime@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role=roles/secretmanager.secretAccessor

# デプロイ用サービスアカウント
gcloud iam service-accounts create ganana-bff-deployer
for role in roles/run.admin roles/artifactregistry.writer; do
  gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:ganana-bff-deployer@${PROJECT_ID}.iam.gserviceaccount.com" --role="$role"
done
gcloud iam service-accounts add-iam-policy-binding \
  "ganana-bff-runtime@${PROJECT_ID}.iam.gserviceaccount.com" \
  --member="serviceAccount:ganana-bff-deployer@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role=roles/iam.serviceAccountUser

# Workload Identity 連携: このリポジトリの main ブランチからのみ利用を許可する
gcloud iam workload-identity-pools create github --location=global
gcloud iam workload-identity-pools providers create-oidc github \
  --location=global --workload-identity-pool=github \
  --issuer-uri=https://token.actions.githubusercontent.com \
  --attribute-mapping='google.subject=assertion.sub,attribute.repository=assertion.repository,attribute.ref=assertion.ref' \
  --attribute-condition="assertion.repository == '${REPO}' && assertion.ref == 'refs/heads/main'"
gcloud iam service-accounts add-iam-policy-binding \
  "ganana-bff-deployer@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role=roles/iam.workloadIdentityUser \
  --member="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/github/attribute.repository/${REPO}"

echo "Cloud Run の URL: https://ganana-bff-${PROJECT_NUMBER}.${REGION}.run.app"
```

続いて、リポジトリの Settings > Secrets and variables > Actions > Variables に以下を登録する（GitHub に秘匿情報は置かない）。

| 名前 | 必須 | 内容 |
| --- | :---: | --- |
| `GCP_PROJECT_ID` | ✓ | GCP のプロジェクト ID |
| `GCP_PROJECT_NUMBER` | ✓ | GCP のプロジェクト番号 |
| `GANANA_SUPABASE_URL` | ✓ | `https://uhitgoiltoqygdwdskll.supabase.co` |
| `GANANA_SUPABASE_ANON_KEY` | ✓ | Supabase の publishable（anon）キー |
| `GANANA_POST_LOGIN_REDIRECT` | | ログイン完了後の遷移先。未設定なら BFF の `/auth/me`（疎通確認用） |
| `GANANA_COOKIE_DOMAIN` | | セッションクッキーの `Domain` 属性 |
| `GANANA_ALLOWED_ORIGINS` | | CORS で許可するオリジン（カンマ区切り） |

最後に、Cloud Run の `/auth/callback`（`https://ganana-bff-<プロジェクト番号>.asia-northeast1.run.app/auth/callback`）を
`supabase/config.toml` の `[auth] additional_redirect_urls` に追加し、PR で反映する（[Supabase のデプロイ](#supabase-のデプロイ)）。

### 疎通確認

デプロイ後、以下の順に確認する。1 はワークフローの Smoke test でも自動確認される。

```sh
URL=https://ganana-bff-<プロジェクト番号>.asia-northeast1.run.app

# 1. ヘルスチェック
curl "$URL/health"
# => {"status":"ok"}

# 2. 未ログイン時は 401
curl -i "$URL/auth/me"
# => HTTP/2 401 / {"error":"unauthorized"}
```

3. ブラウザで `$URL/auth/login` を開き、Google でログインする。
   `$URL/auth/me` に戻り、ログインしたユーザーが JSON で表示されれば成功。

ログは `gcloud run services logs read ganana-bff --region asia-northeast1` で確認できる。

## ディレクトリ構成

```
cmd/server/          エントリポイント
internal/auth/       認証エンドポイントと認証ミドルウェア
internal/config/     設定読み込み（Viper）
internal/handler/    HTTP ハンドラ
internal/middleware/ 共通ミドルウェア（CORS）
internal/session/    セッションの封緘・開封（AES-256-GCM）
internal/supabase/   Supabase Auth クライアント
supabase/            Supabase CLI の設定（config.toml）と migration
docs/                設計・開発ドキュメント
```

## ドキュメント

- [docs/architecture.md](./docs/architecture.md) — アーキテクチャと技術選定
- [docs/development.md](./docs/development.md) — 開発原則・規約
- [docs/adr/](./docs/adr/) — アーキテクチャ決定記録（ADR）

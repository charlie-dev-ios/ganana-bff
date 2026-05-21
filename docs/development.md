# Development

Ganana BFF の開発環境・開発フローのドキュメント。

## 開発原則

実装・レビュー時に従う原則。

- **TDD 必須**: テストを先に書き（Red）、最小実装で通し（Green）、整理する（Refactor）の順で進める。テストの後付けや、単に通すだけのアサーションは避ける。
- **YAGNI / Simplicity**: 今必要なものだけを作る。将来用の抽象化・仮実装・使われない引数や設定値を持ち込まない。
- **AI-First**: 人にも AI にも読みやすい、明示的で機械可読なコードと記述を心がける。暗黙の前提や独自記法を避ける。

## 技術スタック

| 項目 | 採用 |
| --- | --- |
| 言語 | Go |
| Web フレームワーク | Gin |
| 設定管理 | Viper |
| ロギング | `log/slog`（標準ライブラリ） |
| HTTP クライアント | 標準 `net/http` + `golang.org/x/sync/errgroup` |
| テスト | 標準 `testing` + testify |

詳細な技術選定理由は [architecture.md](./architecture.md) を参照。

## セットアップ

（TBD）

## ビルド / 実行

（TBD）

## テスト

（TBD）

## コーディング規約

（TBD）

## Git規約

### コミットメッセージ

[Conventional Commits](https://www.conventionalcommits.org/) に準拠し、**日本語で記述**する。

#### フォーマット

```
<type>[optional scope]: <説明>

[optional body]
```

**重要**: Claude が書いた旨のフッター（`Co-Authored-By` など）は含めない。

#### Type 一覧

| Type | 説明 | 例 |
|------|------|-----|
| `feat` | 新機能の追加 | `feat(auth): ログインエンドポイントを追加` |
| `fix` | バグ修正 | `fix(handler): ヘルスチェックのステータスコードを修正` |
| `docs` | ドキュメントのみの変更 | `docs: README にセットアップ手順を追加` |
| `style` | コードの意味に影響しない変更 | `style: import の並びを整理` |
| `refactor` | バグ修正や機能追加を伴わないコード変更 | `refactor: 設定読み込みを config パッケージへ分離` |
| `perf` | パフォーマンス改善 | `perf: 外部 API 呼び出しを並列化` |
| `test` | テストの追加・修正 | `test(handler): 異常系のテストを追加` |
| `chore` | ビルドプロセスや補助ツールの変更 | `chore: 依存関係を更新` |
| `ci` | CI 設定の変更 | `ci: GitHub Actions を追加` |

#### Breaking Changes

破壊的変更がある場合は `!` を付与、またはフッターに `BREAKING CHANGE:` を記載する。

```
feat(api)!: レスポンス形式を変更

BREAKING CHANGE: API レスポンスがネスト構造で返却されるようになりました
```

### ブランチ戦略

| ブランチ | 用途 |
|---------|------|
| `main` | 本番環境 |
| `feature/*` | 機能開発（例: `feature/cha-123-add-login`） |
| `fix/*` | バグ修正（例: `fix/cha-456-validation-error`） |

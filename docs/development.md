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

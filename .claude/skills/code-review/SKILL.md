---
name: code-review
description: 現在のブランチの未マージ変更（origin/main..HEAD の diff）を、独立コンテキストのレビュアー視点でレビューする。「レビューして」「コードレビュー」「diff 見て」「実装をレビューして」「PR 出す前に確認」等の指示があれば必ず発動する。プロジェクトの規約（Constitution / development.md）に照らした観点で、Critical / Recommended / Optional に分類した指摘を返す。linear-implement スキルからも invoke される。
---

# コードレビュー

現在のブランチで `origin/main..HEAD` に積まれている変更を、**白紙のコンテキストでレビューする** ためのガイドライン。

## 前提

- 呼び出し側（人 or 他スキル）から **独立したコンテキスト** で実行されることを想定。実装者のバイアスを排し、純粋に成果物だけを見て判断する
- レビューの一次基準は以下のプロジェクト固有規約
  - `docs/constitution.md`（開発原則: TDD 必須・AI-First・Simplicity / YAGNI）
  - `docs/development.md`（技術スタック・コーディング規約・テスト規約・Git 規約）
- これらに書かれていない汎用的な観点（セキュリティ・可読性・保守性）も併せて確認する

## 入力

通常は引数なし。呼び出し時に対象範囲が指定されていればそれに従う（例: 特定の commit range、特定ファイル）。

## レビュー手順

### 1. 変更内容の把握

```bash
git fetch origin
git log --oneline origin/main..HEAD          # 含まれるコミット
git diff origin/main...HEAD --stat           # 変更ファイル一覧
git diff origin/main...HEAD                  # 実際の diff
```

差分が大きい場合はファイル単位で分割して読む。

### 2. プロジェクト規約の確認

`Read` で以下を取得し、レビュー観点を頭に入れる:

- `docs/constitution.md`
- `docs/development.md`

「規約に書かれているか」を毎回憶測せず、必ず一次情報を参照する。

### 3. 観点別チェック

以下を体系的に確認する。各カテゴリで該当があればメモする。

#### 3.1 Constitution 準拠

- **TDD**: 変更されたロジック・コンポーネントに対応するテストが存在するか。テストが実装より後付けに見える兆候（例: 単純に通すだけの assertion、エッジケース未カバー）はないか
- **Simplicity / YAGNI**: 不要な抽象化・将来用の仮実装・使われていない引数や設定値はないか
- **AI-First**: 機械可読性を損なう独自記法や暗黙の前提はないか

#### 3.2 development.md 準拠

- 技術スタックの選択（Vitest / Biome / Bun / shadcn-ui 等）から逸脱していないか
- 命名規則・ファイル配置（`*.test.ts(x)` 同階層、ケバブ/PascalCase の区別 等）
- TypeScript: `any` の使用、型の緩さ
- コンポーネント分割（200 行超は分割検討）
- Server / Client Component の境界が適切か
- Git 規約（コミットメッセージの type/scope、subject 行の長さ、Claude フッターの混入）

#### 3.3 汎用的な品質

- **可読性**: 命名の妥当性、不要なコメント（「何を」を説明するコメント、現タスク参照、`// removed` 等の墓標）
- **複雑性**: 早期 return で減らせるネスト、共通化されていない明白な重複
- **エラーハンドリング**: 境界（外部 API / ユーザー入力）以外での過剰な防御。逆に境界での見落とし
- **後方互換のための死コード**: 使われていない re-export、`_` プレフィックスだけ付けた未使用変数
- **セキュリティ**: OWASP Top 10（特に XSS、injection、認証/認可、機微情報のログ出力）
- **依存追加**: `package.json` の差分があれば妥当性を確認

#### 3.4 テストの質

- 実装と同じバグを再現するだけのテストになっていないか
- モックが過剰で「実装の rewording」になっていないか
- 失敗時に原因が分かるアサーション・メッセージか

### 4. 静的検証の追認

```bash
bun run test
bun check
```

を実行し、すべてパスすることを確認する。落ちている場合は **それ自体が Critical 指摘**。

### 5. レポート出力

呼び出し側に以下のフォーマットで返す。指摘がなければ「指摘なし」と明記する。

```
## レビュー結果: <ブランチ名>

対象範囲: origin/main..HEAD (<n> commits, <m> files)
静的検証: bun run test = pass/fail, bun check = pass/fail

### Critical（マージ前に必ず修正）
- [<file>:<line>] <指摘内容>。<理由>。<推奨対応>

### Recommended（修正を強く推奨）
- [<file>:<line>] ...

### Optional（任意・好みの範囲）
- [<file>:<line>] ...

### 良かった点（任意）
- ...
```

**重要**:
- `file_path:line_number` 形式で必ず位置を示す（呼び出し側が即ジャンプできるように）
- 「Critical / Recommended / Optional」の区別を厳格に。何でも Critical にすると判断材料にならない
- 推奨対応は **具体的に**（「テストを足す」ではなく「`UserCard.test.tsx` に `name が空のときの表示テスト` を追加」のように）
- プロジェクト規約違反は Critical or Recommended、好みの問題は Optional

## やってはいけないこと

- 規約に書かれていない自分の好みを Critical に格上げする
- ファイルを **編集する**（このスキルはレビューだけ、修正は呼び出し側の責務）
- 指摘がないのに無理に何かを書く（「指摘なし」で良い）
- 推測でファイル外の文脈を仮定する（必要なら `Read` で実物を確認）

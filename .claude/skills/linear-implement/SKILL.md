---
name: linear-implement
description: Linear のチケットから自動的に実装タスクを進める。ユーザーが Linear のチケット ID または URL（例「CHA-123 やって」「https://linear.app/.../issue/CHA-123」「このチケット実装して: ...」）を指定したら、必ずこのスキルを起動すること。ブランチ作成 → 計画 → TDD 実装 → /commit → push → PR 作成までを一気通貫で行うワークフロー。Linear のチケット番号・課題・タスクへの言及があれば、明示的にスキル名が呼ばれていなくても発動する。
---

# Linear チケット実装ワークフロー

Linear のチケットを起点に、ブランチ作成から PR 作成までを一気通貫で進めるための **プロセス手順書**。

実装規約・コミット規約・テスト規約は他所で管理されている。本スキルはそれらに **委譲する** だけで、内容を重複定義しない。

| 領域 | 一次情報源 |
|------|-----------|
| 開発原則（TDD 必須・YAGNI 等） | `docs/constitution.md` |
| 技術スタック・コーディング規約・テスト規約・Git 規約・開発コマンド | `docs/development.md` |
| コミット手順・メッセージ生成 | `commit` スキル（`.claude/commands/commit.md`） |
| コードレビュー観点 | `code-review` スキル（`.claude/skills/code-review/SKILL.md`） |

各 Step で関連ドキュメントを `Read` し、その指示に従うこと。本スキルに書かれた内容と一次情報源に齟齬がある場合は **一次情報源を優先** する。

---

## 全体フロー

```
1. チケット取得 → 2. 計画 → 3. ブランチ → 4. 実装 → 5. 独立レビュー & 改善
                                                       ↓
                                  8. PR ← 7. push ← 6. /commit
```

**ユーザーの明示的な承認を取るゲート**:

- Step 2 → Step 3 へ進む前（計画への合意）
- Step 5 のレビュー結果を踏まえた改善方針
- Step 8 で PR 本文を提出する前

それ以外のステップは自走してよい。

---

## Step 1: チケット情報の取得

`mcp__plugin_linear_linear__get_issue` を `id` にチケット ID（`CHA-123` 形式の human ID で可）を渡して呼び出す。

取得した情報から以下を取り出し、ユーザーに 3〜5 行で要約提示する:

- タイトル / 説明 / ラベル / ステータス
- `gitBranchName`（Step 3 のブランチ名候補となる）

要件理解にコメントが必要なら `mcp__plugin_linear_linear__list_comments` を `issueId` 指定で呼ぶ。

URL で渡された場合は末尾の `/issue/<ID>/` から ID を抽出。曖昧なら必ず確認する。

---

## Step 2: 実装計画の策定

`Glob` / `Grep` / `Read` で関連コードを調査し、以下を含む計画をユーザーに提示する:

1. 変更対象ファイル（新規 / 既存修正の区別）
2. 設計方針（必要な範囲で）
3. テスト方針（TDD 前提でどの粒度のテストを書くか）
4. 動作確認手順
5. リスク / スコープ外事項

**ユーザーの明示的な承認**（「OK」「進めて」等）を得てから次へ。承認なしに実装を始めない。

---

## Step 3: ブランチ作成

ブランチ名は以下の優先順位で決定する:

1. Step 1 で取得した `gitBranchName` があればそれを採用
2. 無ければ `docs/development.md` の Git 規約（`feature/*` / `fix/*`）に従い、**必ず Linear チケット ID を含める**（例 `feature/cha-123-add-login-page`）

理由: Linear-GitHub 連携はブランチ名のチケット ID で PR を自動紐付けするため、ID を含めることが必須。

```bash
git status                                   # クリーンであること
git fetch origin
git switch -c <branch-name> origin/main
```

未コミットの変更がある場合は **stash や破棄を独断で行わず**、ユーザーに確認。

---

## Step 4: 実装

`docs/constitution.md`（TDD 必須・YAGNI）と `docs/development.md`（コーディング・テスト規約）に従って実装する。本スキルでは順序のみ規定:

1. **Red**: 失敗するテストを書き `bun run test` で落ちることを確認
2. **Green**: テストが通る最小実装
3. **Refactor**: 整理して再度テスト
4. **品質チェック**: `bun run test` と `bun check` がパスすること（`--no-verify` 等の回避禁止）
5. **UI 変更時**: `bun dev`（ポート 3030）で起動し手動確認。確認できなかった項目は PR 本文に記載

---

## Step 5: 独立コンテキストでのレビューと改善

実装・テスト・静的解析・フォーマットが Step 4 で完了している前提。**コミット前** にレビューを差し込み、実装者バイアスを排した目で検証する。

### 5.1 レビューの起動（独立コンテキスト）

`Agent` ツールでサブエージェントを起動し、その中で `code-review` スキルを実行させる。サブエージェントは本スキルとは別コンテキストで動くため、実装中の思い込みが持ち込まれない。

```
Agent(
  description: "実装ブランチのコードレビュー",
  subagent_type: "general-purpose",
  prompt: """
  あなたはコードレビュアーとして独立したコンテキストで呼び出されている。
  以下を実行せよ:

  1. `.claude/skills/code-review/SKILL.md` を Read し、そのガイドラインに従う
  2. 現在のブランチ（git branch --show-current）の origin/main..HEAD をレビュー対象とする
  3. レポートを Critical / Recommended / Optional で分類して返す

  対象チケット: <CHA-123 とタイトル>
  実装計画の要点: <Step 2 で合意した内容を 3〜5 行>
  """
)
```

サブエージェントの結果（レビューレポート）を受け取り、そのままユーザーに提示する。

### 5.2 改善方針の決定（ユーザー承認ゲート）

レビュー結果をユーザーに見せ、対応方針を確認する:

- **Critical**: 原則すべて対応
- **Recommended**: 個別にユーザーに対応可否を確認
- **Optional**: デフォルトは対応せず、ユーザーが希望した場合のみ対応

「Critical だけ直して進める」「全部直す」「このコメントは無視」などの指示を受けたら次へ。

### 5.3 改善の実施

決定した方針に沿って修正する。修正後は **必ず再度** 以下を回す:

```bash
bun run test
bun check
```

両方パスしなければ次へ進まない。

### 5.4 再レビューの判断

Critical 指摘を修正した場合や、修正範囲が広い場合は **5.1 に戻って再度レビュー** を回す。

判断基準:

- Critical 1 件以上を修正 → 再レビュー
- Recommended 複数を修正 → 再レビュー推奨
- Optional のみ / 軽微な typo 修正 → 再レビュー不要

再レビューは結果が「指摘なし」または「Optional のみ」になるまで繰り返す。3 回ループしても収束しない場合はユーザーに方針確認すること（無限ループ防止）。

---

## Step 6: コミット

`Skill(skill="commit")` を invoke する。規約とメッセージ生成は commit スキルの責務。本スキルから伝えるのは:

- 含めるべきファイル候補（実装で変更したもの）
- 論理単位が複数あれば分割したい旨（その場合 commit スキルを複数回 invoke）

---

## Step 7: push

```bash
git status
git log --oneline origin/main..HEAD          # 含めるコミットを確認
git push -u origin HEAD
```

`--force` は独断で実行しない。

---

## Step 8: PR 作成

`gh pr create --base main` で作成する。本スキル固有の必須要素:

- **タイトル**: 直近コミットのメッセージ形式に揃える（規約は `docs/development.md` / commit スキルに準拠）
- **本文**: 以下を含める
  - 概要（何を / なぜ）
  - **Linear リンク**: `Closes CHA-123` または Linear チケット URL ── Linear-GitHub 連携が自動でチケットと紐付け、マージ時に完了状態に遷移させる
  - 変更内容
  - テスト / 確認内容（パスしたチェックと未確認項目）
  - スクリーンショット（UI 変更時）

WIP 提出は `--draft`、レビュワー指定は `--reviewer <login>`。`--fill` は Linear リンクが入らないので使わない。

PR 提出前に本文をユーザーに見せ、承認を得てから `gh pr create` を実行する。作成後は PR URL をユーザーに伝える。

> Linear への PR 添付は GitHub 連携が自動で行う。`mcp__plugin_linear_linear__create_attachment` は base64 ファイルアップロード専用（deprecated）で URL 添付には使えないので呼ばない。

---

## エラー時のリカバリ

| 状況 | 対処 |
|------|------|
| テストが通らない | 原因をユーザーに共有してから方針を決める。計画段階に戻すことも検討 |
| 同名ブランチが既存 | suffix を付けるか、ユーザーに確認 |
| Linear からチケットが取れない | ID とワークスペースを再確認 |
| push が拒否される | 状況をユーザーに共有して指示を仰ぐ（force push しない） |
| 既存テスト基盤でカバーしきれない領域 | Step 2 の計画に戻る |

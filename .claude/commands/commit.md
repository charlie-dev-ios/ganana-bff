---
description: Constitution に従ってコミットを実行（対話的にファイル選択可能）
allowed-tools: Bash(git:*), Bash(bun:*)
---

# Constitution準拠コミット

**`../docs/development.md` に記載されているすべてのルールを厳守してコミットを実行してください。**

## 実行手順

### 1. 現在の変更を確認

!`git status`

### 2. ユーザーにファイル選択を依頼

上記の変更ファイルから、どのファイルを add/commit するか選択してもらってください。
- 複数ファイル選択可能
- すべて選択も可能
- ファイルパスを正確に確認

### 3. 選択されたファイルを add

ユーザーが選択したファイルを `git add` してください。

### 4. ステージング内容を確認

!`git diff --staged`

### 5. Commitメッセージの生成

[developoment.md](../docs/development.md)のGit規約を参照してコミットメッセージを生成してください。

コミットメッセージを生成し、ユーザーの承認を得られた場合はコミットを実行してください。

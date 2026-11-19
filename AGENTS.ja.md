# AGENTS.md

コーディングエージェント向けの指示である。人は [README.ja.md](README.ja.md) から読む。

このファイルには、コマンド、置き場所、一般的な既定と違う決まりだけを書く。
決まりの理由は、その決まりが挙げる決定の記録（[docs/adr/](docs/adr/README.ja.md)）にある。

## コマンド

| コマンド | 使いどころ |
|---|---|
| `make setup` | 固定した道具・依存・Git フックを入れる。 |
| `make fmt` | ファイルを整形する。 |
| `make gen` | 生成物を作り直す。 |
| `make check` | 速いチェック。これが通るまで、変更は終わっていない。 |
| `make check-all` | 速いチェックと遅いチェック。CI が実行する。 |

- 作業中は範囲を絞れる。`CHECK_SCOPE=changed make check` か `CHECK_FILES=<path> make check` を使う。最後は絞らずに `make check` を実行する。
- チェックが失敗すると `file:line: [rule] message` の形で出る。原因を直す。ルールを無効にしない。無視するコメントを足さない。ファイルを対象から外さない。
- ルールが今回の場合に合わないと考えたら、作業を止めてそう伝える。決定を変えるには決定の記録が要る（スキル `write-adr`）。

## 置き場所

| パス | 中身 |
|---|---|
| `backend/` | Go の API。モジュールは1つ。区切られた文脈ごとに `internal/<context>/` を置く。変える前に [backend/AGENTS.ja.md](backend/AGENTS.ja.md) を読む。 |
| `frontend/` | Next.js。文脈ごとに `src/features/<context>/` を置く。変える前に [frontend/AGENTS.ja.md](frontend/AGENTS.ja.md) を読む。 |
| `contracts/` | `http/` に OpenAPI の文書、`events/` に AsyncAPI の文書と各イベントの JSON Schema。生成コードと契約のテストの元になる。 |
| `db/` | データベースの初期設定。 |
| `infra/` | Terraform。`modules/` と、それらを AWS Emulator に適用する `local/`。Terragrunt はデプロイとともに加わる。 |
| `tools/` | このリポジトリのチェックと生成器（Go のモジュールとスクリプト）。 |
| `make/` | make のターゲット。分野ごとに1ファイル。 |
| `docs/adr/` | 決定の記録。`template/` はテンプレートのもの。 |
| `docs/contexts/` | 区切られた文脈ごとの README と用語集。 |
| `docs/problems/` | API が返す problem の種類ごとのページ。 |
| `docs/guides/` | エンドポイントを足す、スキーマを変える、といった繰り返す作業のやり方。`.claude/skills/` のスキルはこれに従う。 |
| `.claude/` | Claude Code のフック・スキル・サブエージェント。 |

アプリケーションのディレクトリは、README のマイルストーンが進むと増える。

## 一般的な既定と違う決まり

- **道具。** 道具をグローバルに入れない。`npx`・`go install`・`pip` で実行しない。`mise.toml`（そのあと `make tools-lock`）・`package.json`・`pyproject.toml`・`tools/go.mod` のどれかでバージョンを固定し、`make` から実行する。（TPL-0022）
- **1つの関心事に道具は1つ。** lint や整形の道具を足す前に、固定済みの道具がその関心事を扱っていないかを確かめる。（TPL-0023）
- **文書。** `foo.md` は英語で、これを正とする。`foo.ja.md` はその日本語の対訳である。2つを同じコミットで変え、`make i18n-stamp` を実行する。コード・コメント・コミットメッセージは英語だけで書く。（TPL-0021、[ガイド](docs/guides/translation.ja.md)）
- **決定の記録。** `docs/adr/template/` を書き換えない。新しい決定や決定の変更は `docs/adr/ADR-NNNN-<slug>.md` に書く。（TPL-0001）
- **生成物とロックファイル。** 手で書き換えない。元を直して `make gen` を実行するか、パッケージマネージャーを使う。`schema.sql` を変えたときは、`make gen-db-docs` も実行する（Docker が要る）。
- **スキーマ。** 変えるのは文脈の `schema.sql` だけであり、マイグレーションのファイルはない。`schema-changes.allow` に自分の判断で行を足さない。危険な SQL 文を通すかどうかは、レビューで決めることである。（TPL-0010）
- **スキャナーの例外。** `.trivyignore.yaml` に自分の判断で項目を足さない。Trivy の指摘を認めるかどうかは、レビューで決めることである。指摘がなくなるようにコードを変えるか、作業を止めて報告する。（TPL-0024）
- **コミット。** Conventional Commits に従い、1つのコミットの目的は1つにする。フックを飛ばさない。
- **GitHub Actions。** ワークフローのステップは、チェックアウトと mise の準備のあとは `make` だけを呼ぶ。Action はコミット SHA で固定する。（TPL-0018、TPL-0024）
- **禁止。** `.env` を読まない。force push をしない。`main` に push しない。prd にインフラを適用しない。

<!-- i18n-source: AGENTS.md sha256:a35c1bf3c23a04a71f521aff62d943c77fa5851078da1846dac0baab65679025 -->

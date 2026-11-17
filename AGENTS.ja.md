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
| `backend/` | Go の API。モジュールは1つ。区切られた文脈ごとに `internal/<context>/` を置く。 |
| `frontend/` | Next.js。文脈ごとに `src/features/<context>/` を置く。 |
| `contracts/` | OpenAPI の文書とイベントのスキーマ。生成コードの元になる。 |
| `db/`、`infra/` | データベースの初期設定と、Terraform・Terragrunt。 |
| `tools/` | このリポジトリのチェックと生成器（Go のモジュールとスクリプト）。 |
| `make/` | make のターゲット。分野ごとに1ファイル。 |
| `docs/adr/` | 決定の記録。`template/` はテンプレートのもの。 |
| `docs/guides/` | 繰り返す作業のやり方。 |
| `.claude/` | Claude Code のフック・スキル・サブエージェント。 |

アプリケーションのディレクトリは、README のマイルストーンが進むと増える。

## 一般的な既定と違う決まり

- **道具。** 道具をグローバルに入れない。`npx`・`go install`・`pip` で実行しない。`mise.toml`（そのあと `make tools-lock`）・`package.json`・`pyproject.toml`・`tools/go.mod` のどれかで版を固定し、`make` から実行する。（TPL-0022）
- **1つの関心事に道具は1つ。** lint や整形の道具を足す前に、固定済みの道具がその関心事を扱っていないかを確かめる。（TPL-0023）
- **文書。** `foo.md` は英語で、これを正とする。`foo.ja.md` はその日本語の対訳である。2つを同じコミットで変え、`make i18n-stamp` を実行する。コード・コメント・コミットメッセージは英語だけで書く。（TPL-0021、[ガイド](docs/guides/translation.ja.md)）
- **決定の記録。** `docs/adr/template/` を書き換えない。新しい決定や決定の変更は `docs/adr/ADR-NNNN-<slug>.md` に書く。（TPL-0001）
- **生成物とロックファイル。** 手で書き換えない。元を直して `make gen` を実行するか、パッケージマネージャーを使う。
- **コミット。** Conventional Commits に従い、1つのコミットの目的は1つにする。フックを飛ばさない。
- **GitHub Actions。** ワークフローは `make` だけを呼ぶ。Action はコミット SHA で固定する。（TPL-0018、TPL-0024）
- **禁止。** `.env` を読まない。force push をしない。`main` に push しない。prd にインフラを適用しない。

<!-- i18n-source: AGENTS.md sha256:a2988957ff3db4ed46fde85c4d015dd1355ec099698a59fda55c44d257e6a59c -->

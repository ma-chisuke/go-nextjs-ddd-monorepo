# 文脈を足す

区切られた文脈は、コード・API の文書・データベース・文書のどこでも同じ名前を持つ（TPL-0003）。
まず、その名前を `facility` のような小文字の1語に決める。名前が Go のパッケージ、スキーマ、2つのロールの接頭辞になるからである。
次に、業務領域の種類を決める。種類によって、何をコピーするかが決まる。補完の文脈（トランザクションスクリプト）なら Facility を、中核の文脈（ポートとアダプターを備えたドメインモデル）なら Reservation をコピーする（[ARCHITECTURE.ja.md](../../ARCHITECTURE.ja.md)）。

## 手順

1. API の文書 `contracts/http/<context>.openapi.yaml` を書く。`facility.openapi.yaml` から、すべての API に共通する部分をコピーする。`info`・`servers`・コメント付きの `security: []`（API に届くのはプライベートなネットワークからだけである。TPL-0014）・応答 `Problem`・スキーマ `Problem` と `InvalidParam` である。そのうえで操作を書く。

   確認：`make check` が `lint-contracts` を実行する。

2. 手本の文脈のコードを `backend/internal/<context>/` にコピーし、古い名前を含むものの名前を変える。

   - `module.go`。`Deps` と `NewHandler` を持つ。
   - `internal/app/`。ユースケースと業務エラーである。中核の文脈では、ポートと、`app/apptest/` のメモリ上の偽物も含む。
   - `internal/domain/`。中核の文脈だけに置く。
   - `internal/adapter/http/`。変えずに使う `oapi-codegen.yaml` と、`handler.go`・`problems.go`・`problems_test.go` である。
   - `internal/adapter/postgres/`。`schema.sql`・`queries/`・`sqlc.yaml` である。`sqlc.yaml` の設定 `database`・`analyzer`・`rules` と、末尾のルール `no-unfiltered-write` は、`lint-sqlc` が頼るので変えない。

   生成された `*.gen.go` のファイルはコピーせず、`make gen` を実行する。`gen-backend.sh` は、`adapter/http/` の API の文書を文脈の名前で見つける。
   ほかの文脈の API を呼ぶには、`internal/adapter/<other>/oapi-codegen.yaml` を足す。中身は Reservation の `adapter/facility/` と同じく、`client: true` と `models: true` である。`make gen` は、そのために `contracts/http/<other>.openapi.yaml` を読む。

   確認：`make check`。`lint-arch` はすべてのルールをディレクトリから導くので、新しい文脈に設定は要らない。

3. `cmd/facility/main.go` をもとに、コマンド `backend/cmd/<context>/main.go` を書く。コマンドは `DATABASE_URL`・`HTTP_ADDR`・`LOG_LEVEL` を読み、`time/tzdata` を import し、`<context>.NewHandler` を呼ぶ。`module.go` のパッケージを import してよいのは、このコマンドだけである。

   確認：ほかのパッケージがそれを import すると、`make check` が失敗する（`arch/module-import`）。

4. 文脈にスキーマとロールを与える。`db/bootstrap.sql` の Facility の部分をコピーし、`facility` を文脈の名前に置き換える。中身は、ロール `<context>_owner` と `<context>_runtime`、所有者のロールが所有するスキーマとそれに対する実行時のロールの権限、両方のロールの検索パス `<context>, extensions` である。2つのロールのパスワードを `db/local.sql` に足す。そのうえで、[スキーマを変える](change-a-schema.ja.md)のとおりに `schema.sql` を書き、各テーブルについて `<context>_runtime` への `GRANT` を含める。

   ```sh
   make db-up        # 2つのファイルをもう一度実行し、すべてのスキーマを適用する
   make gen-db-docs  # docs/generated/db/<context>/ を書く
   ```

   PostgreSQL がこの2つのファイルを自分で実行するのは、データディレクトリを作るときだけである。そのため、すでにある手元のデータベースでは `make db-up` がそれらを実行する。

5. 用語集 `docs/contexts/<context>/glossary.md` と `glossary.ja.md` を書く。見出し行が `| Identifier | Term | Meaning |`（日本語では `| 識別子 | 用語 | 意味 |`）の表を1つ置き、`domain`（補完の文脈では `app`）がエクスポートする型ごとに行を書く。

   確認：エクスポートされた型に行がないと、`make check` が失敗する（`arch/glossary`）。

6. 文脈の README `docs/contexts/<context>/README.md` とその対訳を、Facility と Reservation の README と同じ形で書く。受け持つもの・業務領域の種類とその理由・API と problem・イベント・決まり・ほかの文脈から必要とするものである。`ARCHITECTURE.md` の表にも、この文脈を足す。

7. `problems.go` の新しい slug ごとに、`docs/problems/` のページを書く。そして `make i18n-stamp` を実行する。

8. 仕上げる。

   ```sh
   make fmt
   make check
   CHECK_SCOPE=changed make check-all
   ```

## 欠けているものと、それを教えるチェック

| 欠けているもの | 教えるチェック |
|---|---|
| API の文書 | `make gen` と `gen-check-api`（`gen/no-document`） |
| `handler.go` の操作 | コンパイラー（`lint-go` と `test-go`） |
| 最新の生成コード | `gen-check-api`、`gen-check-sqlc` |
| 用語集、またはその行 | `lint-arch`（`arch/glossary`） |
| 対訳 | `lint-i18n` |
| 業務エラーの種類に対する problem、またはそのページ | `lint-go`（exhaustive）と、`problems_test.go` のテスト |
| クエリを prepare する `sqlc.yaml` の設定 | `lint-sqlc`（`sqlc/no-db-prepare`） |
| `db/bootstrap.sql` のロールやスキーマ、または `db/local.sql` のパスワード | `make db-up` と、スキーマを適用する `make check-all` の各チェック（`pgschema/apply`） |
| 実行時のロールへの `GRANT` | そのロールで接続する、文脈の結合テスト（`test-integration`） |
| テーブルや列のコメント、またはスキーマの文書 | `lint-db-docs` |
| 文脈の README | まだチェックはなく、レビューで見つける（TPL-0003） |

<!-- i18n-source: add-a-context.md sha256:42dc5ab2d1b5be148bdd6afb43968eac1c623877dcd3464961777ed744326d54 -->

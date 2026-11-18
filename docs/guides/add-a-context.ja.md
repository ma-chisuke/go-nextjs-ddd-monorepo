# 文脈を足す

区切られた文脈は、コード・契約・データベース・文書のどこでも同じ名前を持つ（TPL-0003）。契約とは、API を持つ文脈の API の文書と、イベントを発行するか受け取る文脈の AsyncAPI 文書である。
まず、その名前を `facility` のような小文字の1語に決める。名前が Go のパッケージ、スキーマ、2つのロールの接頭辞になるからである。
次に、業務領域の種類と、文脈がリクエストに答えるのか、イベントを受け取るだけなのかを決める。これらによって、何をコピーするかが決まる（[ARCHITECTURE.ja.md](../../ARCHITECTURE.ja.md)）。

- API を持つ補完の文脈なら、Facility をコピーする。トランザクションスクリプトである。
- 中核の文脈なら、Reservation をコピーする。ポートとアダプターを備えたドメインモデルである。
- API を持たない文脈なら、Notification をコピーする。ほかの文脈のイベントを受け取るワーカーで、トランザクションスクリプトで書く。

## 手順

1. 文脈の契約を書く。

   API を持つ文脈では、API の文書 `contracts/http/<context>.openapi.yaml` を書く。`facility.openapi.yaml` から、すべての API に共通する部分をコピーする。`info`・`servers`・コメント付きの `security: []`（API に届くのはプライベートなネットワークからだけである。TPL-0014）・応答 `Problem`・スキーマ `Problem` と `InvalidParam` である。そのうえで操作を書く。

   イベントを受け取る文脈では、`notification.asyncapi.yaml` の形で `contracts/events/<context>.asyncapi.yaml` を書く。受け取るイベントごとに `receive` の操作を置き、発行側の文書のチャネルとメッセージを参照する。ワーカーには API の文書がない。購読に要るほかのものは、[イベントを変更する](event-changes.ja.md#イベントを購読する)にある。

   確認：`make check` が、どちらの文書にも `lint-contracts` を実行する。

2. 手本の文脈のコードを `backend/internal/<context>/` にコピーし、古い名前を含むものの名前を変える。

   - `module.go`。`Deps` と `NewHandler` を持つ。API の `NewHandler` は `http.Handler` を返し、ワーカーの `NewHandler` は、Notification と同じく `event.Handler`（`platform/event`）を返す。
   - `internal/app/`。ユースケースと業務エラーである。中核の文脈と、ポートを持つ補完の文脈（Notification の `Mailer` など）では、ポートと、`app/apptest/` のメモリ上の偽物も含む。
   - `internal/domain/`。中核の文脈だけに置く。
   - `internal/adapter/http/`。API を持つ文脈だけに置く。変えずに使う `oapi-codegen.yaml` と、`handler.go`・`problems.go`・`problems_test.go` である。
   - `internal/adapter/postgres/`。`schema.sql`・`queries/`・`sqlc.yaml` である。`sqlc.yaml` の設定 `database`・`analyzer`・`rules` と、末尾のルール `no-unfiltered-write` は、`lint-sqlc` が頼るので変えない。

   生成された `*.gen.go` のファイルはコピーせず、`make gen` を実行する。API を持つ文脈では、`gen-backend.sh` が、ディレクトリ `adapter/http/` と文脈の名前から API の文書を見つける。`gen-frontend.sh` は、フロントエンドのための API の TypeScript の型 `frontend/src/lib/api/<context>.gen.ts` を書く。文脈に画面を持たせるには、[画面を足す](add-a-screen.ja.md)に従う。
   ほかの文脈の API を呼ぶには、`internal/adapter/<other>/oapi-codegen.yaml` を足す。中身は Reservation の `adapter/facility/` と同じく、`client: true` と `models: true` である。`make gen` は、そのために `contracts/http/<other>.openapi.yaml` を読む。
   ワーカーは、Notification の `internal/app/events.go` のように、自分のモデル変換装置で各イベントのデータを読む。配信は少なくとも1回なので、処理した各イベントの id を、その効果と同じトランザクションの中で、一意制約のもとに記録する（TPL-0013）。

   確認：`make check`。`lint-arch` はすべてのルールをディレクトリから導くので、新しい文脈に設定は要らない。

3. コマンド `backend/cmd/<context>/main.go` を書く。`module.go` のパッケージを import してよいのは、このコマンドだけである。

   - API なら、`cmd/facility/main.go` をもとにする。コマンドは `DATABASE_URL`・`HTTP_ADDR`・`LOG_LEVEL` を読み、`time/tzdata` を import し、`<context>.NewHandler` を `httpserver.Run` で動かす。
   - ワーカーなら、`cmd/notification/main.go` をもとにする。コマンドはさらにキュー（`EVENTS_QUEUE_URL`）とリージョン（`AWS_REGION`）を読み、`awsconfig.Load` で AWS SDK の設定を読み込む。そして、`platform/awsevents` の受け取る側を、文脈のハンドラーとともに `httpserver.RunWorker` の中で動かす。`httpserver.RunWorker` は、その隣でヘルスチェックに答える。
   - API の隣のワーカーは、Reservation の `relay` のように、同じコマンドのモードにする。`httpserver.Main` に `httpserver.Mode{Name: "<mode>", Run: ...}` を渡す。

   確認：ほかのパッケージが `module.go` のパッケージを import すると、`make check` が失敗する（`arch/module-import`）。

4. 文脈にスキーマとロールを与える。`db/bootstrap.sql` の Facility の部分をコピーし、`facility` を文脈の名前に置き換える。中身は、ロール `<context>_owner` と `<context>_runtime`、所有者のロールが所有するスキーマとそれに対する実行時のロールの権限、両方のロールの検索パス `<context>, extensions` である。2つのロールのパスワードを `db/local.sql` に足す。そのうえで、[スキーマを変える](change-a-schema.ja.md)のとおりに `schema.sql` を書き、各テーブルについて `<context>_runtime` への `GRANT` を含める。

   ```sh
   make db-up        # 2つのファイルをもう一度実行し、すべてのスキーマを適用する
   make gen-db-docs  # docs/generated/db/<context>/ を書く
   ```

   PostgreSQL がこの2つのファイルを自分で実行するのは、データディレクトリを作るときだけである。そのため、すでにある手元のデータベースでは `make db-up` がそれらを実行する。
   Git のワークツリーでは、`make db-up` は別のチェックアウトが起動したコンテナを作り直してしまう。代わりに `make db-server` を実行し、作業に専用のデータベースを与える。`PGDATABASE=<name> tools/scripts/db.sh init` が、そのデータベースを作り、2つのファイルを実行し、すべてのスキーマを適用する。

5. ワーカーを手元のシステムで動かす。`compose.yaml` に、`notification` の形でサービスを足す。中身は、`backend` からビルド引数 `CONTEXT` でビルドするイメージ、`x-aws-emulator` の環境変数、CA のためのボリューム `aws-emulator`、自分のキューの URL である。`infra/local/main.tf` では、[イベントを変更する](event-changes.ja.md#イベントを購読する)のとおりに、受け取るイベントを文脈に購読させる。

   確認：`make up` がワーカーを起動し、正常になるまで待つ。

6. 用語集 `docs/contexts/<context>/glossary.md` と `glossary.ja.md` を書く。見出し行が `| Identifier | Term | Meaning |`（日本語では `| 識別子 | 用語 | 意味 |`）の表を1つ置き、`domain`（補完の文脈では `app`）がエクスポートする型ごとに行を書く。

   確認：エクスポートされた型に行がないと、`make check` が失敗する（`arch/glossary`）。

7. 文脈の README `docs/contexts/<context>/README.md` とその対訳を、Facility・Reservation・Notification の README と同じ形で書く。受け持つもの・業務領域の種類とその理由・API と problem（ないならそのこと）・イベント・決まり・ほかの文脈から必要とするものである。`ARCHITECTURE.md` の表にこの文脈を足し、プロセスの表にそのプロセスを足す。

8. API を持つ文脈では、`problems.go` の新しい slug ごとに `docs/problems/` のページを書く。そして `make i18n-stamp` を実行する。

9. 仕上げる。

   ```sh
   make fmt
   make check
   CHECK_SCOPE=changed make check-all
   ```

## 欠けているものと、それを教えるチェック

| 欠けているもの | 教えるチェック |
|---|---|
| API を持つ文脈の API の文書 | `make gen` と `gen-check-api`（`gen/no-document`） |
| `handler.go` の操作 | コンパイラー（`lint-go` と `test-go`） |
| 最新の生成コード | `gen-check-api`、`gen-check-sqlc`、`gen-check-api-ts`（新しい API の文書の TypeScript の型がないときは `gen/missing`） |
| 用語集、またはその行 | `lint-arch`（`arch/glossary`） |
| 対訳 | `lint-i18n` |
| 業務エラーの種類に対する problem、またはそのページ | `lint-go`（exhaustive）と、`problems_test.go` のテスト |
| クエリを prepare する `sqlc.yaml` の設定 | `lint-sqlc`（`sqlc/no-db-prepare`） |
| `db/bootstrap.sql` のロールやスキーマ、または `db/local.sql` のパスワード | `make db-up` と、スキーマを適用する `make check-all` の各チェック（`pgschema/apply`） |
| 実行時のロールへの `GRANT` | そのロールで接続する、文脈の結合テスト（`test-integration`） |
| テーブルや列のコメント、またはスキーマの文書 | `lint-db-docs` |
| 文脈が発行するイベントの JSON Schema、または AsyncAPI 文書のそのメッセージ | `lint-events` |
| 受け取る側の AsyncAPI 文書、または `infra/local/main.tf` のその購読 | まだチェックはなく、レビューで見つける（TPL-0013）。購読がないことに `make smoke` が気づくのは、ワーカーの結果を待つときだけである |
| 文脈の README | まだチェックはなく、レビューで見つける（TPL-0003） |

<!-- i18n-source: add-a-context.md sha256:37f449261ac7500f7e48d769f6aa2a740785231f574a55ce8b7437d2018bf98b -->

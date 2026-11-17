# AGENTS.md（backend）

`backend/` で作業するコーディングエージェント向けの指示であり、ルートの [AGENTS.ja.md](../AGENTS.ja.md) に足すものである。地図は [ARCHITECTURE.ja.md](../ARCHITECTURE.ja.md) にある。

## コマンド

| コマンド | 使いどころ |
|---|---|
| `make gen` | `contracts/http/` からサーバーとクライアントを（oapi-codegen）、`schema.sql` と `queries/` から型付きのクエリを（sqlc）生成し直す。 |
| `make db-up` | 手元の PostgreSQL を Docker で起動し、すべての `schema.sql` を適用する。 |
| `make db-server` | 手元の PostgreSQL が動いていないときだけ起動し、何も変えない。Git のワークツリーではこれを使う。`make db-up` は、ほかのチェックアウトが起動したコンテナを作り直す。 |
| `make db-plan`、`make db-apply` | 手元のデータベースを各 `schema.sql` の状態にする DDL を表示し、実行する。 |
| `make test-integration` | すべてのスキーマを適用してから、ビルドタグ `integration` のテストを手元の PostgreSQL に対して実行する。 |
| `make gen-db-docs` | `schema.sql` を変えたあと、`docs/generated/db/<context>/` を書き直す。 |

- `make db-down` は、すべてのチェックアウトの手元のデータを消す。データベースを1つ共有しているからである。
- 手順は、[エンドポイントを足す](../docs/guides/add-an-endpoint.ja.md)・[スキーマを変える](../docs/guides/change-a-schema.ja.md)・[文脈を足す](../docs/guides/add-a-context.ja.md)・[サービスを手元で動かす](../docs/guides/run-locally.ja.md)にある。スキルは `add-endpoint`・`change-schema`・`add-context` である。

## 文脈の中のコードの置き場所

| コード | `internal/<context>/` の下の置き場所 |
|---|---|
| アダプター・ユースケース・ハンドラーの組み立て | `module.go`。import するのは `cmd/<context>` だけ |
| 中核の文脈の決まり・イベント・業務エラー | `internal/domain/` |
| 1ファイルに1つのユースケース、ポート、ユースケースのエラー | `internal/app/` |
| strict server の操作と、エラーから problem への対応表 | `internal/adapter/http/` |
| `schema.sql`、`queries/`、生成したクエリ、ストア | `internal/adapter/postgres/` |
| ほかの文脈の生成したクライアントと、この文脈の言葉への変換 | `internal/adapter/<other>/` |

すべての文脈が使い、どの文脈のものでもないコードは `internal/platform/` に置く。

## チェックが守らせる決まり

- `arch/context-import`：ほかの文脈のパッケージを import しない。その API を生成したクライアントで呼ぶか、イベントを使う。（TPL-0005）
- `arch/module-import`：`module.go` のパッケージを import するのは `cmd/<context>` だけである。（TPL-0005）
- `arch/platform-import`：`platform/` はどの文脈も import しない。（TPL-0004）
- `arch/layer`：`adapter` は `app` と `domain` を、`app` は `domain` を import し、アダプターどうしは import し合わない。`app` が自分の `adapter/postgres` を import してよいのは、`domain` のない文脈だけである。（TPL-0003、TPL-0004）
- `arch/domain-purity`・`arch/domain-clock`：ドメインは標準ライブラリのうち許したパッケージだけを import し、時計を読まない。時刻と新しい ID は引数で受け取る。（TPL-0004）
- `arch/route-registration`：生成コードと `platform/httpserver` の外で `Handle` や `HandleFunc` を呼ばない。代わりに API の文書に操作を足す。（TPL-0007）
- `arch/schema-reference`：SQL には、自分のスキーマの名前も、ほかの文脈のスキーマの名前も書かない。（TPL-0005、TPL-0009）
- `arch/glossary`：`domain`（`domain` のない文脈では `app`）がエクスポートする型には、文脈の両方の用語集に行がある。（TPL-0003）
- `gen-check-api`・`gen-check-sqlc`：生成コードは `make gen` が書く内容と同じである。（TPL-0007）
- exhaustive と `problems_test.go`：業務エラーの種類にはそれぞれ problem があり、problem にはそれぞれ `docs/problems/` のページがある。（TPL-0008）
- `platform/apitest`：API のテストはこのクライアントを通す。クライアントは、すべての応答を API の文書に照らして検証する。（TPL-0007）
- `make check-all` の `lint-sql-plan`・`lint-sqlc`・`lint-db-docs`：スキーマの変更ごとの安全な DDL、prepare できるクエリ、すべてのテーブルと列のコメント。（TPL-0010、TPL-0011、TPL-0023）

<!-- i18n-source: AGENTS.md sha256:b90631825d380a47cbfd613c92be6cd1364a1b341227cc5545711119a50ac76f -->

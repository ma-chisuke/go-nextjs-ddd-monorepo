# エンドポイントを足す

エンドポイントは、文脈の API の文書の操作として始まり、`make check` が通ったときに完成する。
手順では、文脈 `<context>` のファイルを名指しする。各層が何を持つかは [ARCHITECTURE.ja.md](../../ARCHITECTURE.ja.md) にある。
Facility のようにドメイン層のない補完の文脈では、手順3を飛ばす。

## 手順

1. `contracts/http/<context>.openapi.yaml` に操作を書く。パス・`operationId`・`summary`・パラメーター・本文と、操作が返しうるすべての応答である。4xx と 5xx の応答は、どれも `#/components/responses/Problem` を参照する。説明は英語で書く。

   確認：`make check` が `lint-contracts` と `lint-contracts-breaking` を実行する。`lint-contracts` は、`operationId` のない操作や、`application/problem+json` でないエラーの応答で失敗する。新しい操作は、どのクライアントとの互換性も壊さない。既存の操作を変えるときは、[API を変更する](api-changes.ja.md)を読む。

2. コードを生成する。

   ```sh
   make gen
   ```

   oapi-codegen が、文脈のサーバー `internal/adapter/http/api.gen.go` と、ほかの文脈にあるこの API のクライアントを書き直す。クライアントの例は、Reservation の `internal/adapter/facility/client.gen.go` である。

   確認：ビルドが `missing method <Operation>` で失敗する。`handler.go` が、操作を実装する型が生成された `StrictServerInterface` を満たすことを確かめているからである。`make check` は、これを `lint-go` と `test-go` で報告する。

3. 中核の文脈では、`internal/domain/` に決まりを書く。新しい決まりは、現在時刻を含む値を受け取り、値・イベント・新しい `Kind` の `*domain.Error` を返す関数かメソッドである。その `Kind` は `Kinds()` に加える。テストは素の関数呼び出しで書く。

   確認：ドメインが、許してあるパッケージの一覧にないパッケージを import すると（`arch/domain-purity`）、または時計を読むと（`arch/domain-clock`）、`make check` が失敗する。

4. `internal/app/` にユースケースを書く。
   中核の文脈では、ユースケースは専用のファイルに置く `Reservations` のメソッドである。ドメインが必要とするものを `ports.go` のポートを通して読み込み、ドメインを呼び、結果を、ドメインが返したドメインイベントとともに保存する。ストアは、そのイベントを同じトランザクションの中で送信箱に書く。新しい種類のドメインイベントには、先に統合イベントとその契約が要る。[イベントを変更する](event-changes.ja.md#イベントを足す)に従う。外から新しく必要になったものはポートのメソッドにし、アダプターがそれを実装する。
   補完の文脈では、入力を確かめて生成したクエリを呼ぶメソッドである。
   ドメインの決まりではないユースケースのエラーは、新しい `app.Kind` の `*app.Error` にし、その種類を `app.Kinds()` に加える。

5. ユースケースが読み書きするものに合うクエリがなければ、`internal/adapter/postgres/queries/<table>.sql` に足す。クエリは `-- name: <Name> :one`（または `:many`、`:exec`）の行に続けて書き、もう一度 `make gen` を実行する。
   中核の文脈では、ポートの新しいメソッドを `adapter/postgres/store.go` に実装し、そこで行をドメインの値に詰め替える。
   同じインターフェースを実装するメモリ上の偽物にも、そのメソッドを足す。Reservation では `internal/app/apptest/`、Facility では `internal/adapter/postgres/postgrestest/` である。
   テーブルを変える必要があれば、先に[スキーマを変える](change-a-schema.ja.md)に従う。

   確認：`make check` が `gen-check-sqlc` を実行し、コンパイラーはメソッドの欠けた偽物をそれぞれ挙げる。`make check-all` は、さらにクエリをスキーマに対して prepare し（`lint-sqlc`）、結合テストを実行する。

6. `internal/adapter/http/handler.go` に操作を実装する。コンパイラーが求めるメソッドは、生成されたリクエストをユースケースの入力に詰め替え、ユースケースを呼び、結果を生成された応答に詰め替える。業務エラーは、同じファイルのほかの操作と同じやり方で返す。`problems.go` が、そのエラーを対応する problem に変える。

7. 新しい種類の業務エラーに problem を与える。`internal/adapter/http/problems.go` に、新しい slug・タイトル・ステータスを持つ `problem.Type` を足し、switch にその種類の case を足す。ステータスは、操作が API の文書で宣言しているものにする。ページ `docs/problems/<slug>.md` をほかのページと同じ形で書き、対訳 `docs/problems/<slug>.ja.md` も書いて、`make i18n-stamp` を実行する。

   確認：switch にその種類がない間（`lint-go` のリンター exhaustive）と、ページがない間（`problems_test.go` のテスト）、`make check` は失敗する。

8. `internal/adapter/http/handler_test.go` で、`platform/apitest` のクライアントを通して操作をテストする。成功するリクエストと、操作が返しうる problem ごとのリクエストを送る。

   確認：`make check` がテストを実行する。クライアントはすべての応答を API の文書に照らして検証するので、文書にないステータス・欠けたヘッダー・形の違う本文は、テストを失敗させる。

9. 文脈の文書を更新する。`docs/contexts/<context>/README.md` の操作の表と、変更が `domain`（補完の文脈では `app`）からエクスポートした型ごとの `glossary.md` の行である。対訳も変え、`make i18n-stamp` を実行する。

   確認：エクスポートされた型の行が両方の用語集にない間（`arch/glossary`）と、対訳が古い間（`lint-i18n`）、`make check` は失敗する。

10. 仕上げる。

    ```sh
    make fmt
    make check
    CHECK_SCOPE=changed make check-all   # PostgreSQL（Docker）を要するチェックを加える
    ```

<!-- i18n-source: add-an-endpoint.md sha256:5c9b1f9e613cb91340e7606feb60986db3833b8c028282c8341216eaca0d57f0 -->

# イベントを変更する

`contracts/events/<context>/<Name>.v<N>.json` にあるイベントの JSON Schema は、そのイベントを受け取る文脈との契約である（TPL-0013）。
受け取る側は、スキーマの1つのバージョンに合わせて書かれており、発行側が変わったあとも、そのバージョンのイベントを読み続ける。
`make check` の2つのチェックが、この契約を守る。`lint-events` は、各スキーマが正しい形をしているかを調べる。`lint-events-breaking` は、各スキーマをベースブランチ上のバージョンと比べ、そのバージョンの受け取る側が読めなくなる変更で失敗する。

## 変更の仕方

そのバージョンを受け取る側がどれも読み続けられる変更は、そのバージョンのファイルに直接書く。

- 新しいプロパティ。必須かどうかは問わない。受け取る側は、知らないものを無視する。
- 必須に変わったプロパティ。
- `description`・`title`・`examples`・`$comment`・`deprecated` の変更。

それ以外の変更は、イベントの新しいバージョンにする。

1. `<Name>.v<N>.json` を、同じディレクトリの `<Name>.v<N+1>.json` にコピーし、コピーのほうを変更する。古いファイルは、そのまま残す。新しいファイルの名前は、新しいバージョンの型 `<context>.<Name>.v<N+1>` から決まる。
2. [イベントを足す](#イベントを足す)のとおりに、新しいバージョンのメッセージを AsyncAPI 文書に足す。
3. すべての受け取る側が新しいバージョンを読むようになるまで、両方のバージョンを発行する。古いスキーマには `"deprecated": true` を付ける。これは互換性を保つ変更である。

ベースにあったスキーマのファイルを削除したり、その名前を変えたりすると、`lint-events-breaking` は失敗する。そのバージョンを読む受け取る側がもういない場合も同じである。

## lint-events が調べること

`lint-events` は、`contracts/events/` の中で名前が `.json` で終わるファイルを、すべて調べる。スキーマは文脈の AsyncAPI 文書に依存するので、イベントの契約のどれかを変えると、`CHECK_SCOPE` や `CHECK_FILES` の値にかかわらず、すべてのスキーマを調べる。

- `events/file-name`：パスが `contracts/events/<context>/<Name>.v<N>.json` である。文脈は英小文字、名前は UpperCamelCase、バージョンは先頭に 0 を付けない正の整数で書く。
- `events/json`：ファイルが1つの JSON の値であり、どのオブジェクトにも同じキーが2回現れない。
- `events/dialect`：`$schema` が `https://json-schema.org/draft/2020-12/schema` である。
- `events/compile`：スキーマが JSON Schema 2020-12 としてコンパイルできる。`$ref` は、スキーマと同じ場所にあるファイルを指してよい。ネットワークからは何も読まない。
- `events/title`：`title` が `<Name>` である。
- `events/description`：スキーマに `description` があり、どの深さのプロパティにもある。どれも、空白だけではない文字列である。
- `events/type`：スキーマが JSON のオブジェクトであり、その `type` が `"object"` である。
- `events/closed`：ファイルのどのスキーマも、`additionalProperties` や `unevaluatedProperties` を `false` にしない。受け取る側は、知らないプロパティを無視する。しかも、閉じたオブジェクトでは、プロパティを足すたびに互換性を壊す変更になる。
- `events/required`：ファイルのどのスキーマでも、`required` の各名前が、同じスキーマの `properties` のキーである。
- `events/examples`：`examples` に1つ以上の要素があり、どの要素も、形式（`format`）まで含めてスキーマに合う。
- `events/asyncapi`：文脈の AsyncAPI 文書 `contracts/events/<context>.asyncapi.yaml` に、次のメッセージがある。`name` が `<context>.<Name>.v<N>` であり、`payload` が `$ref` でこのファイルを参照するメッセージである。参照は直接でも、文書の中の参照（`#/...`）を経由してもよい。ここでメッセージとは、文書の中で `name` と `payload` を持つマッピングのすべてを指す。

「どの深さの」と「ファイルのどの」は、スキーマの中にあるすべてのスキーマを指す。それらを持つのは、次のキーワードである。

- `properties`・`patternProperties`・`dependentSchemas`・`$defs` の各メンバー
- `allOf`・`anyOf`・`oneOf`・`prefixItems` の要素
- 配列とオブジェクトに当てはまる `items`・`additionalProperties`・`unevaluatedProperties`・`unevaluatedItems`・`contains`・`propertyNames` の値
- `not`・`if`・`then`・`else`・`contentSchema` の値

## lint-events-breaking が調べること

`lint-events-breaking` は、範囲内の各スキーマを、ベースにある同じファイルと比べる。ベースは、CI と同じく、`origin/main`（`CHECK_BASE` を設定したときはその値）とのマージベースである。
ベースにないスキーマは新しいので、チェックは通る。ベースがなければ、何も比べない。

2つのバージョンは、ルートからキーワードごとに比べる。中にあるスキーマも、すべて同じ規則で比べる。

- `description`・`title`・`examples`・`$comment`・`deprecated` は変えてよい。
- `events/property-removed`：ベースの `properties` のキーが、すべて残っている。新しいキーは通る。
- `events/required-removed`：ベースの `required` の名前が、すべて残っている。新しい名前は通る。
- `events/keyword-changed`：それ以外のキーワードは、どれもベースと同じ値を持つ。ベースになかったキーワードの追加と、ベースにあったキーワードの削除も、この指摘になる。値の取りうる範囲を狭めるだけの変更でも同じである。スキーマを値に持つキーワードは、値全体ではなく、スキーマごとに比べる。
  - `items`・`additionalProperties`・`unevaluatedProperties`・`unevaluatedItems`・`contains`・`propertyNames`：両方のバージョンのスキーマを比べる。
  - `not`・`if`・`then`・`else`・`contentSchema`：両方のバージョンのスキーマを比べる。
  - `allOf`・`anyOf`・`oneOf`・`prefixItems`：両方のバージョンでスキーマの数が同じであり、それらを同じ順に比べる。
  - `patternProperties`・`dependentSchemas`：両方のバージョンでキーが同じである。`$defs` だけは、キーが増えてよい。
- `events/schema-deleted`：ベースのスキーマのファイルが、すべて同じパスに残っている。削除したファイルは範囲に入らないので、スキーマを削除するだけの変更が失敗するのは、すべてのファイルを対象にした実行である。範囲を絞らない `make check` がそれにあたり、CI もそれを実行する。

各指摘は、その場所の JSON Pointer と、変更前と変更後の値を示す。直し方は、新しいバージョンを作ることである。変更を `<Name>.v<N+1>.json` として発行し、古いバージョンのファイルは、ベースの内容のまま残す。

## イベントを足す

1. `contracts/events/<context>/<Name>.v1.json` にスキーマを書く。形は `contracts/events/reservation/ReservationConfirmed.v1.json` にならい、オブジェクトは閉じない。書くのは次のものである。

   - `$schema`・`title`・`"type": "object"`・`required`
   - 何が起きたのかと、文脈がいつ発行するのかを述べる `description`
   - 各プロパティの `description`
   - 実際のデータの例を入れた `examples`

   確認：`lint-events`。

2. 文脈の AsyncAPI 文書 `contracts/events/<context>.asyncapi.yaml` にイベントを足す。文書がなければ、`contracts/events/reservation.asyncapi.yaml` の形で書く。足すのは次の3つである。

   - `<context>.<Name>.v1` という名前のメッセージ。そのペイロードは、`data: {$ref: ./<context>/<Name>.v1.json}` を持つ CloudEvents の封筒である。
   - 文脈のチャネルに置く、そのメッセージ
   - そのメッセージを送る操作

   確認：`lint-contracts` が文書を lint する。`lint-events` は、メッセージがスキーマを参照するまで失敗する。

3. 発行側で、ドメインイベントを統合イベントに詰め替える。`adapter/postgres/events.go` の `integrationEvent` に case を足し、そのデータの構造体を書く。イベントは、`event.New` と型 `event.Type("<context>", "<Name>", 1)` で作る。発行側のテストでは、発行側が作る各イベントを `eventtest.AssertMatchesContract`（`backend/internal/platform/event/eventtest`）に通す。データがその型のスキーマに形式まで含めて合わなければ、テストは失敗する。

   手本は `backend/internal/reservation/internal/adapter/postgres/events_test.go` である。このテストは、ドメインイベントの種類ごとに見本を持ち、それぞれを詰め替えと `AssertMatchesContract` に通す。見本のない種類のイベントがドメインに加わると、このテストは失敗する。詰め替えのないドメインイベントを含む変更は、ストアが拒む。

   確認：`test-go` と `make test-integration`。後者は Docker が必要で、1分ほどかかる。リレーのテストの1つが、発行したイベントのデータ全体を、期待する値と比べる。

4. そのイベントを処理する各受け取る側で、受け取る側のモデル変換装置でデータを読む。そして、スキーマのすべての例を、受け取る側がそれぞれについて行うことに変えるテストを書く。手本は `backend/internal/notification/internal/app/events_test.go` である。このテストはスキーマをファイルとして読むので、新しい例もそのまま試される。そのうえで、[イベントを購読する](#イベントを購読する)のとおりに、受け取る側の購読にそのイベントを足す。

   確認：`test-go`。

## イベントを購読する

新しい受け取る側は、発行側を変えずに購読できる（TPL-0013）。Notification と同じ形で、次のものを用意する。

1. 受け取る側の AsyncAPI 文書 `contracts/events/<context>.asyncapi.yaml`。受け取るイベントごとに、`receive` の操作を書く。チャネルは発行側のチャネルを参照し（たとえば `$ref: ./reservation.asyncapi.yaml#/channels/reservationEvents`）、各操作はそのチャネルのメッセージを参照する。形は `contracts/events/notification.asyncapi.yaml` にならう。`lint-contracts` がこれを lint する。この文書のない受け取る側を報告するチェックはない。
2. `infra/local/main.tf` の購読。モジュール `infra/modules/event-subscription` を呼ぶ。渡すのは、文脈の名前、バス `module.event_bus.name`、各発行側の `source`（`/contexts/<publisher>`）、各イベントの `type` である。モジュールが作るのは、キュー `<context>-events` とそのデッドレターキュー `<context>-events-dlq`、そしてルールである。ルールは、選んだイベントの封筒をキューへ送る。`make aws-apply` が、これを AWS Emulator に適用する。AWS では、購読は受け取る側のユニットが持つ予定である。
3. ハンドラーとコマンド。`module.go` の `NewHandler` は `event.Handler` を返し、`cmd/<context>` はそれを使って `platform/awsevents` の受け取る側を動かす（[文脈を足す](add-a-context.ja.md)の手順3）。ハンドラーは冪等にする。処理した各イベントの id を、その効果と同じトランザクションの中で、一意制約のもとに記録する。処理しない種類のイベントには nil を返す。
4. `compose.yaml` のサービス。形は `notification` にならい、`EVENTS_QUEUE_URL` を `https://sqs.us-east-1.amazonaws.com/000000000000/<context>-events` にする。エミュレーターはキューに AWS と同じ形の名前を付け、その URL に自分のエンドポイントで答える。

確認：`make up` がワーカーを起動し、`make test-integration` がその結合テストを実行する。`make smoke` が購読まで確かめるのは、Notification のメールを待つように、ワーカーの結果を待つときだけである。

<!-- i18n-source: event-changes.md sha256:805ec2579bc2166998b635f118836099ec1ece7d700b933f0b13b7d66d74190e -->

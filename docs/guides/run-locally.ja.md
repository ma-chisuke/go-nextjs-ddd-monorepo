# サービスを手元で動かす

このガイドでは、手元のシステムを起動し、API を通して curl で会議室を予約する。そして、その予約について Notification が送るメールを見る。
要るのは、Docker と、`make setup` で入れる道具である。

## システムを起動する

```sh
make up
```

`make up` は、Docker Compose ですべてを次の順に起動する。

1. PostgreSQL 18。`db/bootstrap.sql` と `db/local.sql` を実行し、各文脈の `schema.sql` を適用する（`tools/scripts/db.sh up`）。
2. AWS Emulator。`infra/local/aws-emulator/Dockerfile` からビルドする（TPL-0017）。
3. イベントバス `contexts` と、Notification のキュー（デッドレターキューとルールも含む）。Terraform が `infra/local/` からエミュレーターの中に作る（`tools/scripts/aws.sh apply`）。エミュレーターの起動後の最初の apply には1分ほどかかる。AWS の provider が、キューを1つ作るたびに25秒ほど待つからである。変更のない apply は数秒で終わる。
4. Mailpit と、すべての文脈のすべてのプロセス。プロセスは `backend/Dockerfile` からビルドする。Facility と Reservation の API、Reservation の中継（`reservation relay`）、Notification のワーカー（`notification`）である。`make up` は、それぞれが正常になるまで待つ。

起動には `make up` を使い、`docker compose up` だけでは起動しない。スキーマと AWS のリソースは、mise が固定した pgschema と Terraform で、ホストから適用する。中継は、自分のバスがなければ起動しない。

各サービスは、ループバックアドレスでだけ待ち受ける。

| サービス | アドレス | 別のポートにする変数 |
|---|---|---|
| PostgreSQL | `127.0.0.1:55432` | `POSTGRES_PORT` |
| AWS Emulator | `https://localhost:59297` | `AWS_PORT` |
| Mailpit の Web の画面 | `http://127.0.0.1:58025` | `MAILPIT_PORT` |
| Facility | `http://127.0.0.1:58081` | `FACILITY_PORT` |
| Reservation | `http://127.0.0.1:58082` | `RESERVATION_PORT` |

中継とワーカーは、ポートを公開しない。Mailpit は、ポート 1025 の SMTP をほかのコンテナからだけ受け付ける。

確認：`curl -i http://127.0.0.1:58081/healthz` と `curl -i http://127.0.0.1:58082/healthz` が `204 No Content` を返す。`docker compose ps` は、すべてのサービスを healthy と示す。

## 会議室を予約する

1. Facility に会議室を登録する。

   ```sh
   curl -i -X POST http://127.0.0.1:58081/rooms \
     -H 'Content-Type: application/json' \
     -d '{"name":"Aoba","capacity":8,"openFrom":"09:00","openUntil":"18:00","timeZone":"Asia/Tokyo"}'
   ```

   答えは `201 Created` で、本文に会議室が、`Location` にその URL が入る。その ID を控える。

   ```sh
   room=<会議室の ID>
   ```

   Aoba という名前の会議室がすでにあれば、答えは problem room-name-taken を持つ `409 Conflict` になる。ほかの名前を選ぶか、`curl http://127.0.0.1:58081/rooms` でその会議室を探す。

2. 2日後の東京の 10:00 から 11:00 まで、会議室を予約する。

   ```sh
   day=$(date -d '+2 days' +%F 2> /dev/null || date -v+2d +%F)   # GNU の date、または macOS の date
   curl -i -X POST http://127.0.0.1:58082/reservations \
     -H 'Content-Type: application/json' \
     -H "Idempotency-Key: aoba-$day-10" \
     -d "{\"roomId\":\"$room\",\"startsAt\":\"${day}T10:00:00+09:00\",\"endsAt\":\"${day}T11:00:00+09:00\",\"attendees\":4,\"organizerEmail\":\"organizer@example.com\"}"
   ```

   Reservation は Facility に会議室を問い合わせ、決まりを適用して、`201 Created` で答える。本文に予約が、`Location` にその URL が、`ETag` に `"1"` が入る。Reservation は、日時を UTC で書く。その ID を控える。

   ```sh
   reservation=<予約の ID>
   ```

   同じ `Idempotency-Key` で同じリクエストをもう一度送ると、同じ予約が返り、新しい予約はできない。

3. 読んだバージョンを示して、予約を変更する。

   ```sh
   curl -i -X PATCH http://127.0.0.1:58082/reservations/$reservation \
     -H 'Content-Type: application/json' -H 'If-Match: "1"' -d '{"attendees":6}'
   ```

   答えは `ETag: "2"` を持つ `200 OK` である。同じリクエストをもう一度送ると `412 Precondition Failed` になる。予約がもうバージョン1ではないからである。

4. 予約を取り消す。

   ```sh
   curl -i -X POST http://127.0.0.1:58082/reservations/$reservation/cancellation -H 'If-Match: "2"'
   ```

   答えは、状態 `cancelled` と `ETag: "3"` を持つ `200 OK` である。

5. Facility を止め、新しい `Idempotency-Key` で手順2の予約を繰り返す。Reservation は、problem facility-unavailable と `Retry-After: 10` を持つ `503 Service Unavailable` で答える。そのあとで、Facility をもう一度起動する。

   ```sh
   docker compose stop facility
   # 手順2の予約を、別の Idempotency-Key で送る
   docker compose start facility
   ```

## メールを見る

予約・変更・取り消しは、それぞれ Reservation の送信箱にイベントを1つ書いた。数秒のうちに、中継が各イベントをバスに発行し、Notification のルールがそれを自分のキューに送り、ワーカーが Mailpit にメールを送る。

<http://127.0.0.1:58025> を開く。`organizer@example.com` 宛てのメールが3通ある。予約と変更の2通は件名が `Reservation confirmed` で始まり、取り消しの1通は `Reservation cancelled` で始まる。手順2で繰り返した予約と、拒まれたリクエストは、メールを送っていない。

各プロセスは、ログに1行ずつ JSON のレコードを書く。イベントを追うには、中継とワーカーのログを読む。各レコードには `event_id` がある。

```sh
docker compose logs reservation-relay notification
```

Mailpit はメールを一時的なファイルシステムに置くので、コンテナが止まるとメールは消える。

## キューを見る

テンプレートは AWS CLI を固定していない。AWS CLI を入れてあれば、`tools/scripts/aws.sh` が出力する環境変数で、エミュレーターに向けられる。

```sh
eval "$(tools/scripts/aws.sh env)"
aws sqs list-queues
aws sqs get-queue-attributes --attribute-names ApproximateNumberOfMessages \
  --queue-url https://sqs.us-east-1.amazonaws.com/000000000000/notification-events-dlq
aws events list-rules --event-bus-name contexts
```

この環境変数は、`AWS_ENDPOINT_URL` でエミュレーターのポートを指し、`AWS_CA_BUNDLE` で CA を指す。CA は、`make up` が `.cache/aws-emulator/ca.pem` にコピーしたものである。認証情報は仮のものを設定する。エミュレーターは署名を確かめない。エミュレーターはキューに AWS と同じ形の名前を付け（アカウントは `000000000000`）、その URL に自分のエンドポイントで答える。

ワーカーが5回処理できなかったメッセージは、`notification-events-dlq` で待つ。ワーカーのログには、試すたびのエラーが残る。

## AWS Emulator が再起動したとき

AWS Emulator は状態をメモリにしか持たず、起動のたびに新しい CA を作る。そのコンテナが再起動すると、バスとキューは消え、中継とワーカーはその証明書を信頼しなくなる。中継とワーカーは、証明書やキューがないことについてのエラーをログに書き、試し直し続ける。

1. バスとキューを作り直し、新しい CA を `.cache/aws-emulator/ca.pem` にコピーする。

   ```sh
   make aws-apply
   ```

   `make up` もこれを行うが、すでに動いているサービスは再起動しない。
2. エミュレーターを呼ぶサービスを再起動し、新しい CA を読ませる。

   ```sh
   docker compose restart reservation-relay notification
   ```

その間、イベントは送信箱で待つので、失われない。中継は、発行できるようになった時点でそれらを発行する。
`make aws-server` は、ほかのサービスを起動せずにエミュレーターだけを起動し、その CA をコピーする。結合テストはこれを使い、`make test-integration` と `make smoke` は動く前にエミュレーターを起動する。

## API をソースから動かす

自分でビルドした API を動かすには、`go run` で手元の PostgreSQL に対して起動する。各コマンドは設定を環境変数から読む。変数の一覧は、`backend/cmd/<context>/main.go` の先頭のコメントにある。どちらの API も既定では `:8080` で待ち受けるので、それぞれに別のポートを与える。
`mise exec --` は、`mise.toml` で指定した Go を `PATH` に入れる。mise を有効にしたシェルでは要らない。

`make up` を実行していなければ、`make db-up` が PostgreSQL だけを起動し、スキーマを適用する。Git のワークツリーでは `make db-server` を使う。これは PostgreSQL が動いていないときだけ起動し、別のチェックアウトが起動したコンテナには手を触れない。

1つ目のターミナルで、Facility を実行時のロールで起動する。

```sh
cd backend
DATABASE_URL='postgres://facility_runtime:facility_runtime@127.0.0.1:55432/app?sslmode=disable' \
HTTP_ADDR=127.0.0.1:8081 \
mise exec -- go run ./cmd/facility
```

2つ目のターミナルで Reservation を起動し、Facility の場所を伝える。

```sh
cd backend
DATABASE_URL='postgres://reservation_runtime:reservation_runtime@127.0.0.1:55432/app?sslmode=disable' \
FACILITY_API_URL=http://127.0.0.1:8081 \
HTTP_ADDR=127.0.0.1:8082 \
mise exec -- go run ./cmd/reservation
```

[会議室を予約する](#会議室を予約する)の呼び出しは、これらのポート `8081` と `8082` に対してもそのまま使える。`make up` の中継とワーカーが動いていれば、これらの予約のイベントも Mailpit に届く。中継は同じ送信箱を読むからである。

中継をソースから動かすには、`tools/scripts/aws.sh` の環境変数でエミュレーターに向ける。

```sh
make aws-server aws-apply
eval "$(tools/scripts/aws.sh env)"
cd backend
DATABASE_URL='postgres://reservation_runtime:reservation_runtime@127.0.0.1:55432/app?sslmode=disable' \
EVENT_BUS_NAME=contexts \
HTTP_ADDR=127.0.0.1:8083 \
mise exec -- go run ./cmd/reservation relay
```

1つの送信箱の上で複数の中継が動けるので、この中継と `make up` の中継はイベントを分け合う。Notification のワーカーは Mailpit に SMTP で送る必要があり、そこに届くのはコンテナだけなので、ワーカーは `make up` で動かす。

## テスト用のデータベース

`make test-integration` は、ビルドタグ `integration` の付いたテストを、同じ PostgreSQL にある専用のデータベース `app_test` で動かす。`tools/scripts/db.sh init` が、すべてのスキーマとともにこれを作る。手元のシステムはデータベース `app` を使う。そのため、動いているシステムの中継がテストの書いた送信箱のイベントを取ることはなく、テストがシステムの行に出会うこともない。

## スモークテストと、止め方

`make smoke` は、サービスが動いていなければ起動し、HTTP で Facility と Reservation をまたぐ予約を1件通す。そして、すべての応答を2つの API の文書と突き合わせ、その予約の3通のメールを Mailpit で待つ。`make check-all` がこれを実行する。

```sh
make smoke
```

`make down` はすべてのコンテナを止め、PostgreSQL のデータは残す。AWS Emulator の状態と Mailpit のメールは消える。PostgreSQL のデータを消すのは `make db-down` である。

## AWS Emulator を更新する

AWS Emulator のバージョンを更新する Renovate のルールはまだない（TPL-0017）。そのため、更新は `infra/local/aws-emulator/Dockerfile` を手で変えて行う。

1. Dockerfile の先頭のコメントが示す一覧で、新しいバージョンを探す。
2. `EMULATOR_VERSION` と、2つのチェックサム `EMULATOR_SHA256_ARM64` と `EMULATOR_SHA256_AMD64` を変える。各パッケージ（`aarch64` 用と `x86_64` 用）のチェックサムは、`curl --location <パッケージの URL> | shasum -a 256` で得る。パッケージの URL は、Dockerfile の `RUN` 命令にある。
3. `make up` でイメージをビルドしてエミュレーターのコンテナを作り直し、`CHECK_SCOPE=changed make check-all` を実行する。新しいバージョンがテンプレートの頼る動きを続けているかは、`platform/awsevents` の結合テストと `make smoke` でわかる。パッケージがチェックサムや AWS の署名に合わなければ、ビルドが失敗する。

<!-- i18n-source: run-locally.md sha256:98e981887ef49ea59d2068ca680248be72d47956a26c6d83ea9e7c690452f6d3 -->

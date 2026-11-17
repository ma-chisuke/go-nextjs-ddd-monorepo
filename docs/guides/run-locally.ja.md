# サービスを手元で動かす

このガイドでは、Facility と Reservation を手元の PostgreSQL に対して動かし、それらの API を通して curl で会議室を予約する。
要るのは、Docker と、`make setup` で入れる道具である。

## データベースを起動する

```sh
make db-up
```

Docker Compose で PostgreSQL 18 を `127.0.0.1:55432` に起動し、`db/bootstrap.sql` と `db/local.sql` を実行して、各文脈の `schema.sql` を適用する。いつ実行し直してもよく、そのたびにスキーマを最新にする。
`make db-down` は PostgreSQL を止め、そのデータを消す。

## 2つのサービスを動かす

各コマンドは設定を環境変数から読む。変数の一覧は、`backend/cmd/<context>/main.go` の先頭のコメントにある。どちらも既定では `:8080` で待ち受けるので、それぞれに別のポートを与える。
`mise exec --` は、`mise.toml` で指定した Go を `PATH` に入れる。mise を有効にしたシェルでは要らない。

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

確認：`curl -i http://127.0.0.1:8081/healthz` と `curl -i http://127.0.0.1:8082/healthz` が `204 No Content` を返す。各サービスは、リクエストごとに JSON を1行、ターミナルに書く。

## 会議室を予約する

1. Facility に会議室を登録する。

   ```sh
   curl -i -X POST http://127.0.0.1:8081/rooms \
     -H 'Content-Type: application/json' \
     -d '{"name":"Aoba","capacity":8,"openFrom":"09:00","openUntil":"18:00","timeZone":"Asia/Tokyo"}'
   ```

   答えは `201 Created` で、本文に会議室が、`Location` にその URL が入る。その ID を控える。

   ```sh
   room=<会議室の ID>
   ```

   Aoba という名前の会議室がすでにあれば、答えは problem room-name-taken を持つ `409 Conflict` になる。ほかの名前を選ぶか、`curl http://127.0.0.1:8081/rooms` でその会議室を探す。

2. 2日後の東京の 10:00 から 11:00 まで、会議室を予約する。

   ```sh
   day=$(date -d '+2 days' +%F 2> /dev/null || date -v+2d +%F)   # GNU の date、または macOS の date
   curl -i -X POST http://127.0.0.1:8082/reservations \
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
   curl -i -X PATCH http://127.0.0.1:8082/reservations/$reservation \
     -H 'Content-Type: application/json' -H 'If-Match: "1"' -d '{"attendees":6}'
   ```

   答えは `ETag: "2"` を持つ `200 OK` である。同じリクエストをもう一度送ると `412 Precondition Failed` になる。予約がもうバージョン1ではないからである。

4. 予約を取り消す。

   ```sh
   curl -i -X POST http://127.0.0.1:8082/reservations/$reservation/cancellation -H 'If-Match: "2"'
   ```

   答えは、状態 `cancelled` と `ETag: "3"` を持つ `200 OK` である。

5. Facility を Ctrl+C で止め、新しい `Idempotency-Key` で手順2の予約を繰り返す。Reservation は、problem facility-unavailable と `Retry-After: 10` を持つ `503 Service Unavailable` で答える。

サービスは Ctrl+C で止める。

## サービスをコンテナで動かす

次のコマンド1つで、すべてのサービスのイメージをビルドし、システム全体を Docker Compose で起動する。起動するのは、すべてのスキーマを適用した PostgreSQL・Facility・Reservation である。

```sh
make up
```

- Facility は `http://127.0.0.1:58081`、Reservation は `http://127.0.0.1:58082` で待ち受ける。別のポートを使うときは、`FACILITY_PORT` か `RESERVATION_PORT` を設定する。
- 各サービスは実行時のロールで PostgreSQL に接続する。Reservation は Compose のネットワークを通して Facility を呼ぶ。
- 起動には `make up` を使い、`docker compose up` だけでは起動しない。`make up` は、mise が固定した pgschema でスキーマを先に適用する。

前の節の呼び出しは、これらのポートに対してもそのまま使える。

`make smoke` は、サービスが動いていなければ起動し、HTTP で2つのサービスをまたぐ予約を1件通して、すべての応答を2つの API の文書と突き合わせる。`make check-all` がこれを実行する。

```sh
make smoke
```

`make down` はサービスと PostgreSQL を止め、データは残す。データを消すのは `make db-down` である。

<!-- i18n-source: run-locally.md sha256:768b17a880006ff744c6b9f96a988cc7a87d6230efa71a6174f6159cc54d59a0 -->

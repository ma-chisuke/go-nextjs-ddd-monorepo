# Facility

Facility は、人が予約できる会議室を受け持つ。会議室の名前・定員・利用時間・タイムゾーンである。
ある時刻に会議室が空いているかや、誰が予約したかは受け持たない。それは Reservation が受け持つ。
以下の言葉は[用語集](glossary.ja.md)の言葉である。

## 業務領域の種類

補完の業務領域である。会議室は、誰かが一度登録してほかの人が読むデータであり、決まりは2つしかない。製品をほかと分ける要素はここにない。
そこで Facility はトランザクションスクリプトで書く（TPL-0003）。`domain` パッケージを持たず、`internal/app/` のユースケースが入力を確かめて、sqlc が生成したクエリを呼ぶ。
曜日ごとに違う利用時間のように決まりが増えたら、そのときこの文脈は Reservation と同じようなドメインモデルに移す。

## 公開する API

[`contracts/http/facility.openapi.yaml`](../../../contracts/http/facility.openapi.yaml) である。ほかの文脈は、会議室をこの API からだけ読み、データベースからは読まない。

| 操作 | すること | この操作に固有の problem |
|---|---|---|
| `POST /rooms`（`registerRoom`） | 会議室を登録し、201 と、`Location` に入れたその URL で答える | [room-name-taken](../../problems/room-name-taken.ja.md)、[invalid-opening-hours](../../problems/invalid-opening-hours.ja.md)、[unknown-time-zone](../../problems/unknown-time-zone.ja.md) |
| `GET /rooms`（`listRooms`） | 会議室を名前の順に、1ページずつ一覧する | [invalid-cursor](../../problems/invalid-cursor.ja.md) |
| `GET /rooms/{roomId}`（`getRoom`） | 1つの会議室を返す | [room-not-found](../../problems/room-not-found.ja.md) |

どの操作も、すべての API に共通する problem で答えることもある。[malformed-request](../../problems/malformed-request.ja.md)・[validation-error](../../problems/validation-error.ja.md)・[unsupported-media-type](../../problems/unsupported-media-type.ja.md)・[not-found](../../problems/not-found.ja.md)・[method-not-allowed](../../problems/method-not-allowed.ja.md)・[internal-error](../../problems/internal-error.ja.md) である。
この API は独自の認証を持たない。この API に届くのはプライベートなネットワークからだけだからである（TPL-0014）。

## イベント

Facility はイベントを発行せず、購読もしない。

## 決まり

- 会議室には、ほかのどの会議室とも違う名前がある。
- 定員は 1 から 1000 までの整数である。
- 開く時刻は閉まる時刻より早い。そのため、日付をまたいで開いている会議室はない。
- タイムゾーンは IANA のタイムゾーンであり、開く時刻と閉まる時刻はこのタイムゾーンで読む。

最初の3つの決まりは、データベースも `rooms` の制約として持つ。タイムゾーンを確かめるのはユースケースだけである。

## ほかの文脈から必要とするもの

ない。

<!-- i18n-source: README.md sha256:5b4603f68bb93b47c18afa45d2f30995fd3ba6170ebdeffefdcb3bfdee6d7d40 -->

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

## 画面

フロントエンドは、機能 `frontend/src/features/facility/` で会議室を見せる。この機能は、Next.js のサーバーからだけこの API を呼ぶ（TPL-0014）。

| ページ | 見せるもの | 操作 |
|---|---|---|
| `/` | 会議室の一覧。名前の順に並び、それぞれが自分のページへのリンクを持つ | `listRooms`。最大100件の1ページ |
| `/rooms/new` | 会議室を登録するフォーム。タイムゾーンは、Next.js のサーバーが知っているものから選ぶ | `registerRoom` |
| `/rooms/{roomId}` | 会議室の名前・定員・利用時間・タイムゾーン。その下に Reservation の予約がある | `getRoom` |

- フォームは、room-name-taken を名前についてのメッセージとして表示する。invalid-opening-hours は閉まる時刻の項目に、unknown-time-zone はタイムゾーンの項目に表示する（`registration.ts`）。ほかの problem は、title と detail をそのまま表示する。
- 存在しない会議室では、`frontend/src/app/rooms/[roomId]/not-found.tsx` のページを表示する。

<!-- i18n-source: README.md sha256:4b60719d4c6d5c6758eec208a43dd8c978e1cb585adedbd661900d7d90c58d19 -->

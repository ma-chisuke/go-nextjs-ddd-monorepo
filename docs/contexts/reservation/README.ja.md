# Reservation

Reservation は、会議室の予約を受け持つ。ある時間帯に会議室を予約し、予約を変更し、予約を取り消す。1つの会議室の確定した予約どうしが重ならないようにする。
会議室そのものは受け持たない。会議室が要るたびに Facility へ問い合わせ、写しは持たない。予約を主催者に知らせるのは Notification の役目であり、Notification は Reservation のイベントから予約のことを知る。
以下の言葉は[用語集](glossary.ja.md)の言葉である。

## 業務領域の種類

中核の業務領域である。予約の決まりこそが製品をほかと分ける要素であり、それらの決まりは互いに依存する。会議室のタイムゾーンによって時間帯がどの日に属するかが決まり、その日によって利用時間が決まる。重なりは、同時に届いた2つのリクエストの間でも起こりうる。
そこで Reservation は、ポートとアダプターを備えたドメインモデルで書く（TPL-0003、TPL-0004）。
`internal/domain/` の集約 `Reservation` は、I/O をせずに決まりを適用し、現在時刻と新しい ID を引数で受け取る。`internal/app/` のユースケースは、集約が必要とするものをポートを通して読み込み、結果を保存する。

## 公開する API

[`contracts/http/reservation.openapi.yaml`](../../../contracts/http/reservation.openapi.yaml) である。

| 操作 | すること | この操作に固有の problem |
|---|---|---|
| `POST /reservations`（`bookRoom`） | 会議室を予約し、201 で予約を返す。`Location` にはその URL が、`ETag` にはそのバージョンが入る。`Idempotency-Key` が要る。同じキーと同じ本文で繰り返したリクエストは、最初のリクエストが作った予約を受け取る | 予約の決まり（下記）、[unknown-room](../../problems/unknown-room.ja.md)、[room-already-booked](../../problems/room-already-booked.ja.md)、[idempotency-key-reused](../../problems/idempotency-key-reused.ja.md)、[facility-unavailable](../../problems/facility-unavailable.ja.md) |
| `GET /reservations`（`listReservations`） | 確定した予約と取り消された予約を、始まりの順に1ページずつ一覧する。1つの会議室か、ある期間で絞り込める | [invalid-cursor](../../problems/invalid-cursor.ja.md) |
| `GET /reservations/{reservationId}`（`getReservation`） | 1つの予約を、`ETag` に入れたそのバージョンとともに返す | [reservation-not-found](../../problems/reservation-not-found.ja.md) |
| `PATCH /reservations/{reservationId}`（`rescheduleReservation`） | 確定した予約の時間帯か参加人数を変える。クライアントが読んだバージョンを `If-Match` に入れる必要がある | 予約の決まり、[reservation-cancelled](../../problems/reservation-cancelled.ja.md)、[reservation-not-found](../../problems/reservation-not-found.ja.md)、[unknown-room](../../problems/unknown-room.ja.md)、[room-already-booked](../../problems/room-already-booked.ja.md)、[facility-unavailable](../../problems/facility-unavailable.ja.md)、[precondition-required](../../problems/precondition-required.ja.md)、[precondition-failed](../../problems/precondition-failed.ja.md) |
| `POST /reservations/{reservationId}/cancellation`（`cancelReservation`） | 確定した予約を取り消す。`If-Match` が要る | [cancellation-too-late](../../problems/cancellation-too-late.ja.md)、[reservation-cancelled](../../problems/reservation-cancelled.ja.md)、[reservation-not-found](../../problems/reservation-not-found.ja.md)、[precondition-required](../../problems/precondition-required.ja.md)、[precondition-failed](../../problems/precondition-failed.ja.md) |

予約の決まりのうち時間帯の決まりに反すると、[period-out-of-order](../../problems/period-out-of-order.ja.md)・[period-misaligned](../../problems/period-misaligned.ja.md)・[period-too-long](../../problems/period-too-long.ja.md)・[period-in-past](../../problems/period-in-past.ja.md) のどれかで答える。参加人数と会議室の決まりに反すると、[no-attendees](../../problems/no-attendees.ja.md)・[over-capacity](../../problems/over-capacity.ja.md)・[outside-opening-hours](../../problems/outside-opening-hours.ja.md) のどれかで答える。
どの操作も、すべての API に共通する problem で答えることもある。[malformed-request](../../problems/malformed-request.ja.md)・[validation-error](../../problems/validation-error.ja.md)・[unsupported-media-type](../../problems/unsupported-media-type.ja.md)・[not-found](../../problems/not-found.ja.md)・[method-not-allowed](../../problems/method-not-allowed.ja.md)・[internal-error](../../problems/internal-error.ja.md) である。
この API は独自の認証を持たない。この API に届くのはプライベートなネットワークからだけだからである（TPL-0014）。

## イベント

Reservation は2つの統合イベントを発行する。[`contracts/events/reservation.asyncapi.yaml`](../../../contracts/events/reservation.asyncapi.yaml) がそれらを記述し、[`contracts/events/reservation/`](../../../contracts/events/reservation/) の JSON Schema がそれぞれのデータを定める。

| イベント | Reservation が発行するとき |
|---|---|
| `reservation.ReservationConfirmed.v1` | 会議室を予約したとき。予約を変更するたびにも、新しい時間帯と参加人数で改めて発行する |
| `reservation.ReservationCancelled.v1` | 予約を取り消したとき |

- ドメインは、変更のたびにそのドメインイベント（`ReservationConfirmed` か `ReservationCancelled`）を返す。ストアはそれを統合イベントに変え、変更と同じトランザクションの中でテーブル `outbox` に書く（TPL-0013）。統合イベントは CloudEvents の封筒であり、その `subject` は予約の ID である。したがって、変更がコミットされればイベントも必ず保存され、コミットされなければ保存されない。拒否された変更はイベントを保存しない。同じ冪等キーで再び届いた予約のリクエストも、イベントを保存しない。
- プラットフォームの中継（`internal/platform/outbox`）は、イベントの ID の順に送信箱を読む。この順は、変更の順にほぼ一致する。中継は各イベントをブローカーに渡し、そのあとで発行済みの印を付ける。`module.go` の `NewRelay` が、ブローカーを受け取って中継を組み立てる。中継は同時にいくつも動かせ、各イベントは最初にそれを取った中継が発行する。
- 中継は、独立したプロセス `reservation relay` として動く。これは Reservation のコマンドに引数 `relay` を与えたもので、API と同じイメージから動く（TPL-0014）。中継は `platform/awsevents` を通して、`EVENT_BUS_NAME` が指す EventBridge のバスに発行する。そのバスがなければ、起動しない。
- 配信は少なくとも1回である。中継がイベントをブローカーに渡したあと、印を付ける前に止まると、そのイベントは同じ `id` のまま再び発行される。1つの予約のイベントが、順序を入れ替えて届くこともある。そのため受け取る側は、処理した各イベントの `id` を記録し、順序が問題になるときは `time` を見る。
- 中継は、発行済みのイベントを、保持期間（既定では7日）が過ぎてから送信箱から消す。
- イベントを黙って捨てることはない。統合イベントを持たないドメインイベントがあれば、その変更は失敗する。送信箱の中の読めないイベントは送信箱に残り、エラーとしてログに記録され、中継はほかのイベントの処理を続ける。
- テストは、ストアが作る統合イベントをすべて、その `type` が指す JSON Schema で検証する。

Reservation はどのイベントも購読しない。

## 決まり

- 予約は主催者に代わり、ある参加人数のために、1つの会議室をある時間帯だけ押さえる。予約は確定しているか、取り消されているかのどちらかである。
- 時間帯は始まりより後に終わり、15分刻みの時刻に始まって終わり、長くても4時間で、過去には始まらない。始まりを含み終わりを含まないので、ある予約が終わる時刻に次の予約を始められる。
- 参加人数は1人以上で、会議室の定員に収まる。
- 時間帯は、会議室のタイムゾーンで読んだ、ある1日の利用時間の中に収まる。
- 1つの会議室の確定した予約どうしは重ならない。まずユースケースが確かめ、同じ瞬間に保存しようとした2つのリクエストの間では、排他制約 `reservations_no_overlap` が決める。
- 変更した予約は、新しい予約と同じ決まりに従う。取り消した予約は、変更も、もう一度の取り消しもできない。
- 予約は、始まる30分前まで取り消せる。取り消した時点から、その時間帯の会議室は空く。
- どの変更も、読んだときの予約のバージョンを示し、それ以降に予約が変わっていれば失敗する（楽観的ロック）。
- 同じ冪等キーと同じ内容で再び届いた予約のリクエストは、何も新しく予約せず、最初の予約を受け取る。内容が違えば失敗する。

数値はドメインの定数であり、`internal/domain/period.go` の `PeriodStep`・`LongestPeriod`・`CancellationNotice` である。

## ほかの文脈から必要とするもの

Reservation は、予約と変更のたびに、会議室の定員・利用時間・タイムゾーンを必要とする。これらは Facility が持つ。

- `internal/app/` のポート `RoomDirectory` が、Reservation に必要なものを Reservation 自身の言葉で表す。
- アダプター `internal/adapter/facility/` がこのポートを実装する。`contracts/http/facility.openapi.yaml` から oapi-codegen が生成したクライアントで、`GET /rooms/{roomId}` を呼ぶ。Facility の会議室は `domain.Room` に変換するので、Facility の型がアダプターの外に出ることはない（TPL-0005）。
- Facility の problem である room-not-found は [unknown-room](../../problems/unknown-room.ja.md) になる。Facility の失敗、使えない答え、3秒以内に答えがないことは、`Retry-After` 付きの [facility-unavailable](../../problems/facility-unavailable.ja.md) になる。
- コマンドは、Facility のベース URL を `FACILITY_API_URL` から読む。
- アダプターのテストは、Facility の偽物を相手に動く。偽物は、アダプターが送る各リクエストと、偽物自身が返す各答えを、Facility の API の文書に照らして検証する。そのため、アダプターが追従していない変更がその文書に入ると、このテストが失敗する。

## 画面

フロントエンドは、機能 `frontend/src/features/reservation/` で会議室を予約する。この機能は、Next.js のサーバーからだけこの API を呼ぶ（TPL-0014）。機能は、会議室を自分の言葉で知る（`room.ts`）。ページが Facility から会議室を得て、`frontend/src/app/_compose/` がそれをその言葉に変える。ここで `adapter/facility` がすることと同じである。

| ページ | 見せるもの | 操作 |
|---|---|---|
| `/rooms/{roomId}` | 会議室のタイムゾーンでの、ある日付の予約。前後の日付へのリンクと、その日付に会議室を予約するフォームがある | `listReservations`、`bookRoom` |
| `/reservations/{reservationId}` | 予約とその会議室。確定している間は、予約を変更するフォームと取り消すフォームがある | `getReservation`、`rescheduleReservation`、`cancelReservation` |

- 人は、会議室のタイムゾーンで時間帯を読み、入力する。`period.ts` がそれを API の日時に変え、そのタイムゾーンの時計が飛ばす時刻であれば、そう伝える。
- 予約するフォームは、フォームを描くときに Next.js のサーバーが作る `Idempotency-Key` を送る。フォームは、そのキーと、そのキーで送った予約のフィンガープリントを持ち続ける。同じ予約をもう一度送っても予約は1回だけで、同じフォームから別の予約を送ると新しいキーになる（`booking.ts`）。
- 変更するフォームと取り消すフォームは、ページが表示したバージョンを `If-Match` で送る。precondition-failed か reservation-cancelled のときは、ページがメッセージと予約の今の状態を表示する（`changes.ts`）。
- 人が対処できる problem の種類には、それぞれ専用の文言がある。関係する項目があれば、文言はその項目に出る（`problems.ts`）。ほかの problem は、title と detail をそのまま表示する。

<!-- i18n-source: README.md sha256:7ccc9a86e690bed20d83a3a66063496eb512ae03f745a5c4fcec1d586f79f875 -->

# 画面を足す

フロントエンドの画面は、1つの区切られた文脈の API が返すものを見せるか、複数の文脈を1つのページに組み合わせる。画面の上のフォームは、API が持つものを変える。
手順では、`frontend/src/` の下にある文脈 `<context>` のファイルを名指しする。各置き場所が何を持つかは [ARCHITECTURE.ja.md](../../ARCHITECTURE.ja.md#フロントエンド) にある。
小さい例は facility の機能（`features/facility/`）で、一覧・ページ・会議室を登録するフォームからなる。大きい例は reservation の機能で、冪等キーを持つフォームと、バージョンを送るフォームがある。

Next.js 16 は、読み手の知っている Next.js と違うことがある。ある部分を書く前に、`frontend/node_modules/next/dist/docs/01-app/` からそのガイドを読む。

## 手順

1. API から始める。画面が必要とする操作が文脈の API になければ、先にそれを足す（[エンドポイントを足す](add-an-endpoint.ja.md)）。`make gen` は、すべての API の文書の型を `src/lib/api/<context>.gen.ts` に書く。
   ある文脈の最初の画面では、その文脈のクライアントを `facilityApi` と同じ形で `src/lib/api/server/clients.ts` に足し、ベース URL を読む変数を決める。その変数を、`compose.yaml` のサービス `web` と、`make/frontend.mk` のターゲット `frontend-dev` に渡す。

   確認：`make check` が `gen-check-api-ts` を実行する。型が古い間（`gen/stale`）、ない間（`gen/missing`）、元の文書がない間（`gen/no-document`）、これは失敗する。

2. サーバーでデータを読む。置き場所は `src/features/<context>/server/<name>.ts` である。このモジュールは `server-only` を import し、`lib/api/server/outcome.ts` の `outcome` を通して文脈のクライアントを呼び、`Result` を返す。`Result` は、答えの値か、その problem である。`outcome` は、答えない API も problem に変える。さらにリクエストを待つので、ビルドのデータが静的なページに入ることはない。ページとその `generateMetadata` の両方が呼ぶ関数は、`getRoom` と同じく React の `cache` で包み、API に1回だけ問い合わせる。

   確認：`server/` の下のモジュールが `server-only` を import していないと、`make check` が失敗する（`lint-frontend` の `no-restricted-syntax`）。API の文書が許さない呼び出し（宣言されていないパラメーターなど）でも失敗する（`lint-frontend-types` の `TS2353`）。

3. 純粋なロジックを `src/features/<context>/<name>.ts` に書く。フォームが持つものとサーバー関数によるその読み方、API の文書が与える型（`components["schemas"]`）でのリクエスト、problem の種類ごとに画面が言うことである。
   - フォームの各値は `lib/input.ts` で絞り込む。タイムゾーンの中の日付と時刻の変換には `lib/zoned-time.ts` を使う。
   - problem は `problemKind` で見分ける。これは `docs/problems/` にある problem のページの名前である。人が対処できる種類には、関係する項目に専用の文言を出す。それ以外の problem の不正な部分は、`problemFieldErrors` で項目に対応づける。title や detail は解析しない（TPL-0008）。
   - Next.js のものは何も import せず、ほかのモジュールは拡張子 `.ts` 付きの相対パスで import する。Node のテストランナーが、ファイルをそのまま実行するからである。

   隣の `<name>.test.ts` で、`node:test` と `node:assert/strict` を使ってテストする。例は `registration.test.ts` である。

   確認：`make check` がテストを実行し（`test-frontend`）、どこからも使われていない export で失敗する（`lint-frontend-unused`）。

4. サーバー関数を、`"use server"` のもとの `src/features/<context>/actions.ts` に書く。サーバー関数は、フォームが表示している状態と送信の `FormData` を受け取り、次の状態を返す。
   - 手順3のロジックでフォームを読み、合わない項目があれば、その項目へのメッセージを持つ状態を返す。
   - 手順2のサーバーのモジュールを呼び、API が拒んだら、その problem の状態を返す。
   - 成功したら、`redirect` で次のページへ移るか、`next/cache` の `refresh` でページを読み直す。

   サーバー関数には誰でも POST できる。そのため、フォームのどの値も信用せず、想定内の失敗では例外を投げずに状態を返す。置き場所が `server/` の外なのは、クライアントコンポーネントであるフォームがこれを import するからである。

5. コンポーネントを `src/features/<context>/components/` に書く。
   - サーバーコンポーネントは、手順2を通してデータを読み、problem を `lib/components/` の `ProblemNotice` で表示する。
   - フォームはクライアントコンポーネント（`"use client"`）である。サーバー関数を `useActionState` で呼び、各項目を `Field` で描き、フォーム全体へのメッセージを `ProblemNotice` で表示し、送信が失敗したら `useFocusOnProblem` でフォーカスを移す。`noValidate` を付ける。すべての項目をサーバー関数が確かめるからである。
   - 隠し項目は、サーバー関数が必要とし、人が入力しないものを運ぶ。会議室の ID、予約のバージョン、冪等キーなどである。新しい冪等キーのようにリクエストごとに違うべき値は、`connection()` のあとでサーバーコンポーネントが作る。ビルドの時点では作らない。

   確認：クライアントコンポーネントが `server/` の下のモジュールを import すると、`make check` が失敗する（`lint-frontend` の `no-restricted-syntax`）。

6. ページを `src/app/<route>/page.tsx` に組み立てる。ページは、静的な部分をすぐに描き、リクエストのたびにデータを読む部分を、それぞれフォールバック付きの `<Suspense>` で包む。
   2つの文脈を見せるページは、それぞれを自分の機能を通して読み、一方が渡すものを `src/app/_compose/` の関数を通してもう一方に渡す。例として、`reservationRoom` は Facility の会議室を Reservation の `Room` に変える。
   存在しない ID には `notFound()` を呼び、ページの隣に `not-found.tsx` を書く。ページへのリンクには `Link` を使う。

   確認：機能がほかの機能を import すると、`make check` が失敗する（`lint-frontend` の `boundaries/dependencies`）。存在しないルートへのリンクでも失敗する（`lint-frontend-types`）。ページがリクエストのデータやキャッシュしないデータを `<Suspense>` の外で読むと、イメージのビルドが失敗する。`make check` はこのビルドを行わず、`make up` と `make e2e` が行う。

7. 新しいページか新しいフォームには、`frontend/e2e/` にシナリオを足す。人がすることを、最初の画面から変更が引き起こすことまで、`e2e/support.ts` の補助関数を使って書く。各シナリオは、自分の会議室と主催者を `unique` で作るので、シナリオは並んで動く。項目は `field` で探す。`field` は見えている項目を選ぶ。Next.js は、人が直前にいたページを、隠したまま文書に残しているからである。
   すでにあるページの表示を変えるだけなら、そのためのシナリオは要らない。ロジックの単体テスト（手順 3）が、それを確かめる。変更が触れるところをシナリオが読んでいるときは、そのシナリオの期待を直す。

   確認：変更が `page.tsx` を足したのに、シナリオを足しも変えもしていないと、`make check` が失敗する（`lint-new-pages`、`e2e/new-page`）。`make e2e` が、手元のシステム（Docker）に対してシナリオを実行する。失敗したシナリオは、トレースとスクリーンショットを `.cache/playwright/results/` に、レポートを `.cache/playwright/report/` に残す。

8. 文脈の README（`docs/contexts/<context>/README.md`）の画面の部分と、その対訳を更新する。そのあと `make i18n-stamp` を実行する。

9. 仕上げる。

   ```sh
   make fmt
   make check
   make e2e   # ブラウザーでのシナリオ（Docker）
   ```

## どのチェックも報告しないこと

- 何をするかを決めるために、problem の title や detail を読む画面。
- フォームの値を絞り込む前に使うサーバー関数。
- フォームの冪等キーなしで予約を送るフォームや、ページが表示したバージョンなしで変更を送るフォーム。どちらも、`features/reservation/booking.ts` と `changes.ts` の先頭のコメントが説明している。
- 文脈の README が名指ししていない画面。
- すでにあるページに足した、シナリオのない新しいフォーム。`lint-new-pages` が見るのは、新しいページだけである。

## 既知の制限

- 本番のビルドでは、リクエストのたびにデータを読むページで JavaScript なしの送信が失敗すると、答えはページのうちストリーミングされる部分だけになる。JavaScript があるとき、および送信が成功したときは、ページ全体が返る。これは Next.js 16.4.6 で見つかったもので、Next.js にはまだ報告していない。
- 存在しない会議室や予約のページは、ステータス 200 と `not-found.tsx` の画面で答える。探す処理が `<Suspense>` の中で動き、そのときにはステータスがもう送られているからである。
- 会議室を登録するフォームのタイムゾーンの一覧は、サーバーの `Intl` から来る。これは、Facility が確かめに使う Go のタイムゾーンのデータと違うことがある。Next.js のサーバーが知らないタイムゾーンの会議室では、予約を表示しない。
- `outcome` は答えの本文を残し、`Retry-After` のようなヘッダーを捨てる。そのため画面は、どれだけ待てばよいかを言えない。
- 始まった予約を変更すると、過去の時間帯と同じ種類の `period-in-past` が返る。そのため画面は、2つの原因を1つのメッセージで伝える。
- 単体テストは、純粋なロジックのモジュールを対象とする。コンポーネントとサーバー関数は、`make e2e` がブラウザーでテストするだけである。
- テンプレートには、デザインシステムもサインインもない。CSS は素の CSS（`app/globals.css`）で、コンポーネントライブラリは使わない。どちらも、テンプレートをコピーしたチームが選ぶ。

<!-- i18n-source: add-a-screen.md sha256:3d8edc7a486a2caca8c419c8c1c6ba1b3a1561c27f9e5a9546228942da4707d9 -->

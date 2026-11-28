# AGENTS.md（frontend）

`frontend/` で作業するコーディングエージェント向けの指示であり、ルートの [AGENTS.ja.md](../AGENTS.ja.md) に足すものである。地図は [ARCHITECTURE.ja.md](../ARCHITECTURE.ja.md) にある。

## 最初に、インストールされた Next.js の文書を読む

このアプリは、Next.js 16 の App Router、React 19、TypeScript 6 で動く（`package.json`）。Next.js 16 は読み手の知っている Next.js と違うことがあり、このアプリは Cache Components、partial prefetching、型付きのルートを使う（`next.config.ts`）。
コードを書く前に、このディレクトリの `node_modules/next/dist/docs/01-app/` から該当するガイドを読む。これはインストールされたバージョンの文書である。`next dev` はそう指示するブロックをこのファイルに書き込むが、`next.config.ts` がそれを止めている（`agentRules: false`）。

## コマンド

| コマンド | 使いどころ |
|---|---|
| `make gen` | 各 `contracts/http/<context>.openapi.yaml` から `src/lib/api/<context>.gen.ts` を生成し直す（openapi-typescript）。 |
| `make test-frontend` | 単体テスト `src/**/*.test.ts` を Node のテストランナーで実行する。 |
| `make frontend-dev` | `make up` が起動した手元のシステムの API に対して、`next dev` を <http://127.0.0.1:3000> で動かす。 |
| `make up` | 手元のシステムを起動する。フロントエンドは <http://127.0.0.1:13000> にある。 |
| `make e2e` | 手元のシステムを起動し、`e2e/` のシナリオを Playwright で Chromium に実行させる。Docker が要る。 |

- `make check` は `next build` を実行しない。ビルドするのはイメージ（`Dockerfile`）であり、`make up`・`make e2e`・`make check-all` の中で行われる。ページがリクエストのデータやキャッシュしないデータを `<Suspense>` の外で読むと、ビルドが失敗する。
- 手順は、[画面を足す](../docs/guides/add-a-screen.ja.md)・[API を変更する](../docs/guides/api-changes.ja.md)・[サービスを手元で動かす](../docs/guides/run-locally.ja.md)にある。スキルは `add-screen` である。

## `src/` の中のコードの置き場所

| コード | 置き場所 |
|---|---|
| 機能を組み合わせるページと、その `not-found.tsx` | `app/<route>/` |
| ある文脈が渡すものを、別の文脈が必要とする形にする変換 | `app/_compose/` |
| `outcome` を通した、文脈の API の呼び出し | `features/<context>/server/*.ts`。`server-only` を import する |
| 文脈のフォームのサーバー関数 | `features/<context>/actions.ts`（`"use server"`） |
| 純粋なロジック（フォームの読み取り、problem の種類ごとの文言） | `features/<context>/*.ts`。隣に `*.test.ts` を置く |
| 文脈のコンポーネント | `features/<context>/components/` |
| API のクライアントと `outcome`、problem、フォームの入力、タイムゾーンの中の時刻、共有のコンポーネント | `lib/api/server/`、`lib/problem.ts`、`lib/input.ts`、`lib/zoned-time.ts`、`lib/components/` |

単体テストのあるモジュールは、Next.js のものを何も import せず、ほかのモジュールを拡張子 `.ts` 付きの相対パスで import する。Node がそのまま実行するからである。

## チェックが守らせる決まり

- `boundaries/dependencies`：機能はほかの機能を import せず、`lib` は機能を import しない。それらを組み合わせるのはページである。（TPL-0003）
- `no-restricted-syntax`：`server/` の下のモジュールは `server-only` を import し、クライアントコンポーネントは `server/` の下のモジュールを import しない。API を呼ぶのは Next.js のサーバーだけである。（TPL-0014）
- `gen-check-api-ts`：`src/lib/api/*.gen.ts` は `make gen` が書く内容と同じである。手で編集しない。（TPL-0006、TPL-0007）
- `lint-frontend-types`：API の文書が許さない呼び出しや、存在しないルートへのリンクがあると `tsc` が失敗する。（TPL-0007）
- `lint-frontend-unused`：どこからも使われていないファイル・export・依存を置かない。
- `test-frontend` と、`make check-all` の `e2e`・`test-mutation-frontend`：純粋なロジックには、そのどの変更にも気づく単体テストがある。画面の振る舞いはブラウザーでテストする。（TPL-0023）
- `lint-new-pages`：`page.tsx` を足す変更は、`e2e/` のシナリオを足すか変える。（TPL-0023）

## レビューが守らせる決まり

- 画面は、problem を `problemKind` で見分け、不正な部分を `problemFieldErrors` で項目に対応づける。title や detail を解析しない。（TPL-0008）
- サーバー関数は、`FormData` の各値を `lib/input.ts` で絞り込み、想定内の失敗では例外を投げずに状態を返す。
- 予約はフォームの冪等キーを送り、変更はページが表示したバージョンを `If-Match` で送る。`features/reservation/booking.ts` と `changes.ts` の先頭のコメントを読む。
- すでにあるページに足す新しいフォームには、`e2e/` のシナリオがある。ページの表示を変えるだけなら、ロジックの単体テストがあればよく、そのためのシナリオは要らない。
- CSS は `app/globals.css` の素の CSS で書き、コンポーネントライブラリも日付のライブラリも使わない。既知の制限は、ガイド[画面を足す](../docs/guides/add-a-screen.ja.md#既知の制限)にある。

<!-- i18n-source: AGENTS.md sha256:59b7867318f2e982d3398f408dbc5f40d7e73139d3540c1bf558106743c366d8 -->

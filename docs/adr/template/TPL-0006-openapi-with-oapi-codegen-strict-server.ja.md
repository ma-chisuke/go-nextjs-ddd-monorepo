---
status: accepted
date: 2026-11-20
---

# TPL-0006: HTTP の契約を OpenAPI 3.1 で書き、Go のサーバーを oapi-codegen の strict server で生成する

## 背景と課題

区切られた文脈はどれも HTTP の API を提供し、Next.js の Web アプリやほかの文脈がそれを呼ぶ（TPL-0005）。
Go のサーバー、画面の TypeScript のコード、文脈の間のクライアントは、パス・パラメーター・本文・ステータスコードのすべてで一致していなければならない。
それぞれを手で書くと、両側は少しずつずれる。Go でフィールドの名前を変えても TypeScript には古い名前が残り、誤りは実行して初めてわかる。

両側をそろえ続ける方法は、2つの選択で決まる。
1つ目は、どちらを正とするかである。API を記述した文書か、API を実装したコードかを選ぶ。
2つ目は、正とするものから両側のコードを作る道具である。

各 API をどう記述し、両側のコードをどうやってその記述に従わせるか。

## 判断の基準

- 契約を、それだけで独立した変更としてレビューする。実装のコードより前に、またはコードと一緒にレビューする。
- 契約に合わないハンドラーは、コンパイルが通らない。
- サーバーは標準ライブラリの `net/http` を使い、Web フレームワークは使わない。
- 形式は、TPL-0007 のチェックと TypeScript の道具が読める、公開された標準である。
- API は HTTP の意味論に従う。ステータスコード、`Location`、`ETag` と `If-Match`、エラーを表す problem details（TPL-0008）がそれにあたる。

## 検討した選択肢

- 先に OpenAPI 3.1 を書き、oapi-codegen の strict server と openapi-typescript で生成する
- Go のコードやその注釈から OpenAPI を導く
- 先に OpenAPI 3.1 を書き、ogen で生成する
- gRPC か Connect と Protocol Buffers
- GraphQL

## 決定

採用：「先に OpenAPI 3.1 を書き、oapi-codegen の strict server と openapi-typescript で生成する」。契約そのものをレビューして正とでき、Go のコードとの食い違いをコンパイルエラーとして検出でき、しかもサーバーを標準ライブラリのまま保てるからである。

- `contracts/http/<context>.openapi.yaml` が、各文脈の API の正である。API を変えるときは、このファイルを変えるところから始める。
- oapi-codegen が、Go のサーバーのために strict server のインターフェースと型を `net/http` の上に生成する。strict server は、操作ごとに型の付いたリクエストのオブジェクトと応答のオブジェクトを用意する。そのため、ハンドラーが返せるのは、文書が宣言したステータスと本文だけである。
- 別の文脈がこの API を呼ぶためのクライアントも、oapi-codegen で生成する（TPL-0005）。
- Next.js の Web アプリが使う TypeScript の型は、openapi-typescript で生成する。API の文書1つにつき1ファイル `frontend/src/lib/api/<context>.gen.ts` を、`tools/scripts/gen-frontend.sh` が書く。
- Next.js の Web アプリのクライアントは、openapi-typescript と同じプロジェクトの openapi-fetch である。これは API の生成された `paths` を受け取るので、文書が宣言していないパス・パラメーター・本文での呼び出しはコンパイルが通らない。実行時の依存としては小さく、契約がフロントエンドに加える実行時の依存はこれだけである。
- 生成コードはコミットする。ファイル名は `*.gen.go` と `*.gen.ts` にし、`.gitattributes` の設定によってプルリクエストの差分では畳んで表示する。これらのファイルは `make gen` が書き、手では編集しない。
- 文書の中の説明は英語で書く（TPL-0021）。
- ヘルスチェック `GET /healthz` は、API の文書の操作ではない。これは、プラットフォームの HTTP サーバー `backend/internal/platform/httpserver` が、生成されたルートの隣に登録する。ヘルスチェックは API を動かすサービスのものであり、クライアントとの契約のものではないからである。
- 文脈の API は独自の認証を持たず、その文書は `security: []` を宣言する。API に届くのは、プライベートなネットワークからだけだからである（TPL-0014）。

最初の文脈で、OpenAPI 3.1 の文書が oapi-codegen v2.8.2・kin-openapi・Redocly CLI・oasdiff を通ることを確かめた。そのため、文書を OpenAPI 3.0.3 で書き直す必要はない。
フロントエンドのマイルストーンで、openapi-typescript 7.13.0 が同じ文書をそのまま読めることを確かめた。
TPL-0007 の層5を動かす Schemathesis 4.33.4 も、2つの API の文書をそのまま読み、対応していないキーワードについての警告も出さない。そのため、OpenAPI 3.0 に戻すための記録は要らない。
その最初の実行で、3.1 の性質が1つわかった。3.1 は JSON Schema 2020-12 から、`4.0` のように小数部を付けて書いた数も、値が整数なら integer に数える扱いを受け継いでいる。生成した Go のコードは、こうした値を拒んでいた。そこで、いまは `platform/httpserver` のリクエストの検証が、こうした値を整数の形で書き直す。

これは最初の計画と1点で異なる。最初の計画では、Next.js の Web アプリのために API の文書を1つにまとめていた。代わりに、各文書がそれぞれ自分のファイルを生成する。理由は3つある。生成器を動かす前に文書をまとめる手順が要らない。どの文書も同じ名前で定義するスキーマ（`Problem` など）がぶつからない。そして、フロントエンドもバックエンドと同じく文脈を分けておける（TPL-0003）。機能は、自分の文脈の型を import する。

### 結果

- 良い点：レビューする人は、API の変更を、コードとは別の1つの YAML ファイルの変更として見られる。
- 良い点：ハンドラーの実装漏れや、パラメーターや応答の型の誤りがあると、コンパイルが通らない。
- 良い点：Go のサーバー、TypeScript の型、文脈の間のクライアントがすべて同じファイルから作られる。そのため、`make gen` を実行するたびに、これらは互いに一致する。
- 良い点：OpenAPI を読める道具は多い。文書とコードの一致を確かめる道具（TPL-0007）も、その中に入る。
- 悪い点：生成コードがリポジトリに入るので、リポジトリが大きくなる。文書を変えるたびに生成し直す必要もある。
- 悪い点：生成器が、文書で使える書き方を制限する。`oneOf` のようなスキーマの書き方は扱いにくい Go の型になるので、文書を生成器に合わせて書くことになる。
- 悪い点：生成コードは、一部のエラー（解析できないパラメーターなど）に自分で応答する。すべてのエラーを problem details で返すには、テンプレートがその応答を差し替えなければならない（TPL-0008）。
- 悪い点：人もエージェントも、コードより先に文書を書く。コードが数行で済む変更では、そのぶん遅くなる。
- 悪い点：openapi-typescript 7 は TypeScript 5 を peer に宣言しているが、フロントエンドは TypeScript 6 を使う。`pnpm-workspace.yaml` がこの組み合わせを許している（`peerDependencyRules`）。後のバージョンが同じ型を生成しなくなれば、`gen-check-api-ts` かフロントエンドの型の検査が失敗する。

### 確認の方法

次のチェックが、この決定を守る。

- コンパイラー：生成コードは strict server のインターフェースの実装を受け取るので、ハンドラーの実装漏れや型の誤りはコンパイルエラーになる。`lint-go` と `test-go` がこれを報告する。
- 生成物の更新漏れ：`make check` の `gen-check-api` が、`oapi-codegen.yaml` ごとに oapi-codegen をもう一度実行し、結果がコミットしたファイルと違えば失敗する。`gen-check-api-ts` は、API の文書ごとに openapi-typescript をもう一度実行する。生成したファイルが違うとき（`gen/stale`）、ないとき（`gen/missing`）、元の文書がないとき（`gen/no-document`）に失敗する。
- TypeScript のコンパイラー：`lint-frontend-types` がフロントエンドに `tsc` を実行するので、生成された型が拒む openapi-fetch の呼び出しは失敗する。自己テストは、Facility の文書が宣言していないクエリパラメーターで、これを確かめる（`case_lint_frontend_types__api`）。
- ルーティング：`lint-arch` のルール `arch/route-registration` は、生成コードと `platform/httpserver` 以外のコードがルートを登録すると失敗する。そのため、API のすべてのルートは、その文書から生まれる。

これらのチェックと、型では表せないものを捕まえるチェックは、TPL-0007 で説明する。

## 選択肢の長所と短所

### 先に OpenAPI 3.1 を書き、oapi-codegen の strict server と openapi-typescript で生成する

- 良い点：strict server は文書を Go のインターフェースに変え、コンパイラーが実装をそのインターフェースと照らし合わせる。
- 良い点：サーバーは `net/http` のままで、ミドルウェアもふつうの `net/http` のミドルウェアである。
- 悪い点：生成コードは、型では表せない制約（パターンや範囲など）を検査しない。別の層で検査する必要がある（TPL-0007）。

### Go のコードやその注釈から OpenAPI を導く

- 良い点：正とするものが1つで済み、サーバーの生成コードもない。
- 悪い点：文書がコードの後を追う。API の変更はコードを書いたあとでしか見えず、画面の作業はサーバーを待つことになる。
- 悪い点：Go の構造体を変えると、誰も意図しないまま API が変わりうる。それに気づく手段は、レビューしかない。

### 先に OpenAPI 3.1 を書き、ogen で生成する

- 良い点：ogen は、文書にある制約（長さやパターンなど）を検査するコードも生成する。
- 悪い点：生成したサーバーは、`http.ServeMux` ではなく自前のルーターでリクエストを振り分け、JSON も独自のライブラリで読み書きする。省略できるフィールドも、独自の型で包む。
- 悪い点：oapi-codegen より使っているプロジェクトが少ない。人もエージェントも、手本にできる例を見つけにくい。

### gRPC か Connect と Protocol Buffers

- 良い点：Protocol Buffers は簡潔で型の付いた契約になり、Buf のような道具で互換性を壊す変更を検出できる。
- 良い点：Connect は素の HTTP と JSON でも応答するので、ブラウザーから呼べる。
- 悪い点：API が HTTP の意味論に従わなくなる。ステータスコード、`ETag` を使う条件付きリクエスト、problem details は、RPC のエラーにうまく対応しない。
- 悪い点：Schemathesis や oasdiff など、TPL-0007 のチェックが読むのは OpenAPI であり、Protocol Buffers ではない。

### GraphQL

- 良い点：画面は、必要なフィールドだけを、複数の型から1回のリクエストで取り出せる。
- 悪い点：エラーはたいてい、ステータス 200 の応答の本文で届く。ステータスコードも problem details（TPL-0008）も当てはまらない。
- 悪い点：画面のために、文脈ごとの API を1つのグラフにまとめなければならない。そのためのゲートウェイが増え、文脈どうしのスキーマが結び付く。

## 補足

- [OpenAPI Specification 3.1.0](https://spec.openapis.org/oas/v3.1.0)
- [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen)
- [openapi-typescript](https://openapi-ts.dev/)

<!-- i18n-source: TPL-0006-openapi-with-oapi-codegen-strict-server.md sha256:687f21cf743bf9b319ec386d2228ab58290fe161127b8974985ab72a27122926 -->

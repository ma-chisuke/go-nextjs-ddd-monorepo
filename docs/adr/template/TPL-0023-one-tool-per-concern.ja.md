---
status: accepted
date: 2026-11-20
---

# TPL-0023: 1つの関心事に道具を1つだけ使い、どの道具も make から動かし、`make check` に時間の上限を設ける

## 背景と課題

Go・TypeScript・Terraform・シェルスクリプト・YAML・2つの言語の Markdown を持つリポジトリには、整形の道具・リンター・スキャナーが数多く要る。
これらの道具の多くは、受け持つ範囲が重なっている。2つのリンターが同じ指摘を出し、2つの整形の道具が同じファイルを別々の形に書き換え、2つのスキャナーが同じ脆弱性を別の言葉で報告する。
1つの問題に2つの指摘を受けたり、食い違う2つの指摘を受けたりしたエージェントは、ターンを無駄にするか、2つの指摘の間を行き来する。

チェックは、増えるにつれて遅くもなる。
数分かかるチェックは、人もエージェントも実行する回数が減る。エージェントのターンの終わりに毎回遅いチェックが走れば、どのターンも遅くなる。

どの問題も1回だけ報告されるように道具を選び、変更のたびに実行できる速さをチェックに保つにはどうすればよいか。

## 判断の基準

- `make` の中では、どの種類の指摘も1つの道具から出る。
- 人・エージェント・CI・Git のフックが、同じコマンドを実行する。
- `make check` は、エージェントのターンの終わりに毎回実行できるほど速い。
- 出力は短い。チェックごとに1行を出し、指摘は失敗したチェックの分だけを出す。
- 指摘は1件1行で、少なくともファイルと行を示す。エージェントは、人の手を借りずにそれをもとに動ける。
- どのチェックも触れてはならないファイルを、1か所にまとめて書く。

## 検討した選択肢

- 1つの関心事に道具を1つだけ使い、`make` から動かし、`make check` に時間の上限を設ける
- MegaLinter や super-linter のような、リンターを束ねる道具
- pre-commit フレームワークを実行役にする
- どの道具も既定の設定のまま、CI でだけ動かす

## 決定

採用：「1つの関心事に道具を1つだけ使い、`make` から動かし、`make check` に時間の上限を設ける」。どの道具が何を報告するかと、チェックにかかる時間の両方を制御できる選択肢は、これだけだからである。

`make` の中では、どの関心事にも道具は1つである。「check-all」と書いたターゲットは、`make check-all` でだけ動く。「CI」と書いたターゲットは、`make check` と `make check-all` のどちらにも含まれず、CI がそれぞれ専用のステップで実行する。

| 関心事 | 道具 | ターゲット |
|---|---|---|
| Go のコードの整形 | golangci-lint（整形には gofmt と goimports を使う） | `fmt-check-go` |
| Go のコードの lint（`go vet` を含む） | golangci-lint | `lint-go` |
| Go のテスト | `go test -race -shuffle=on` | `test-go` |
| 手元のシステムの端から端まで（Docker Compose のサービスをまたぐ1つのシナリオを HTTP で動かし、すべての応答をその API の文書と照らし合わせる） | `go test -tags smoke` と kin-openapi（`backend/test/smoke`） | `smoke`（check-all） |
| テストのカバレッジ（外部のサービスを使わずに報告する） | `go test -coverprofile` と octocov | `coverage` と `coverage-report`（CI） |
| 単体テストが、テストするコードの変更に気づくこと。対象は、中核の文脈のドメインとユースケース、フロントエンドの純粋なロジックである | Go には gomutants v0.7.3（`tools/go.mod`）。フロントエンドには StrykerJS 10.1.1 とその tap ランナー（`frontend/package.json`）。どちらも、`equivalent-mutants.allow` にあるレビュー済みの許可を使う | `test-mutation`、`test-mutation-frontend`（check-all） |
| 実際に呼ばれている Go のコードにある既知の脆弱性 | govulncheck | `vuln-go`（check-all） |
| フロントエンドの lint（機能どうしの境界を含む） | ESLint（eslint-config-next・typescript-eslint・eslint-plugin-boundaries を使う） | `lint-frontend` |
| フロントエンドの型 | TypeScript のコンパイラー（`tsc --noEmit`） | `lint-frontend-types` |
| フロントエンドの、どこからも使われていないファイル・export・依存 | knip | `lint-frontend-unused` |
| API の TypeScript の型が、生成器がいまの API の文書から書く内容と同じであること（TPL-0006、TPL-0007） | openapi-typescript（`tools/scripts/gen-frontend.sh`） | `gen-check-api-ts` |
| フロントエンドの単体テスト（純粋なロジックのモジュール） | Node のテストランナー（`node --test`）。TypeScript をそのまま動かす | `test-frontend` |
| ブラウザーで見た手元のシステム（画面からメールまでの、人のシナリオ） | Playwright と Chromium（`frontend/e2e`） | `e2e`（check-all） |
| YAML・JSON・JSON5・JavaScript・TypeScript・CSS の整形 | Prettier | `fmt-check-prettier` |
| YAML の lint（キーの重複や曖昧な値など） | yamllint | `lint-yaml` |
| TOML の整形と lint | taplo | `fmt-check-toml`・`lint-toml` |
| 設定ファイルのスキーマによる検証 | check-jsonschema | `lint-schema`・`lint-schema-remote`（check-all） |
| シェルスクリプトの整形 | shfmt | `fmt-check-shell` |
| シェルスクリプトの lint | shellcheck | `lint-shell` |
| Dockerfile の lint | hadolint | `lint-dockerfile` |
| ベースイメージのダイジェストによる固定（TPL-0024） | `tools/scripts/image-pins.sh` のパターン照合 | `lint-image-pins` |
| ワークフローの構文 | actionlint | `lint-actions` |
| ワークフローのセキュリティ | zizmor | `lint-actions-security` |
| ワークフローの方針（権限とタイムアウト） | ghalint | `lint-actions-policy` |
| アクションのコミット SHA の全桁での固定 | pinact | `lint-actions-pins` |
| Terraform のファイルの整形 | `terraform fmt` | `fmt-check-terraform` |
| それ以外の HCL のファイル（`terragrunt.hcl` や `root.hcl` など）の整形 | `terragrunt hcl fmt` | `fmt-check-terragrunt` |
| Terraform のコードの lint | tflint（AWS のルールセットを使う） | `lint-terraform` |
| 古くなったモジュールの README | terraform-docs | `lint-terraform-docs` |
| Terraform のコードの妥当性と、各モジュールのテスト（`tests/*.tftest.hcl`。AWS provider のモックに対して plan する） | `terraform validate` と `terraform test` | `validate-terraform`（check-all） |
| Terragrunt のツリーの形。どの環境も同じユニットを持ち、各ユニットに定義があり、どの `env.hcl` も同じキーを持ち、各文脈にユニットがあること（TPL-0015） | `tools/scripts/infra.sh lint` | `lint-infra-live` |
| Terragrunt のすべてのユニットの妥当性。ほかのユニットの出力はモックにする | Terragrunt と `terraform validate`（`tools/scripts/infra.sh validate`） | `validate-live`（check-all） |
| 1つの環境をデプロイと同じ順序で AWS Emulator に適用し、ほかの環境をそれに対して plan すること（TPL-0017） | Terragrunt と AWS Emulator（`tools/scripts/infra.sh rehearse`） | `rehearse-infra`（check-all） |
| データベースを準備するイメージ。スーパーユーザーではないマスターユーザーとして、手元の PostgreSQL に対して動かす（TPL-0016） | Docker と psql（`tools/scripts/db-image.sh`） | `test-db-image`（check-all） |
| SQL のファイル | squawk | `lint-sql` |
| スキーマの変更ごとの DDL（pgschema の plan） | squawk | `lint-sql-plan`（check-all） |
| 文脈のスキーマに対して prepare したクエリ | `sqlc vet` | `lint-sqlc`（check-all） |
| テーブルと列のコメント、およびスキーマの文書 | tbls | `lint-db-docs`（check-all） |
| OpenAPI と AsyncAPI の文書 | Redocly CLI（`@redocly/cli`） | `lint-contracts` |
| OpenAPI の文書の互換性を壊す変更 | oasdiff | `lint-contracts-breaking` |
| API の文書から生成したリクエストを、チェックアウトからビルドした API に送ること（TPL-0007 の層5） | Schemathesis（`pyproject.toml`・`tools/scripts/api-fuzz.sh`） | `test-api-fuzz`（check-all） |
| イベントの JSON Schema（その形、例、それを参照する AsyncAPI 文書のメッセージ。TPL-0013） | このリポジトリの events コマンド（`tools/cmd/events`） | `lint-events` |
| イベントの JSON Schema の互換性を壊す変更 | このリポジトリの events コマンド | `lint-events-breaking` |
| Markdown の整形と構造 | markdownlint-cli2 | `lint-md` |
| 英語の文章 | Vale（Google のスタイル） | `lint-prose-en` |
| 日本語の文章 | textlint | `lint-prose-ja` |
| リンク | lychee | `lint-links`・`lint-links-remote`（check-all） |
| 決定の記録とその一覧（TPL-0001） | このリポジトリの docs コマンド（`tools/cmd/docs`） | `lint-adr` |
| 英語版と照らし合わせた対訳（TPL-0021） | このリポジトリの docs コマンド | `lint-i18n` |
| Go のバックエンドの境界（文脈と層のあいだの import、I/O のないドメイン、手で登録したルート、SQL に書いたスキーマの名前、文脈ごとの用語集。TPL-0003・TPL-0004・TPL-0005・TPL-0007・TPL-0009） | このリポジトリの arch コマンド（`tools/cmd/arch`） | `lint-arch` |
| `AGENTS.md` の長さ（TPL-0020） | `make/agents.mk` で行数を数える | `lint-agents-md` |
| 無視のコメント（TPL-0020） | `tools/scripts/ignore-comments.sh`。チェックを止めるコメントを `ignore-comments.allow` と比べる | `lint-ignore-comments` |
| 新しいページごとの、ブラウザーのシナリオ | `tools/scripts/new-pages.sh`。`page.tsx` を足す変更の中に、シナリオを探す | `lint-new-pages` |
| つづり | typos | `lint-typos` |
| 改行コード・末尾の改行・行末の空白・インデントの種類 | editorconfig-checker | `lint-editorconfig` |
| 秘密情報 | gitleaks | `lint-secrets` |
| Dockerfile と Terraform の設定ミス | Trivy | `scan-config` |
| 依存の脆弱性とライセンス | Trivy | `scan-deps`（check-all） |
| サービスのイメージの脆弱性 | Trivy | `scan-images`（check-all） |
| サービスのイメージの SBOM | Trivy | `sbom`（まだどのワークフローも実行しない。TPL-0024） |
| エージェントのフック（TPL-0020） | 各フックに見本のイベントを与えるテスト（`tools/tests/hooks_test.sh`） | `test-hooks` |
| ハーネスの評価の採点を、エージェントなしで確かめること。どの作業も、いまのリポジトリでは不合格になり、その解答では合格する（TPL-0020） | `evals/run.sh verify` | `test-evals`（check-all） |
| コミットメッセージを検査するスクリプト | 見本の件名を、いくつかのロケールで試すテスト（`tools/tests/commit_msg_test.sh`） | `test-commit-msg` |
| チェックそのもの（既知の悪い入力で失敗し、既知の良い入力で通るか） | `tools/selftest/selftest.sh` | `selftest`（check-all） |
| コミットメッセージとプルリクエストの題（Conventional Commits） | `tools/scripts/commit-msg.sh` | lefthook の `commit-msg` フックと、`lint-pr-title`（CI） |

ミューテーションテストの決まりは次のとおりである。

- 生き残ったミュータントは、1つでもチェックを失敗させる。どのテストも実行しない行も同じである。到達すべき割合は置かない。閾値を置くと、規則が誰にも気づかれずにすり抜ける。割合の数字だけでは、読み手は何をすればよいかわからない。指摘なら、行と変更を示す。
- 対象の1つは、中核の文脈のドメインとユースケースである（`make/go.mk` の `MUTATION_PACKAGES`）。業務の規則はここにある（TPL-0003）。もう1つは、フロントエンドのモジュールのうち、隣に単体テストがあるものである（`frontend/stryker.config.mjs` がテストから見つける）。フォームを読む、問題に言葉を選ぶ、時刻を変換する、といった純粋なロジックがこれにあたる。補完の文脈はトランザクションスクリプトで、そのテストはデータベースを通して動く。そこでミューテーションテストを実行しても、遅いうえに、わかることが少ない。フロントエンドのコンポーネントとサーバーのモジュールは、ブラウザーでテストする。
- 等価なミュータント（どのテストもコードと区別できないミュータント）が生き残ってよいのは、理由を書いた行が `equivalent-mutants.allow` にあるときだけである。その行を足すかどうかは、レビューで決めることである。生き残ったミュータントのどれにも一致しない行は、チェックを失敗させる。行は、そのミュータントが指すファイルの言語のチェックに属する。いま、このファイルに行はない。Go とフロントエンドで見つかった等価なミュータントは、コードを単純にして取り除いた。
- 中核の文脈での最初の実行では、ミュータントは349個で、そのうち47個が生き残り、30個はどのテストも実行しない行にあった。テストを足してコードを単純にしたあとは、ミュータントは338個になった。323個を殺し、15個はコンパイルできず、生き残るものはない。誤っていた業務の規則はなく、テストが規則のすべてを守っていなかっただけである。1回の実行は、プロセッサの半分を使って1分ほどかかる。
- 道具は gomutants である。mutago・gremlins・gomu・avito-tech/go-mutesting・cervo-mutants・ooze ではなく、これを選んだ。gomutants・mutago・gremlins・gomu は、ドメインのパッケージに対して動かした。残りの3つは動かしていない。avito-tech/go-mutesting はソースファイルをその場で書き換え、リリースもない。cervo-mutants は、メンテナーが開発の休止を告知している。ooze は、2023年からリリースがない。意味のあるミュータントを最も多く見つけたのは gomutants で、28種類の変え方で167個だった。gremlins は5種類で27個にとどまり、テストが実行するコードを実行されないと報告した。mutago は、関数をまるごと見落とした。gomutants は各ミュータントに関数と種類で名前を付けるので、ファイルのほかの箇所を変えても名前は変わらない。また、どのテストも実行しない行を報告し、ソースのファイルには何も書き込まない。
- フロントエンドのミュータントは、StrykerJS 10.1.1 が作る。その tap ランナーは、単体テストを Node のテストランナーでそのまま動かすので、2つめのテストフレームワークは要らない。最初の実行では、ミュータントは857個で、そのうち123個が生き残り、23個はどのテストも実行しない行にあった。テストを足してコードを単純にしたあとは、ミュータントは737個になった。729個を殺し、8個は Stryker が無限ループを止める回数の上限で止まり、生き残るものはない。1回の実行は、プロセッサの半分を使って75秒ほどかかる。誤っていた規則はなかった。生き残っていたものの多くは、型の検査だけが必要としていた条件と、どのテストも読んでいなかった文だった。StrykerJS は予算の中で動いたので、ほかの道具は試していない。
- StrykerJS はミュータントに連番を振る。そこで `tools/stryker/mutants.jq` が、各ミュータントに名前を付ける。名前は、ファイル・最上位の宣言・変え方の種類・その中での順番からなる。gomutants が Go のミュータントに付ける名前と同じ考え方である。Stryker は、無限ループを回数で止め、それ以外を時間で止める。時間の下限は30秒にしてある。忙しいマシンで、時間切れが、生き残るミュータントを隠さないようにするためである。
- 画面に出す言葉も、ミュータントにする。事実（数、規則、どの入力欄の話か）を運ぶ文は、単体テストが一字一句まで守る。それ以外の文は、「メッセージは文である」という規則で守る。そのため、事実を変えない言い回しの変更では、テストは変わらない。言葉を持つ5つのモジュールで文字列のミュータントを外していれば、737個のうち119個が減っていた。実際には、何も外していない。

重なりは、次のように解消した。

- `go vet` は、ほかの Go のリンターと一緒に golangci-lint の中で動き、別の手順にはしない。gofmt と goimports も、整形の道具としてそこで動く。
- つづりは typos が受け持つので、Vale のつづりのルールは切ってある（`.vale.ini`）。
- Vale は英語の Markdown だけを、textlint は日本語の Markdown だけを読む。
- Markdown の整形は、Prettier ではなく markdownlint-cli2 の修正機能が行う。そのため、Markdown の整形の道具とリンターが食い違うことはない。
- YAML は Prettier が整形し、yamllint が lint する。yamllint の配置のルールは Prettier と合わせてあり、行の長さは Prettier だけが受け持つ（`.yamllint.yaml`）。
- シェルの書式は `.editorconfig` に書き、shfmt はそれを読む。そのため、エディターと shfmt は同じ設定を使う。
- editorconfig-checker はインデントの幅を検査しない。インデントの幅は整形の道具が受け持つ。
- `terraform fmt` は Terraform の言語のファイルを、`terragrunt hcl fmt` はそれ以外の HCL のファイルを整形する。そのため、1つのファイルを2つの整形の道具が扱うことはない。
- Trivy を選んだ理由は TPL-0024 にある。重なりは1つだけ認めている。Trivy はすべてのロックファイルにある脆弱なモジュールをすべて報告し、govulncheck は Go についての指摘を、実際に呼ばれているコードに絞る。そのため、呼ばれている Go のコードにある脆弱性は、両方から報告されることがある。Trivy は、同じモジュールを `scan-images` でもう一度報告することがある。`scan-images` は、各バイナリに組み込まれたモジュールを読むからである。
- GitHub の側のスキャナー（CodeQL とシークレットスキャン。TPL-0024）は、`make` の外で動く。golangci-lint（gosec）や gitleaks と重なるが、これは意図したものである。手元で何が起きても GitHub の上で動く、もう1つの層だからである。
- コミットメッセージとプルリクエストの題は、このリポジトリの短いスクリプト `tools/scripts/commit-msg.sh` が検査する。commitlint のような Node.js のパッケージも、別のバイナリも使わない。固定するバイナリには、`mise.lock` の4つのプラットフォームのそれぞれでビルドが要る（TPL-0022）。最初に選んだ committed は Linux の arm64 向けのビルドを出していない。スクリプトなら依存が要らない。
- OpenAPI と AsyncAPI の文書は、どちらも Redocly CLI で lint する。最初の計画では、AsyncAPI のコマンドラインの道具と、候補として Spectral も挙げていた。どちらも入れていない。1つの道具で両方の契約を扱え、リポジトリが抱える大きな依存のツリーが1つ減るからである。
- 層の import のルールと、ドメインで時計を読むことを禁じるルールは、golangci-lint が動かす depguard と forbidigo ではなく、`tools/cmd/arch` に置く。このコマンドはすべてのルールをディレクトリの構成から導くので、文脈を足しても設定は要らない。depguard と forbidigo には、すべての Go のファイルに当てはまるルールだけを残す。
- 別の文脈の `internal/` の下のパッケージを import するとコンパイルが通らないので、`lint-go` と `test-go` がそれを報告し、`lint-arch` も報告する（`arch/context-import`）。この重なりは認めている。記録を挙げ、代わりに文脈どうしをどうつなぐかを示すのは、`lint-arch` の指摘だけだからである（TPL-0005）。
- squawk は、SQL のファイルと、各 plan の DDL を読む。設定はそれぞれに1つある。plan 用の `.squawk.plan.toml` は、列の型のようにテーブルが何を持つかについてのルールを `lint-sql` に任せる。そのため、そうした問題は `schema.sql` の上で1回だけ報告される。
- フロントエンドの単体テストは、TypeScript をそのまま動かす Node のテストランナーで動かす。Vitest や Jest のようなテストのフレームワークは使わない。使えば、依存のツリーと専用の設定が1つずつ増えるからである。対象は純粋なロジックのモジュールで、これらは Next.js のものを何も import せず、互いを拡張子 `.ts` 付きで import する。画面の振る舞いは、エンドツーエンドテストの唯一の道具である Playwright がブラウザーでテストする。
- 新しいページと新しいフォームには、ブラウザーのシナリオをつける。コンポーネントとサーバー関数には、ほかにテストがないからである。新しいページについては、`lint-new-pages` がこの決まりを守らせる。変更が `page.tsx` を足したのに、シナリオを足しも変えもしていなければ、このチェックが失敗する。すでにあるページの表示を変えるだけなら、ロジックの単体テストで足り、そのためのシナリオは要らない。この決まりは、以前はもっと広く、ガイドだけが守らせていた。ハーネスの評価では、そういう変更にシナリオを書いた実行は、4回のうち1回だった（TPL-0020）。また、画面のどの変更がシナリオに値するかは、スクリプトでは決められない。
- `smoke` と `e2e` は、どちらも手元のシステムを端から端まで動かし、どちらも予約のメールを待つ。ただし、それぞれが確かめるのは、もう一方には見えないものである。スモークテストは、HTTP ですべての応答を API の文書と照らし合わせる。エンドツーエンドテストは、画面・サーバー関数・ストリーミングされるページの部分を確かめる。この重なりは認めている。
- Terraform のモジュールは `terraform test` でテストする。固定した Terraform が、各モジュールの隣の HCL のファイルを `mock_provider "aws"` に対して実行する。Terratest は使わない。使えば、テストのための Go のコードと専用のモジュールが増える。しかも、そのテストは本物のアカウントへ適用する。
- `validate-live` と `rehearse-infra` は、どちらもすべてのユニットを組み立てる。検証は Docker もエミュレーターも要らず、`env.hcl` の値を読まない。リハーサルは stg の値を適用し、prd の値を plan する。この重なりは認めている。
- `coverage-go` は、どのテストも実行しない行をすべてのパッケージについて報告するが、それで失敗することはない。`test-mutation` は、対象のパッケージにそうした行があると失敗する。この重なりは認めている。

道具の動かし方の決まりは次のとおりである。

- `make check` の時間の上限は60秒であり、`tools/scripts/check.sh` がこれを守らせる。
- `make setup` のあとは、`make check` はネットワークを使わない。`make setup` が Go のモジュールをダウンロードして Go のビルドのキャッシュを満たし、tflint のプラグインを入れる。lychee と zizmor はオフラインで動き、pinact は GitHub の API を使わずに固定を検査し、check-jsonschema は同梱のスキーマを使い、Trivy はバイナリに組み込まれたチェックを使う。
- `make check` はチェックを並列に動かし、チェックごとに1行を出し、指摘は失敗したチェックの分だけを出す。`VERBOSE=1` を付けると、すべてのチェックの出力が出る。
- 遅いチェックや、ネットワークが要るチェックは `make check-all` に回す。`make check-all` は CI が実行する。
- `make check` と `make check-all` のどのターゲットも `tools/scripts/task.sh` を通して動き、出力の形式がそろう。ファイルの一覧を受け取るチェックは、その一覧を `tools/scripts/files.sh` から得る。
- 外から持ち込んだファイルと生成されたファイルを、ファイルの一覧を受け取るチェックから外す一覧は、`.lintignore` の1つだけである。`files.sh` がこの一覧を適用する。ツリーを自分でたどる gitleaks と Trivy は、依存とキャッシュのディレクトリを自分の設定で飛ばす（`.gitleaks.toml`・`make/security.mk`）。Go のモジュール全体を読む golangci-lint は、生成された Go のファイルを先頭の注記で見分けて飛ばす（`.golangci.yml`）。
- `CHECK_SCOPE`（`staged` または `changed`）と `CHECK_FILES` は、`files.sh` からファイルを受け取るどのチェックも同じように絞り込む。ただし、変更が道具の固定やチェックの定義（`mise.toml`・ロックファイル・`Makefile`・`make/`・`tools/scripts/`・`tools/go.mod`・ルートの設定ファイル）に及ぶときは、すべてのファイルを範囲に入れる。そうした変更は、どのファイルの結果も変えうるからである。lefthook とエージェントのフックはこれらの変数を使い、CI はプルリクエストの遅いチェックにだけ使う（TPL-0018）。

### 結果

- 良い点：どの問題も、それを受け持つ道具から1回だけ報告される。例外は、意図して重ねた Trivy と govulncheck、およびスモークテストとエンドツーエンドテストだけである。
- 良い点：`make check` は、エージェントのターンの終わりにもコミットの前にも毎回実行できるほど速い。
- 良い点：新しいチェックは、`make/<area>.mk` の1つにターゲットを足し、自己テストにケースを足すだけで、範囲の絞り込み・除外の一覧・並列の実行・出力の形式を引き継ぐ。
- 悪い点：1つの関心事に道具を1つしか選ばないので、選ばなかった道具にしかないルールは失われる。
- 悪い点：道具ごとに設定ファイルがあり、yamllint と Prettier の配置のルールのような、道具の間の取り決めは手で保つ必要がある。
- 悪い点：時間の上限があるので、既知の脆弱性やリポジトリの外へのリンクのような役に立つチェックが `make check` から外れる。それらの指摘は、プッシュしたあとに CI で初めて出る。
- 悪い点：時間はチェックを動かしたマシンで測るので、速いマシンでは上限に収まる実行が、遅いマシンでは上限を超えることがある。2026年11月の時点で、ほかの作業をしていないノート PC では、すべてのファイルに対する `make check` がフロントエンドを含めて20秒ほどで終わった。ほかの作業で忙しいマシンでは、すべてのチェックが通る実行でも60秒を超え、上限だけで失敗することがある。同じ実行を繰り返せば通る。
- 悪い点：`test-mutation` の道具である gomutants は、v0.x の若いプロジェクトである。
- 悪い点：StrykerJS は、開発用のパッケージを124個持ち込む。`pnpm-workspace.yaml` には、その dashboard レポーターのために `qs` の上書きも要る。チェックは、このレポーターを使わない。

### 確認の方法

- 実行が `CHECK_BUDGET`（60秒）より長くかかると、`tools/scripts/check.sh` が `make check` を失敗させる。そのメッセージには、この記録が書かれている。
- `make check` と `make check-all` のどのターゲットも `tools/scripts/task.sh` を通して動く。ファイルの一覧を受け取るターゲットは、その一覧を `tools/scripts/files.sh` から得る。`files.sh` は `.lintignore`・`CHECK_SCOPE`・`CHECK_FILES` を適用する。
- `make check-all` は `selftest` を実行する。`selftest` は、`make check` のターゲットにケースがないとき、またはチェックが既知の悪い入力で失敗しなくなったときに失敗する。
- `test-mutation` と `test-mutation-frontend` は、`equivalent-mutants.allow` の行を確かめる。生き残ったミュータントのどれにも一致しない行があれば、失敗する（`mutation/unused-allowance`）。理由を書いていない行でも失敗する（`mutation/allowance-format`）。新しい行が正しいかどうかは、レビューで決める。
- 新しい道具は、レビューでこの記録の表と照らし合わせる。すでに道具のある関心事のための道具は、新しい記録を書いてその道具を置き換えるか、入れないかのどちらかである。

## 選択肢の長所と短所

### 1つの関心事に道具を1つだけ使い、make から動かし、時間の上限を設ける

- 良い点：どの種類の指摘も出どころが1つで、形式もわかっている。
- 良い点：人・エージェント・Git のフック・CI が、同じターゲットを使える。
- 悪い点：Makefile とそのスクリプトは、このリポジトリが保守するコードである。

### MegaLinter や super-linter のような、リンターを束ねる道具

- 良い点：1つのコンテナで、少ない設定で数十のリンターが手に入る。
- 悪い点：既定では1つの言語に複数のリンターを動かすので、指摘が重なる。それを1つずつ止める手間は、道具を選ぶ手間と変わらない。
- 悪い点：大きなコンテナイメージとして配られ、道具のバージョンはこのリポジトリの固定ではなく、リンターを束ねる道具のリリースに従う。

### pre-commit フレームワークを実行役にする

- 良い点：広く使われており、ほとんどの道具にフックが用意されている。
- 悪い点：フックごとに専用の環境を作って入れるので、道具のバージョンを `mise.toml` やロックファイルとは別の場所でも固定することになる。
- 悪い点：`make` と並ぶ2つめの実行役になる。テストやスキャンのようにフックでないものには、CI もエージェントも結局 `make` を必要とする。

### どの道具も既定の設定のまま、CI でだけ動かす

- 良い点：書く設定がない。
- 悪い点：人もエージェントも、プッシュしたあとでしか問題に気づけない。
- 悪い点：道具ごとの既定の設定は、範囲が重なり、食い違う。

## 補足

- `make help` が主なターゲットを一覧にし、`make/<area>.mk` が領域ごとのターゲットを持つ。
- これらの道具の固定は TPL-0022 に、Trivy と GitHub の側のスキャナーを選んだ理由は TPL-0024 にある。
- [ミューテーションテスト](../../guides/mutation-tests.ja.md)が、`test-mutation` と `test-mutation-frontend` の指摘の読み方を説明している。画面に出す言葉をテストがどう扱うかと、`equivalent-mutants.allow` の行がふさわしい場合も、同じガイドにある。

<!-- i18n-source: TPL-0023-one-tool-per-concern.md sha256:abc88f1226c4c54e9318d820029880cced64161dcf2fbe3cb2bca771feb1c438 -->

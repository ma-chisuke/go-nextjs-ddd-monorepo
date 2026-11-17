---
status: accepted
date: 2026-11-17
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
| テストのカバレッジ（外部のサービスを使わずに報告する） | `go test -coverprofile` と octocov | `coverage` と `coverage-report`（CI） |
| 実際に呼ばれている Go のコードにある既知の脆弱性 | govulncheck | `vuln-go`（check-all） |
| フロントエンドの lint（機能どうしの境界を含む） | ESLint（eslint-config-next・typescript-eslint・eslint-plugin-boundaries を使う） | `lint-frontend` |
| フロントエンドの型 | TypeScript のコンパイラー（`tsc --noEmit`） | `lint-frontend-types` |
| フロントエンドの、どこからも使われていないファイル・export・依存 | knip | `lint-frontend-unused` |
| YAML・JSON・JSON5・JavaScript・TypeScript・CSS の整形 | Prettier | `fmt-check-prettier` |
| YAML の lint（キーの重複や曖昧な値など） | yamllint | `lint-yaml` |
| TOML の整形と lint | taplo | `fmt-check-toml`・`lint-toml` |
| 設定ファイルのスキーマによる検証 | check-jsonschema | `lint-schema`・`lint-schema-remote`（check-all） |
| シェルスクリプトの整形 | shfmt | `fmt-check-shell` |
| シェルスクリプトの lint | shellcheck | `lint-shell` |
| Dockerfile の lint | hadolint | `lint-dockerfile` |
| ワークフローの構文 | actionlint | `lint-actions` |
| ワークフローのセキュリティ | zizmor | `lint-actions-security` |
| ワークフローの方針（権限とタイムアウト） | ghalint | `lint-actions-policy` |
| アクションのコミット SHA の全桁での固定 | pinact | `lint-actions-pins` |
| Terraform のファイルの整形 | `terraform fmt` | `fmt-check-terraform` |
| それ以外の HCL のファイル（`terragrunt.hcl` や `root.hcl` など）の整形 | `terragrunt hcl fmt` | `fmt-check-terragrunt` |
| Terraform のコードの lint | tflint（AWS のルールセットを使う） | `lint-terraform` |
| 古くなったモジュールの README | terraform-docs | `lint-terraform-docs` |
| Terraform のコードの妥当性 | `terraform validate` | `validate-terraform`（check-all） |
| SQL のファイル | squawk | `lint-sql` |
| OpenAPI と AsyncAPI の文書 | Redocly CLI（`@redocly/cli`） | `lint-contracts` |
| Markdown の整形と構造 | markdownlint-cli2 | `lint-md` |
| 英語の文章 | Vale（Google のスタイル） | `lint-prose-en` |
| 日本語の文章 | textlint | `lint-prose-ja` |
| リンク | lychee | `lint-links`・`lint-links-remote`（check-all） |
| 決定の記録とその一覧（TPL-0001） | このリポジトリの docs コマンド（`tools/cmd/docs`） | `lint-adr` |
| 英語版と照らし合わせた対訳（TPL-0021） | このリポジトリの docs コマンド | `lint-i18n` |
| Go のバックエンドの境界（文脈と層のあいだの import、I/O のないドメイン、手で登録したルート、SQL に書いたスキーマの名前、文脈ごとの用語集。TPL-0003・TPL-0004・TPL-0005・TPL-0007・TPL-0009） | このリポジトリの arch コマンド（`tools/cmd/arch`） | `lint-arch` |
| `AGENTS.md` の長さ（TPL-0020） | `make/agents.mk` で行数を数える | `lint-agents-md` |
| つづり | typos | `lint-typos` |
| 改行コード・末尾の改行・行末の空白・インデントの種類 | editorconfig-checker | `lint-editorconfig` |
| 秘密情報 | gitleaks | `lint-secrets` |
| Dockerfile と Terraform の設定ミス | Trivy | `scan-config` |
| 依存の脆弱性とライセンス | Trivy | `scan-deps`（check-all） |
| エージェントのフック（TPL-0020） | 各フックに見本のイベントを与えるテスト（`tools/tests/hooks_test.sh`） | `test-hooks` |
| コミットメッセージを検査するスクリプト | 見本の件名を、いくつかのロケールで試すテスト（`tools/tests/commit_msg_test.sh`） | `test-commit-msg` |
| チェックそのもの（既知の悪い入力で失敗し、既知の良い入力で通るか） | `tools/selftest/selftest.sh` | `selftest`（check-all） |
| コミットメッセージとプルリクエストの題（Conventional Commits） | `tools/scripts/commit-msg.sh` | lefthook の `commit-msg` フックと、`lint-pr-title`（CI） |

次のチェックは、後のマイルストーンで加わる。`sqlc vet`（TPL-0011）・tbls の lint・pgschema の plan の DDL に対する squawk（TPL-0010）・oasdiff（TPL-0007）は、最初の区切られた文脈とともに加わる。Schemathesis は、それを加えるマイルストーンで加わる（TPL-0007）。

重なりは、次のように解消した。

- `go vet` は、ほかの Go のリンターと一緒に golangci-lint の中で動き、別の手順にはしない。gofmt と goimports も、整形の道具としてそこで動く。
- つづりは typos が受け持つので、Vale のつづりのルールは切ってある（`.vale.ini`）。
- Vale は英語の Markdown だけを、textlint は日本語の Markdown だけを読む。
- Markdown の整形は、Prettier ではなく markdownlint-cli2 の修正機能が行う。そのため、Markdown の整形の道具とリンターが食い違うことはない。
- YAML は Prettier が整形し、yamllint が lint する。yamllint の配置のルールは Prettier と合わせてあり、行の長さは Prettier だけが受け持つ（`.yamllint.yaml`）。
- シェルの書式は `.editorconfig` に書き、shfmt はそれを読む。そのため、エディターと shfmt は同じ設定を使う。
- editorconfig-checker はインデントの幅を検査しない。インデントの幅は整形の道具が受け持つ。
- `terraform fmt` は Terraform の言語のファイルを、`terragrunt hcl fmt` はそれ以外の HCL のファイルを整形する。そのため、1つのファイルを2つの整形の道具が扱うことはない。
- Trivy を選んだ理由は TPL-0024 にある。重なりは1つだけ認めている。Trivy はすべてのロックファイルにある脆弱なモジュールをすべて報告し、govulncheck は Go についての指摘を、実際に呼ばれているコードに絞る。そのため、呼ばれている Go のコードにある脆弱性は、両方から報告されることがある。
- GitHub の側のスキャナー（CodeQL とシークレットスキャン。TPL-0024）は、`make` の外で動く。golangci-lint（gosec）や gitleaks と重なるが、これは意図したものである。手元で何が起きても GitHub の上で動く、もう1つの層だからである。
- コミットメッセージとプルリクエストの題は、このリポジトリの短いスクリプト `tools/scripts/commit-msg.sh` が検査する。commitlint のような Node.js のパッケージも、別のバイナリも使わない。固定するバイナリには、`mise.lock` の4つのプラットフォームのそれぞれでビルドが要る（TPL-0022）。最初に選んだ committed は Linux の arm64 向けのビルドを出していない。スクリプトなら依存が要らない。
- OpenAPI と AsyncAPI の文書は、どちらも Redocly CLI で lint する。最初の計画では、AsyncAPI のコマンドラインの道具と、候補として Spectral も挙げていた。どちらも入れていない。1つの道具で両方の契約を扱え、リポジトリが抱える大きな依存のツリーが1つ減るからである。
- 層の import のルールと、ドメインで時計を読むことを禁じるルールは、golangci-lint が動かす depguard と forbidigo ではなく、`tools/cmd/arch` に置く。このコマンドはすべてのルールをディレクトリの構成から導くので、文脈を足しても設定は要らない。depguard と forbidigo には、すべての Go のファイルに当てはまるルールだけを残す。
- 別の文脈の `internal/` の下のパッケージを import するとコンパイルが通らないので、`lint-go` と `test-go` がそれを報告し、`lint-arch` も報告する（`arch/context-import`）。この重なりは認めている。記録を挙げ、代わりに文脈どうしをどうつなぐかを示すのは、`lint-arch` の指摘だけだからである（TPL-0005）。

道具の動かし方の決まりは次のとおりである。

- `make check` の時間の上限は60秒であり、`tools/scripts/check.sh` がこれを守らせる。
- `make setup` のあとは、`make check` はネットワークを使わない。`make setup` が Go のモジュールをダウンロードして Go のビルドのキャッシュを満たし、tflint のプラグインを入れる。lychee と zizmor はオフラインで動き、pinact は GitHub の API を使わずに固定を検査し、check-jsonschema は同梱のスキーマを使い、Trivy はバイナリに組み込まれたチェックを使う。
- `make check` はチェックを並列に動かし、チェックごとに1行を出し、指摘は失敗したチェックの分だけを出す。`VERBOSE=1` を付けると、すべてのチェックの出力が出る。
- 遅いチェックや、ネットワークが要るチェックは `make check-all` に回す。`make check-all` は CI が実行する。
- `make check` と `make check-all` のどのターゲットも `tools/scripts/task.sh` を通して動き、出力の形式がそろう。ファイルの一覧を受け取るチェックは、その一覧を `tools/scripts/files.sh` から得る。
- 外から持ち込んだファイルと生成されたファイルを、ファイルの一覧を受け取るチェックから外す一覧は、`.lintignore` の1つだけである。`files.sh` がこの一覧を適用する。ツリーを自分でたどる gitleaks と Trivy は、依存とキャッシュのディレクトリを自分の設定で飛ばす（`.gitleaks.toml`・`make/security.mk`）。Go のモジュール全体を読む golangci-lint は、生成された Go のファイルを先頭の注記で見分けて飛ばす（`.golangci.yml`）。
- `CHECK_SCOPE`（`staged` または `changed`）と `CHECK_FILES` は、`files.sh` からファイルを受け取るどのチェックも同じように絞り込む。ただし、変更が道具の固定やチェックの定義（`mise.toml`・ロックファイル・`Makefile`・`make/`・`tools/scripts/`・`tools/go.mod`・ルートの設定ファイル）に及ぶときは、すべてのファイルを範囲に入れる。そうした変更は、どのファイルの結果も変えうるからである。lefthook とエージェントのフックはこれらの変数を使い、CI はプルリクエストの遅いチェックにだけ使う（TPL-0018）。

### 結果

- 良い点：どの問題も、それを受け持つ道具から1回だけ報告される。例外は、意図して重ねた Trivy と govulncheck だけである。
- 良い点：`make check` は、エージェントのターンの終わりにもコミットの前にも毎回実行できるほど速い。
- 良い点：新しいチェックは、`make/<area>.mk` の1つにターゲットを足し、自己テストにケースを足すだけで、範囲の絞り込み・除外の一覧・並列の実行・出力の形式を引き継ぐ。
- 悪い点：1つの関心事に道具を1つしか選ばないので、選ばなかった道具にしかないルールは失われる。
- 悪い点：道具ごとに設定ファイルがあり、yamllint と Prettier の配置のルールのような、道具の間の取り決めは手で保つ必要がある。
- 悪い点：時間の上限があるので、既知の脆弱性やリポジトリの外へのリンクのような役に立つチェックが `make check` から外れる。それらの指摘は、プッシュしたあとに CI で初めて出る。
- 悪い点：時間はチェックを動かしたマシンで測るので、速いマシンでは上限に収まる実行が、遅いマシンでは上限を超えることがある。

### 確認の方法

- 実行が `CHECK_BUDGET`（60秒）より長くかかると、`tools/scripts/check.sh` が `make check` を失敗させる。そのメッセージには、この記録が書かれている。
- `make check` と `make check-all` のどのターゲットも `tools/scripts/task.sh` を通して動く。ファイルの一覧を受け取るターゲットは、その一覧を `tools/scripts/files.sh` から得る。`files.sh` は `.lintignore`・`CHECK_SCOPE`・`CHECK_FILES` を適用する。
- `make check-all` は `selftest` を実行する。`selftest` は、`make check` のターゲットにケースがないとき、またはチェックが既知の悪い入力で失敗しなくなったときに失敗する。
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

<!-- i18n-source: TPL-0023-one-tool-per-concern.md sha256:79edab17fcf59dec9fd98d9a1529e4d74227b2c51402ced929e48494dff53551 -->

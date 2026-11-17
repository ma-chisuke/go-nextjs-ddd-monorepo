---
status: accepted
date: 2026-11-17
---

# TPL-0023: 1つの関心事にツールを1つだけ使い、どのツールも make から動かし、`make check` に時間の上限を設ける

## 背景と課題

Go・TypeScript・Terraform・シェルスクリプト・YAML・2つの言語の Markdown を持つリポジトリには、整形ツール・linter・スキャナーが数多く要る。
これらのツールの多くは、受け持つ範囲が重なっている。2つの linter が同じ指摘を出し、2つの整形ツールが同じファイルを別々の形に書き換え、2つのスキャナーが同じ脆弱性を別の言葉で報告する。
1つの問題に2つの指摘を受けたり、食い違う2つの指摘を受けたりしたエージェントは、ターンを無駄にするか、2つの指摘の間を行き来する。

チェックは、増えるにつれて遅くもなる。
数分かかるチェックは、人もエージェントも実行する回数が減る。エージェントのターンの終わりに毎回遅いチェックが走れば、どのターンも遅くなる。

どの問題も1回だけ報告されるようにツールを選び、変更のたびに実行できる速さをチェックに保つにはどうすればよいか。

## 判断の基準

- どの指摘も、ちょうど1つのツールから出る。
- 人・エージェント・CI・Git のフックが、同じコマンドを実行する。
- `make check` は、エージェントのターンの終わりに毎回実行できるほど速い。
- 出力は短い。チェックごとに1行を出し、指摘は失敗したチェックの分だけを出す。
- 指摘は1件1行で、少なくともファイルと行を示す。エージェントは、人の手を借りずにそれをもとに動ける。
- どのチェックも触れてはならないファイルを、1か所にまとめて書く。

## 検討した選択肢

- 1つの関心事にツールを1つだけ使い、`make` の後ろに置き、`make check` に時間の上限を設ける
- MegaLinter や super-linter のような、linter を束ねるツール
- pre-commit フレームワークを実行役にする
- どのツールも既定の設定のまま、CI でだけ動かす

## 決定

採用：「1つの関心事にツールを1つだけ使い、`make` の後ろに置き、`make check` に時間の上限を設ける」。どのツールが何を報告するかと、チェックにかかる時間の両方を制御できる選択肢は、これだけだからである。

どの関心事にも、ツールはちょうど1つである。「check-all」と書いたターゲットは、`make check-all` でだけ動く。

| 関心事 | ツール | ターゲット |
|---|---|---|
| Go のコードの整形 | golangci-lint（整形には gofmt と goimports を使う） | `fmt-check-go` |
| Go のコードの lint（`go vet` を含む） | golangci-lint | `lint-go` |
| Go のテスト | `go test -race -shuffle=on` | `test-go` |
| 実際に呼ばれている Go のコードにある既知の脆弱性 | govulncheck | `vuln-go`（check-all） |
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
| Markdown の整形と構造 | markdownlint-cli2 | `lint-md` |
| 英語の文章 | Vale（Google のスタイル） | `lint-prose-en` |
| 日本語の文章 | textlint | `lint-prose-ja` |
| つづり | typos | `lint-typos` |
| リンク | lychee | `lint-links`・`lint-links-remote`（check-all） |
| 改行コード・末尾の改行・行末の空白・インデントの種類 | editorconfig-checker | `lint-editorconfig` |
| 秘密情報 | gitleaks | `lint-secrets` |
| Dockerfile と Terraform の設定ミス | Trivy | `scan-config` |
| 依存の脆弱性とライセンス | Trivy | `scan-deps`（check-all） |
| コミットメッセージ | committed | lefthook の `commit-msg` フック |
| OpenAPI と AsyncAPI の文書 | Redocly（`@redocly/cli`） | 最初の契約ができるまではない |

すでに固定してあり、担当の領域ができたときにターゲットを持つツールもある。インフラの Terraform・Terragrunt・tflint・terraform-docs、SQL の squawk、テストのカバレッジの octocov である。

重なりは、次のように解消した。

- `go vet` は、ほかの Go の linter と一緒に golangci-lint の中で動き、別の手順にはしない。gofmt と goimports も、整形ツールとしてそこで動く。
- つづりは typos が受け持つので、Vale のつづりのルールは切ってある（`.vale.ini`）。
- Vale は英語の Markdown だけを、textlint は日本語の Markdown だけを読む。
- Markdown の整形は、Prettier ではなく markdownlint-cli2 の修正機能が行う。そのため、Markdown の整形ツールと linter が食い違うことはない。
- YAML は Prettier が整形し、yamllint が lint する。yamllint の配置のルールは Prettier と合わせてあり、行の長さは Prettier だけが受け持つ（`.yamllint.yaml`）。
- シェルの書式は `.editorconfig` に書き、shfmt はそれを読む。そのため、エディターと shfmt は同じ設定を使う。
- editorconfig-checker はインデントの幅を検査しない。インデントの幅は整形ツールが受け持つ。
- 設定ミス・脆弱性・ライセンス、そして後には SBOM も Trivy が受け持つので、2つめのスキャナーはない。最初の計画では、SBOM のもう1つの候補として syft を挙げていた。
- コミットメッセージは、Node.js のパッケージ（commitlint など）ではなく、単体のバイナリである committed が検査する。
- OpenAPI と AsyncAPI の文書は、どちらも Redocly で lint する。最初の計画では、AsyncAPI のコマンドラインツールと、候補として Spectral も挙げていた。どちらも入れていない。1つのツールで両方の契約を扱え、リポジトリが抱える大きな依存のツリーが1つ減るからである。

ツールの動かし方の決まりは次のとおりである。

- `make check` の時間の上限は60秒であり、`tools/scripts/check.sh` がこれを守らせる。
- `make check` はネットワークを使わない。lychee と zizmor はオフラインで動き、check-jsonschema は同梱のスキーマを使い、Trivy はバイナリに組み込まれたチェックを使う。
- `make check` はチェックを並列に動かし、チェックごとに1行を出し、指摘は失敗したチェックの分だけを出す。`VERBOSE=1` を付けると、すべてのチェックの出力が出る。
- 遅いチェックや、ネットワークが要るチェックは `make check-all` に回す。`make check-all` は CI が実行する。
- 外から持ち込んだファイルと生成されたファイルをすべてのチェックから外す一覧は、`.lintignore` の1つだけである。`tools/scripts/files.sh` がこの一覧を適用する。
- `CHECK_SCOPE`（`staged` または `changed`）と `CHECK_FILES` は、どのターゲットも同じように絞り込む。lefthook とエージェントのフックは、これらを使う。

### 結果

- 良い点：どの問題も、それを受け持つツールから1回だけ報告される。
- 良い点：`make check` は、エージェントのターンの終わりにもコミットの前にも毎回実行できるほど速い。
- 良い点：新しいツールは、`make/<area>.mk` の1つにターゲットを足すだけで、範囲の絞り込み・除外の一覧・並列の実行・出力の形式を引き継ぐ。
- 悪い点：1つの関心事にツールを1つしか選ばないので、選ばなかったツールにしかないルールは失われる。
- 悪い点：ツールごとに設定ファイルがあり、yamllint と Prettier の配置のルールのような、ツールの間の取り決めは手で保つ必要がある。
- 悪い点：時間の上限があるので、既知の脆弱性やリポジトリの外へのリンクのような役に立つチェックが `make check` から外れる。それらの指摘は、プッシュしたあとに CI で初めて出る。
- 悪い点：時間はチェックを動かしたマシンで測るので、速いマシンでは上限に収まる実行が、遅いマシンでは上限を超えることがある。

### 確認の方法

- 実行が `CHECK_BUDGET`（60秒）より長くかかると、`tools/scripts/check.sh` が `make check` を失敗させる。そのメッセージには、この記録が書かれている。
- どのチェックのターゲットも `tools/scripts/task.sh` と `tools/scripts/files.sh` を通して動く。この2つが、出力の形式・`.lintignore`・`CHECK_SCOPE`・`CHECK_FILES` を適用する。
- 新しいツールは、レビューでこの記録の表と照らし合わせる。すでにツールのある関心事のためのツールは、新しい記録を書いてそのツールを置き換えるか、入れないかのどちらかである。

## 選択肢の長所と短所

### 1つの関心事にツールを1つだけ使い、make の後ろに置き、時間の上限を設ける

- 良い点：どの指摘も出どころが1つで、形式もわかっている。
- 良い点：人・エージェント・Git のフック・CI が、同じターゲットを使える。
- 悪い点：Makefile とそのスクリプトは、このリポジトリが保守するコードである。

### MegaLinter や super-linter のような、linter を束ねるツール

- 良い点：1つのコンテナで、少ない設定で数十の linter が手に入る。
- 悪い点：既定では1つの言語に複数の linter を動かすので、指摘が重なる。それを1つずつ止める手間は、ツールを選ぶ手間と変わらない。
- 悪い点：大きなコンテナイメージとして配られ、ツールの版はこのリポジトリの固定ではなく、linter を束ねるツールのリリースに従う。

### pre-commit フレームワークを実行役にする

- 良い点：広く使われており、ほとんどのツールにフックが用意されている。
- 悪い点：フックごとに専用の環境を作って入れるので、ツールの版を `mise.toml` やロックファイルとは別の場所でも固定することになる。
- 悪い点：`make` と並ぶ2つめの実行役になる。テストやスキャンのようにフックでないものには、CI もエージェントも結局 `make` を必要とする。

### どのツールも既定の設定のまま、CI でだけ動かす

- 良い点：書く設定がない。
- 悪い点：人もエージェントも、プッシュしたあとでしか問題に気づけない。
- 悪い点：ツールごとの既定の設定は、範囲が重なり、食い違う。

## 補足

- `make help` が主なターゲットを一覧にし、`make/<area>.mk` が領域ごとのターゲットを持つ。
- これらのツールの固定は TPL-0022 に、Trivy をただ1つのスキャナーにする理由は TPL-0024 にある。

<!-- i18n-source: TPL-0023-one-tool-per-concern.md sha256:2c8de124df5072566e1c044136448ca854f3a6f288635d243323dd906d9288b4 -->

# go-nextjs-ddd-monorepo

Go の API と Next.js の画面を1つのリポジトリに置く製品のためのテンプレートである。ドメイン駆動設計の区切られた文脈ごとにコードを分け、コーディングエージェントと人が同じチェックのもとで作業できるように作ってある。

[English](README.md)

## 状態

テンプレートはマイルストーンごとに作る。最初に品質の基盤を作るので、アプリのコードは最初のコミットからチェックを受ける。

| マイルストーン | 中身 | 状態 |
|---|---|---|
| M0 | 品質の基盤：道具のバージョンの固定、`make check`、整形と lint、セキュリティの検査、CI、Git フック、エージェントのフック、決定の記録 | 完了 |
| M1 | 中核と補完の文脈を1つずつ。OpenAPI からユースケースとドメインを通って PostgreSQL まで | 予定 |
| M2 | イベント：送信箱、EventBridge、SQS、手元の AWS エミュレーター | 予定 |
| M3 | Next.js の画面と、端から端までのテスト | 予定 |
| M4 | stg と prd の Terraform・Terragrunt と、デプロイ | 予定 |
| M5 | エージェントのハーネスの評価、性質にもとづく API のテスト、ミューテーションテスト | 予定 |

見本の業務は会議室の予約で、施設（Facility）・予約（Reservation）・通知（Notification）の3つの文脈からなる。

## テンプレートが決めてあること

どの決定にも、理由と、選ばなかった選択肢を書いた記録がある。一覧は [docs/adr/](docs/adr/README.ja.md) にある。

- **アーキテクチャ。** 文脈どうしは HTTP の API とイベントだけで依存する。文脈の中はクリーンアーキテクチャで、ドメイン層は I/O をしない。
- **契約。** OpenAPI を先に書く。Go のサーバーのコードと TypeScript の型を生成し、何層かのチェックで文書とコードの一致を保つ。エラーは RFC 9457 の problem details で返す。
- **データ。** PostgreSQL 18 のデータベースを1つ使い、文脈ごとにスキーマとロールを分ける。スキーマは宣言的に管理し、クエリには型を付ける。
- **AWS。** 文脈ごとの ECS、Aurora、EventBridge と SQS、Terraform と Terragrunt を使う。stg と prd はアカウントを分ける。手元の環境は AWS のアカウントなしで動く。
- **品質。** 1つの関心事に道具を1つ割り当て、`make check` から実行する。道具のバージョンはチェックサムとともに固定する。サプライチェーンを守る設定を入れ、CI では手元と同じコマンドを実行する。
- **エージェント。** 決まりは、直し方を出力するチェックで守る。`AGENTS.md` は短く保つ。
- **言語。** 英語を正とし、どの文書にも日本語の対訳を隣に置く。

## 始め方

Git、GNU Make、[mise](https://mise.jdx.dev/installing-mise.html) が要る。ほかの道具はすべて、mise が `mise.toml` で固定したバージョンを入れる。

```sh
make setup   # 道具、依存、Git フック
make check   # 速いチェック。数秒で終わる
```

| コマンド | 使いどころ |
|---|---|
| `make setup` | 固定した道具・依存・Git フックを入れる。 |
| `make fmt` | ファイルを整形する。 |
| `make gen` | 生成物を作り直す。 |
| `make check` | 速いチェック。ネットワークを使わない。コミットの前に必ず実行する。 |
| `make check-all` | 速いチェックと遅いチェック。CI が実行する。 |
| `make help` | 主なターゲットの一覧を出す。 |

`.devcontainer/` の dev container を使うと、手元に何も入れずに同じ環境が手に入る。

## テンプレートの使い方

1. GitHub で、このテンプレートからリポジトリを作る。
2. クローンして `make setup` を実行する。
3. `make init MODULE=github.com/<owner>/<repository> NAME=<project>` を実行し、モジュールのパスとプロジェクト名を置き換える。
4. [GitHub の設定のガイド](docs/guides/github-settings.ja.md)に沿って、リポジトリの設定を適用する。
5. 自分たちの決定を `docs/adr/` に記録する。`docs/adr/template/` はそのままにしておく。そうすれば、自分たちのコピーをテンプレートの新しいバージョンと比べられる。

## `make check` が見るもの

| 分野 | チェック |
|---|---|
| すべてのファイル | つづり、EditorConfig の決まり、秘密情報 |
| Go | 整形、golangci-lint（go vet を含む）、race detector 付きのテスト。既知の脆弱性は `make check-all` |
| TypeScript と Next.js | Prettier、import の境界を含む ESLint、型の検査、使っていないコード |
| シェルと Dockerfile | shfmt、shellcheck、hadolint |
| GitHub Actions | actionlint、zizmor、ghalint、pinact |
| Terraform と Terragrunt | 整形、tflint、Trivy。validate とモジュールの文書は `make check-all` |
| SQL | squawk |
| OpenAPI と AsyncAPI | Redocly |
| YAML・JSON・TOML | Prettier、yamllint、taplo、設定ファイルの JSON Schema による検証 |
| Markdown | markdownlint、リンク、英語の文体（Vale）、日本語の文体（textlint） |
| 文書 | 決定の記録の形式とその一覧、英語版と一致する対訳 |
| 依存 | Trivy による脆弱性とライセンスの検査。`make check-all` |

`make check` には60秒の上限があり、ネットワークを使わない。指摘は1件が1行で、ファイル・行・ルールが出る。

## 文書

- [AGENTS.ja.md](AGENTS.ja.md)：コーディングエージェント向けの指示。
- [docs/adr/](docs/adr/README.ja.md)：決定の記録。
- [docs/guides/](docs/guides/)：繰り返す作業のやり方。
- [SECURITY.ja.md](SECURITY.ja.md)：脆弱性の報告のしかた。

## 第三者のファイル

- `.vale/styles/Google/`：Vale 用の Google developer documentation style。MIT License。
- `.claude/skills/japanese-tech-writing/`：鹿野桂一郎さんによる日本語の技術文書の文章規範。Unlicense。

## ライセンス

[MIT No Attribution](LICENSE)。表示を残さずに、テンプレートをコピーして自分のものにできる。

<!-- i18n-source: README.md sha256:5230753be82869f18f3c6eaebf964c8fae0f54a147e029e29cba7a2956935a55 -->

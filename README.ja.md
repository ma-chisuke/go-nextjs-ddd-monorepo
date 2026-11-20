# go-nextjs-ddd-monorepo

Go の API と Next.js の画面を1つのリポジトリに置く製品のためのテンプレートである。ドメイン駆動設計の区切られた文脈ごとにコードを分け、コーディングエージェントと人が同じチェックのもとで作業できるように作ってある。

[English](README.md)

## 状態

テンプレートはマイルストーンごとに作った。最初に品質の基盤を作ったので、アプリのコードは最初のコミットからチェックを受けている。どのマイルストーンも完了した。M4 と M5 が残したことは、表に書いてある。

| マイルストーン | 中身 | 状態 |
|---|---|---|
| M0 | 品質の基盤：道具のバージョンの固定、`make check`、整形と lint、セキュリティの検査、CI、Git フック、エージェントのフック、決定の記録 | 完了 |
| M1 | 中核と補完の文脈を1つずつ。OpenAPI からユースケースとドメインを通って PostgreSQL まで | 完了 |
| M2 | イベント：送信箱、EventBridge、SQS、手元の AWS エミュレーターと、文脈 Notification | 完了 |
| M3 | Next.js のフロントエンドと、エンドツーエンドテスト | 完了 |
| M4 | stg と prd の Terraform・Terragrunt と、デプロイ | 完了。ただし AWS にはまだ適用していない。モジュールは AWS provider のモックに対してテストし、すべてのユニットを検証し、stg を AWS Emulator でリハーサルしている |
| M5 | エージェントのハーネスの評価、性質にもとづく API のテスト、ミューテーションテスト | 完了。ただし1つが残っている。評価の実行は、6つの作業のそれぞれの条件で1回ずつ、計12回だけである。測定ではなく最初の観察にとどまる |

見本の業務は会議室の予約で、施設（Facility）・予約（Reservation）・通知（Notification）の3つの文脈からなる。

## テンプレートが決めてあること

どの決定にも、理由と、選ばなかった選択肢を書いた記録がある。一覧は [docs/adr/](docs/adr/README.ja.md) にある。

- **アーキテクチャ。** 文脈どうしは HTTP の API とイベントだけで依存する。文脈の中はクリーンアーキテクチャで、ドメイン層は I/O をしない。
- **契約。** OpenAPI を先に書く。Go のサーバーのコードと TypeScript の型を生成し、何層かのチェックで文書とコードの一致を保つ。エラーは RFC 9457 の problem details で返す。
- **データ。** PostgreSQL 18 のデータベースを1つ使い、文脈ごとにスキーマを1つとロールを2つ持たせる。スキーマは宣言的に管理し、クエリには型を付ける。
- **AWS。** 文脈ごとの ECS（API は非公開で、公開のロードバランサーは1つ）、Aurora、EventBridge と SQS、Terraform と Terragrunt を使う。stg と prd はアカウントを分け、GitHub Actions から OIDC でデプロイする。手元の環境は AWS のアカウントなしで動く。
- **品質。** 1つの関心事に道具を1つ割り当て、`make check` から実行する。道具のバージョンはチェックサムとともに固定する。サプライチェーンを守る設定を入れ、CI では手元と同じコマンドを実行する。
- **エージェント。** 決まりは、直し方を出力するチェックで守る。`AGENTS.md` は短く保ち、指示を足すのは、どのチェックでも捕まえられない失敗を `evals/` の評価が示したときだけにする。
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
| `make check-all` | 速いチェックと遅いチェック。手元の PostgreSQL と AWS Emulator を要するチェックも含む。CI が実行する。 |
| `make up` | 手元のシステムを Docker で起動する。すべてのスキーマを適用した PostgreSQL、イベントバスとキューを作った AWS Emulator、Mailpit、すべての文脈のすべてのプロセス、<http://127.0.0.1:13000> のフロントエンドである。`make down` で止める。 |
| `make db-up` | 手元の PostgreSQL を Docker で起動し、すべての文脈のスキーマを適用する。 |
| `make infra-plan ENV=stg` | AWS の環境のすべてのユニットを、そのアカウントの認証情報で plan する。 |
| `make deploy ENV=stg` | 変わったイメージをプッシュし、データベースを準備し、環境のすべてのユニットを適用する。prd へのデプロイは、ワークフロー `Deploy` からしかできない。 |
| `make infra-bootstrap ENV=stg` | AWS アカウントごとに1回、管理者として実行する。Terraform の state のバケットと、GitHub Actions のロールを作る。 |
| `make infra-lock` | `infra/live/providers.hcl` を変えたあとに、provider のチェックサムを書く。 |
| `make help` | 主なターゲットの一覧を出す。 |

`.devcontainer/` の dev container を使うと、手元に何も入れずに同じ環境が手に入る。

手元のシステムと `make check-all` には Docker が要る。`make up` のあと <http://127.0.0.1:13000> を開くと、会議室を登録して予約できる。[サービスを手元で動かす](docs/guides/run-locally.ja.md)では、API を通して curl でも会議室を予約し、予約から生じるメールを Mailpit で見る。

| サービス | アドレス |
|---|---|
| フロントエンド | <http://127.0.0.1:13000> |
| Mailpit（手元のシステムのメール） | <http://127.0.0.1:18025> |
| Facility と Reservation の API | <http://127.0.0.1:18081>、<http://127.0.0.1:18082> |
| PostgreSQL、AWS Emulator | `127.0.0.1:15432`、`https://localhost:19297` |

## テンプレートの使い方

1. GitHub で、このテンプレートからリポジトリを作る。
2. クローンして `make setup` を実行する。
3. `make init MODULE=github.com/<owner>/<repository> NAME=<project>` を実行し、モジュールのパスとプロジェクト名を置き換える。
4. [GitHub の設定のガイド](docs/guides/github-settings.ja.md)に沿って、リポジトリの設定を適用する。
5. デプロイするには、stg と prd の AWS アカウントを1つずつ準備する。手順は[AWS にデプロイする](docs/guides/deploy.ja.md)が説明する。それまでは、AWS に届くワークフローはスキップされる。
6. 自分たちの決定を `docs/adr/` に記録する。`docs/adr/template/` はそのままにしておく。そうすれば、自分たちのコピーをテンプレートの新しいバージョンと比べられる。

## `make check` が見るもの

| 分野 | チェック |
|---|---|
| すべてのファイル | つづり、EditorConfig の決まり、秘密情報 |
| Go | 整形、golangci-lint（go vet を含む）、race detector 付きのテスト。`make check-all` では、既知の脆弱性、PostgreSQL と AWS Emulator に対する結合テスト、手元のシステム全体のスモークテスト、中核の文脈のドメインとユースケースに対するミューテーションテスト（`test-mutation`） |
| バックエンドの構成 | `lint-arch`。文脈と層の間の import、I/O をしないドメイン、API の文書からだけ作るルート、SQL に書いたスキーマの名前、文脈ごとの用語集 |
| 生成コード | API の文書と各文脈の SQL から生成した Go のコードと、API の文書から生成した TypeScript の型（`gen-check-api-ts`）が、生成器がいま書く内容と同じであること |
| TypeScript と Next.js | Prettier、import の境界を含む ESLint、型の検査、使っていないコード、Node のテストランナーで動く単体テスト（`test-frontend`）。`make check-all` では、Playwright によるブラウザーでのエンドツーエンドテスト（`e2e`）と、純粋なロジックのミューテーションテスト（`test-mutation-frontend`） |
| シェルと Dockerfile | shfmt、shellcheck、hadolint |
| GitHub Actions | actionlint、zizmor、ghalint、pinact |
| Terraform と Terragrunt | 整形、tflint、Trivy、モジュールの README（terraform-docs）、stg と prd のユニットが同じであること（`lint-infra-live`）。`make check-all` では、各モジュールの検証とテスト（`validate-terraform`）、すべてのユニットの検証（`validate-live`）、AWS Emulator での stg のリハーサル（`rehearse-infra`） |
| SQL | squawk。`make check-all` では、スキーマの変更ごとの DDL に対する squawk（`lint-sql-plan`）、スキーマに対するすべてのクエリの prepare（`lint-sqlc`）、各スキーマのコメントと文書（`lint-db-docs`）、AWS でデータベースを準備するイメージ（`test-db-image`） |
| OpenAPI と AsyncAPI | Redocly。oasdiff による、ベースブランチと比べた互換性を壊す変更（`lint-contracts-breaking`）。`make check-all` では、Schemathesis が各 OpenAPI の文書から生成し、チェックアウトからビルドした API に送るリクエスト（`test-api-fuzz`） |
| イベントの JSON Schema | その形、例、AsyncAPI 文書のメッセージ（`lint-events`）。ベースブランチと比べた互換性を壊す変更（`lint-events-breaking`） |
| YAML・JSON・TOML | Prettier、yamllint、taplo、設定ファイルの JSON Schema による検証 |
| Markdown | markdownlint、リンク、英語の文体（Vale）、日本語の文体（textlint） |
| 文書 | 決定の記録の形式とその一覧、英語版と一致する対訳 |
| エージェントのハーネス | 見本のイベントを与えるフックのテスト（`test-hooks`）、`AGENTS.md` の長さ。`make check-all` では、エージェントなしで確かめる評価の採点（`test-evals`） |
| 依存 | Trivy による脆弱性とライセンスの検査。`make check-all` |

`make check` には60秒の上限があり、ネットワークを使わない。指摘は1件が1行で、ファイル・行・ルールが出る。

## 文書

- [ARCHITECTURE.ja.md](ARCHITECTURE.ja.md)：製品の地図。文脈とそのプロセス、層、フロントエンド、画面からのリクエストの流れとイベントの流れ、データベース、AWS の環境と、それらを保つチェック。
- [docs/contexts/](docs/contexts/)：区切られた文脈ごとの README と用語集。
- [AGENTS.ja.md](AGENTS.ja.md)：コーディングエージェント向けの指示。バックエンドとフロントエンドについての指示は、`backend/AGENTS.ja.md` と `frontend/AGENTS.ja.md` にある。
- [docs/adr/](docs/adr/README.ja.md)：決定の記録。
- [docs/guides/](docs/guides/)：繰り返す作業のやり方。
- `evals/README.ja.md`：エージェントのハーネスの評価と、これまでの実行が示したこと。
- [SECURITY.ja.md](SECURITY.ja.md)：脆弱性の報告のしかた。

## 第三者のファイル

- `.vale/styles/Google/`：Vale 用の Google developer documentation style。MIT License。
- `.claude/skills/japanese-tech-writing/`：鹿野桂一郎さんによる日本語の技術文書の文章規範。Unlicense。

## ライセンス

[MIT No Attribution](LICENSE)。表示を残さずに、テンプレートをコピーして自分のものにできる。

<!-- i18n-source: README.md sha256:1fb5aed1d01132b424e70051f3fdced332d464ffd1fbfc9bfdb4d1cda9f0da59 -->

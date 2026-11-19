# AWS にデプロイする

このガイドでは、環境のための AWS アカウントを準備し、そこにデプロイする。そのあとにワークフローが何をするかも説明する。
背景にある決定は TPL-0014 から TPL-0018 に、全体の地図は [ARCHITECTURE.ja.md](../../ARCHITECTURE.ja.md) にある。

このテンプレートは、AWS アカウントなしで作った。
ここに書いたものはすべて、[AWS アカウントなしで確かめる](#aws-アカウントなしで確かめる)のチェックを通っている。ただし、AWS にはまだ何も適用していない。
それらのチェックでは確かめられないことは、[最初の適用で確かめること](#最初の適用で確かめること)に挙げた。

## 環境とは

stg と prd は、同じユニットを持つ 2 つの AWS アカウントである。ユニットは `infra/live/<environment>/` にある。
ユニットは、`terragrunt.hcl` のあるディレクトリである。Terraform の state を 1 つ持ち、Terragrunt が、`infra/live/_units/` の定義が名指しするモジュールで適用する。

| ユニット | モジュール | 作るもの |
|---|---|---|
| `ci` | `github-oidc` | GitHub Actions が引き受けるロール |
| `network` | `network` | VPC とそのサブネット、AWS のサービスのエンドポイント |
| `registry` | `registry` | イメージのリポジトリ |
| `database` | `database` | Aurora のクラスター、各文脈のロールのシークレット、データベースを準備するタスク |
| `platform` | `platform` | ECS のクラスター、Service Connect の名前空間、イベントバス、サービスのセキュリティグループ |
| `contexts/<context>` | `context-service` | 1 つの文脈のサービス、ロール、ログ、イベントの購読 |
| `web` | `web` | Next.js のサーバーと、公開のロードバランサー |

- 環境ごとの違いは `infra/live/<environment>/env.hcl` にあり、ほかにはない。`terragrunt.hcl` はどれも同じファイルである。2 つのツリーが食い違うと、`make check` が失敗する（`lint-infra-live`）。
- すべての環境に共通のものは `infra/live/system.hcl` にある。システムの名前、GitHub のリポジトリ、文脈の一覧である。
- provider のバージョンは `infra/live/providers.hcl` に、そのチェックサムは `providers.lock.hcl` にある。バージョンを変えたら、`make infra-lock` を実行する。

## アカウントを準備する

stg について 1 度、prd について 1 度行う。prd では、`stg` の代わりに環境の名前を書く。

1. **環境の値を埋める。** `infra/live/stg/env.hcl` の仮の値を置き換える。アカウント ID、リージョンとそのアベイラビリティーゾーン、サイトのドメイン名と Amazon Route 53 の公開ホストゾーンの ID、メールの送信元のアドレスとその ID である。`infra/live/system.hcl` には、システムの名前とリポジトリを書く。システムの名前は、state のバケットの名前の先頭になる。
2. **メールの ID を検証する。** Amazon SES で、環境のリージョンについて、`mail_identity` のドメインを検証し、本番アクセスを申請する。サンドボックスの中では、SES は検証済みのアドレスにしか送らない。
3. **state のバケットとロールを作る。** アカウントの管理者の権限を持つ認証情報で、次を実行する。

   ```sh
   make infra-bootstrap ENV=stg
   ```

   Terraform の state を置く S3 バケットを作り、続けてユニット `ci` を適用して、2 つのロール `github-plan` と `github-deploy` の ARN を表示する。Terraform は、適用の前に確認を求める。
4. **GitHub に伝える。** リポジトリの設定で、次を行う。
   - GitHub の環境 `stg` を作り、`main` からのデプロイだけを許可する。`prd` では、さらにレビュアーを必須にする。prd のジョブは、その人の承認を待つようになる。
   - リポジトリの変数 `AWS_REGION`、`AWS_PLAN_ROLE_ARN_STG`、`AWS_DEPLOY_ROLE_ARN_STG` を設定する（prd では、`_PRD` で終わる 2 つの変数）。これらはシークレットではない。ロールの変数を設定するまで、そのロールを使うジョブはスキップされる。

## デプロイする

準備が済めば、`main` へのマージでデプロイされる。ワークフロー `Deploy` が `make deploy ENV=stg` を実行し、承認のあとに `make deploy ENV=prd` を実行する。
stg の最初のデプロイは、手元のマシンから実行してもよい。Docker と、そのアカウントの認証情報が要る。`make deploy ENV=prd` は、ワークフローの外では実行を拒む。

デプロイは、次の 4 つをこの順に行う（`tools/scripts/infra.sh`）。

1. ユニット `registry` を適用する。続けて、レジストリにまだないタグのイメージをビルドしてプッシュする（`tools/scripts/images.sh`）。
2. ユニット `network`、`database`、`platform` を適用する。
3. データベースを準備するタスクを実行し、その終了を待つ。タスクは、`db/bootstrap.sql` を実行し、ロールのパスワードを設定し、各文脈の `schema.sql` を pgschema で適用する。手元で `make db-up` が行うことと同じである。
4. すべてのユニットを適用する。

イメージのタグは、そのイメージの元になるファイルのハッシュである。
そのため、1 つの文脈を変えると、その文脈のタスク定義だけが変わる。手順 4 が再起動するのはその文脈のサービスだけで、ほかの文脈は動き続ける。
新しいタスクが正常にならないサービスは、ECS が元に戻す（デプロイのサーキットブレーカー）。

手順 3 は、手順 4 が新しいコードを起動する前に、古いコードが動いているままスキーマを変える。
そのため、スキーマの変更は、新旧どちらのコードでも動かなければならない。先に足し、削るのは後のデプロイにする（[スキーマを変える](change-a-schema.ja.md)）。

## プルリクエストの plan を読む

ワークフロー `Plan` は、インフラ、文脈、フロントエンド、データベースのどれかを変えるプルリクエストで、stg と prd について `make infra-plan` を実行する。
各ジョブのログは、マージがその環境の何を変えるかを、ユニットごとに示す。
ジョブは `github-plan` としてサインインする。このロールは、読めるが、変えられない。
フォークからのプルリクエストでは plan が走らない。GitHub がトークンを渡さないからである。

## 環境を変える

| 変えるもの | 編集するファイル | そのあと |
|---|---|---|
| 1 つの環境の値（データベースのキャパシティなど） | `infra/live/<environment>/env.hcl` | どの `env.hcl` も同じキーを設定する |
| ユニットが作るもの（すべての環境で） | `infra/live/_units/<unit>.hcl`、またはそのモジュール（`infra/modules/`） | モジュールを変えたら `make gen` |
| 新しい文脈のユニット | `infra/live/_units/contexts/<context>.hcl`、各環境の同じ `terragrunt.hcl`、`system.hcl` の `contexts` | [文脈を足す](add-a-context.ja.md) |
| provider のバージョン | `infra/live/providers.hcl` | `make infra-lock` |
| タスクが AWS の外で届く先 | ユニット `network` の `nat_gateways` と、その文脈のセキュリティグループのルール | `infra/modules/network` の README を見る |

## AWS アカウントなしで確かめる

3 つのチェックが、アカウントなしで環境を動かす。CI は、インフラに触れるプルリクエストのたびに、これらを実行する。

- `make validate-terraform` は、各モジュールを検証し、そのテスト（`tests/*.tftest.hcl`）を実行する。テストは、AWS provider のモックに対して plan する。
- `make validate-live` は、Terragrunt が組み立てたとおりの各ユニットで `terraform validate` を実行する。ほかのユニットの出力は、モックで置き換える（`infra/live/mocks.hcl`）。1 つの環境で、すべての環境を代表させる。どの環境も同じユニットを持つからである。Docker は要らない。
- `make rehearse-infra` は、stg のユニットを、デプロイと同じ順序で AWS Emulator へ適用する。続けて、prd のユニットをそれに対して plan する。適用できないモジュール、ほかのユニットにない出力を読むユニット、モジュールが拒む `env.hcl` の値は、ここで失敗する。

リハーサルはデプロイではない。エミュレーターは、Terraform が作ったものを覚えるだけで、タスクを起動しない。そのため、リハーサルはイメージをプッシュせず、データベースも準備しない。また、3 つの入力が AWS と違う。エミュレーターは Aurora PostgreSQL をバージョン 17 までしか知らず、Amazon SES の API のエンドポイントを持たず、証明書を発行しない。証明書は、リハーサルが自分でリクエストして、ユニット `web` に渡す。
データベースを準備するタスクは、`make test-db-image` が確かめる。イメージをビルドし、手元の PostgreSQL に対して実行する。

## 最初の適用で確かめること

- **タスクの起動。** エンドポイントを通したイメージの取得、データベースのシークレットの読み出し、Service Connect による `http://facility:8080` の名前解決。
- **Amazon SES の API のエンドポイント。** サービス名 `email` は SES の文書に従ったが、文書は名前をそのままの形では書いていない（[モジュールの README](../../infra/modules/network/README.ja.md)）。
- **トークンのサブジェクト。** プルリクエストと GitHub の環境のサブジェクトを、ロールが期待する形で AWS が受け入れるか（[モジュールの README](../../infra/modules/github-oidc/README.ja.md)）。
- **読み取り専用のロールでの plan。** `ReadOnlyAccess` だけで、すべてのリソース、とくにシークレットのバージョンを読み直せるか。
- **Aurora。** エンジンのバージョン、0 ACU での一時停止、クラスターの再開のあいだにデータベースを準備するタスクが待つ時間。
- **証明書。** ホストゾーンを通した検証。これを実行するチェックはない。

<!-- i18n-source: deploy.md sha256:eeb15e4974a485b6df7e089d90ff5f336dbd51b41a30e67c7ae4ac8360d9c198 -->

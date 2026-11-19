# context-service

1 つの区切られた文脈を ECS で動かすためのものをすべて作るモジュールである（TPL-0014）。API があればその API のサービス、ワーカーごとのサービス、タスクのロール、ログ、セキュリティグループ、イベントの購読（TPL-0013）を作る。
動かすプロセスは `compose.yaml` のものと同じで、同じイメージを同じコマンドで動かす。

| プロセス | ECS のサービス | コマンド | 呼ぶときのアドレス |
|---|---|---|---|
| API（`api`） | `<name>` | イメージをそのまま動かす | `http://<name>:8080`。ECS Service Connect を通じて呼ぶ |
| ワーカー（`workers`） | `<name>-<key>` | `command`（`["relay"]` など）。空のリストなら、イメージをそのまま動かす | なし |

テンプレートの 3 つの文脈では、次のようになる。

| 文脈 | `api` | `workers` | `publishes_events` | `subscription` |
|---|---|---|---|---|
| Facility | `{}` | | | |
| Reservation | `{}` | `{ relay = { command = ["relay"] } }` | `true` | |
| Notification | | `{ worker = { command = [] } }` | | `/contexts/reservation` の、予約の確定と取り消し |

## 外から届かない作り

このモジュールが作るものには、VPC の外から届かない。どの変数を変えても、それは変わらない。タスクはパブリック IP アドレスを持たず、どのサービスもロードバランサーの後ろに置かず、セキュリティグループはアドレス範囲を指定しない。
`tests/` のテストがこれを確かめる。公開の入口を作るのが Web フロントエンドのモジュールだけであることは、レビューで確かめる。

- すべてのタスクは非公開サブネットで動き、2 つのセキュリティグループに加わる。1 つはモジュール [platform](../platform/README.md) が作るサービスのグループ、もう 1 つは文脈のグループ `<environment>-<name>` である。文脈のグループからは、データベースのセキュリティグループの PostgreSQL（5432）に届く。モジュールは、その接続を受け入れるルールもデータベースのグループに足す。
- すべてのサービスで ECS Service Connect を有効にする。そのため、どのプロセスも、別の文脈の API を `http://facility:8080` のように名前で呼べる。Docker Compose の中でも同じアドレスである。API のサービスは、文脈の名前とポート `http`（8080）で、呼ばれる側にもなる。
- これより多くの通信（AWS の外のサービスの呼び出しなど）が要る文脈は、自身のグループ（`security_group_id`）にルールを足す。

## 設定とシークレット

プロセスは設定を環境変数から読み（TPL-0014）、文脈のすべてのプロセスが同じ環境変数を受け取る。

- `environment_variables`。たとえば `FACILITY_API_URL = "http://facility:8080"`。
- `AWS_REGION`。provider のリージョンである。
- `DATABASE_URL`。シークレット `database.secret_arn` のキー `url` から取る。ECS はタスクの起動時に Secrets Manager からこれを読む。そのため、コードはこれを読むために AWS の API を呼ばず、タスク定義が持つのも値ではなくシークレットの ARN である。モジュールは、`environment_variables` に `DATABASE_URL` があると拒否する。
- `EVENT_BUS_NAME`。`publishes_events` が true のときに渡す。
- `EVENTS_QUEUE_URL`。`subscription` があるときに渡す。モジュールが [event-subscription](../event-subscription/README.md) で作るキューの URL である。

## ロール

- タスク実行ロール。ECS がタスクを起動するときに使う。プルできるのはリポジトリ `repository_arn` だけ、書き込めるのは文脈のロググループだけ、読めるのはデータベースのシークレットだけである。シークレットをカスタマーマネージドキーで暗号化している場合は、`database.kms_key_arn` への `kms:Decrypt` も持つ。プロセスがこのロールの権限を得ることはない。
- タスクロール。プロセスが AWS を呼ぶときに使う。権限は、文脈が呼ぶものだけである。`publishes_events` が true なら、バスへの `events:DescribeEventBus` と `events:PutEvents` を持つ。購読があれば、文脈のキューへの `sqs:ReceiveMessage` と `sqs:DeleteMessage` を持つ。ほかに `iam_statements` の各ステートメント（Amazon SES でメールを送る文脈の `ses:SendEmail` など）を持つ。このロールを引き受けられるのは、同じアカウントと同じリージョンの ECS だけである。

2 つのロールは文脈ごとにあるので、1 つの文脈の API とワーカーはこれを共有する。

## タスク

- Fargate で動き、ネットワークモードは `awsvpc`、CPU アーキテクチャは `cpu_architecture`（既定は `X86_64`）である。
- ヘルスチェックには、API とワーカーのどちらでもイメージのもの（`/service healthcheck`）を使い、間隔などは `backend/Dockerfile` に合わせる。
- `compose.yaml` と同じく、ルートファイルシステムは読み取り専用で、Linux ケーパビリティは 1 つも持たない。イメージは root 以外のユーザーでプロセスを動かす。
- すべてのプロセスとそのプロキシのログは、ロググループ `/<environment>/<name>` に送る。ストリームの名前は、`api/` や `relay/` のようにプロセスの名前で始まる。
- タスクが正常にならないデプロイは止まり、ロールバックする（デプロイのサーキットブレーカー）。ECS Exec は使えない。
- Service Connect のプロキシは、各タスクの中でプロセスと並んで動き、CPU とメモリをプロセスと分け合う。AWS は、プロキシのために CPU を 256 ユニット、メモリを 64 MiB 以上足すよう勧めている。既定の 256 CPU ユニットと 512 MiB では、プロキシは小さなタスクの一部を分けてもらうだけになる。

AWS にはまだ適用していない。AWS Emulator は、モジュール network、registry、platform と組み合わせた 3 つの文脈分のこのモジュールを受け付けたが、タスクを 1 つも起動しない。タスクがイメージをプルできること、シークレットを読めること、別の文脈に名前で届くことは、初めて AWS で動かすときに確かめる。

`tests/` のテストは、AWS provider のモックに対してモジュールを plan する。そのため、`make check-all` は AWS のアカウントなしでテストを実行する（`validate-terraform`）。

入力、出力、リソースの一覧は、terraform-docs がコードから[英語版](README.md)に書き出す。コードの中の説明は英語なので、一覧は英語版だけにある。

<!-- i18n-source: README.md sha256:696bd9c9ea4e1367a0082da43d6b3ba52f222874a0bcbe140b0847da687a33fc -->

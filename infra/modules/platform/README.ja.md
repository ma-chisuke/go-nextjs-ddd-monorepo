# platform

1 つの環境のすべてのサービスが共有するものを作るモジュールである（TPL-0014、TPL-0015）。ECS クラスター、サービスが互いを見つけるための名前空間、統合イベントのイベントバス、すべてのタスクが加わるセキュリティグループを作る。

| リソース | 名前 | 用途 |
|---|---|---|
| ECS クラスター | `<environment>` | 環境のすべてのサービス。Fargate で動く |
| AWS Cloud Map の HTTP 名前空間 | `<environment>` | サービスが ECS Service Connect で互いを呼ぶときの名前 |
| EventBridge のイベントバス | `event_bus_name`（既定は `contexts`） | すべての文脈の統合イベント（TPL-0013）。モジュール [event-bus](../event-bus/README.md) で作る |
| セキュリティグループ | `<environment>-services` | すべてのサービスのすべてのタスク |

## 文脈が別の文脈を呼ぶ方法

文脈の API は非公開で、その前にロードバランサーは置かない。
文脈は、別の文脈を ECS Service Connect を通じてその文脈の名前で呼ぶ。たとえば Reservation は `http://facility:8080` を呼ぶ。Docker Compose の中でも同じアドレスである。
Service Connect を有効にしたサービスでは、ECS が各タスクにプロキシを足す。プロキシは環境の名前空間で名前を解決し、その文脈のタスクの 1 つに接続する。接続先はラウンドロビンで選び、再試行と受動的なヘルスチェックも行う。

セキュリティグループ `<environment>-services` が許可するのは、すべてのタスクに要る通信だけである。

- グループのタスクとのあいだの、8080 への TCP。8080 は、すべてのプロセスが待ち受けるポートである。クライアントのプロキシは、サーバーのプロキシにコンテナポートで接続し、コンテナポートはサーバーが待ち受けるポートと同じである。
- インターフェイスエンドポイントのセキュリティグループと、Amazon S3 のプレフィックスリストへの HTTPS。どちらもモジュール [network](../network/README.md) の出力である。タスクは、これらを通じて AWS に届く。

アドレス範囲を指定するルールはないので、タスクは VPC の外のどこにも届かない。
文脈だけが必要とする通信（データベースへの接続など）は、その文脈自身のセキュリティグループで許可する（モジュール [context-service](../context-service/README.md)）。

- クラスターでは Container Insights を有効にする。Container Insights は、書き込むメトリクスとログの分だけ課金される。stg のような環境では、`container_insights = "disabled"` で無効にできる。
- 名前空間は DNS レコードを作らない。エンドポイントは ECS が各プロキシに渡すので、その名前を解決できるのは名前空間のサービスだけである。

AWS にはまだ適用していない。AWS Emulator は、モジュール network、context-service と組み合わせたこのモジュールを受け付けたが、タスクを 1 つも起動しない。そのため、プロキシが名前を解決できることまでは確かめられていない。

`tests/` のテストは、AWS provider のモックに対してモジュールを plan する。そのため、`make check-all` は AWS のアカウントなしでテストを実行する（`validate-terraform`）。

入力、出力、リソースの一覧は、terraform-docs がコードから[英語版](README.md)に書き出す。コードの中の説明は英語なので、一覧は英語版だけにある。

<!-- i18n-source: README.md sha256:b9923fa2ab529b55d4f5371a67c0354e22b0fb298d022b83d63206e44051f1a2 -->

# event-bus

EventBridge のイベントバスを作るモジュールである。すべての区切られた文脈の統合イベントは、このバスを通る（TPL-0013）。
発行側の文脈は、送信箱の中継を通して、イベントをこのバスに載せる。
受け取る側の文脈は、モジュール [event-subscription](../event-subscription/README.ja.md) で自分のルールを足す。そのため、発行側は受け取る側を知らない。

手元の環境は、このモジュールを AWS Emulator に適用する（`infra/local/`）。AWS 上の環境も同じモジュールを使う。モジュール [platform](../platform/README.ja.md) が、このモジュールを呼ぶ（TPL-0015）。

入力、出力、リソースの一覧は、terraform-docs がコードから[英語版](README.md)に書き出す。コードの中の説明は英語なので、一覧は英語版だけにある。

<!-- i18n-source: README.md sha256:bf88c0ab0acda78f2a17bf6454968bf0056ab408b2f0c8078326c8434ef3f44e -->

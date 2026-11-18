# event-bus

すべての境界づけられた文脈の統合イベントを運ぶ、EventBridge のイベントバスである（TPL-0013）。
発行側の文脈は、送信箱の中継を通して、イベントをこのバスに載せる。
受け取る側の文脈は、モジュール [event-subscription](../event-subscription/README.ja.md) で自分のルールを足す。そのため、発行側は受け取る側を知らない。

ローカル環境は、このモジュールを AWS Emulator に適用する（`infra/local/`）。AWS 上の環境も同じモジュールを使う。

入力、出力、リソースの一覧は、terraform-docs がコードから[英語版](README.md)に書き出す。コードの中の説明は英語なので、一覧は英語版だけにある。

<!-- i18n-source: README.md sha256:66e8960a9713f09db0ee692dfe142ad3cc685c79e050d14856a1fa85e4576e8f -->

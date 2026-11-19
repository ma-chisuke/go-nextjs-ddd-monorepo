# event-subscription

受け取る側の文脈1つが、バス上のイベントを購読するためのモジュールである（TPL-0013）。デッドレターキューの付いた SQS のキューと、イベントを `source` と `type` で選んでそのキューに送る EventBridge のルールを作る。
ここにあるものはすべて受け取る側のものなので、受け取る側を足しても発行側は何も変わらない。

キューが受け取るのは、発行側が書いたままの CloudEvents の封筒だけである（入力パス `$.detail`）。
受け取る側が `max_receive_count` 回処理できなかったメッセージは、デッドレターキューに移る。そのため、残りのメッセージを止めることも、失われることもない。

手元の環境は、このモジュールを AWS Emulator に適用する（`infra/local/`）。AWS 上の環境も同じモジュールを使う。イベントを受け取る文脈について、モジュール [context-service](../context-service/README.ja.md) が、このモジュールを呼ぶ（TPL-0015）。

入力、出力、リソースの一覧は、terraform-docs がコードから[英語版](README.md)に書き出す。コードの中の説明は英語なので、一覧は英語版だけにある。

<!-- i18n-source: README.md sha256:2be2f6be7dd89c976339292a0cf036ba840304836721c536e54b19d68afcdf26 -->

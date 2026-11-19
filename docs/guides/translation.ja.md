# 文書を訳す

このリポジトリは英語を正とし、どの文書にも日本語の対訳を隣に置く（TPL-0021）。
このガイドは、英語版と日本語版をそろえておく方法を説明する。

## 決まり

- `foo.md` は英語である。`foo.ja.md` はその日本語の対訳で、同じディレクトリに置く。
- 2つのファイルは同じコミットで変える。先に英語版を変える。
- 対訳の最後の行に、英語版のどのバージョンを訳したかを書く。

  ```markdown
  <!-- i18n-source: foo.md sha256:<foo.md のハッシュ> -->
  ```

  `make i18n-stamp` が、すべての対訳にこの行を書く。実行するのは日本語の本文を直したあとであり、直す前には実行しない。
- 対訳のない文書があるとき、または対訳に書かれたハッシュが英語版と違うとき、`make check` は失敗する。
- コード・コメント・コミットメッセージと、契約（OpenAPI と AsyncAPI の文書、イベントの JSON Schema）の説明は英語だけで書く。
- `.claude/` と `.github/` の下のファイルは道具への指示なので、英語だけで書く。例外は `.claude/skills/japanese-tech-writing/` の日本語の文章規範だけである。これは第三者のファイルで、元のまま置いている。
- `infra/modules/` にある Terraform のモジュールの README は、地の文だけを訳す。入力・出力・リソースの一覧は、terraform-docs がコードの中の説明から書く。その説明は英語なので、一覧は英語版にだけ置く。

## 日本語の書き方

- 常体（である）で書く。敬体（です・ます）は使わない。
- 意味を1文ずつ、自然な日本語に訳す。英語の語順や慣用句を残さない。
- `.claude/skills/japanese-tech-writing/SKILL.md` の文章規範に従う。
- 識別子・コマンド・ファイルのパス・製品名は元のまま書く。
- 日本語と英字の単語の間に半角の空白を入れるかどうかは、そのファイルの前後の書き方に合わせる。1つのファイルに2つの書き方を混ぜない。

## 用語

日本語の用語は次の表に合わせる。新しい用語は、使う前にこの表へ足す。

| 英語 | 日本語 |
|---|---|
| template（このリポジトリ） | テンプレート |
| project、team（テンプレートをコピーした側） | 利用者、チーム |
| decision record（ADR） | 記録、ADR |
| check | チェック |
| bounded context | 区切られた文脈（2回目からは「文脈」） |
| core subdomain | 中核の業務領域 |
| supporting subdomain | 補完の業務領域 |
| generic subdomain | 一般の業務領域 |
| domain model | ドメインモデル |
| transaction script | トランザクションスクリプト |
| ports and adapters | ポートとアダプター |
| aggregate | 集約 |
| value object | 値オブジェクト |
| domain event | ドメインイベント |
| integration event | 統合イベント |
| use case | ユースケース |
| outbox | 送信箱 |
| relay（送信箱の） | 中継 |
| envelope（イベントの） | 封筒 |
| open-host service | 共用サービス |
| published language | 公開された言語 |
| anticorruption layer | モデル変換装置 |
| ubiquitous language | 同じ言葉 |
| glossary | 用語集 |
| translation（文書の） | 対訳 |
| local environment | 手元の環境 |
| supply chain | サプライチェーン |
| provenance | 来歴 |
| pin（バージョンを） | 固定する |
| harness（コーディングエージェントの） | ハーネス |
| generated code | 生成コード |
| monorepo | モノレポ |
| contract（API やイベントの） | 契約 |
| source（生成したファイルの元になるファイル）、authority（2つの文面が食い違ったときに決め手になる文面） | 正 |
| user（製品の） | ユーザー |
| heuristic | 経験則 |
| tactical design pattern | 戦術的なパターン |
| active record | アクティブレコード |
| event-sourced domain model | イベントソーシングを使うドメインモデル |
| layered architecture | レイヤードアーキテクチャ |
| app layer（パッケージ `app`） | アプリケーション層 |
| mapping（層の間の） | 詰め替え |
| business error | 業務エラー |
| modular monolith | モジュラーモノリス |
| shared kernel | 共有カーネル |
| eventual consistency | 結果整合性 |
| idempotent | 冪等 |
| provider、consumer（API の） | 提供側、利用側 |
| consumer-driven contract test | コンシューマ駆動の契約テスト |
| breaking change | 互換性を壊す変更 |
| problem details（RFC 9457） | problem details（訳さない） |
| source（対訳の） | 原文 |
| deterministic check | 決定的なチェック |
| finding（チェックの） | 指摘 |
| concern（道具が受け持つ） | 関心事 |
| time budget（`make check` の） | 時間の上限 |
| evaluation（ハーネスの） | 評価 |
| lockfile | ロックファイル |
| misconfiguration | 設定ミス |
| secret（ファイルに書かれた認証情報） | 秘密情報 |
| secret（GitHub に保存したもの） | シークレット |
| required status check | 必須チェック |
| trust policy（IAM ロールの） | 信頼ポリシー |
| desired state（スキーマの） | あるべき状態 |
| publisher（イベントの） | 発行側 |
| consumer（イベントの） | 受け取る側 |
| subscribe（イベントを） | 購読する |
| at-least-once delivery | 少なくとも1回の配信 |
| dead-letter queue | デッドレターキュー |
| ingress、entry（リクエストの） | 入口 |
| unit（Terragrunt の） | ユニット |
| ruleset（GitHub のブランチの） | ルールセット |
| pull request | プルリクエスト |
| workflow（GitHub Actions） | ワークフロー |
| environment（GitHub やデプロイ先の） | 環境 |
| push protection | プッシュ保護 |
| squash merge | スカッシュマージ |
| secret scanning（GitHub の） | シークレットスキャン |
| tool | 道具 |
| version | バージョン（「英語版」のように文書の言語を指すときは「版」） |
| linter | リンター |
| notice（動詞） | 気づく |
| fake（テストの） | 偽物 |
| self-test（`make selftest`） | 自己テスト |
| room | 会議室 |
| capacity（会議室の） | 定員 |
| opening hours | 利用時間 |
| time zone | タイムゾーン |
| time of day | 時刻 |
| exported（Go の） | エクスポートされた |
| reservation | 予約 |
| book（会議室を） | 予約する |
| reschedule | 変更する |
| cancel（予約を） | 取り消す |
| period（予約の） | 時間帯 |
| attendees | 参加人数 |
| organizer | 主催者 |
| confirmed（予約が） | 確定 |
| idempotency key | 冪等キー |
| core context、supporting context | 中核の文脈、補完の文脈 |
| problem（RFC 9457 の形式の1つのエラー応答） | problem（訳さない） |
| operation（API の文書の） | 操作 |
| health check | ヘルスチェック |
| access log | アクセスログ |
| wire（`module.go` で文脈の部品を） | 組み立てる |
| generator（コードの） | 生成器 |
| owner role、runtime role（文脈の） | 所有者のロール、実行時のロール |
| privilege（データベースの） | 権限 |
| search path（データベースのロールの） | 検索パス |
| exclusion constraint | 排他制約 |
| scratch database | 作業用のデータベース |
| merge base | マージベース |
| integration test | 結合テスト |
| optimistic locking | 楽観的ロック |
| container image | コンテナイメージ |
| smoke test | スモークテスト |
| worktree（Git の） | ワークツリー |
| allowance（`schema-changes.allow` に書く、危険な文の） | 許可 |
| worker（リクエストに答えないプロセス） | ワーカー |
| mode（コマンドの） | モード |
| handle（イベントを） | 処理する |
| event bus | イベントバス |
| queue | キュー |
| emulator | エミュレーター |
| entry（EventBridge の `PutEvents` の） | エントリー |
| long polling（SQS の） | ロングポーリング |
| visibility timeout（SQS の） | 可視性タイムアウト |
| certificate（TLS の） | 証明書 |
| unique constraint | 一意制約 |
| unit test | 単体テスト |
| mail | メール |
| mail server | メールサーバー |
| mailer | メーラー |
| recipient（メールの） | 宛先 |
| subject（メールの） | 件名 |
| body（メールの） | 本文 |
| sender（メールの） | 差出人 |
| frontend | フロントエンド |
| screen | 画面 |
| page（アプリの、1つの URL にあるもの） | ページ |
| feature（ディレクトリ `frontend/src/features/<context>/`） | 機能 |
| form | フォーム |
| field（フォームの） | 項目 |
| hidden field（フォームの） | 隠し項目 |
| browser | ブラウザー |
| build（アプリやイメージの） | ビルド |
| server component、client component | サーバーコンポーネント、クライアントコンポーネント |
| Server Function（Next.js の） | サーバー関数 |
| stream（ページの一部を） | ストリーミングする |
| shell（ページの静的な部分） | 骨組み |
| fingerprint（予約の） | フィンガープリント |
| end-to-end test | エンドツーエンドテスト |
| scenario（エンドツーエンドテストの） | シナリオ |
| headless mode（ブラウザーの） | ヘッドレスモード |
| region（AWS の） | リージョン |
| identity（Amazon SES の） | ID |
| verify（Amazon SES の ID を） | 検証する |
| sandbox（Amazon SES の） | サンドボックス |
| throttling | スロットリング |
| configuration set（Amazon SES の） | 設定セット |
| bounce（メールの） | バウンス |
| cluster（ECS、Aurora の） | クラスター |
| container | コンテナ |
| task、task definition（ECS の） | タスク、タスク定義 |
| task execution role、task role（ECS の） | タスク実行ロール、タスクロール |
| deployment circuit breaker（ECS の） | デプロイのサーキットブレーカー |
| namespace（Service Connect の） | 名前空間 |
| endpoint（VPC の）。gateway endpoint、interface endpoint | エンドポイント。ゲートウェイエンドポイント、インターフェイスエンドポイント |
| public subnet、private subnet | 公開サブネット、非公開サブネット |
| load balancer | ロードバランサー |
| Availability Zone | アベイラビリティーゾーン |
| hosted zone（Amazon Route 53 の） | ホストゾーン |
| secret（Secrets Manager の） | シークレット |
| customer managed key（AWS KMS の） | カスタマーマネージドキー |
| lifecycle policy（Amazon ECR の） | ライフサイクルポリシー |
| push、pull（イメージやブランチを） | プッシュ、プル |
| promote（イメージを stg から prd へ） | 昇格させる |
| capacity（データベースの） | キャパシティ |
| connection pool | 接続プール |
| master user（Aurora の） | マスターユーザー |
| pause、resume（Aurora Serverless v2 の） | 一時停止する、再開する |
| writer、reader（Aurora のインスタンス） | ライター、リーダー |
| snapshot | スナップショット |
| plan（Terraform、pgschema の） | plan（訳さない） |
| apply（Terraform、pgschema の） | 適用する |
| state（Terraform の） | state（訳さない） |
| ephemeral resource、ephemeral value（Terraform の） | エフェメラルリソース、エフェメラルな値 |
| write-only argument（Terraform の） | 書き込み専用の引数 |
| mock（provider の、またはユニットの出力の） | モック |
| rehearsal（`make rehearse-infra`） | リハーサル |

<!-- i18n-source: translation.md sha256:e986d7db1138fe1deffc66fc3b29c543510d3aa54591a44dc5616083c5c94f73 -->

# 文書を訳す

このリポジトリは英語を正とし、どの文書にも日本語の対訳を隣に置く（TPL-0021）。
このガイドは、英語版と日本語版をそろえておく方法を説明する。

## 決まり

- `foo.md` は英語である。`foo.ja.md` はその日本語の対訳で、同じディレクトリに置く。
- 2つのファイルは同じコミットで変える。先に英語版を変える。
- 対訳の最後の行に、英語版のどの版を訳したかを書く。

  ```markdown
  <!-- i18n-source: foo.md sha256:<foo.md のハッシュ> -->
  ```

  `make i18n-stamp` が、すべての対訳にこの行を書く。実行するのは日本語の本文を直したあとであり、直す前には実行しない。
- 対訳のない文書があるとき、または対訳に書かれたハッシュが英語版と違うとき、`make check` は失敗する。
- コード・コメント・コミットメッセージ・OpenAPI の説明は英語だけで書く。
- `.claude/` と `.github/` の下のファイルは道具への指示なので、英語だけで書く。

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
| pin（版を） | 固定する |
| harness（コーディングエージェントの） | ハーネス |
| generated code | 生成コード |
| monorepo | モノレポ |
| contract（API やイベントの） | 契約 |
| source（ほかのファイルを生成したり訳したりする元） | 正 |
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
| concern（ツールが受け持つ） | 関心事 |
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

<!-- i18n-source: translation.md sha256:1e5c0579c4c6190b7eb363278cdd35f97c2d58bbf0b020ae7a701ec533f117af -->

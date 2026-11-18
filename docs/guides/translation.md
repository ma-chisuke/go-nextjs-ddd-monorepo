# Translate a document

English is the source language of this repository, and every document has a Japanese translation next to it (TPL-0021).
This guide tells you how to keep the two in step.

## Rules

- `foo.md` is English. `foo.ja.md` is its Japanese translation and sits in the same directory.
- Change both files in the same commit. Change the English file first.
- The last line of a translation records which version of the English file it translates:

  ```markdown
  <!-- i18n-source: foo.md sha256:<hash of foo.md> -->
  ```

  `make i18n-stamp` writes this line for every translation. Run it after you have brought the Japanese text up to date, never before.
- `make check` fails when a document has no translation, or when the hash in a translation differs from the English file.
- Code, comments, commit messages, and the descriptions in the contracts (OpenAPI and AsyncAPI documents, and the JSON Schemas of events) are in English only.
- Files under `.claude/` and `.github/` are instructions for tools, and are in English only. The one exception is the Japanese writing norm in `.claude/skills/japanese-tech-writing/`, a third-party file kept as it is.
- The README of a Terraform module in `infra/modules/` has a translation of its prose only. terraform-docs writes the lists of inputs, outputs, and resources from the descriptions in the code, which are in English, so those lists are in the English file only.

## How to write the Japanese text

- Write in the plain style (である), not in the polite style (です・ます).
- Translate the meaning, sentence by sentence, into natural Japanese. Do not keep English word order or English idioms.
- Follow the writing rules in `.claude/skills/japanese-tech-writing/SKILL.md`.
- Keep identifiers, commands, file paths, and product names as they are.
- Put a half-width space between Japanese text and a word in Latin letters only where the surrounding text of the file already does so; do not mix the two styles in one file.

## Terms

Use these Japanese terms. The table grows when a new term appears; add the term here before you use it.

| English | Japanese |
|---|---|
| template (this repository) | テンプレート |
| project, team (who copied the template) | 利用者、チーム |
| decision record (ADR) | 記録、ADR |
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
| relay (of the outbox) | 中継 |
| envelope (of an event) | 封筒 |
| open-host service | 共用サービス |
| published language | 公開された言語 |
| anticorruption layer | モデル変換装置 |
| ubiquitous language | 同じ言葉 |
| glossary | 用語集 |
| translation (of a document) | 対訳 |
| local environment | 手元の環境 |
| supply chain | サプライチェーン |
| provenance | 来歴 |
| pin (a version) | 固定する |
| harness (for coding agents) | ハーネス |
| generated code | 生成コード |
| monorepo | モノレポ |
| contract (of an API or an event) | 契約 |
| source (the file that generated files come from), authority (the text that decides when two texts disagree) | 正 |
| user (of the product) | ユーザー |
| heuristic | 経験則 |
| tactical design pattern | 戦術的なパターン |
| active record | アクティブレコード |
| event-sourced domain model | イベントソーシングを使うドメインモデル |
| layered architecture | レイヤードアーキテクチャ |
| app layer (package `app`) | アプリケーション層 |
| mapping (between layers) | 詰め替え |
| business error | 業務エラー |
| modular monolith | モジュラーモノリス |
| shared kernel | 共有カーネル |
| eventual consistency | 結果整合性 |
| idempotent | 冪等 |
| provider, consumer (of an API) | 提供側、利用側 |
| consumer-driven contract test | コンシューマ駆動の契約テスト |
| breaking change | 互換性を壊す変更 |
| problem details (RFC 9457) | problem details (not translated) |
| source (of a translation) | 原文 |
| deterministic check | 決定的なチェック |
| finding (of a check) | 指摘 |
| concern (that a tool covers) | 関心事 |
| time budget (of `make check`) | 時間の上限 |
| evaluation (of the harness) | 評価 |
| lockfile | ロックファイル |
| misconfiguration | 設定ミス |
| secret (a credential in a file) | 秘密情報 |
| secret (stored on GitHub) | シークレット |
| required status check | 必須チェック |
| trust policy (of an IAM role) | 信頼ポリシー |
| desired state (of a schema) | あるべき状態 |
| publisher (of an event) | 発行側 |
| consumer (of an event) | 受け取る側 |
| subscribe (to an event) | 購読する |
| at-least-once delivery | 少なくとも1回の配信 |
| dead-letter queue | デッドレターキュー |
| ingress, entry (of requests) | 入口 |
| unit (of Terragrunt) | ユニット |
| ruleset (of a branch on GitHub) | ルールセット |
| pull request | プルリクエスト |
| workflow (GitHub Actions) | ワークフロー |
| environment (GitHub or deployment) | 環境 |
| push protection | プッシュ保護 |
| squash merge | スカッシュマージ |
| secret scanning (GitHub) | シークレットスキャン |
| tool | 道具 |
| version | バージョン（「英語版」のように文書の言語を指すときは「版」） |
| linter | リンター |
| notice (verb) | 気づく |
| fake (in a test) | 偽物 |
| self-test (`make selftest`) | 自己テスト |
| room | 会議室 |
| capacity (of a room) | 定員 |
| opening hours | 利用時間 |
| time zone | タイムゾーン |
| time of day | 時刻 |
| exported (Go) | エクスポートされた |
| reservation | 予約 |
| book (a room) | 予約する |
| reschedule | 変更する |
| cancel (a reservation) | 取り消す |
| period (of a reservation) | 時間帯 |
| attendees | 参加人数 |
| organizer | 主催者 |
| confirmed (reservation) | 確定 |
| idempotency key | 冪等キー |
| core context, supporting context | 中核の文脈、補完の文脈 |
| problem (one error response in the form of RFC 9457) | problem (not translated) |
| operation (of an API document) | 操作 |
| health check | ヘルスチェック |
| access log | アクセスログ |
| wire (the parts of a context, in `module.go`) | 組み立てる |
| generator (of code) | 生成器 |
| owner role, runtime role (of a context) | 所有者のロール、実行時のロール |
| privilege (in the database) | 権限 |
| search path (of a database role) | 検索パス |
| exclusion constraint | 排他制約 |
| scratch database | 作業用のデータベース |
| merge base | マージベース |
| integration test | 結合テスト |
| optimistic locking | 楽観的ロック |
| container image | コンテナイメージ |
| smoke test | スモークテスト |
| worktree (Git) | ワークツリー |
| allowance (of a dangerous statement, in `schema-changes.allow`) | 許可 |
| worker (a process that serves no requests) | ワーカー |
| mode (of a command) | モード |
| handle (an event) | 処理する |
| event bus | イベントバス |
| queue | キュー |
| emulator | エミュレーター |
| certificate (TLS) | 証明書 |
| unique constraint | 一意制約 |
| unit test | 単体テスト |
| mail | メール |
| mail server | メールサーバー |
| mailer | メーラー |
| recipient (of a mail) | 宛先 |
| subject (of a mail) | 件名 |
| body (of a mail) | 本文 |
| sender (of a mail) | 差出人 |

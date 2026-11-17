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

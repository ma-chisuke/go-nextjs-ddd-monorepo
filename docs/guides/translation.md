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
- Code, comments, commit messages, and the descriptions in OpenAPI documents are in English only.
- Files under `.claude/` and `.github/` are instructions for tools, and are in English only.

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

# GitHub のリポジトリを設定する

このリポジトリの決まりには、ファイルではなく GitHub 上のリポジトリの設定にあるものがある。
CI が `main` を守れるのはルールセットが CI のチェックを必須にしている場合だけであり、Renovate が更新をマージできるのはリポジトリが自動マージを許している場合だけである。
スクリプト `tools/github/apply-settings.sh` がこれらの設定を書き込む。設定をスクリプトに置くことで、プルリクエストでレビューでき、いつでも適用し直せる。
このガイドは、スクリプトの実行方法、各設定の意味、スクリプトにはできない作業（リリース用の App、デプロイの環境、Renovate の App）を説明する。

## 設定を適用する

リポジトリの管理者としてログインした `gh` が必要である。
スクリプトは `make` から実行する。`make` が、固定したバージョンの `gh` と `jq` を `PATH` に入れる。

```sh
make github-settings                        # ドライラン：違いを表示するだけで、何も変えない
make github-settings APPLY=1                # 違う設定を変える
make github-settings REPO=<owner>/<name>    # 今いるリポジトリ以外を対象にする
```

出力の各行は1つの設定を表す。

- `ok` は、その設定がすでに望む値であることを表す。
- `change` は、今の値と望む値を表示する。`APPLY=1` を付けると、望む値を書き込む。
- `note` は、スクリプトでは変えられないことを伝える。

スクリプトは書き込む前にすべての設定を比べる。そのため、`APPLY=1` で2回目を実行すると、変える設定は見つからない。

テンプレートをコピーしたチームは、初めて実行する前に、スクリプトの先頭にある2つの値を変える。`is_template=false` と、自分たちの `topics` である。

## スクリプトが変える設定

### リポジトリ

| 設定 | 値 | 理由 |
|---|---|---|
| テンプレートリポジトリ | オン | 利用者はこのリポジトリから自分のプロジェクトを作る。 |
| トピック | `go`、`nextjs`、`ddd` など | 利用者はトピックからテンプレートを見つける。 |
| マージの方法 | スカッシュだけ | どのプルリクエストも `main` では1つのコミットになり、履歴は一直線のまま保たれる。 |
| スカッシュのコミットメッセージ | プルリクエストのタイトルと、そのコミットのメッセージ | release-please は `main` 上のタイトルを読む。そのため CI は、タイトルが Conventional Commit の形かどうかを確かめる（`make lint-pr-title`）。 |
| マージ後のブランチ削除 | オン | マージしたブランチはもう使わない。 |
| 自動マージ | 許可 | Renovate は、パッチ、ダイジェスト、固定の更新を、必須のチェックが通った時点でマージする。 |

### デフォルトブランチのルールセット

ルールセット `main` はデフォルトブランチに適用され、管理者を含めて誰も迂回できない。

- 変更はすべてプルリクエストを通る。承認のレビューは求めない。メンテナーが1人なら、自分のプルリクエストを自分で承認できないからである。チームで使うなら、この数を増やす。
- チェック `check` が成功しなければならず、その結果は GitHub Actions から届いたものでなければならない。`check` は `.github/workflows/ci.yml` の最後のジョブで、CI のほかのジョブが1つでも失敗すると失敗する。そのため、CI にジョブを足してもルールセットは変わらない。
- 履歴は一直線で、プルリクエストはスカッシュでだけマージできる。
- 誰もこのブランチに強制プッシュできず、削除もできない。

ルールセットは、マージの前にブランチが `main` に追いついていることを求めない。求めると、何かがマージされるたびに、Renovate のものを含む開いているプルリクエストがすべてリベースを待つことになる。

### セキュリティ

| 設定 | 値 | 理由 |
|---|---|---|
| シークレットスキャンとプッシュ保護 | オン | 既知の種類のシークレットを含むプッシュを GitHub が拒む。`make check` もコミットの前に gitleaks でシークレットを探す。 |
| 脆弱性の非公開報告 | オン | `SECURITY.md` は、これを通じて報告するよう求めている。GitHub はこの機能を公開リポジトリにだけ提供する。 |
| Dependabot アラート | オン | GitHub の依存関係グラフが把握する依存パッケージについて、既知の脆弱性を GitHub が知らせる。 |
| Dependabot セキュリティアップデート | オフ | 修正は Renovate が提案する。ほかの更新と同じコミットメッセージで、同じチェックを通る。 |
| コードスキャンの既定の設定 | オフ | 代わりに `.github/workflows/codeql.yml` が CodeQL を実行する。この2つは同時には動かない。 |

### GitHub Actions

| 設定 | 値 | 理由 |
|---|---|---|
| アクションをコミットの完全な SHA で固定する | 必須 | タグやブランチでアクションを参照するワークフローを GitHub が拒む。`make check` も同じことを確かめる（pinact、ghalint）。 |
| `GITHUB_TOKEN` の既定の権限 | 読み取り | 各ジョブが必要な権限を自分で求める。 |
| GitHub Actions によるプルリクエストの作成と承認 | 不可 | ワークフローが自分の変更を承認できないようにする。リリースは代わりに App を使う（後述）。 |

### 環境 `release`

環境 `release` はリリース用の App の秘密鍵を持ち、これを使えるのはデフォルトブランチで動くジョブだけである。
そのため、ほかのブランチのワークフローは、誰かがワークフローを書き換えても鍵を読めない。

## リリース用の App を用意する

`.github/workflows/release.yml` は、`main` へのプッシュのたびに release-please を実行する。
release-please は、`chore(main): release <version>` という題のリリース用プルリクエストを開いたままにする。
このプルリクエストは `CHANGELOG.md` と、`.github/release-please-manifest.json` に書かれたバージョンを更新する。
このプルリクエストをマージすると、リリースにタグ（`v0.1.0`）が付き、リリースノートが公開される。

release-please は GitHub App のトークンで書き込む。理由は2つある。上の設定が GitHub Actions によるプルリクエストの作成を禁じていること、そして `GITHUB_TOKEN` で開いたプルリクエストでは CI が動かないことである。
App ができるまで、リリースのワークフローは実行されない。App は次の手順で作る。

1. リポジトリを所有するアカウントで GitHub App を作る（**Settings** > **Developer settings** > **GitHub Apps** > **New GitHub App**）。Webhook とコールバック URL は要らない。
2. リポジトリの権限として次の3つを与え、ほかは与えない。**Contents** の読み書き、**Pull requests** の読み書き、**Issues** の読み書きである。
3. App をこのリポジトリだけにインストールする。
4. App の秘密鍵を生成する。
5. クライアント ID をリポジトリの変数として、秘密鍵を環境 `release` のシークレットとして保存する。

   ```sh
   make github-settings APPLY=1   # 先に環境 release を作る
   gh variable set RELEASE_APP_CLIENT_ID --body '<client ID>'
   gh secret set RELEASE_APP_PRIVATE_KEY --env release < private-key.pem
   ```

6. ダウンロードした鍵のファイルを削除する。

次に `main` へプッシュすると、バージョン 0.1.0 の最初のリリース用プルリクエストが開く。
それ以降は、`main` 上のコミットメッセージが次のバージョンを決める。

- `feat` はマイナーバージョンを上げる。`fix` とそれ以外の型はパッチバージョンを上げる。
- 互換性を壊す変更（`feat!:` や `BREAKING CHANGE:` のフッター）は、バージョンが 1.0.0 未満のあいだはマイナーバージョンを、それ以降はメジャーバージョンを上げる。
- リリースノートに載るのは `feat`、`fix`、`perf`、`revert`、`build`、`ci`、`docs` である。`refactor`、`test`、`style`、`chore` は載らず、これらだけからなるリリースは作られない。

## デプロイの環境を用意する

ワークフロー `Plan` と `Deploy` は、stg と prd の AWS アカウントに届く（TPL-0018）。これらには、GitHub の環境2つと、リポジトリの変数5つが要る。スクリプトは、これらを設定も比較もしない。[AWS にデプロイする](deploy.ja.md#アカウントを準備する)のとおりにアカウントを準備したら、手で設定する。ロールの変数を設定するまで、それを使うジョブはスキップされる。

| 設定 | 値 | 理由 |
|---|---|---|
| 環境 `stg` | デプロイできるブランチ：`main` だけ | stg のロール `github-deploy` は、この環境のジョブだけを信頼する。そのため、デプロイするのは `main` へのマージだけである。 |
| 環境 `prd` | デプロイできるブランチ：`main` だけ。必須のレビュー担当者：1人以上 | prd のジョブは、その人が承認するまで待つ。 |
| 変数 `AWS_REGION` | 環境のリージョン。たとえば `ap-northeast-1` | どのジョブも、そこで AWS にサインインする。 |
| 変数 `AWS_PLAN_ROLE_ARN_STG` と `AWS_PLAN_ROLE_ARN_PRD` | 各アカウントのロール `github-plan` の ARN。`make infra-bootstrap` が表示する | `Plan` のジョブがこれを引き受ける。 |
| 変数 `AWS_DEPLOY_ROLE_ARN_STG` と `AWS_DEPLOY_ROLE_ARN_PRD` | 各アカウントのロール `github-deploy` の ARN | `Deploy` のジョブがこれを引き受ける。 |

環境は **Settings** > **Environments** で作り、変数は `gh` で設定する。

```sh
gh variable set AWS_REGION --body ap-northeast-1
gh variable set AWS_PLAN_ROLE_ARN_STG --body '<ARN of github-plan in stg>'
gh variable set AWS_DEPLOY_ROLE_ARN_STG --body '<ARN of github-deploy in stg>'
```

これらの変数はシークレットではない。ARN は何の権限も与えず、どのジョブがロールを引き受けられるかは、各ロールの信頼ポリシーが決める。

## Renovate を入れる

リポジトリに [Renovate の GitHub App](https://github.com/apps/renovate) をインストールする。
Renovate は `renovate.json` を読み、毎週月曜日に、更新ごと、または更新のグループごとに1つのプルリクエストを開く。

- リリースから3日待つ。pnpm と uv も同じだけ待つ。
- 同じタグのまま出るベースイメージの新しいダイジェストは、曜日を問わず、すぐに届く。固定が動くまで、`scan-images` が失敗するからである。
- パッチ、ダイジェスト、固定の更新は、`check` が通ればそのままマージされる。マイナーの更新は人がレビューする。
- メジャーの更新は、Dependency Dashboard で待つ。プルリクエストにするには、そこでチェックボックスを入れる。
- セキュリティの修正は、予定を待たずにすぐ届く。
- ロックファイルは毎月1日に更新する。
- Dependency Dashboard の issue に、保留中の更新がすべて並ぶ。

Renovate は `mise.toml` と一緒に `mise.lock` も更新する。
Renovate のプルリクエストが `mise.toml` を変えて `mise.lock` を変えていないと、CI で `make setup` が失敗する。
その場合は、そのプルリクエストのブランチをチェックアウトし、`make tools-lock` を実行して、結果をプッシュする。

Renovate は、`infra/live/providers.hcl` の Terraform の provider も更新する。`infra/local/` のものと一緒に、1つのプルリクエストにまとめる（グループ `Terraform providers`）。
Renovate は `infra/live/providers.lock.hcl` を書かない。そのため、そのブランチでは Terragrunt の実行がどれも失敗し、CI の `validate-live` も失敗する。
そのブランチをチェックアウトし、`make infra-lock` を実行して、結果をプッシュする。

Renovate がオプションの名前や形を変えると、Dependency Dashboard は `renovate.json` を移行するプルリクエストを提案する。
check-jsonschema に同梱されたスキーマが新しい形を知らないあいだ、そのプルリクエストは `lint-schema` で失敗する。
その間も Renovate は古い形を読めるので、プルリクエストは閉じる。移行は、check-jsonschema を更新するプルリクエストで行う。

pnpm と uv は、リリースから3日たっていないバージョンを拒む。今日リリースされたセキュリティの修正も例外ではない。
そうした修正をすぐ取り込むには、そのパッケージを `pnpm-workspace.yaml`（`minimumReleaseAgeExclude`）か `pyproject.toml`（`exclude-newer-package`）で決まりから外す。
リリースから3日たったら、その例外を消す。

<!-- i18n-source: github-settings.md sha256:f1dc0fd4b216eac0a7de3594a8b39d762ec38ded9b08911faaea563094ab3ce9 -->

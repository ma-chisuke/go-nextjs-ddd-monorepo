---
status: accepted
date: 2026-11-17
---

# TPL-0024: サプライチェーンを層を重ねて守る

## 背景と課題

このリポジトリで動くコードの大半は、他人が書いたものである。CI のアクション、各マシンの道具、npm・Python・Go のパッケージがそうであり、のちにはコンテナのベースイメージも加わる。
どれもソースコードにアクセスできる状態で動き、CI ではトークンにもアクセスできる。
攻撃者がそのどれか1つを乗っ取れば、それを使うすべてのリポジトリに手が届く。

このテンプレートが使う道具も、実際にこの種の攻撃を受けた。
2026年3月の攻撃（CVE-2026-33634）では、Trivy v0.69.4 が改ざんされ、GitHub Actions の `trivy-action` と `setup-trivy` のタグが書き換えられた。

テンプレートは、この影響を何倍にもする。テンプレートが信頼するものは、そのすべてのコピーが信頼することになる。

乗っ取られた依存にできることをどう限り、乗っ取られたことにどう気づくか。

## 判断の基準

- このリポジトリが依存するものの変更は、誰かがレビューできる、このリポジトリへの変更を通してだけ入ってくる。
- 改ざんされたダウンロードは、インストールに失敗する。
- 新しいリリースは、インストールの前に一定の期間を待つ。悪意のあるリリースが、その間に見つかって取り下げられるようにするためである。
- 守りは、できる限り人の注意ではなく、`make check`・`make check-all`・パッケージマネージャー・GitHub のどれかが守らせる。
- 守りに有料のサービスを必要としない。

## 検討した選択肢

- すでに固定した道具で、層を重ねて守る
- 配布元のタグと最新のバージョンを信頼する
- すべての依存をリポジトリに取り込む
- サプライチェーンのセキュリティのための商用プラットフォーム

## 決定

採用：「すでに固定した道具で、層を重ねて守る」。層ごとに違う種類の攻撃を止められ、全体で有料のサービスが要らず、チェックの時間もほとんど延びないからである。

GitHub Actions には次の決まりを置き、`make check` の pinact・zizmor・ghalint が守らせる（TPL-0018）。

- どのアクションもコミット SHA の全桁で固定する。タグが動かされても、何も変わらない。
- `permissions` は既定で読み取りだけにし、各ジョブは必要な権限だけを求める。
- どのジョブにも `timeout-minutes` を書く。

道具には次の決まりを置き、`make setup` が道具を入れるときに mise が守らせる（TPL-0022）。

- mise は、`mise.lock` に書かれたもの、つまりプラットフォームごとの URL とチェックサムに合うものだけを入れる。配布元が来歴（GitHub の artifact attestations・cosign・SLSA）を提供している場合、mise はロックを書くときにそれを検証する。
- Trivy は、mise がバージョンとチェックサムを固定して入れる。この記録で述べた攻撃があったため、`trivy-action` と `setup-trivy` のアクションは使わない。
- `make check` は、固定した Trivy のバイナリに組み込まれた設定ミスのチェックで Trivy を動かす（`--skip-check-update`）。この指定がないと、Trivy は実行のたびに最新のチェックをダウンロードする。そうなると、チェックの最中にバージョンを固定していないダウンロードが起こり、結果も日によって変わる。

npm のパッケージには、`pnpm-workspace.yaml` の次の設定を pnpm が守らせる。

- 新しいリリースは、公開から3日たつまで入れられない（`minimumReleaseAge`）。
- バージョンは範囲ではなく、正確な値で保存する（`saveExact`）。
- 推移的な依存のうち、Git リポジトリや tarball の URL のような信頼できない場所から来るものは拒む（`blockExoticSubdeps`）。
- 公開者の信頼の度合いが以前のリリースより下がったリリースは拒む（`trustPolicy: no-downgrade`）。
- `allowBuilds` に載っていない依存は、ビルドのスクリプトを実行しない（`strictDepBuilds`）。

Python のパッケージには、`pyproject.toml` の次の設定を uv が守らせる。

- `uv.lock` はすべてのパッケージのハッシュを持つ。ロックが合わなければ、`uv sync --locked` は失敗する。
- 公開から3日たっていないリリースは無視する（`exclude-newer`）。
- uv は mise が入れた Python を使い、別の Python をダウンロードしない（`python-downloads = "never"`）。

Go のモジュールには次の決まりを置き、Go のツールチェーンと Makefile が守らせる。

- `go.sum` は、すべてのモジュールのチェックサムを持つ。
- `GOFLAGS=-mod=readonly` により、コマンドは `go.mod` や `go.sum` を書き換える代わりに失敗する。`GOTOOLCHAIN=local` により、Go は別のツールチェーンをダウンロードしない。
- govulncheck は、実際に呼ばれているコードにある既知の脆弱性を報告する（`make check-all` の `vuln-go`）。

Trivy は、ロックファイルをもとに、既知の脆弱性と、禁止したライセンスの依存を探す（`make check-all` の `scan-deps`、および TPL-0019）。ライセンスのスキャンは製品とともに配るものだけを対象にし、開発のための依存と `tools/` のモジュールは外す。設定ミス・脆弱性・ライセンス・SBOM のスキャナーは Trivy だけである。固定した1つのバイナリで4つを扱えるので、固定して見張るスキャナーがもう1つ増えることはない。Go については、govulncheck が脆弱性を実際に呼ばれているコードに絞る（TPL-0023）。

秘密情報は、2つの仕組みでコードの中から探す。

- gitleaks は、コミットの前にはステージした変更を、`make check` では作業ツリーを調べる（`lint-secrets`）。
- GitHub のシークレットスキャンとプッシュ保護は、すべてのプッシュを調べる。

リポジトリには、GitHub の次のサービスを使う。

- CodeQL が Go と TypeScript のコードを解析する（`.github/workflows/codeql.yml`）。
- OpenSSF Scorecard が、リポジトリのセキュリティの慣行を評価する（`.github/workflows/scorecard.yml`）。
- private vulnerability reporting を使えば、脆弱性を公開せずに報告できる。
- Dependabot は、脆弱な依存について警告を出す。プルリクエストは開かず、更新は Renovate が担う（TPL-0025）。

コンテナはコードやインフラと一緒に入り、次の決まりに従う。

- ベースイメージは最小限のものを選び、ダイジェストで固定する。
- コンテナは root 以外のユーザーで動かす。
- Trivy がすべてのイメージをスキャンし、その SBOM を書く。
- ビルドには、GitHub の artifact attestations で来歴を付ける。

この決定には、最初の計画と違う点が2つある。どちらも意図したものである。

- 最初の計画では、`trivy-action` だけを避けることにしていた。同じ攻撃で `setup-trivy` のタグも書き換えられたので、どちらのアクションも使わない。
- 最初の計画では、SBOM を Trivy か syft で作るとしていた。Trivy はすでにイメージをスキャンしており、1つの関心事には道具を1つだけ使う（TPL-0023）ので、SBOM も Trivy が書く。

### 結果

- 良い点：タグが動かされても、リリースが差し替えられても、ダウンロードが改ざんされても、そのコードは動く前にインストールの時点で失敗する。
- 良い点：悪意のある npm や Python のリリースが3日以内に見つかって取り下げられれば、このリポジトリには届かない。
- 良い点：CI のステップが乗っ取られても、それが持つのはジョブが求めた権限だけであり、それもジョブが動いている間だけである。その時間はタイムアウトが区切る。
- 悪い点：更新は、セキュリティの修正も含めて3日遅れて届く。誰かが待ちを解除しない限り、この遅れは縮まらない。
- 悪い点：道具を足す手間が増える。`mise.toml` に項目を書き、4つのプラットフォームの分のロックを作り、場合によっては `allowBuilds` にも項目を足す。
- 悪い点：来歴を公開していない道具もある。それらについては、`mise.lock` はチェックサムしか記録しない（gitleaks・shellcheck・Vale など）。チェックサムが示すのは、どのマシンもロックしたときと同じファイルを受け取ることだけであり、そのファイルが作者のビルドしたものであることまでは示さない。
- 悪い点：このリポジトリは、Go のモジュールにも `mise.toml` の道具にも待ちの期間を設けていない。人が手で足したバージョンは、`go get` でも `mise.toml` でもすぐに入る。待つのは、Renovate が提案する更新だけである（TPL-0025）。
- 悪い点：GitHub の守りのいくつかは、ファイルではなく設定である。`make github-settings` がそれらを `docs/guides/github-settings.md` と比べるが、`make check` からは見えない。

### 確認の方法

- `make check` は、ワークフローに `lint-actions-pins`・`lint-actions-security`・`lint-actions-policy` を、ファイルに `lint-secrets` を実行する。Trivy に組み込まれたチェックで `scan-config` も実行する。
- `make setup` は、`locked = true` のもとで `mise install` を実行する。`pnpm-workspace.yaml` の設定のもとでの `pnpm install --frozen-lockfile` と、`uv sync --locked` も実行する。CI も同じターゲットを実行する。
- `make check-all` は `scan-deps` と `vuln-go` を実行する。
- `trivy-action` と `setup-trivy` を使うワークフローがないことは、レビューで確かめる。これを守らせるチェックはない。
- CodeQL と Scorecard は、ワークフローとして動く。シークレットスキャン・プッシュ保護・private vulnerability reporting・Dependabot alerts は、リポジトリの設定である。`make github-settings` がこれらを有効にし、`docs/guides/github-settings.md` と違うものがあれば報告する。
- コンテナについては、`make check` が `lint-image-pins` と `scan-config` を実行する。`lint-image-pins` はダイジェストのないベースイメージで失敗し、`scan-config` の Trivy のチェックは root で動くイメージの Dockerfile で失敗する。サービスのイメージ（`backend/Dockerfile`）は、distroless static のユーザー `nonroot` で動く。
- `make check-all` は `scan-images` を実行し、すべてのサービスのイメージに既知の脆弱性がないかを Trivy でスキャンする。`make sbom` は、イメージごとに CycloneDX の SBOM を `dist/sbom/` に書く。
- GitHub の artifact attestations による来歴は、イメージを公開するワークフローと一緒に、インフラのマイルストーンで入る。そのワークフローは、`make sbom` の SBOM にも attestation を付ける。

## 選択肢の長所と短所

### すでに固定した道具で、層を重ねて守る

- 良い点：ほとんどの層は、チェック・パッケージマネージャー・mise が守らせる。残りは GitHub の設定であり、`make github-settings` とレビューで確かめる。
- 良い点：有料のサービスが要らない。
- 悪い点：設定が多くのファイルに散らばっており、変える前にそれぞれを理解する必要がある。

### 配布元のタグと最新のバージョンを信頼する

- 良い点：更新が、手間なしですぐに届く。
- 悪い点：タグが動かされたり悪意のあるリリースが出たりすると、テンプレートのすべてのコピーに次の実行で届く。

### すべての依存をリポジトリに取り込む

- 良い点：ビルドの最中に何もダウンロードせず、依存の変更はすべて差分に出る。
- 悪い点：リポジトリがすべての依存の分だけ大きくなり、更新の差分が大きすぎてレビューできない。
- 悪い点：4つのプラットフォームの分のバイナリを置くと、どのクローンも大きくなる。

### サプライチェーンのセキュリティのための商用プラットフォーム

- 良い点：プラットフォームによっては、勧告が出る前に、悪意のあるパッケージをその振る舞いから見つける。
- 悪い点：テンプレートが有料のアカウントを必要とするわけにはいかない。また、そのプラットフォームが、コードにアクセスできる相手として1つ増える。

## 補足

- [pnpm の設定](https://pnpm.io/settings)・[uv の設定](https://docs.astral.sh/uv/reference/settings/)・[mise.lock](https://mise.jdx.dev/dev-tools/mise-lock.html)
- [OpenSSF Scorecard](https://github.com/ossf/scorecard)
- 道具の固定は TPL-0022 に、1つの関心事に道具を1つだけ使う決まりは TPL-0023 に、更新は TPL-0025 にある。

<!-- i18n-source: TPL-0024-supply-chain-defenses.md sha256:86823a2229479987965f2be152014d43cdcccaf0c3178741543acb3a01d952f4 -->

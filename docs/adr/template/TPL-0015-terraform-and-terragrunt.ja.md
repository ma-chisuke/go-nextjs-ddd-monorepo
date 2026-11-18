---
status: accepted
date: 2026-11-17
---

# TPL-0015: インフラを Terraform と Terragrunt で管理し、stg と prd を別々の AWS アカウントに置く

## 背景と課題

テンプレートは AWS の上で動く。ネットワーク、Aurora のクラスタ（TPL-0016）、ECS Gateway を備えた ECS のクラスタ（TPL-0014）、EventBridge のバス（TPL-0013）、文脈ごとのサービス、Next.js がその中身である。
これを stg と prd の2つの環境に、それぞれ作る。

インフラはコードとして書き、リポジトリのほかの部分と同じく、プルリクエストでレビューして plan を確かめる。
このとき、向きの違う3つの要件がある。

- 環境どうしは同じ形でなければならない。stg で動いたものが prd でも動くようにするためである。一方で、大きさ・キャパシティ・ドメイン名などの値は環境ごとに違う。
- ある文脈への変更では、その文脈だけを plan し apply したい。全部を1つの state にすると、plan はいつも遅く、apply はいつも危うくなる。
- stg での誤りが prd に及んではならず、一方の環境の認証情報がもう一方で使えてもならない。

インフラをどの道具で書き、コードをどう分け、環境どうしをどう隔てればよいか。

## 判断の基準

- stg と prd は同じ形を持ち、その形は1回だけ書く。違うのは値だけである。
- ある文脈への変更は、その文脈の state だけを plan し apply する。
- stg での誤りや漏れた認証情報が、prd に届かない。
- 広く知られた道具であり、リンターとスキャナーがそろっている。
- 道具のバージョンを1か所で固定する（TPL-0022）。

## 検討した選択肢

- Terraform のモジュールを Terragrunt で組み立て、環境ごとに AWS アカウントを分ける
- Terraform だけを使い、環境ごとにワークスペースか変数ファイルを分ける
- Terraform の代わりに OpenTofu
- AWS CDK
- Pulumi

## 決定

採用：「Terraform のモジュールを Terragrunt で組み立て、環境ごとに AWS アカウントを分ける」。たくさんの小さなユニットに対して、state と provider の設定を Terragrunt が1回の記述から作り出せる。また、別々のアカウントは、AWS が提供するなかでいちばん強い stg と prd の境界だからである。

決まりは次のとおりである。

- Terraform のモジュールは `infra/modules/` に置く。`network`・`database`・`platform`（ECS のクラスタ、ECS Gateway、EventBridge のバス）・`context-service`・`web` の5つである。
- `platform` と `context-service` のうち、手元の環境と AWS が共有する部分は、2つの小さなモジュールにする。`platform` の EventBridge のバスである `event-bus` と、受け取る文脈1つのキュー・デッドレターキュー・ルールである `event-subscription` である（TPL-0013）。ルート `infra/local/` が、これらを AWS Emulator に適用する（TPL-0017）。`infra/local/` はユニットではない。手元のマシンの上のエンドポイントしか受け付けない。その state はリポジトリの外に置き、エミュレーターが再起動すれば捨てる。
- Terragrunt のツリー `infra/live/{stg,prd}/` が、モジュールを組み立てる。モジュールを使うディレクトリはそれぞれユニットであり、自分の state を持つ。ユニットは `network`・`database`・`platform`・`web` と、`contexts/<context>/` の下の文脈ごとのユニットである。
- `infra/live/root.hcl` が、すべてのユニットの state の保存先と provider の設定を作る。
- state は S3 に置き、S3 のネイティブのロック（`use_lockfile`）を使う。state はユニットごとに1つである。
- ユニットは、Terragrunt の依存関係を通してほかのユニットの出力を読む。たとえば文脈のユニットは、`network`・`database`・`platform` の出力を読む。
- stg と prd は別々の AWS アカウントにする。2つのツリーの形は同じで、違うのは `env.hcl` の値だけである。
- Terraform と Terragrunt のバージョンは `mise.toml` で固定する（TPL-0022）。
- CI は OIDC で各アカウントにサインインし、長期のアクセスキーを持たない（TPL-0018）。

### 結果

- 良い点：ある文脈への変更は、小さな state を1つ plan し apply するだけで済む。plan は速く、apply がほかの文脈を変えることもない。
- 良い点：形はモジュールに1回だけ書くので、stg と prd は同じコードで動く。
- 良い点：アカウントが別なので、環境どうしが隔離される。stg の権限は prd で何の効力も持たず、クォータと費用も分かれる。
- 良い点：Terraform は provider とモジュールのエコシステムがいちばん大きく、リンター（tflint）とスキャナー（Trivy）も対応している。
- 悪い点：覚えて固定する道具が2つになる。Terragrunt は、Terraform の上に設定の層を1つ重ねる。
- 悪い点：小さな state が互いに依存する。たとえば `network` の出力を変えたら、それを読むユニットより先に apply する必要があり、順番を守る手間がかかる。
- 悪い点：アカウントごとに、state 用のバケット、OIDC 用のロール、DNS の準備が要る。
- 悪い点：Terraform のライセンスは Business Source License であり、オープンソースのライセンスではない。ただし、チームが自分のインフラを管理するために Terraform を使うことは制限していない。

### 確認の方法

- `make check` は5つのチェックを実行する。`fmt-check-terraform` は、Terraform の言語のファイルに `terraform fmt` をかける。`fmt-check-terragrunt` は、それ以外の HCL のファイル（`root.hcl` や `terragrunt.hcl` など）に `terragrunt hcl fmt` をかける。`lint-terraform` は、AWS のルールセットを使う tflint を動かす。`lint-terraform-docs` は、terraform-docs が書くモジュールの README が最新かどうかを調べる。`scan-config` は、Trivy で設定ミスを検査する。
- `.tflint.hcl` が固定する tflint のプラグインは `make setup` が入れるので、これらのチェックはネットワークを使わない。
- `make check-all` は、さらに `validate-terraform`（`terraform validate`）を実行する。これは `terraform init` を実行し、provider をダウンロードする。
- これらの道具は `mise.toml` で固定してある（TPL-0022）。イベントを扱うマイルストーンから、これらのチェックは `infra/modules/event-bus`・`infra/modules/event-subscription`・`infra/local/` を検査する。
- `infra/local/.terraform.lock.hcl` は、AWS の provider を4つのプラットフォームのハッシュとともに固定する。`tools/scripts/aws.sh` は `-lockfile=readonly` で初期化するので、ロックにない provider は入らない。
- `make check-all` に含まれる `make smoke` は、`infra/local/` を AWS Emulator に適用し、それが作るバスとキューにイベントを流す。そのため、適用できないモジュールや、イベントを届けないモジュールは、そこで失敗する（TPL-0017）。

ほかのモジュールと `infra/live/` の Terragrunt のユニットは、インフラのマイルストーンで加わる。それ以降は、`infra/` を変えるプルリクエストごとに、CI がすべてのユニットを plan する（`terragrunt run --all plan`）。

## 選択肢の長所と短所

### Terraform のモジュールを Terragrunt で組み立て、環境ごとに AWS アカウントを分ける

- 良い点：state と provider の設定を1回だけ書けば済み、ユニットごとの state は小さく、ユニットどうしの依存も宣言できる。2つの環境は同じ形になる。
- 悪い点：道具が2つあり、覚えるべき設定の層があり、ユニットの間の順番を守る必要がある。

### Terraform だけを使い、環境ごとにワークスペースか変数ファイルを分ける

- 良い点：道具が1つで済み、Terraform のドキュメントの範囲を超えるものがない。
- 悪い点：1つのディレクトリのワークスペースは、state の保存先を共有する。そのため、アカウントを分ける形とは相性が悪い。Terraform のドキュメント自身も、デプロイごとに認証情報とアクセス制御を分ける必要がある場合は、ワークスペースは隔離の手段に向かないと書いている。
- 悪い点：インフラを多くの state に分けると、どのルートモジュールにも state の保存先と provider のブロックを繰り返し書き、state の間の出力の受け渡しも手で書くことになる。

### Terraform の代わりに OpenTofu

- 良い点：Linux Foundation のもとにある、オープンソース（MPL 2.0）の Terraform のフォークである。Terraform のコードの大半と互換で、Terragrunt も対応している。state の暗号化のような独自の機能も加わっている。
- 悪い点：リリースを重ねるごとに Terraform との違いが広がる。テンプレートをコピーするチームにとっても、いまも使う人の多い Terraform のほうがなじみのある可能性は高い。

### AWS CDK

- 良い点：インフラを TypeScript や Go のような汎用の言語で、型とループを使って書ける。state は CloudFormation が持つ。
- 悪い点：デプロイされるのはコードから生成した CloudFormation のテンプレートである。レビューする人が読むのはテンプレートを生成するコードであり、tflint や terraform-docs のように Terraform 向けに選んだ道具は使えない。
- 悪い点：AWS でしか使えない。どの変更も CloudFormation のスタックを通り、更新に失敗したスタックは、手で復旧しなければならない場合もある。

### Pulumi

- 良い点：インフラを汎用の言語で書け、エンジンはオープンソース（Apache 2.0）であり、provider も多い。
- 悪い点：state の保存先は、既定では Pulumi のホスティングサービスであり、アカウントが要る。テンプレートは、アカウントなしで動く必要がある。Terraform と同じく、S3 に置くこともできる。
- 悪い点：Terraform ほど知られていない。また、プログラムとして書いたインフラは、CDK と同じくレビューの手間が大きい。

## 補足

- テンプレートの設計メモには Terragrunt 1.0 と書いていた。テンプレートは、作業を始めた日の最新の 1.x のリリースから始め、その後は Renovate が更新する（TPL-0025）。
- OpenTofu を使いたいチームは、利用者の記録を書いて切り替えられる。モジュールは HCL のままで、Terragrunt はどちらにも対応している。
- [Terragrunt のドキュメント](https://terragrunt.com/)
- `use_lockfile` を含む、[Terraform の S3 のバックエンド](https://developer.hashicorp.com/terraform/language/backend/s3)の説明

<!-- i18n-source: TPL-0015-terraform-and-terragrunt.md sha256:9ef9277b9c464ba9578299da515dbcee9df6942323a71159ffcc40a0eba04ff8 -->

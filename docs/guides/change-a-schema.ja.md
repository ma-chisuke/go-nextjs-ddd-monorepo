# スキーマを変える

文脈の `schema.sql` は、そのスキーマのあるべき状態である（TPL-0010）。
マイグレーションのファイルはない。pgschema がデータベースとこのファイルを比べ、データベースをその状態にする DDL を plan として求める。sqlc と tbls も同じファイルを読む。

## 手順

1. `backend/internal/<context>/internal/adapter/postgres/schema.sql` を編集する。

   - 名前にスキーマを付けない。文脈のロールの検索パスがスキーマを決める。
   - 新しいテーブルには、`DEFAULT uuidv7()` を持つ `uuid` 型の主キーを与える。ただし、主キーが外部キーでもある場合は除く（TPL-0012）。
   - 新しいテーブルと列には、`COMMENT ON TABLE` と `COMMENT ON COLUMN` でコメントを付ける。
   - 新しいテーブルについて、実行時のロールに権限を与える。`GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE <table> TO <context>_runtime;` である。pgschema が権限を管理し、bootstrap は既定の権限を設定しないので、この行がなければサービスはそのテーブルを使えない。
   - インデックスは素の `CREATE INDEX` で書く。既存のテーブルに対しては、pgschema が自分で `CONCURRENTLY` を付けて作る。

   確認：`make check` がこのファイルを squawk で検査し（`lint-sql`）、SQL がスキーマの名前を書くと失敗する（`arch/schema-reference`）。

2. 手元のデータベースに変更を適用する。PostgreSQL がまだ動いていなければ、`make db-up` が起動して、すべてのスキーマを適用する。動いていれば、次のようにする。

   ```sh
   make db-plan    # 文脈ごとの DDL を表示する。何も変えない
   make db-apply   # それを実行する
   ```

   確認：そのあと `make db-plan` は SQL 文を何も表示しない。

3. 型付きのクエリを生成し直す。

   ```sh
   make gen
   ```

   sqlc は新しいスキーマを読むので、もうない列を名指しするクエリはここで失敗する。`queries/` のクエリと、生成された型を使うコードを直す。中核の文脈ではストア、補完の文脈ではユースケース、それにメモリ上の偽物である。

   確認：`make check`（`gen-check-sqlc`・`lint-go`・`test-go`）。

4. `docs/generated/db/<context>/` にあるスキーマの文書を生成し直す。

   ```sh
   make gen-db-docs
   ```

   `make gen` はデータベースを使わないので、これらの文書を書かない。

   確認：`make check-all` の `lint-db-docs` は、コメントのないテーブルや列、インデックスのない外部キー、tbls が書く内容と違う文書で失敗する。

5. PostgreSQL を要するチェックを実行する。

   ```sh
   CHECK_SCOPE=changed make check-all
   ```

   `lint-sql-plan` は、次の節で説明するとおり、変更の DDL を検査する。`lint-sqlc` は、すべてのクエリを新しいスキーマに対して prepare する。`test-integration` は結合テストを実行し、実行時のロールの権限も試す。
   plan は `.cache/sql-plan/<context>.sql` に残る。CI はまだ plan をプルリクエストに示さないので、レビュアーはチェックを実行してから、そこで plan を読む。

## lint-sql-plan が防ぐもの

`lint-sql-plan` は、ベースのコミットの `schema.sql` を作業用のデータベースに適用し、変更後の `schema.sql` への plan を pgschema に求める。そして、その DDL を squawk で検査する（`.squawk.plan.toml`）。
ベースのコミットは `origin/main` とのマージベースであり、`CHECK_BASE` を設定したときはそれとのマージベースである。
この DDL は、データを持ち、リクエストに応えているデータベースに対して実行される。そのため、このチェックは次のような SQL 文で失敗する。

- `DROP COLUMN` と `DROP TABLE`（`ban-drop-column`、`ban-drop-table`）。データが失われ、まだ動いているコードがそのデータを読むかもしれない。
- `NOT NULL` で既定値のない新しい列（`adding-required-field`）。既存の行は、その列の値を持てない。
- 列の型の変更（`changing-column-type`）。PostgreSQL が、読み書きを止めるロックのもとでテーブルを書き直す。
- 既存のテーブルへの新しい `UNIQUE` 制約（`disallowed-unique-constraint`）。インデックスを作る間、ロックが読み書きを止める。
- 既存のテーブルへの新しい `CHECK` 制約（`constraint-missing-not-valid`）。pgschema は、制約の追加と検証を1つのトランザクションで行う。

NULL を許す列、定数の既定値を持つ列、新しいインデックスは通る。

## 安全な段階に分けた変更

このチェックが拒む変更は、たいてい何段階かに分けて行うべき変更である。各段階をマージして適用してから、次の段階に進む。そうすれば、ある段階の間に動いているコードは、その段階の前と後のどちらのスキーマでも動く（TPL-0010）。

列の名前を変えるには、次のようにする。

1. 新しい列を、NULL を許すか既定値を持つ形で足す。
2. データを写す。pgschema はデータを動かさないので、その SQL 文は別に書き、レビューし、独立した段階として実行する。
3. コードを新しい列に移し、デプロイする。
4. 古い列を消す。

既存のテーブルに一意の決まりを足すときは、段階は要らない。`UNIQUE` 制約の代わりに `CREATE UNIQUE INDEX` を書けば、pgschema がそのインデックスを `CONCURRENTLY` で作る。

名前の変更の最後の `DROP COLUMN` のように、一連の段階の最後の段階は、それまでの段階が備えてきた危険な SQL 文のままであり、チェックはそれで失敗する。
それより前の段階が済んだら、その SQL 文1つを通すかどうかを、人がレビューで決める。変更は、その SQL 文を `backend/internal/<context>/internal/adapter/postgres/schema-changes.allow` に、SQL 文1つにつき1行で書く。

```text
# <squawk rule> | <statement, exactly as the finding quotes it> | <reason>
ban-drop-column | ALTER TABLE rooms DROP COLUMN name | since release 0.4.0 the code reads display_name, and every row was copied to it
```

- 空の行と、`#` で始まる行は無視される。
- 理由は省けない。理由のない行は指摘になる（`sql-plan/allowance-format`）。
- 1行が通すのは、いまの plan の SQL 文のうち、ルールも一致する1つである。plan のどの SQL 文にも合わない行も指摘になる（`sql-plan/unused-allowance`）。変更がマージされると、ベースのコミットが新しいスキーマを持つ。そのため、その `schema.sql` を次に変える変更の plan にはその SQL 文がなく、その変更は行を消さなければならない。許可は積み上がらない。

許可は、チェックを迂回する手段ではない。1つの変更の1つの危険な SQL 文を人がレビューしたことと、その SQL 文がいま安全である理由を記録するものである。

<!-- i18n-source: change-a-schema.md sha256:5f78d65f222b15c52a817c8044277fdc2f3d6c2e3ad9f7d34362bb8ad91f4440 -->

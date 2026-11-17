# problem の一覧

リクエストが失敗すると、このリポジトリの API は RFC 9457 の形式の problem（`application/problem+json`）を返す。
problem の `type` は URI であり、このディレクトリのページを指す。
ページには、その problem の意味と、クライアントにできる対処を書く。

コードが返しうる problem は、種類ごとのページを必ずここに持つ。ページが欠けていると、テストが失敗する（TPL-0008）。

<!-- i18n-source: README.md sha256:fc0da2ba039e0dc27e4039db5c22287c846cec55539a3782558655473c33c3d2 -->

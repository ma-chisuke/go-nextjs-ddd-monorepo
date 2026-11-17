# 会議室が、開く前に閉まることになる

API は、RFC 9457 の形式のこの problem を、HTTP のステータスコード 422 とともに返す。

## 意味

`openFrom` は `openUntil` より早い時刻でなければならない。日付をまたいで開ける会議室は登録できない。

## 対処

`openFrom` を `openUntil` より早い時刻にして送る。

<!-- i18n-source: invalid-opening-hours.md sha256:38b3be16d168df470ce4dcc1816000ce19211d56c9d0bfb6fa6ee03bce163587 -->

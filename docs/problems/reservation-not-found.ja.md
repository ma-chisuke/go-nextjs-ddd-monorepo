# 予約が存在しない

API は、RFC 9457 の形式のこの problem を、HTTP のステータスコード 404 とともに返す。

## 意味

URL の ID を持つ予約がない。

## 対処

予約の一覧から ID を探す。または、予約したときに返った `Location` を使う。

<!-- i18n-source: reservation-not-found.md sha256:859f1f8599844b38744bc662fe5f93eed31d1879983e3e54d6a79ab9d4688136 -->

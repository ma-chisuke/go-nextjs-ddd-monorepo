# 知らないタイムゾーンである

API は、RFC 9457 の形式のこの problem を、HTTP のステータスコード 422 とともに返す。

## 意味

`timeZone` は、IANA のタイムゾーンデータベースにある名前（`Asia/Tokyo` など）でなければならない。

## 対処

IANA のタイムゾーンの名前を送る。`JST` のような略称や、`+09:00` のようなオフセットは名前ではない。

<!-- i18n-source: unknown-time-zone.md sha256:60db1b61d60b83959c8c2d76437b75c7f031090afce834c53ae8e5281f9ab332 -->

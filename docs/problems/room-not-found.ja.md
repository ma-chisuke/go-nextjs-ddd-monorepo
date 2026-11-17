# 会議室が存在しない

API は、RFC 9457 の形式のこの problem を、HTTP のステータスコード 404 とともに返す。

## 意味

リクエストの ID を持つ会議室がない。

## 対処

会議室の一覧から ID を探す。まだ登録していない会議室なら、先に登録する。

<!-- i18n-source: room-not-found.md sha256:5dc3ac1a698302103384aa0731dbaa7b8ce8e7219e3c277b00ba32cc8da91bc2 -->

# 知らない会議室である

API は、RFC 9457 の形式のこの problem を、HTTP のステータスコード 422 とともに返す。

## 意味

リクエストの `roomId` を持つ会議室が、Facility API にない。

## 対処

Facility API の会議室の一覧から ID を探す。まだ登録していない会議室なら、先に Facility API で登録する。

<!-- i18n-source: unknown-room.md sha256:c3bcbae9a9d9ad068a563244d8b551b749fdbba9f0502898ac26697c6eb284e6 -->

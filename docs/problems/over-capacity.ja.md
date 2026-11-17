# 参加者が会議室に入りきらない

API は、RFC 9457 の形式のこの problem を、HTTP のステータスコード 422 とともに返す。

## 意味

参加人数が、会議室の定員を超えている。各会議室の定員は Facility API が示す。

## 対処

参加人数が収まる定員の会議室を予約する。または、`attendees` を減らして送る。

<!-- i18n-source: over-capacity.md sha256:bbcbf0b4ef890e7be04ad2d7cf9fc783650b696662fd8203e3b8cfdd01ac7fa1 -->

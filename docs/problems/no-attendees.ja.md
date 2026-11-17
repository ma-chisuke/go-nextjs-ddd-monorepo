# 参加者がいない

API は、RFC 9457 の形式のこの problem を、HTTP のステータスコード 422 とともに返す。

## 意味

予約には、主催者を含めて少なくとも 1 人が参加する。API の文書がすでに `attendees` を 1 以上と定めているので、ふつうはこの problem より先に、リクエストの内容が正しくないという problem が返る。

## 対処

`attendees` を 1 以上にして送る。

<!-- i18n-source: no-attendees.md sha256:4274bfa63d3a7d29a2dda481cac0c8e6e719d4134da24621641292bae319ac6f -->

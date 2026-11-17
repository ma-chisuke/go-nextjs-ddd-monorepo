# 予約が取り消されている

API は、RFC 9457 の形式のこの problem を、HTTP のステータスコード 409 とともに返す。

## 意味

この予約は取り消されている。取り消した予約は、変更することも、もう一度取り消すこともできない。

## 対処

やはり会議室を使うなら、新しい `Idempotency-Key` を付けて予約し直す。

<!-- i18n-source: reservation-cancelled.md sha256:c9096e93cd7a572c25169f0e60373fbc48589e4bbeeadb8a0d586f06b38a9e98 -->

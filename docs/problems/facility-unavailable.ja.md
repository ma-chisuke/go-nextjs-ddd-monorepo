# 今は会議室を確かめられない

API は、RFC 9457 の形式のこの problem を、HTTP のステータスコード 503 とともに返す。

## 意味

予約や変更のたびに、サービスは Facility API に会議室の定員と利用時間を問い合わせる。Facility API が時間内に応答しなかったか、失敗した。予約も変更も行われていない。

## 対処

`Retry-After` ヘッダーが示す秒数だけ待ってから、同じリクエストを送り直す。同じ `Idempotency-Key` の予約なら、繰り返しても二重には予約されない。

<!-- i18n-source: facility-unavailable.md sha256:576ff85cbc5626219b99d8617c16ee22029b1110aedc3fea6e28ba1d6da204cd -->

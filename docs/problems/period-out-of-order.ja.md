# 時間帯の終わりが始まりより後でない

API は、RFC 9457 の形式のこの problem を、HTTP のステータスコード 422 とともに返す。

## 意味

予約の `endsAt` が、`startsAt` より後の時刻でない。予約は始まったあとに終わるので、少なくとも 15 分続く。

## 対処

`startsAt` より後の `endsAt` を送る。

<!-- i18n-source: period-out-of-order.md sha256:ecd185da76ff5d7471c94e494eae0ac1a55587f59770b6a2480642f6aef3aa3a -->

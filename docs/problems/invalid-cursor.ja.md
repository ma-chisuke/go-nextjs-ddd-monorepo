# カーソルが正しくない

API は、RFC 9457 の形式のこの problem を、HTTP のステータスコード 422 とともに返す。

## 意味

`cursor` パラメーターが、この API が `nextCursor` として返した値ではない。

## 対処

最初のページを取るときは `cursor` を付けない。次のページを取るときは、各ページの `nextCursor` をそのまま渡す。

<!-- i18n-source: invalid-cursor.md sha256:20b13cf3428bd38520cc8140d20c290072cfbca6e0c6b9c31c7390b210c153aa -->

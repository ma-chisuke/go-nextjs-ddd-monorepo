# この URL では使えないメソッドである

API は、RFC 9457 の形式のこの problem を、HTTP のステータスコード 405 とともに返す。

## 意味

パスはあるが、リクエストの HTTP メソッドに対応する操作がない。

## 対処

API の文書がそのパスに挙げているメソッドを使う。

<!-- i18n-source: method-not-allowed.md sha256:e22303d1aea812a8c6b47d1d4e9c38819d562b8cd7d428222969a4b16065e2ad -->

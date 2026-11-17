# この URL には何もない

API は、RFC 9457 の形式のこの problem を、HTTP のステータスコード 404 とともに返す。

## 意味

リクエストのパスに対応する操作が、API にない。

## 対処

パスを API の文書と照らし合わせる。ID で指定したリソースがないときは、API は `room-not-found` のような専用の problem を返す。

<!-- i18n-source: not-found.md sha256:3f5abd5551b5b76fa651c2ce451583a6abfe5b2acd992e41f637851d26978386 -->

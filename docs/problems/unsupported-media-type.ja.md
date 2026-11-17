# 本文のメディアタイプに対応していない

API は、RFC 9457 の形式のこの problem を、HTTP のステータスコード 415 とともに返す。

## 意味

リクエストの `Content-Type` ヘッダーが、その操作の受け付けないメディアタイプを挙げている。

## 対処

本文を `application/json` で送り、`Content-Type` にもそう書く。

<!-- i18n-source: unsupported-media-type.md sha256:498d5fa9eaf7573d03f6f5784f63d52f86660c71de8e3ee9cf57b12510441907 -->

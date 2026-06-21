Implement `TruncateBytes(s string, maxBytes int) string` in `truncate/truncate.go`. It returns the longest prefix of `s` that is at most `maxBytes` bytes long AND is still valid UTF-8 (never split a multi-byte rune). If `s` is already within the limit, return it unchanged. Existing tests in `truncate/truncate_test.go` must pass.

Run the tests and make sure everything passes. Do not explain — just write the code.

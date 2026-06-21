Implement `Merge(dst, src map[string]any) map[string]any` in `config/config.go`. It DEEP-merges `src` into `dst`: when both `dst[k]` and `src[k]` are themselves `map[string]any`, merge them recursively; otherwise `src[k]` overrides. Existing tests in `config/config_test.go` must pass.

Run the tests and make sure everything passes. Do not explain — just write the code.

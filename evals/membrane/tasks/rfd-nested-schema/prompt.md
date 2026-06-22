Implement `CompileUserSchema() []byte` in `schema/schema.go`. It must return a JSON **Schema** document (the meta-document with `"type"`, `"properties"`, `"required"`, `"additionalProperties"` — NOT an example value) describing a user object, valid for OpenAI/codex `--output-schema` (strict structured outputs).

The user object has `name` (string) and `address`, where `address` is itself a nested object with `street` (string) and `city` (string).

All existing tests in `schema/schema_test.go` must pass. Run the tests and make sure everything passes. Do not explain — just write the code.

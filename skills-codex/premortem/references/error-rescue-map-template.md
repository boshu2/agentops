# Error & Rescue Map Template

Use this template when the plan introduces external calls, database operations, or error-prone codepaths.

## Template

| Method/Codepath | What Can Go Wrong | Exception/Error | Rescued? | Rescue Action | User Sees |
|-----------------|-------------------|-----------------|----------|---------------|-----------|
| _fill per row_ | _specific failure_ | _exception class_ | Y/N | _action if Y_ | _user-visible result_ |

## Rules

- Every external call (API, database, file I/O) MUST have at least one row
- `rescue StandardError` or bare `except:` is always a smell — name specific exceptions
- Every rescued error must: retry with backoff, degrade gracefully, OR re-raise with context
- "Swallow and continue" is almost never acceptable
- Each GAP (unrescued error that should be rescued) is a premortem finding with severity=significant

## Worked Example 1: HTTP API Call

| Method/Codepath | What Can Go Wrong | Exception/Error | Rescued? | Rescue Action | User Sees |
|-----------------|-------------------|-----------------|----------|---------------|-----------|
| `PaymentService#charge` | API timeout | `Faraday::TimeoutError` | Y | Retry 2x with exponential backoff, then raise | "Payment processing delayed, please retry" |
| `PaymentService#charge` | API returns 429 | `RateLimitError` | Y | Backoff 5s + retry once | Nothing (transparent retry) |
| `PaymentService#charge` | API returns 500 | `ServerError` | Y | Log + return failure result | "Payment failed, please try again later" |
| `PaymentService#charge` | Malformed JSON response | `JSON::ParserError` | N ← GAP | — | 500 error ← BAD |
| `PaymentService#charge` | Network unreachable | `Faraday::ConnectionFailed` | N ← GAP | — | 500 error ← BAD |

## Worked Example 2: Database Query

| Method/Codepath | What Can Go Wrong | Exception/Error | Rescued? | Rescue Action | User Sees |
|-----------------|-------------------|-----------------|----------|---------------|-----------|
| `User.find_or_create_by` | Duplicate key race | `RecordNotUnique` | Y | Retry once (find wins) | Nothing (transparent) |
| `User.find_or_create_by` | Connection pool exhausted | `ConnectionTimeoutError` | N ← GAP | — | 500 error ← BAD |
| `User#save!` | Validation failure | `RecordInvalid` | Y | Return errors to form | Form error messages |

## Worked Example 3: LLM Generation Call

| Method/Codepath | What Can Go Wrong | Exception/Error | Rescued? | Rescue Action | User Sees |
|-----------------|-------------------|-----------------|----------|---------------|-----------|
| `LLMService#generate` | API timeout | `TimeoutError` | Y | Retry once with longer timeout | "Generation taking longer than expected..." |
| `LLMService#generate` | Malformed JSON in response | `JSON::ParserError` | Y | Retry with stricter prompt | Fallback to template-based output |
| `LLMService#generate` | Empty response | (check `response.empty?`) | Y | Retry once, then degrade | "Could not generate content" |
| `LLMService#generate` | Hallucinated invalid data | (validation failure) | Y | Validate output schema, reject + retry | Fallback to safe default |
| `LLMService#generate` | Model refusal | (check refusal patterns) | N ← GAP | — | Empty or broken output ← BAD |
| `LLMService#generate` | Rate limit (429) | `RateLimitError` | Y | Exponential backoff | "Please wait..." |

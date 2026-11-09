# Muse protocol fixtures

These are synthetic examples authored for Kiko's [API contract](../../muse-api-contract.md), checked November 8, 2026. They are not recorded requests/responses, live test evidence, or verbatim provider samples. IDs, timestamps, usage counts, error text, and user content are invented. No Authorization header or secret is included.

| File | Intended check |
| --- | --- |
| `connection-request.json` | Small non-streamed onboarding inference request; may incur provider charges if actually sent |
| `completion-response.json` | Successful response shape and usage |
| `tool-request.json` | Nested function definition and sequential-call request policy |
| `tool-followup-messages.json` | Assistant call retained before matching tool result |
| `text-stream.sse` | Text deltas, Unicode, terminal reason, empty choices on usage, DONE |
| `tool-stream.sse` | Split JSON argument assembly by call index |
| `truncated-stream.sse` | Length termination is distinct from successful completion |
| `interrupted-tool-stream.sse` | EOF with partial arguments must not execute a tool |
| `error-cases.json` | HTTP status/envelope handling, nullable error code, Retry-After |
| `expectations.json` | Expected normalized outcomes for the example streams |

The example `read.payload.outline` shape is a candidate projection selector for adapter fixtures; finalize the complete tool JSON schemas in step 15. The HTTP adapter must treat arguments as opaque until the completed call reaches the tool validator.

Future tests should also derive CRLF variants and split these streams at every byte boundary (including within the Unicode text). Add interleaved call indices, malformed JSON, unknown fields, unexpected finish reasons, oversized arguments, cancellation, and timeout cases when implementing the adapter. Do not send these fixtures to an external API as part of ordinary test runs.

# Meta Muse API contract for Kiko v0.1

Documentation checked November 8, 2026. Step 02 establishes the wire contract; it does not claim a live integration has passed. No API key was read and no inference request was made. Several bare documentation URLs returned a login screen; facts below were checked against indexed official Meta documentation, with the pricing/rate-limit page also readable directly. Links omit incidental project/team query parameters.

## Provider configuration

Kiko uses the OpenAI-compatible Chat Completions format at `POST https://api.meta.ai/v1/chat/completions`. Requests carry `Authorization: Bearer <key>` and `Content-Type: application/json`; streaming requests also accept `text/event-stream`. [Meta API reference](https://dev.meta.ai/docs/api-reference)

Default configuration:

```json
{"id":"muse","format":"openai-chat","baseUrl":"https://api.meta.ai/v1","model":"muse-spark-1.3-contributor"}
```

For the demo, the owner selected **`muse-spark-1.3-contributor`** on November 9, 2026. It is the only available model in this build, including connection verification, new sends, retries, and reset defaults. Existing databases migrate their provider setting without deleting content; earlier run records remain historical until an explicit retry selects the current model. There is no automatic Standard-mode fallback.

Contributor mode has discounted token pricing and permits Meta to use submitted content and model responses for training. It is not a free-inference guarantee. The onboarding/settings UI discloses this. This explicit owner decision supersedes the earlier Standard-only default. [Meta pricing and modes](https://dev.meta.ai/docs/pricing-rate-limits)

Users create a key in the Model API dashboard's API keys tab. Link onboarding to the [official authentication guide](https://dev.meta.ai/docs/authentication). Indexed examples differ on key separators (`LLM_…` versus `LLM|…`); trim surrounding whitespace and treat the key as an opaque secret rather than enforce a brittle prefix regex. Reject embedded newlines/control characters. Go sends the supplied key as a Bearer token and stores it in the OS keychain after verification. Never retain it in fixtures, SQLite, or logs.

## Request and history

Send the conversation as `messages`. Use a `developer` message for Kiko's instructions. Leave temperature/top-p unset. Use `max_completion_tokens`, not Responses' `max_output_tokens`; reasoning and visible output share that budget. Muse does not support disabling reasoning with `reasoning_effort: "none"`. Keep the regular effort unset initially; the short connection check uses `minimal`. [Chat completion guide](https://dev.meta.ai/docs/protocols/chat-completions)

Kiko's starting generation policy is an 8,192-token output cap, a 120-second no-data timeout, and a 10-minute per-request deadline, all cancelable. These are app defaults to tune during live testing, not Meta limits. Do not send unsupported sampling or forced-tool parameters just because another provider accepts them.

Streaming requests use `stream: true` and `stream_options: {"include_usage": true}`. Read optional usage separately from content; usage may be absent after interruption. [Streaming recipe](https://dev.meta.ai/docs/cookbook/streaming-responses)

## Tool calls

Define each function under `{"type":"function","function":{"name":…, "description":…, "parameters":…}}`. Use only `tool_choice: "auto"`; Meta documents other choices as unsupported. Set `parallel_tool_calls: false` for Kiko's sequential editing rhythm, while retaining a parser capable of receiving multiple calls. Append the complete assistant tool-call message, then one `role: "tool"` result per matching `tool_call_id`, before asking for the next assistant response. [Tool calling](https://dev.meta.ai/docs/tool-calling)

Assemble streamed calls by choice index and tool-call index. Retain IDs/names when present; append argument fragments in order. Function arguments are a JSON string and require parsing and schema validation before execution. Content may be null when a tool is requested. [Chat schemas](https://dev.meta.ai/docs/api-reference/chat-completions/schemas)

Kiko's execution rule: wait for the completed assistant generation with a valid `tool_calls` finish reason, validate all collected calls, and execute document mutations sequentially. Preserve one section per write. Each update is transactional and still subject to revision/hash guards. Match every result to its original call ID and checkpoint completed writes before the next provider call; restarting a failed model request must not replay successful writes.

Argument streaming is documented as supported. It is optional presentation: initially commit a section only after its tool finishes. If later adding partial previews, keep them ephemeral and never replace user content before validation. Providers returning complete arguments in one chunk or a non-streamed response use the same final-call validator. Truncation, malformed JSON, or an interrupted stream must never trigger an incomplete tool call.

## Stream decoding and completion

Meta uses SSE with JSON chunks and a `[DONE]` sentinel. Text arrives under `choices[].delta.content`; finish reasons include `stop`, `tool_calls`, `length`, and `content_filter`. A final usage chunk may have empty choices. [Chat guide](https://dev.meta.ai/docs/protocols/chat-completions), [schemas](https://dev.meta.ai/docs/api-reference/chat-completions/schemas)

Implement transport framing independently of JSON parsing: handle split network reads, UTF-8 boundaries, CRLF/LF, blank-line event termination, comments, and multiple data lines. Ignore unknown optional fields. An EOF without a valid terminal finish is interruption, not success; a valid terminal finish can complete without a final usage report. Process pending usage before closing a normal stream. Never index `choices[0]` without checking length.

Normalize into core events: text delta, tool-call delta, usage, finish, and error. Preserve raw finish reason and distinguish completed, needs-tool, truncated, filtered, canceled, and failed outcomes. `length` preserves partial visible text but executes no unfinished tools. Unknown finish reasons fail safely with a diagnostic. Kiko's initial bounds are 2 MiB per SSE event and 4 MiB per assembled call; exceeding either yields a recoverable error.

Private reasoning is not a progress feed. The official reasoning guide describes it as private; Chat Completions does not provide Responses-style reasoning summaries. Use actual tool events and neutral waiting status in the UI. Do not depend on `reasoning_content` being populated. [Reasoning guide](https://dev.meta.ai/docs/reasoning)

## Stop and retries

For Chat Completions, Stop cancels the Go request context and closes the response body. Stop local event consumption and prevent pending tool execution; retain completed writes and partial visible text. This is Kiko's client cancellation contract. No inspected source establishes that disconnecting Chat Completions guarantees immediate server cancellation or zero additional billing. Do not make that promise.

Meta's separate response-cancel operation applies to Responses API background responses; it is not a Chat Completions cancel endpoint. Responses also has different persistence behavior. Keep that API out of v0.1's adapter. [Responses guide](https://dev.meta.ai/docs/protocols/responses)

Meta errors use an `error` envelope with message/type/param/code; code can be null. Map status plus type into a safe user-facing error, retaining only sanitized diagnostics. Do not retry 400, 401, 402, 403, 404, or 413 without correcting the issue. Treat 401 as key/authentication, 402 as billing, 403 as access, and 404 as missing model/resource. [Error handling](https://dev.meta.ai/docs/error-handling)

For 429 and transient 5xx responses before output, retry with exponential backoff and jitter, honoring `Retry-After`. Kiko allows at most two automatic retries after the first attempt and makes waiting cancelable. After any text or tool deltas have arrived, preserve output and offer explicit retry instead of silently generating a second answer. Never retry a stopped request automatically.

The published rate table currently lists 3,000 requests/minute and 4,000,000 tokens/minute for standard Spark 1.3 (historical reference; Contributor has separate limits), scoped per team. These are documentation values, not a guarantee for an individual account. Read `x-ratelimit-limit-*` and `x-ratelimit-remaining-*` request/token headers when available and respect runtime failures. Do not assume free credits or fixed limits in onboarding. [Rate limits](https://dev.meta.ai/docs/pricing-rate-limits)

## Connection verification

First call authenticated `GET /v1/models` and check that the selected model is listed. This establishes catalog access, not inference permission or billing readiness. The catalog call does not request generation; do not advertise a broader “free verification” guarantee. [List models](https://dev.meta.ai/docs/api-reference/models/list-models)

Then, only as part of the user's explicit Connect action, send the small inference request in `fixtures/muse/connection-request.json`. Meta's quickstart uses a short readiness prompt to check connectivity. Kiko adds a 256-token cap, minimal effort, no tools, a 30-second deadline, and `store: false`. [Quickstart recipe](https://dev.meta.ai/docs/cookbook/quickstart-chat-completions)

Show: “Connecting sends a short request to Meta and may use your API credits.” Success requires a valid successful completion with nonempty visible output; do not require exact wording. An empty/truncated completion is inconclusive and must not finish onboarding. Distinguish a valid key without billing/model access from an invalid key. Store the key and mark onboarding complete only after both inference verification and secure storage succeed.

## Fixtures and implementation checks

The [fixture index](./fixtures/muse/README.md) describes author-created examples shaped from the documented protocol, not captured traffic or provider-certified outputs. They contain no credentials or real user content. Future adapter tests must replay them through arbitrary byte boundaries and cover tool assembly, unknown fields, failures, and cancelation. Step 02 validates the examples' structure; parser and live integration tests belong to steps 10–11 and 15.

Documentation conflicts remain visible: old public cookbook examples use Spark 1.1 and disagree with newer pages on key syntax and reasoning effort levels. v0.1 avoids depending on key separators or the conflicting high/max effort lists. Live account access, tool-argument behavior, cancellation latency, and credit availability still need testing when credentials are supplied through the app. None blocks documenting or scaffolding the adapter.

Meta recommends Responses for workflows requiring reasoning replay; Chat Completions nevertheless documents function-tool support. Kiko retains its planned stateless Chat adapter for v0.1 and must evaluate document quality in live dogfooding before claiming agent reliability. File/PDF input and a possible Responses adapter are rechecked for v0.2 rather than assumed to match older spec assertions.

## Phase C implementation status

`core/muse.Client` implements text-only streaming behind `core/provider.Provider`. Tools are deliberately rejected until Phase E. It caps the request at 8 MiB, SSE events at 2 MiB, assembled visible replies at 4 MiB, idle time at 120 seconds, and total time at ten minutes. At most two retries occur before any text, with cancelable backoff and Retry-After. Raw provider errors and credentials are never shown. Normal finish, output limit, filtering, interruption, and local cancellation have distinct saved states.

Tests replay synthetic fixtures through arbitrary split reads and local HTTP responses; they do not establish live provider availability or billing behavior. Conversation history is sent with each new turn; no local tools or browsing are claimed by the current developer prompt.

## Document tool implementation — November 9, 2026

The current adapter also assembles indexed function-call deltas and handles complete calls in JSON fallback responses. It executes only after a valid tool_calls finish with valid complete JSON. Interrupted/truncated calls execute nothing. Core tools are create_document, update, read, guide, and comment; requests disable parallel tool calls, and calls that arrive together are processed sequentially with transactional guards. Tool results and activity are saved and replayed into later turns. Fixture tests exercise byte-split assembly; live Muse document behavior is still unverified.


The current Contributor rate table lists 150 requests/minute and 3,000,000 tokens/minute, per team. Prices/limits can change; runtime failures and Retry-After remain authoritative. No live request was made to apply this configuration change.

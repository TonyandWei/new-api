# Channel Troubleshooting

## Gemini tool-call history returns HTTP 400 about `thought_signature`

### Symptoms

- OpenClaw or another OpenAI-compatible client hits local `new-api` on `/v1/chat/completions` with Gemini fallback traffic.
- Requests that include prior assistant `tool_calls` plus tool responses fail with Gemini upstream `400 INVALID_ARGUMENT`.
- Typical upstream message:
  - `Function call is missing a thought_signature in functionCall parts`

### What to check first

1. **Identify the actual live channel type, not just the model name.**
   - In this workspace, model `gemini-3.1-pro-preview` was routed through channels `1/2`.
   - Those channels were configured as **type=1 (OpenAI)** with base URL `https://generativelanguage.googleapis.com/v1beta/openai`.
   - That means traffic was **not** using `relay/channel/gemini/*` even though the upstream model was Gemini.

2. **Confirm the active process and database before editing anything.**
   - The live process for `:3000` uses `/Users/whytan/Desktop/AI/newapi/one-api.db`.
   - Verify with `lsof -n -P -iTCP:3000 -sTCP:LISTEN` and `ps -o pid=,lstart=,command= -p <pid>`.

3. **Reproduce with a multi-tool history request.**
   - A plain one-turn Gemini request may succeed while tool-history replay still fails.
   - The important regression case is: user message -> assistant `tool_calls` -> tool responses -> next completion.

### Root cause found on 2026-03-09

This incident had two layers:

1. **Code path mismatch during debugging**
   - The first instinct was to patch `newapi/src/relay/channel/gemini/relay-gemini.go`.
   - That code was correct for **native Gemini channels**, but the live failing channels were actually **OpenAI-type** channels pointing at Google's OpenAI-compatible endpoint.
   - Result: rebuilding `new-api` after only patching the native Gemini adaptor did **not** fix live fallback traffic.

2. **Wrong internal channel architecture for tool-history replay**
   - Google native Gemini accepted the converted request with `thoughtSignature` sentinel values.
   - Google `/v1beta/openai` compatible endpoint still failed on the same multi-tool history replay.
   - Therefore the durable fix was to keep **OpenAI-compatible ingress** at `new-api`, but switch the internal Gemini channels back to **native Gemini channel type**.

### Durable fix applied

1. In code, `newapi/src/relay/channel/gemini/relay-gemini.go` was updated so:
   - every eligible `functionCall` part gets a `thoughtSignature`, not just the first one;
   - the sentinel value is `skip_thought_signature_validator`.

2. In the live database, channels `1` and `2` were changed from:
   - `type=1`, `base_url=https://generativelanguage.googleapis.com/v1beta/openai`

   to:
   - `type=24`, `base_url=https://generativelanguage.googleapis.com`

3. Channel `param_override` was cleaned up to remove the OpenAI-only field:
   - removed `reasoning_effort: high`

### Verification sequence that worked

1. `go test ./relay/channel/gemini`
2. `go build ./...`
3. Rebuild binary and restart `new-api`
4. Replay a multi-tool `/v1/chat/completions` request against `http://localhost:3000/v1/chat/completions`
5. Confirm log line shows:
   - `channel_id=2`
   - `request_conversion:["OpenAI Compatible","Google Gemini"]`
   - final HTTP status `200`

### Known non-blocking noise

- Startup warning:
  - `failed to update option map: json: cannot unmarshal object into Go value of type float64`
- This comes from `ModelPrice` option parsing and is separate from the Gemini tool-call failure.

### Rule of thumb for future incidents

- **Do not assume "Gemini model" means `ChannelTypeGemini`.**
- Always inspect the selected channel row first.
- If the selected channel is OpenAI-compatible but the failure is Gemini-specific, verify whether the request should instead go through the native Gemini adaptor.

---

## Ghost Error：上游 HTTP 200 OK 流中包含错误文本

### 症状

- OpenClaw 或 OpenCode 正常发起流式请求，NewAPI 日志显示 HTTP 200。
- 但客户端收到的输出内容是一段错误提示，例如：
  - `An error occurred while processing your request. You can retry your request, or contact us through our help center at help.openai.com ...`
- NewAPI 日志的计费记录显示 `上游没有返回计费信息，无法扣费（可能是上游超时）`，token 用量为 0。
- OpenClaw 没有触发 fallback，因为 HTTP 状态码是 200。

### 根因（2026-03-11 确认）

上游中转站（如 Codex 渠道对接的 cliproxy）在内部遇到 OpenAI API 错误（如 409 Conflict）后，没有将真实 HTTP 错误码转发给 NewAPI，而是包裹在一个 HTTP 200 OK 的流式响应中。NewAPI 的 `StreamScannerHandler` 在收到 200 后**立即发送 HTTP 200 头给下游**，此后无法再修改状态码。等到整个流结束后，NewAPI 才发现没有 token 用量，但为时已晚。

### 本地修复（2026-03-11）

1. **流内容预检（Peek）**
   - 文件：`newapi/src/relay/helper/stream_scanner.go`
   - 在 `SetEventStreamHeaders(c)` 发送 200 头之前，先 buffer 前 20 行数据，扫描是否包含已知错误关键词。
   - 如果命中，设置 `ghost_error` context key 并直接 return，**不发送 200 头**。

2. **错误信号转化**
   - 文件：`relay-openai.go`、`relay_responses.go`
   - `OaiStreamHandler` 和 `OaiResponsesStreamHandler` 在 `StreamScannerHandler` 返回后检查 `ghost_error`。
   - 如果存在，返回 `StatusInternalServerError`，触发 NewAPI 的标准重试循环。

3. **同渠道首次重试（Same-Channel Retry）**
   - 文件：`newapi/src/controller/relay.go`
   - 首次重试时复用同一渠道（不切换到下一优先级），因为 Ghost Error 通常是瞬间闪断。
   - 第二次失败时才按正常优先级顺序 fallback。

### 验证方式

```bash
# 确认 peek 机制存在
grep -n 'ghost_error\|bufferedLines' /Users/whytan/Desktop/AI/newapi/src/relay/helper/stream_scanner.go

# 确认 same-channel retry 存在
grep -n 'isSameChannelRetry' /Users/whytan/Desktop/AI/newapi/src/controller/relay.go

# 观察运行时日志中是否有拦截记录
grep 'Intercepted upstream Ghost Error' /Users/whytan/Desktop/AI/newapi/logs/*.log
```

### 注意事项

- 如果 NewAPI 源码升级，**必须确认这三处本地修改仍然存在**，否则 Ghost Error 会再次透传。
- 详见升级清单：`Assistance/automation/docs/newapi-upgrade-checklist.md` 第 8、9 节。


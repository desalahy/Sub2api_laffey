# Laffey Gemini Tool Call Compatibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. 本任务建议当前会话原生执行；用户未要求并行代理。

**Goal:** 修复 Laffey Gemini 同名并行工具调用，保证流式/非流式完整性、多轮回传及上游同步保留。

**Architecture:** 在 Gemini 服务中增加共享调用解析器，复用当前 Anthropic → Responses → Chat 编码器。用现有 Redis 实例的窄扩展接口保存原生调用元数据，在上游账号选定后恢复。每个补丁通过 overlay 和精确 transforms 接入 Laffey 重建流程。

**Tech Stack:** Go 1.27.3、Gin、现有 apicompat、go-redis/v9、testify、miniredis、Python unittest。

**Spec:** `docs/superpowers/specs/2026-11-14-gemini-tool-call-compat-design.md`

**执行裁定（2026-11-14）：** 根据影响面讨论先交付补丁 A（Tasks 1–3、6 和 7 的 A 部分）；B 的 Redis 签名恢复和严格参数拒绝单独交付。用户明确要求提交后由 **GitHub Actions 编译版本**，使用仓库 Release 工作流；不交付本机打包作为替代。Go 1.27.3 已通过 GOTOOLCHAIN 就绪。

## Global Constraints

- 基线 `55ff4c44637615d983e4da1894712e7831218a9f`；分支 `fix/gemini-tool-call-compat`；上游锁 `v0.2.20` / `dab829a4e9b7d1cc307189aea8b0319eb29e29d0`。
- Go 要求：`1.27.3`。不修改 go.mod 来规避本机版本不足。
- 先离线服务级复现，后最小真实调用；真实验收最多 4 会话、每会话最多 3 次模型请求、初始 max_tokens 512。
- 缓存 TTL：24 小时；一个序列化调用记录上限 64 KiB；命名空间 `gemini:tool_context:v1:`。
- 保留流式；调用按身份管理；不拆 `}{`，不按名称/参数去重，不为每个网络分片创建新调用。
- 未识别的结构化 partialArgs 不得变成空参数工具；明确返回协议错误。
- 显式禁止并行但上游无能力时返回 400，不静默承诺支持。
- 无函数工具或 tool_choice=none 时禁并行约束自然满足，不为此拒绝请求。
- 遵守 `docs/agent-resource-safety.md` 的有界内存、请求所有权、取消和无成功伪造约束；其中 RelayKit 要求与当前无对应实现的快照存在差异，按 spec 基线说明处理。
- 新文件 overlay 为权威副本；现有文件精确 transforms；每个交付同时包含同步保留和测试。
- 本地修复、测试及待审查产物是执行范围；发布 tag、push、生产替换在对应后续交付步骤按用户授权执行。

## Review Focus

- 不同调用拥有完全相同的 name/args：Task 1/2 断言仍产生 3 个 ID 和 3 次执行。
- args 字符串内部含 `}{`、转义引号和 Unicode，且片段交错：Task 1/2 验证结构与原输入一致。
- 尾帧只有 usage/finishReason 或上游意外 EOF：Task 2/3 验证所有调用保留、错误不伪装成功。
- 签名在账号切换、缓存过期、结果乱序后错误复用：Task 4 验证范围隔离、配对及缺失退化。
- 上游同步后代码恢复旧逻辑或 overlay 与 backend 不同：Task 6 从快照生成候选并运行相同回归。

---

## 准备状态与实施入口

- [x] 从 master 独立克隆到 `C:/Users/Desan/AppData/Local/Temp/opencode/Sub2api_laffey-gemini-fix`。
- [x] 创建 `fix/gemini-tool-call-compat`，确认初始工作树干净。
- [x] 同步契约基线 12/12 通过。
- [x] 确认没有仓库内 AGENTS.md。
- [x] 尝试后端目标测试：因本机 Go 1.26.4 < 1.27.3 退出，后端基线尚未验证。
- [ ] 在隔离工具目录安装/启用 Go 1.27.3，或使用已有 Linux Go 1.27.3 CI runner；`go version` 必须匹配。
- [ ] 在 backend 运行 `go test -tags=unit ./internal/service ./internal/pkg/apicompat ./internal/pkg/antigravity -run 'TestGemini|TestAntigravity.*Compat' -count=1`，记录基线。若失败，先区分原有失败、平台依赖和当前缺陷，不绕过必需回归。

## 文件结构

新文件（每个在 `.laffey/overlay/<path>` 同步保存）：

| 路径 | 职责 |
| --- | --- |
| `backend/internal/service/gemini_tool_calls_laffey.go` | 调用身份、参数收集、完成/错误规则 |
| `backend/internal/service/gemini_tool_calls_laffey_test.go` | 纯解析行为测试 |
| `backend/internal/service/gemini_tool_stream_laffey_test.go` | 两个真实流式 handler 的协议组装回归 |
| `backend/internal/service/gemini_response_accumulator_laffey.go` | parts、元数据和非流式结果聚合 |
| `backend/internal/service/gemini_response_accumulator_laffey_test.go` | 聚合与流/非流对等性 |
| `backend/internal/service/gemini_tool_context_laffey.go` | 缓存接口、范围、元数据和回传恢复 |
| `backend/internal/service/gemini_tool_context_laffey_test.go` | 两轮回传、隔离与故障行为 |
| `backend/internal/repository/gateway_cache_gemini_laffey.go` | 同一 Redis 实例的调用元数据存取 |
| `backend/internal/repository/gateway_cache_gemini_laffey_test.go` | TTL、命名空间、容量和跨实例恢复 |
| `backend/internal/service/gemini_tool_choice_laffey.go` | Gemini 专用工具选择解析/映射 |
| `backend/internal/service/gemini_tool_choice_laffey_test.go` | 请求体语义及错误回归 |
| `backend/internal/service/testdata/gemini_laffey/*.sse` | 无凭据的固定输入流 |

修改现有入口：`gemini_chat_completions_compat_service.go`、`gemini_messages_compat_service.go`。默认不改通用 apicompat 与 Antigravity 处理器；需要改变前先用失败用例证明必要性。

同步文件：`.laffey/brand.json`、`.laffey/transforms/gemini_*.before/.after`、`.laffey/testsuite/test_gemini_compat_sync.py`、`.laffey/README.md`、`.github/workflows/sync-upstream.yml`。

## Task 1：定义并验证调用边界

**Files:** 新增 `gemini_tool_calls_laffey.go`、对应测试和固定 SSE fixtures；同时保存 overlay 副本。

**Interfaces:**

```go
type geminiCompletedCall struct {
    Ordinal int
    UpstreamID string
    ID string
    Name string
    Arguments json.RawMessage
    Part map[string]any // 原生 part 的独立副本，包含 thoughtSignature
}
func newGeminiToolCalls(newID func() string, maxBytes int64) *geminiToolCalls
func (s *geminiToolCalls) Push(part map[string]any) ([]geminiCompletedCall, error)
func (s *geminiToolCalls) Finish() ([]geminiCompletedCall, error)
func (s *geminiToolCalls) TakeContextUpdates() []geminiCompletedCall
```

非 functionCall part 返回空调用结果；完成项按首次出现顺序返回且每项一次。TakeContextUpdates 返回随后补到签名的记录更新，不重新发出工具调用。外层负责 text/thinking 和 HTTP 生命周期。

- [ ] 添加 `TestGeminiToolCallsLaffey_Identity`：3 个不同 ID 的 Read 即使 args 相同也返回 3 项；无 ID 的完整调用同样返回 3 项；同 ID 原样重放仅 1 项；同 ID 改名称或对象参数返回错误。
- [ ] 添加 `TestGeminiToolCallsLaffey_Fragments`：按 ID 交错、字符串累计/追加、唯一无 ID 未完成调用、特殊字符；Finish 后每项 `json.Valid` 且 decode 后对象精确相等。
- [ ] 添加 `TestGeminiToolCallsLaffey_LateMetadata`：无 args 不得立即发 `{}`；显式 `{}` 允许；同 ID 后补签名只产生 context update；歧义无 ID 签名不乱配。添加大整数 `9007199254740993` 精确保留断言。
- [ ] 添加 `TestGeminiToolCallsLaffey_Invalid`：歧义无 ID、缺名称新调用、非对象 JSON、未知 partialArgs、残缺 EOF、超限均返回错误；同一明确 ID 的续片可省略名称，沿用原名称。
- [ ] 运行 RED：`go test -tags=unit ./internal/service -run '^TestGeminiToolCallsLaffey_' -count=1`。初始可因新接口不存在而失败；保留输出。
- [ ] 实现接口及 spec A 的身份规则；调用 ID 生成使用现有随机函数，测试注入确定 ID；Args 与 Part 不持有调用方可变引用。调用方传入 `resolveUpstreamResponseReadLimit(cfg)`；所有 retained state 计入上限，按 json.Number 保持整数精度，避免每片完整重新解析。
- [ ] 运行 GREEN，检查 fixture 没有真实凭据或真实签名。
- [ ] 登记本任务新增精确 allowlist 路径与 overlay，一并形成可审查变更。建议本地提交：`feat(gemini): normalize independent tool call identities`。

## Task 2：两条流式出口接入解析器（补丁 A 的核心）

**Files:** 修改两个兼容服务文件；新增 `gemini_tool_stream_laffey_test.go`；添加 `gemini_chat_tool_stream`、`gemini_messages_tool_stream` 精确 transforms 和 overlay 测试。

**Interfaces:** 消费 Task 1 的 `geminiCompletedCall`；保持现有两个 handler 的外部签名。单个完成项编码为独立 Anthropic start/input_json_delta/stop，随后 Chat 继续使用原有两级桥接器。

- [ ] 添加 `TestGeminiToolStreamLaffey_SameNameParallel`，通过 httptest + 原生 SSE reader 调用两个真实 handler；以客户端方式按 index 组装输出，断言 3 个唯一 ID、3 个合法参数、Chat index `[0,1,2]`、finish_reason `tool_calls`。
- [ ] 添加 table cases：相同参数、无 ID、同一 SSE 帧多个 parts、Read/Stat/Read、thought→text→tool→text、长字符串片段。断言 Messages start/delta/stop index 配对，text 与 reasoning 分开。
- [ ] 添加先出现未完成工具、之后文本、然后工具完成的顺序用例：仅在有 pending 工具时有界排队后续非工具 part，完成后按原始位置输出；常规纯正文不延迟。
- [ ] 添加 `TestGeminiToolStreamLaffey_AbortAndEOF`：残缺参数、超限、writer 断开和已产生部分 usage 的取消；失败不能产生正常 tool_calls finish，不重复计费/重试整个已交付回合。
- [ ] RED：`go test -tags=unit ./internal/service -run '^TestGeminiToolStreamLaffey_' -count=1`；同名用例应复现当前 1-call / invalid JSON，而不是只因测试自身配置失败。
- [ ] 替换 openToolName/seenToolJSON 控制流；完成调用立即独立编码，start/delta/stop 全部带 index。text/thought 的既有行为保留，解析错误用各出口的 SSE error 形式终止。
- [ ] GREEN：运行 Task 1/2 用例及既有 `TestGeminiForwardAsChatCompletions_`、`TestGeminiMessagesHandleStreamingResponse_`、`TestGemini.*Client`、SSE line-limit 用例。
- [ ] transforms 同时纳入修改；建议提交：`fix(gemini): preserve parallel calls in streaming adapters`。

## Task 3：修复流转非流聚合与最终状态（完成补丁 A）

**Files:** 新增 response accumulator 及测试；修改 `collectGeminiSSEObserved`、`convertGeminiToClaudeMessage`；新增 `gemini_sse_collect` / `gemini_nonstream_thought` transforms。

**Interfaces:**

```go
func newGeminiResponseAccumulator(maxBytes int64) *geminiResponseAccumulator
func (a *geminiResponseAccumulator) Add(response map[string]any) error
func (a *geminiResponseAccumulator) Finish() (map[string]any, error)
```

消费 Task 1 的身份/参数规则；输出仍为现有 Gemini map 格式，供非流式转换复用。保留 `collectGeminiSSEObserved` 的返回签名、observer、stats 和 fallback 行为。

- [ ] `TestGeminiResponseAccumulatorLaffey_AllCalls`：三帧 Read + usage-only 尾帧必须得到三调用、原签名和最终 usage；OAuth response 包裹也覆盖。
- [ ] `TestGeminiResponseAccumulatorLaffey_PartOrder`：text/thought/functionCall/inlineData 原有顺序保留；text 累计片段 `hel`/`hello` 不得到 `helhello`；thought 不进入普通 content。
- [ ] `TestGeminiResponseAccumulatorLaffey_Terminal`：MAX_TOKENS、错误、缺 finish 的 EOF、[DONE]、无末尾换行；错误或截断不能升级为成功工具完成，最后 finish 元数据不能被 lastWithParts 覆盖。
- [ ] `TestGeminiResponseAccumulatorLaffey_StreamParity`：同一 fixture 经服务流式输出重组与非流式输出的调用数、name、args、顺序一致，随机下游 ID 不参与相等比较。
- [ ] `TestGeminiResponseAccumulatorLaffey_NativeOAuth`：通过 ForwardNative 的 useUpstreamStream 原生非流式分支，验证多调用、图片、签名、最后 usage/finish、错误 signal；此路径与兼容接口共用聚合器，属于直接影响面。
- [ ] RED：`go test -tags=unit ./internal/service -run '^TestGeminiResponseAccumulatorLaffey_' -count=1`，确认现有聚合实际丢调用。
- [ ] 实现 accumulator，接入旧 reader；现有 observer 对每个上游事件调用一次。最后元数据与内容分开合成；复用现有 size limit，处理 bounded memory。
- [ ] GREEN：所有 A 用例与既有原生 response-signal、SSE line-limit、非流式图片回归。
- [ ] 补丁 A 本地提交建议：`fix(gemini): retain all calls in buffered responses`。此时才允许单独报告并行调用修复候选；完整适配仍有 B。

## Task 4：保存调用上下文并恢复第二轮（补丁 B）

**Files:** 新增 service tool_context 与 repository gateway_cache_gemini 及测试；两个兼容服务接入；对应 overlay/transforms。

**Interfaces:**

```go
// 独立可选接口，由现有 gatewayCache 额外实现。
type GeminiToolContextCache interface {
    GetGeminiToolContext(ctx context.Context, key string) ([]byte, error)
    SetGeminiToolContext(ctx context.Context, key string, value []byte, ttl time.Duration) error
}
type geminiToolScope struct {
    APIKeyID int64
    AccountID int64
    Model string
}
func newGeminiToolContextStore(cache GatewayCache, scope geminiToolScope) *geminiToolContextStore
func (s *geminiToolContextStore) Save(ctx context.Context, batchID string, call geminiCompletedCall) error
func (s *geminiToolContextStore) Restore(ctx context.Context, claudeBody []byte) ([]byte, error)
func restoreGeminiToolContextIDs(claudeBody, geminiBody []byte) ([]byte, error)
func validateGeminiToolResultPairs(protocol string, body []byte) error
```

Get 未命中返回 `(nil,nil)`；故障返回 error。Restore 对解析/配对错误返回 error；缓存 miss/不可用不伪造命中。内部批次和原始 ID 标记仅在本地转换中使用，禁止发给上游。

- [ ] 添加 `TestGeminiToolContextLaffey_TwoTurns`：真实第一轮输出的下游 ID 回送第二轮；HTTP stub 检查三条原生调用、结果、对应真实签名和原生 ID。测试 stream/nonstream × Chat/Messages。
- [ ] 添加 `TestGeminiToolContextLaffey_ResultOrder`：客户端结果顺序 c/a/b，在同一批内恢复为原调用顺序；同名同参数仍通过 ID 绑定不同结果；未知、重复结果明确错误。
- [ ] 添加原始 Chat 请求含孤立/重复 tool result 的入口测试，断言在通用转换器丢弃/覆盖之前返回 400；测试合法多批调用按各自 assistant turn 配对。
- [ ] 添加 `TestGeminiToolContextLaffey_Scope`：不同 APIKeyID、AccountID、mapped model 均不命中；模拟切换账号后绝不能恢复旧签名。
- [ ] repository miniredis 用例 `TestGatewayCacheGeminiLaffey`：TTL 精确 24h、超过 64 KiB 拒写、第二个 service 实例命中、过期未命中、Redis 关闭返回错误。
- [ ] RED：`go test -tags=unit ./internal/service ./internal/repository -run 'TestGeminiToolContextLaffey_|TestGatewayCacheGeminiLaffey' -count=1`。
- [ ] 在 service 中从已鉴权 `c.Get("api_key")` 的 `*APIKey` 读取数字 ID，使用已选 Account.ID 和映射模型构建 scope；测试缺鉴权上下文时明确禁用缓存，不使用共享空 scope。
- [ ] 新窄接口由现有 repository `gatewayCache` 实现，通过类型断言复用 `s.cache`。使用单独 Redis prefix；不借用 reasoning cache 字段，不扩展 GatewayCache 必需方法。
- [ ] response 工具 ID 公布前 Save；Restore 在 Claude→Gemini 转换前、上游账号已确定后执行。仅恢复元数据到中间体；`restoreGeminiToolContextIDs` 在既有 stripGeminiFunctionIDs 之后按调用顺序验证并成对恢复原生 ID。
- [ ] `validateGeminiToolResultPairs` 在原始请求进入通用转换器之前运行；历史合法调用仍可重复出现在会话回放中，不把历史回放误判为本轮执行重复。每次 Push 后处理 TakeContextUpdates 更新相同缓存条目，晚到签名不重新发送调用。
- [ ] 对当前 stream handler 的元数据桥接采用请求级 context 附件，包含 store/batchID；不变更其公开调用参数列表。非流式逐调用使用相同身份分配/保存流程。
- [ ] 验证 Antigravity 的 buildAntigravityCompatGeminiBody 请求体保持既有语义；它调用相同 convertClaudeMessagesToGeminiGenerateContent/ensureGeminiFunctionCallThoughtSignatures，因此恢复逻辑必须在 Gemini 服务局部接入。
- [ ] GREEN；缓存错误只记录 request ID / scope 内部 ID / 错误类别，不打印参数、凭据、签名。验证无缓存路径仍能运行 A 的全部测试。
- [ ] 建议提交：`fix(gemini): restore tool context across result turns`，包含同步保留。

## Task 5：明确工具选择及禁并行参数语义（完成补丁 B）

**Files:** 新增 tool_choice 文件与测试；两个 Gemini 请求入口的精确 transforms。

**Interfaces:**

```go
type geminiToolChoice struct { Mode string; AllowedNames []string }
func parseGeminiToolChoice(protocol string, body []byte) (geminiToolChoice, error)
func applyGeminiToolChoice(body []byte, choice geminiToolChoice) ([]byte, error)
```

protocol 只接受 `chat_completions` / `messages`。从原始客户端 body 解析，避免在跨协议转换后才发现字段已丢失。

- [ ] `TestGeminiToolChoiceLaffey_Mapping`：Chat auto/none/required/named 和 Messages auto/none/any/tool 精确映射 spec B；指定未声明工具报错。
- [ ] `TestGeminiToolChoiceLaffey_ParallelConstraint`：Chat false / Messages disable=true 在任何上游请求前返回 400 invalid_request_error；true/省略正常允许并行；不得少发一个工具来伪装串行。
- [ ] 补充无工具、tool_choice=none 的 false 参数用例：约束自然满足，正常转发；客户端若默认 false 且有工具，必须在验收中明确识别，不能把预期 400 算作兼容成功。
- [ ] `TestGeminiToolChoiceLaffey_UpstreamBody`：通过 HTTP stub 检查实际 toolConfig.functionCallingConfig；显式 Hydrogen high/xhigh 模型 ID 与现有 thinking tier 规则保持一致。
- [ ] RED：`go test -tags=unit ./internal/service -run '^TestGeminiToolChoiceLaffey_' -count=1`。
- [ ] 实现局部解析和映射，允许的字段使用明确 JSON 对象，不影响别的平台请求。
- [ ] GREEN，运行既有 `gemini_compat_thinking_test.go` 对应测试。
- [ ] 建议提交：`fix(gemini): enforce supported tool choice semantics`，包含同步保留。

## Task 6：从上游快照证明补丁可重建

**Files:** `.laffey/testsuite/test_gemini_compat_sync.py`、brand.json、README、sync workflow、所有 overlay/transforms；保存本 spec/plan 的精确 preserve_paths。

**Interfaces:** 复用 `.laffey/tools/laffey_sync.py` 的 `apply_branding`、`verify_candidate`、`build_candidate`，无需增加通用同步引擎。

- [ ] `test_gemini_overlay_matches_runtime`：新文件 overlay 与运行文件逐字节相同。
- [ ] `test_gemini_transforms_reapply_idempotently`：用固定 upstream before fixture 生成候选，第一次应用得到修复代码，第二次应用零替换且摘要不变。
- [ ] `test_gemini_transform_drift_blocks_sync`：改变任一精确替换的关键上下文必须抛 BrandingError，并指出文件。
- [ ] `test_gemini_patch_paths_are_explicit`：实际差异都在精确 allowlist，禁止 backend/** 通配放宽。
- [ ] 更新说明，声明 Gemini 兼容补丁是 Laffey 保留例外；sync PR 模板移除与此相冲突的“本地网关实现全部丢弃”表述。
- [ ] 对两份被 docs/* 忽略的 spec/plan 使用精确 `git add -f`，添加精确 preserve_paths/allowlist；不修改整个 docs 忽略策略。
- [ ] 运行：`python -m unittest discover -s .laffey/testsuite -v`，全部通过。
- [ ] 拉取锁定上游：`git fetch https://github.com/Wei-Shaw/sub2api.git refs/tags/v0.2.20:refs/remotes/upstream-release/v0.2.20`；验证剥离 tag 后 commit 与 lock 一致。
- [ ] 使用包含补丁的本地已提交 HEAD 作为 base：`python .laffey/tools/laffey_sync.py build --repo . --upstream-ref refs/remotes/upstream-release/v0.2.20 --base-ref HEAD --upstream-tag v0.2.20 --output ../laffey-gemini-verified-candidate`。输出目录须未存在。
- [ ] 在生成候选 backend 运行 `go test -tags=unit ./internal/service ./internal/repository -run 'TestGemini.*Laffey|TestGatewayCacheGeminiLaffey' -count=1`，证明测试的是重建产物。
- [ ] 建议提交：`test(laffey): preserve Gemini fixes across upstream sync`。

## Task 7：完整验证与可审查发布包

**Files:** 后续新增 `.laffey/verification/gemini-tool-compat-<commit>.md`，只记录摘要、命令、用例与结果，不写凭据/真实签名。

- [ ] 格式化改动 Go 文件并检查 overlay 一致；`git diff --check` 无输出。
- [ ] 在 backend 运行 `go test -tags=unit ./...`；集成环境具备 Docker/PostgreSQL/Redis 后运行 `go test -tags=integration ./...`，这是现有 sync workflow 的必需门禁。
- [ ] 运行 `golangci-lint run ./...`，使用仓库 CI 对应版本；Linux 上对新状态/cache 用例运行 `go test -race -tags=unit ./internal/service ./internal/repository -run 'TestGemini.*Laffey|TestGatewayCacheGeminiLaffey' -count=1`。
- [ ] 对新参数累计器加入 4/16/64 KiB 分片输入 benchmark：`go test -run '^$' -bench '^BenchmarkGeminiToolCallsLaffey' -benchmem ./internal/service`，报告累计分配；确认增长近似线性、没有每片重新解析前缀或无界 pending 队列。
- [ ] 检查实际账号路由：Gemini APIKey/OAuth/ServiceAccount 进入本服务；Antigravity 分支不得误报覆盖。若需新增 Antigravity 修改，先补真实入口失败回归并更新 spec 文件范围。
- [ ] 准备隔离候选实例、记录版本与镜像 digest、配置少量真实验收。先按 spec 的 4 会话上限验收，检查两个客户端各执行 3 次 Read 并完成第二轮；任何失败停止扩展探测。
- [ ] A、B 分开灰度；B 上线前估算调用量 × 平均缓存记录大小 × 24h 的 Redis 占用及读写量，实测增加的请求延迟；不能用单条 64 KiB 上限声称总内存有界。明确列出禁并行 400 等行为变化，客户端验收未通过时不发布 B。
- [ ] 整理 PR 级变更说明、离线/集成/客户端证据、仍未覆盖的上游形态。
- [ ] 只读确认生产实际镜像与运行二进制版本/摘要，给出对应精确回退步骤；不要仅依赖容器 image 标签推断运行代码。
- [ ] 发布时按 `.laffey/upstream.lock.json` 推导合法版本；若基线仍为 v0.2.20 且最新为 laffey.1，则候选 `v0.2.20-laffey.2`，先检查 tag 未被占用。
- [ ] 将可审查镜像、验收记录、灰度入口、回退操作交给用户确认生产替换。替换后的同一小型用例通过，才能宣布线上修复完成。

## 自审结果

- A 覆盖已复现的合并/吞调用/聚合缺失；B 覆盖工具回合和参数语义，没有把单次 HTTP 200 当作 Agent 可用性。
- 所有新增类型和方法由上游任务定义，后续任务仅依赖上述接口；没有借改全局 GatewayCache 扩散变更。
- 五项 Review Focus 均有归属测试；未知 partialArgs 明确报错，未声称实现。
- 缺内部 index 已从根因结论降级为事件完整性要求。
- 本轮只创建计划/设计；产品代码、测试新增和部署均尚未执行。

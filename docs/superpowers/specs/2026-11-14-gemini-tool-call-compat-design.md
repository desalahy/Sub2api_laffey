# Laffey Gemini 工具调用适配设计

状态：最终候选设计，已完成源码调查；本轮交付计划与准备工作，实施前由用户审阅。

实施状态（2026-11-14）：用户已授权开始修改、提交并在 GitHub 构建版本。按影响面讨论先交付补丁 A；B 的缓存/参数拒绝保留为后续独立补丁。A 审查后增加 Gemini Messages handler 对错误后已观测用量的结算回归、原生多候选和累计图片处理；对应共享文件修改仅限 Gemini 分支。

## 目标与基线

让 OpenCode、WorkBuddy 通过 Laffey `/v1/chat/completions` 使用 Gemini 时，连续同名工具调用保持独立，工具结果可以正确回传，并保留流式体验。同步修复同源 `/v1/messages` 转换与 Gemini SSE 聚合问题。

- 仓库：`desalahy/Sub2api_laffey`。
- 实施基线：`55ff4c44637615d983e4da1894712e7831218a9f`，master，`v0.2.20-laffey.1` 工作树。
- 锁定上游：`.laffey/upstream.lock.json` 中 `v0.2.20` / `dab829a4e9b7d1cc307189aea8b0319eb29e29d0`。
- `sync/upstream-v0.2.21` 的 `a02a0b719ed45054a3721c70276f534274f35a77` 仍有同名合并逻辑；不能以升级候选分支代替修复。
- Go 要求：`1.27.3`。本机当前 `1.26.4`，后端测试命令已因版本不足退出，尚未证明基线后端测试通过。
- Python 同步契约基线：`python -m unittest discover -s .laffey/testsuite -v`，12/12 通过。
- 已有线上复现证明流式输出非法；本轮离线抽取原分支代码回放又复现了同名合并、相同参数去重、聚合仅保留最后调用。这不是完整服务集成测试。
- 线上二进制的准确源码提交仍未建立；发布前必须记录运行版本、二进制摘要与镜像 digest。
- 已阅读 `docs/agent-resource-safety.md`。其中要求协议兼容优先使用 RelayKit，但当前 backend/go.mod、backend/internal 与文件树均未发现 RelayKit 实现/依赖。本次基于用户明确要求修复已调查的活跃 Gemini 入口制定计划；这是与文档表述存在的架构差异，不应声称已经遵循某个不存在的 RelayKit 接口。实施前再次核对工作基线，若基线新增该入口则优先在那里复现和修复。

## 已证实的问题

1. `gemini_chat_completions_compat_service.go:738` 与 `gemini_messages_compat_service.go:2397` 使用 `openToolName` 判断新调用，不读取 `functionCall.id`。同名不同参数被拼接，同名相同参数被吞掉。
2. `collectGeminiSSEObserved` 累积文本、保存最后一个带 parts 的响应，无法保存分散于多个事件的工具调用。OAuth 携带 project_id 的部分非流式请求也经过这里。
3. Gemini 兼容响应未保存真实 `thoughtSignature`；后续请求缺签名时使用占位值。当前转换删除 functionCall/functionResponse ID。
4. Gemini 请求构造未完整映射工具选择。`parallel_tool_calls` 到 Responses 后继续转换时丢失。
5. `.laffey` 每次从上游快照重建；只修改 backend 中现有文件无法长期保留修复。

Chat 内部部分 Anthropic delta/stop 事件缺 Index，但当前 `AnthropicEventToResponsesEvents` 顺序处理并不读取它。不能把缺 Index 当成已经证实的第二根因。新事件编码仍应输出完整 Index。

## 方案选择

采用 Gemini 专用的小型调用解析模块，继续使用已有 Anthropic → Responses → Chat 编码器。请求调度、鉴权、计费及上游 HTTP 生命周期继续由当前服务承担。

其他已评估方案：

- 全请求非流式后包装 SSE：增加首段等待时间，且现有 OAuth 聚合仍会丢调用，不作为主方案。
- OpenCode 原生 Google provider：官方支持，可作诊断或临时绕过；不能代替 WorkBuddy 所需的兼容层修复。
- 新建 Gemini → OpenAI 全量协议转换器：重写范围明显超过当前缺陷，首轮不采用。

以上是执行者根据用户“制定最终计划、准备修复”的请求确定的推荐方案，尚不表示用户已批准发布或部署。

## A：先恢复独立工具调用

### 调用身份和参数

- 每次响应有自己的调用表，以原生 ID 为首选身份；下游仍使用网关生成的稳定 `toolu_` ID。
- 完整 `args` 对象且无 ID：每个 functionCall part 都是一条独立调用，即使名称和参数相同。
- 完整 `args` 对象且有 ID：首次出现是一条调用；同 ID、同名、相同结构参数的重放不产生第二条；同 ID 冲突参数/名称必须报协议错误，不能再次执行。
- name/ID 到达但 args 字段尚未出现时等待后续；只有显式 `{}` 才代表完整空参数。后续带同一 ID 的签名补齐允许更新元数据，但不重复交付调用。没有 ID 的独立签名 part 只有在唯一待关联调用时才关联，歧义时不能猜测。
- 支持已有第三方兼容输入中的 args 字符串：完整合法对象按完整调用处理；未完成的 JSON 必须关联到明确 ID，或者唯一未结束且同名的无 ID 调用。分别维护累积快照和追加片段，最终只交付一个合法对象。多个无 ID 未完成调用造成歧义时明确报错。
- 片段携带 ID 时允许交错收集；按首次出现顺序交付完成的调用，以满足当前 Anthropic 顺序编码器。
- 不用 `}{` 字符串切割，不按名称/参数去重，不对每个网络分片递增 index。
- 原生 `partialArgs`/`willContinue` 是不同的结构化增量协议。当前入口不主动启用；若实际收到，返回明确不支持的协议错误，禁止误发 `{}`。其完整实现单独立项。
- 调用参数仅在完整时进入下游 tool block；正文和 thought 摘要仍实时传递。一次完整 functionCall 立即产生 start/delta/stop，不等待整段回答。
- 新状态使用已有上游响应大小配置约束累计缓存；超过限制或 EOF 时仍有未完成参数必须失败，不得正常结束并让客户端执行残缺工具。
- 上限来源为 `resolveUpstreamResponseReadLimit(cfg)`，包括所有 pending/completed 调用持有的内存和已见 ID 记录；文本排队也必须计入。JSON 增量扫描保持线性，不能每片重新解码全部前缀。保持 `json.Number` 精度；工具参数中的大整数不得因 map 解码被舍入。

### 两条出口及聚合

- Chat 和 Messages 消费同一解析结果；保留各自错误格式、usage、cancel 行为。
- 工具 start 带唯一 ID/name；参数 delta 带正确 index；tool_calls index 连续为 0、1、2；Messages block index 包含 text/thinking 的位置。
- 每个工具参数仅交付一次；正常结束只产生一个终止事件；流中协议错误后不再产生成功终止语义。
- SSE 聚合保留全部 functionCall、thought/signature、inlineData 与文本相对顺序，最后 usage-only / finish-only 事件的元数据必须合入最终结果。
- 文本使用现有累计/增量处理惯例，普通文本与 thought 分开累计；不把 thought 拼入正文。
- 非流式响应转换也遵守 thought 分离；函数调用计数与同一组数据的流式组装结果相同。

## B：保证后续工具回合

### 元数据与回传

标准 Chat 客户端可能丢掉扩展字段，因此不能依赖客户端回传签名。采用现有 Redis 上的新窄接口保存 Gemini 调用元数据；不扩展全局 GatewayCache 必需接口，避免修改所有测试替身。

- 缓存内容：下游调用 ID、上游调用 ID、名称、规范化参数 SHA-256、该调用所在 assistant 批次与顺序、原生 functionCall part、原生 thoughtSignature。
- 命名空间：`gemini:tool_context:v1:`。键包含 API key ID、上游账号 ID、映射后模型和下游调用 ID的摘要；只使用内部数字 ID，不使用明文 API key。
- TTL：24 小时；一个序列化调用记录上限 64 KiB；超限不写缓存并记录不含参数/签名的诊断。
- 在对客户端公布工具 ID 前保存上下文。普通缓存故障不触发额外模型请求；按现有缺签名兼容路径继续，不能宣称签名连续性已保证。
- 上游后续补到同一 ID 的签名时更新同一缓存记录。签名只保留在对应 part，不把第一条调用签名复制到其他调用。
- 回传必须逐条验证调用名称和参数摘要；同名、同参数的不同 ID 仍分别恢复。
- 在已选择上游账号后恢复真实签名，账号或映射模型变化时缓存不命中，不能把旧账号签名注入新账号。
- 显式历史 signature 保持现有处理；本补丁保证网关生成并缓存的元数据不会跨账号恢复。不能把未知来源签名视为已验证有效。
- 只有缓存证明上游原本使用 ID 时才恢复该原生 ID；调用和结果成对恢复。没有元数据的旧会话维持原有无原生 ID 形式。
- 工具结果按调用 ID 配对，必要时按原始调用顺序排列同一批结果；未知/重复 tool result 在新恢复路径上明确报错，不能降格成名字为 `tool` 的任意结果。
- Chat 的原始请求必须先校验结果配对，再进入 Chat→Responses→Anthropic。通用转换器当前会丢弃孤立结果、覆盖重复结果，等转换后再校验已经太晚。此校验局限于 Gemini 入口。
- 缓存命中、未命中、不可用均有离线测试；缓存失效后不靠拆 JSON 或重放已执行工具修复。

### 工具选择

在 Gemini 兼容服务局部解析并映射：Chat auto → AUTO；none → NONE；required → ANY；指定函数 → ANY + allowedFunctionNames。Messages auto/none/any/tool 对应相同语义。指定函数必须存在于工具声明。

当前已查阅原生 FunctionCallingConfig 没有已核实的禁并行字段。因此显式 `parallel_tool_calls:false` 或 `disable_parallel_tool_use:true` 返回 400 `invalid_request_error`，指明该 Gemini 路由暂不支持此约束。true/省略保持允许并行；不静默丢字段、不靠提示词假装强制约束、不丢弃模型已经返回的其他调用。

如果请求没有函数工具，或者 tool_choice 明确为 none，禁并行约束自然满足，不因无实际作用的 false 单独拒绝。客户端验收需核实其默认是否发送 false；若发送，补丁 B 不得宣布兼容完成，应先解决客户端设置或设计真正可保证的串行协议。

现有显式 high/xhigh 模型 ID 原样传递；上下文/输出配置沿用现状。本修复不通过 token 数或延迟判断实际思考档位。

## 同步与文件归属

- 新源码、回归测试及固定测试数据的权威副本位于 `.laffey/overlay/`；同步后生成 backend 对应文件，两份内容必须相同。
- 现有文件使用 `.laffey/transforms/gemini_*.before/.after` 做精确替换；上下文漂移应阻断候选生成。
- `.laffey/brand.json` 仅增加实际改动的精确路径，避免放开 backend/**。
- 设计与计划位于 docs/superpowers，执行时加入精确 preserve_paths 和 allowlist，使上游重建后仍存在。
- `.gitignore` 的 `docs/*` 会忽略这两份文档，本轮仅用 `git add -f` 将具体文件纳入本地待审查变更；不放开整棵 docs。后续仍需声明 preserve_paths。
- `.laffey/README.md` 及 sync workflow 的 PR 说明要反映 Gemini 例外，不能继续声称所有网关代码完全沿用上游。
- A 和 B 各自同时提交运行文件、overlay、transforms、同步测试，避免中间提交遗失修复。

## 验收、成本与发布

### 影响面核对（2026-11-14）

| 路径/模块 | 影响 | 风险与边界 |
| --- | --- | --- |
| Gemini `/v1/chat/completions`、`/v1/messages` | 直接改变工具调用转换；覆盖进入 GeminiMessagesCompatService 的各账号类型及所有模型版本 | A 中等范围，必须回归纯文本、thought、单工具及多工具 |
| Gemini 原生非流式 generateContent | ForwardNative 在 useUpstreamStream 分支调用 collectGeminiSSEObserved | Task 3 必须加原生端到端 fixture，覆盖调用、图片、签名、usage 和错误 signal；不能声称原生接口完全不受影响 |
| Gemini 原生流式/直接非流式 | 不经过本次工具编码或聚合分支 | 以路由回归确认路径隔离，不将其他路径成功外推 |
| Antigravity Gemini | 独立响应处理器，但共用 convertClaudeMessagesToGeminiGenerateContent / ensureGeminiFunctionCallThoughtSignatures | B 的恢复与 tool_choice 在 Gemini 服务入口局部接入，避免改变共用 helper 的默认语义；保留 Antigravity 请求体回归 |
| GPT、Claude、Grok 及通用 apicompat | 计划不修改其协议入口和通用编码器 | 逻辑影响低，但同一进程发布/重启及共享 Redis 资源仍有运维影响 |
| Redis | B 增加逐调用记录读写，24h TTL | TTL 和单条上限不等于总量上限；上线前根据调用量估算容量与延迟，验证失败退化 |
| `.laffey` 同步 | 增加受控 backend 例外及上下文漂移失败条件 | 可能阻断未来上游同步，属于显式可维护成本 |

全计划不能描述为只改两行的小补丁。建议发布顺序保持 A 先行，B 在跨账号/缓存回归完成后独立灰度。尤其显式禁并行改为 400、孤立/重复结果拒绝、未知 partialArgs 拒绝都是客户端可见行为变化；它们必须单独列在 B 的兼容性验收与变更说明中。A 的异常残缺流拒绝也需要单独验证。

### 验收步骤

1. 固定 SSE/JSON 数据先跑服务级真实转换函数，再跑后端单元与必要集成测试。抽取逻辑脚本只能作诊断证据。
2. 对 A：同名不同参数、同名相同参数、无 ID、不同工具、交错片段、特殊字符串、多个 part 同帧、usage-only 尾帧、EOF/cancel 均有回归。
3. 对 B：串乱的结果顺序、真实签名恢复、不同用户/账号/模型隔离、TTL、缓存故障、非法 tool_choice 均有回归。
4. 在固定上游快照重新生成候选并重新运行 Gemini 回归，证明同步保留。
5. 只有离线验证通过才做最多 4 个小型真实验收会话：2 个直接 API 完整工具回合（stream/nonstream）和 OpenCode、WorkBuddy 各 1 个小任务。每会话最多 3 次模型请求、响应正文只需简短标记，初始 max_tokens 512；因思考截断时先报告证据，再调整，禁止自动扩展为 benchmark。
6. 客户端任务在三个临时小文件中各读取一次，回传后输出短校验结果；检查实际执行次数与下一轮成功，不能只看 HTTP 200。
7. 发布前只读核对线上运行版本/digest。通过隔离实例验证后生成可审查的镜像、变更清单、回退命令；生产替换属于后续明确部署动作。

## 交付边界

A 和 B 都属于最终适配计划，A 是可提前验收的故障修复。原生 Gemini `partialArgs` 新协议、公共 `/v1/responses` 新路由、全面重写通用 apicompat、数据库迁移、客户端安装包修改不属于本次补丁。

Antigravity 使用独立 `antigravity.StreamingProcessor`。通过既有相关测试检查回归；若实际流量只读路由检查落到此入口，先补该入口最小失败用例，再决定是否纳入同一缺陷修复，不能把 Gemini 服务的测试外推为全部路由已修好。

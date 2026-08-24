# 二开改动说明（Fork Changes）

> 本文档记录本 fork（`github.com/yuanzc188/new-api`）相对官方 `github.com/QuantumNous/new-api` 的所有自定义改动。
> **目的**：合并官方更新时对照本文档，避免二开功能被覆盖或漏掉。
> 每次新增/修改二开功能后，请同步更新本文档。

## 目录

- [功能一览](#功能一览)
- [功能 1：/v1/videos（异步任务）执行渠道参数覆盖](#功能-1v1videos异步任务执行渠道参数覆盖)
- [功能 2：任务日志详情内嵌视频播放](#功能-2任务日志详情内嵌视频播放)
- [功能 3：API Key 掩码开关](#功能-3api-key-掩码开关)
- [功能 4：sora 任务递归提取上游嵌套视频直链](#功能-4sora-任务递归提取上游嵌套视频直链)
- [功能 5：视频任务按上游实际时长多退少补](#功能-5视频任务按上游实际时长多退少补)
- [功能 6：fish.audio 声音克隆（按次计费）](#功能-6fishaudio-声音克隆按次计费)
- [基础设施：自建 CI（推自己的 Docker Hub）](#基础设施自建-ci推自己的-docker-hub)
- [全部改动文件清单](#全部改动文件清单)
- [合并官方更新的检查清单](#合并官方更新的检查清单)

---

## 功能一览

| # | 功能 | 类型 | 关键标识（grep 用） |
|---|------|------|------|
| 1 | `/v1/videos` 等异步任务执行渠道参数覆盖 | 后端 | `applyTaskParamOverride` |
| 2 | 任务日志详情内嵌视频播放弹窗 | 前端 | `video-preview-dialog`、`result_url` |
| 3 | API Key 掩码开关（可手动关闭掩码） | 前后端 | `TokenKeyMaskEnabled` |
| 4 | sora 任务递归提取上游嵌套视频直链 | 后端 | `extractVideoURLFromBody` |
| 5 | 视频任务按上游实际时长多退少补 | 后端 | `AdjustBillingOnComplete`、`defaultEstimateSeconds` |
| 6 | fish.audio 声音克隆（按次计费） | 后端 | `FishVoiceCloneHelper`、`RelayModeAudioVoiceClone` |
| - | 自建 CI 推自己的 Docker Hub | CI | `deploy-main.yml` |

---

## 功能 1：/v1/videos（异步任务）执行渠道参数覆盖

**背景**：官方普通链路（chat/image 等）会在各 handler marshal 请求体后调用 `ApplyParamOverrideWithRelayInfo` 执行渠道 `param_override`；但异步任务链路（`/v1/videos` 等，走 `RelayTaskSubmit`）不执行参数覆盖。

**改动**：在 `RelayTaskSubmit` 中 `BuildRequestBody` 之后、`DoRequest` 之前，统一对 **JSON 请求体**应用参数覆盖（multipart 等非 JSON 透传跳过）。一处改动覆盖所有视频/异步任务平台（sora/kling/doubao/vidu/suno…）。

| 文件 | 改动 |
|------|------|
| `relay/relay_task.go`（M） | 新增函数 `applyTaskParamOverride(c, info, body)`；在 `RelayTaskSubmit` 的「8.5 应用渠道参数覆盖」步骤调用（位于 `adaptor.BuildRequestBody` 与 `adaptor.DoRequest` 之间） |

**合并注意**：`relay/relay_task.go` 是官方高频改动文件。合并后务必确认 `applyTaskParamOverride` 的调用仍在 `BuildRequestBody` 之后、`DoRequest` 之前，且 `info.ParamOverride`、`ApplyParamOverrideWithRelayInfo` 接口未被官方改名。

---

## 功能 2：任务日志详情内嵌视频播放

**背景**：官方 `web/`（前端）的任务日志详情列，视频任务只做「新标签打开 `/content` 代理端点」，且判定用 `fail_reason.startsWith('http')`，不够准确、无内嵌播放。后端 `result_url` 字段其实已返回。

**改动**：详情列改为优先用 `result_url`（回退 `fail_reason` 兼容旧数据），点击弹出内嵌 `<video>` 播放器，带错误回退（新标签打开 / 复制链接）。

| 文件 | 改动 |
|------|------|
| `web/src/features/usage-logs/types.ts`（M） | `TaskLog` 接口新增 `result_url?: string` |
| `web/src/features/usage-logs/components/columns/task-logs-columns.tsx`（M） | 新增 `VideoPreviewCell`；`DetailsCell` 视频分支改用 `result_url`（回退 `fail_reason`），渲染视频弹窗 |
| `web/src/features/usage-logs/components/dialogs/video-preview-dialog.tsx`（A，新增） | 新组件 `VideoPreviewDialog`，`<video controls>` 内嵌播放 + 错误回退 |
| `web/src/i18n/locales/zh.json` / `en.json`（M） | 新增词条 `Video Preview`、`Video playback failed`（另 `API Key Masking` 等见功能 3） |

**合并注意**：`types.ts` 与 `i18n` 是「追加型」改动，一般自动合并。`task-logs-columns.tsx` 官方若重构，需确认视频分支仍用 `result_url` + `VideoPreviewCell`。

---

## 功能 3：API Key 掩码开关

**背景**：官方令牌列表/详情接口固定对 key 掩码（`buildMaskedTokenResponse` → `GetMaskedKey`），无法关闭。

**改动**：新增全局配置 `TokenKeyMaskEnabled`（默认 `true` = 掩码开启），走标准 OptionMap 机制；关闭后接口返回完整 key。前端在「系统设置 → 运营设置 → 系统行为」加一个开关。

**后端**：

| 文件 | 改动 |
|------|------|
| `common/constants.go`（M） | 新增 `var TokenKeyMaskEnabled = true` |
| `model/option.go`（M） | `InitOptionMap()` 注册 `OptionMap["TokenKeyMaskEnabled"]`；`updateOptionMap()` 新增 `case "TokenKeyMaskEnabled"`（依赖官方已有的 `HasSuffix(key,"Enabled")` bool 分支） |
| `controller/token.go`（M） | `buildMaskedTokenResponse` 内按 `common.TokenKeyMaskEnabled` 选择 `GetMaskedKey()` / `GetFullKey()` |

**前端**：

| 文件 | 改动 |
|------|------|
| `web/src/features/system-settings/types.ts`（M） | `OperationsSettings` 新增 `TokenKeyMaskEnabled: boolean` |
| `web/src/features/system-settings/operations/index.tsx`（M） | `defaultOperationsSettings` 新增 `TokenKeyMaskEnabled: true` |
| `web/src/features/system-settings/operations/section-registry.tsx`（M） | `SystemBehaviorSection` 的 `defaultValues` 传入 `TokenKeyMaskEnabled` |
| `web/src/features/system-settings/general/system-behavior-section.tsx`（M） | zod schema 加字段；新增「API Key 掩码」`Switch` |
| `web/src/i18n/locales/zh.json` / `en.json`（M） | 词条 `API Key Masking`、`When enabled, API keys are masked in the token list; turn off to show full keys` |

**合并注意**：命名以 `Enabled` 结尾是有意的——`model/option.go` 靠该后缀走统一 bool 解析；`controller/option.go` 的 `GetOptions` 会过滤 `Token`/`Secret`/`Key` 结尾的敏感 key，本配置名不能改成这些后缀。合并后如官方新增了绕过 `buildMaskedTokenResponse` 直接返回 key 的接口，需补上开关判断。

---

## 功能 4：sora 任务递归提取上游嵌套视频直链

**背景**：sora（OpenAI Video 兼容）适配器在任务 `completed` 时**故意留空 URL**，回退到 `/v1/videos/{id}/content` 代理端点（假设走真 OpenAI 的认证 `/content`）。但很多中转上游把真实视频链接（`.mp4` 直链）嵌在任务结果 JSON 的深层字段里（如 `data.video_url`、`output[].url`、`result.data.video_url`），导致代理端点拿不到、视频打不开。

**改动**：sora `ParseTaskResult` 在 `completed` 时，递归遍历上游返回 JSON，挖出首个「视频直链」（URL path 以 `.mp4/.mov/.webm/.mkv/.m4v/.m3u8/.avi` 结尾的 http 链接，忽略签名 query）；挖到就存真实链接（前端直接播放），挖不到才回退代理端点（真 OpenAI 行为不变）。封面图（`.jpg/.png`）天然排除。

| 文件 | 改动 |
|------|------|
| `relay/channel/task/sora/video_url.go`（A，新增） | `extractVideoURLFromBody` / `findVideoURL` / `isVideoURL`，通用递归提取 |
| `relay/channel/task/sora/video_url_test.go`（A，新增） | 单元测试（真实 dreamina 返回、封面图排除、无链接回退三种场景） |
| `relay/channel/task/sora/adaptor.go`（M） | `ParseTaskResult` 的 `case "completed"` 改为 `taskResult.Url = extractVideoURLFromBody(respBody)` |

**注意**：此提取在任务被查询（fetch）时执行，**只对更新后新产生的任务生效**；历史任务的 `result_url` 已存成代理端点，不会自动修复（如需回填需另写脚本从 `data` 字段重解析）。

**合并注意**：`video_url.go/_test.go` 是新增文件不冲突；`adaptor.go` 的改动仅在 `completed` 分支一行，合并后确认该行仍在。

---

## 功能 5：视频任务按上游实际时长多退少补

**背景**：数字人（音频驱动）类模型走 `/v1/videos`（sora / OpenAI 视频格式），生成时长由音频长度决定，请求里不带 `seconds` 参数。官方 `EstimateBilling` 在缺省时固定按 4 秒预扣，导致所有请求都是同一个价（如 $0.5 × 4 = $2），无法按实际生成时长计费。

**改动**：改成「预扣 + 完成时差额结算」：

1. `EstimateBilling` 缺省秒数由 `4` 改为常量 `defaultEstimateSeconds = 60`（预扣 60 秒的钱）。
2. sora adaptor 实现 `AdjustBillingOnComplete`：任务轮询到终态时，从上游返回体（`task.Data`）读 `seconds` 字段（gjson，兼容字符串/数字两种形式），按 `实际额度 = 预扣额度 × 实际秒数 ÷ 预扣秒数` 重算，交给 `RecalculateTaskQuota` 多退少补。拿不到 `seconds` 时返回 0（保持预扣额度）并打一条 `SysLog`。
3. `settleTaskBillingOnComplete` 调整优先级：**adaptor 的 `AdjustBillingOnComplete` 提到「按次计费跳过结算」之前**。原来按次计费（`PerCallBilling`）直接 return，adaptor 的调整根本不会执行；数字人渠道正是按次计费 + seconds 倍率，必须放行。按次计费仍然跳过 token 重算。

| 文件 | 改动 |
|------|------|
| `relay/channel/task/sora/adaptor.go`（M） | 新增常量 `defaultEstimateSeconds`；`EstimateBilling` 缺省秒数改用它；新增方法 `AdjustBillingOnComplete`；新增 `gjson` import |
| `relay/channel/task/sora/billing_test.go`（A，新增） | 差额结算表驱动测试（退款/补扣/数字类型/等值/缺字段/上限钳制/无 BillingContext） |
| `service/task_polling.go`（M） | `settleTaskBillingOnComplete` 中 adaptor 调整与 `PerCallBilling` 早退的顺序对调 |
| `service/task_billing_test.go`（M） | 官方 `TestSettle_PerCallBilling_SkipsAdaptorAdjust` 断言的是旧契约，改写为 `TestSettle_PerCallBilling_HonorsAdaptorAdjust` |

**上限保护**：上游返回的时长是外部输入，钳制到 `relaycommon.MaxTaskDurationSeconds`（3600）后才作为计费乘数，符合 AGENTS.md 的计费安全不变量。

**顺带修的 bug**：`responseTask.Seconds` 原本声明为 `string`，上游若把 `seconds` 返回成**数字**（自建数字人常见），`DoResponse` 和 `ParseTaskResult` 的 unmarshal 会直接报错——提交返回 500，或任务永远停在进行中、既不出结果也不结算。该字段仅用于把上游响应原样回写给客户端（代码里没有任何地方读它），已改为 `json.RawMessage`，字符串和数字两种形式都兼容。

**注意**：
- 预扣从 4 秒变 60 秒后，单次请求预扣额度变为原来的 15 倍（如 $2 → $30），**余额不足的用户会在提交阶段就被拒**。若要调小，改 `defaultEstimateSeconds` 一个常量即可。
- 只对更新后新产生的任务生效，历史任务不回补。
- 上游必须在完成时返回顶层 `seconds` 字段；嵌套字段（如 `metadata.seconds`）目前不解析。

**合并注意**：`service/task_polling.go` 的 `settleTaskBillingOnComplete` 是官方文件，合并后务必确认 adaptor 调整仍在 `PerCallBilling` 早退**之前**。

---

## 功能 6：fish.audio 声音克隆（按次计费）

**背景**：fish.audio 的 TTS 已经用 OpenAI 兼容格式跑在普通渠道上，但「创建音色模型（声音克隆）」是 fish 私有端点 `POST https://api.fish.audio/model`（multipart），官方网关没有对应路由。官方的「高级自定义渠道」只支持固定白名单端点（chat / responses / messages / rerank / images / embeddings / gemini，见 `relaykit/dto/channel_settings.go` 的 `advancedCustomEndpointTypeFromIncomingPath`），**无法在后台自行配置任意路径透传**，所以只能走代码适配。

**改动**：新增网关路由 `POST /v1/audio/voices`，multipart 请求体原样透传到上游 `{base_url}/model`，响应原样回写，按次计费。

| 文件 | 改动 |
|------|------|
| `relay/fish_voice_clone_handler.go`（A，新增） | `FishVoiceCloneHelper`：透传 multipart 到 `{base_url}/model`，走渠道代理设置，成功后 `PostTextConsumeQuota` 结算 |
| `relay/constant/relay_mode.go`（M） | 新增 `RelayModeAudioVoiceClone`；`Path2RelayMode` 新增 `/v1/audio/voices` 分支 |
| `relay/constant/relay_mode_test.go`（M） | 补 `/v1/audio/voices` 与其它 `/v1/audio/*` 前缀的区分用例 |
| `router/relay-router.go`（M） | 注册 `httpRouter.POST("/audio/voices", ...)`，复用 `types.RelayFormatOpenAIAudio` |
| `controller/relay.go`（M） | `relayHandler` 新增 `case relayconstant.RelayModeAudioVoiceClone` |
| `middleware/distributor.go`（M） | `/v1/audio` 分支新增 `/v1/audio/voices`：multipart 请求从表单里取 `model`，并设置 `relay_mode` |

**后台配置方式**：
1. 建一个渠道（类型「自定义渠道 Custom」或 OpenAI 兼容均可），Base URL 填 `https://api.fish.audio`，Key 填 fish 的 API Key。
2. 该渠道模型列表里加一个克隆用的模型名，如 `fish-voice-clone`。
3. 「系统设置 → 模型固定价格」给 `fish-voice-clone` 配一个单价 —— 这就是一次克隆的价格。**不配固定价格会退回按 token 计费，等于几乎不收钱。**
4. 同一个渠道可以继续用 `/v1/audio/speech` 跑 fish 的 OpenAI 兼容 TTS。

**调用方式**：客户端把 fish 官方 `POST /model` 的 multipart 字段（`voices` 音频文件、`title`、`train_mode`、`visibility`、`type` 等）原样发到本网关 `POST /v1/audio/voices`，额外带一个 `model=fish-voice-clone` 表单字段用于选渠道和计价。

**已知取舍**：请求体是**原样透传**的，`model` 字段会一并发给 fish（fish 的表单解析会忽略未知字段）。若实测 fish 报错，改成重建 multipart 并剔除 `model` 即可（可参考 `relay/channel/task/sora/adaptor.go` 的 `BuildRequestBody` multipart 分支）。

---

## 基础设施：自建 CI（推自己的 Docker Hub）

| 文件 | 改动 |
|------|------|
| `.github/workflows/deploy-main.yml`（A，新增） | push `main` 或手动触发时，构建 **amd64** 镜像推到 `${{ secrets.DOCKERHUB_USERNAME }}/new-api`（`:latest` + `:main-日期-sha`）。需配置仓库 secret `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN` |

- 官方的 `docker-build.yml` / `docker-image-branch.yml` 推 `calciumion/new-api` 且带 cosign 签名，**fork 里别用**（只在打 tag / 手动 dispatch 时触发，平时不跑）。
- 服务器更新：`docker pull yuanzc188/new-api:latest && docker compose up -d`。

---

## 全部改动文件清单

新增（A）7 个，修改（M）19 个，共 26 个：

```
A  .github/workflows/deploy-main.yml                                                  # 功能: CI
M  common/constants.go                                                                # 功能 3
M  controller/token.go                                                                # 功能 3
M  model/option.go                                                                    # 功能 3
M  relay/relay_task.go                                                                # 功能 1
M  relay/channel/task/sora/adaptor.go                                                 # 功能 4/5
A  relay/channel/task/sora/video_url.go                                               # 功能 4
A  relay/channel/task/sora/video_url_test.go                                          # 功能 4
A  relay/channel/task/sora/billing_test.go                                            # 功能 5
M  service/task_polling.go                                                            # 功能 5
M  service/task_billing_test.go                                                       # 功能 5
A  relay/fish_voice_clone_handler.go                                                  # 功能 6
M  relay/constant/relay_mode.go                                                       # 功能 6
M  relay/constant/relay_mode_test.go                                                  # 功能 6
M  router/relay-router.go                                                             # 功能 6
M  controller/relay.go                                                                # 功能 6
M  middleware/distributor.go                                                          # 功能 6
M  web/src/features/system-settings/general/system-behavior-section.tsx       # 功能 3
M  web/src/features/system-settings/operations/index.tsx                      # 功能 3
M  web/src/features/system-settings/operations/section-registry.tsx           # 功能 3
M  web/src/features/system-settings/types.ts                                  # 功能 3
M  web/src/features/usage-logs/components/columns/task-logs-columns.tsx        # 功能 2
A  web/src/features/usage-logs/components/dialogs/video-preview-dialog.tsx     # 功能 2
M  web/src/features/usage-logs/types.ts                                       # 功能 2
M  web/src/i18n/locales/en.json                                               # 功能 2/3 词条
M  web/src/i18n/locales/zh.json                                               # 功能 2/3 词条
```

---

## 合并官方更新的检查清单

1. **拉官方更新**
   ```bash
   git remote add upstream https://github.com/QuantumNous/new-api.git   # 首次
   git fetch upstream
   ```
2. **评估冲突面**（求二开文件与官方改动文件的交集）
   ```bash
   BASE=$(git merge-base main upstream/main)
   git diff --name-only $BASE main | sort > /tmp/mine.txt          # 二开文件
   git diff --name-only $BASE upstream/main | sort > /tmp/up.txt   # 官方文件
   comm -12 /tmp/mine.txt /tmp/up.txt                             # 重叠 = 冲突风险
   ```
3. **合并**：`git merge upstream/main --no-edit`，解决冲突时**保留二开逻辑**（对照上文各功能）。
4. **确认二开标识都在**：
   ```bash
   grep -rn "applyTaskParamOverride\|extractVideoURLFromBody\|TokenKeyMaskEnabled\|buildMaskedTokenResponse" --include="*.go" .
   grep -rn "AdjustBillingOnComplete\|defaultEstimateSeconds" --include="*.go" relay/channel/task/sora/
   grep -rn "FishVoiceCloneHelper\|RelayModeAudioVoiceClone" --include="*.go" .
   ls web/src/features/usage-logs/components/dialogs/video-preview-dialog.tsx
   grep -c result_url web/src/features/usage-logs/types.ts
   ```
5. **验证**（缺一不可）：
   - 后端业务包编译（绕开 main 的 `go:embed`，本地无 dist 会报 `web/*/dist: no matching files`，属正常）：
     ```bash
     go build ./relay/... ./controller/... ./model/... ./common/... ./service/... ./middleware/... ./setting/... ./pkg/... ./dto/... ./constant/...
     ```
   - 相关测试：`go test ./relay/channel/task/sora/ ./relay/constant/ ./service/`
   - **Docker 完整构建**（唯一能验证前端语义的方式，因 `web/` 用 bun `catalog:` 依赖，npm/pnpm 装不了）：`docker build -t new-api:merge .`
6. **推送**：`git push origin main`（自动触发 CI 出新镜像）。
7. **实测**：更新服务器后跑一个视频任务，确认视频链接、计费、掩码开关正常。

> 官方偶有 DB migration（新表/字段），更新会自动迁移；**更新前建议备份数据库**。

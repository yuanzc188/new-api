# 二开改动说明（Fork Changes）

> 本文档记录本 fork（`github.com/yuanzc188/new-api`）相对官方 `github.com/QuantumNous/new-api` 的所有自定义改动。
> **目的**：合并官方更新时对照本文档，避免二开功能被覆盖或漏掉。
> 每次新增/修改二开功能后，请同步更新本文档。

## 目录

- [功能一览](#功能一览)
- [功能 1：/v1/videos（异步任务）执行渠道参数覆盖](#功能-1v1videos异步任务执行渠道参数覆盖)
- [功能 3：API Key 掩码开关](#功能-3api-key-掩码开关)
- [功能 4：sora 任务递归提取上游嵌套视频直链](#功能-4sora-任务递归提取上游嵌套视频直链)
- [功能 7：任务插件枚举维度不拦提交](#功能-7任务插件枚举维度不拦提交)
- [功能 5：视频任务按上游实际时长多退少补](#功能-5视频任务按上游实际时长多退少补)
- [功能 6：fish.audio 私有端点透传（声音克隆 / 原生 TTS）](#功能-6fishaudio-私有端点透传声音克隆--原生-tts)
- [基础设施：自建 CI（推自己的 Docker Hub）](#基础设施自建-ci推自己的-docker-hub)
- [全部改动文件清单](#全部改动文件清单)
- [合并官方更新的检查清单](#合并官方更新的检查清单)

---

## 功能一览

| # | 功能 | 类型 | 关键标识（grep 用） |
|---|------|------|------|
| 1 | `/v1/videos` 等异步任务执行渠道参数覆盖 | 后端 | `applyTaskParamOverride` |
| 3 | API Key 掩码开关（可手动关闭掩码） | 前后端 | `TokenKeyMaskEnabled` |
| 4 | sora 任务递归提取上游嵌套视频直链 | 插件 | `findVideoUrl`（`plugins/tasks/sora/plugin.js`） |
| 5 | 按次计费任务仍执行 adaptor 差额结算 | 后端 | `settleTaskBillingOnComplete` |
| 6 | fish.audio 私有端点透传（声音克隆 / 原生 TTS） | 后端 | `FishVoiceCloneHelper`、`FishTTSHelper` |
| 7 | 任务插件枚举维度不拦提交（兼容第三方中转） | 后端 + 插件 | `validateResolvedUsageValue` |
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

**背景**：sora（OpenAI Video 兼容）适配器在任务完成时不返回真实视频链接，而是让客户端回源到 `/v1/videos/{id}/content` 代理端点（假设上游就是真 OpenAI）。但很多中转上游根本没实现这个端点，真实视频直链（`.mp4`）藏在任务结果 JSON 的深层字段里（如 `data.video_url`、`output[].url`），导致视频打不开。

**改动**（上游 #7076 把任务适配器换成沙箱 JS 插件后，逻辑已从 Go 移植到插件）：`plugins/tasks/sora/plugin.js` 的 `buildContentRequest` 先递归遍历任务体（`ctx.data`，map 按 key 排序保证结果确定）挖第一个「视频直链」——path 以 `.mp4/.mov/.webm/.mkv/.m4v/.m3u8/.avi` 结尾的 http(s) 链接，忽略签名 query，封面图 `.jpg/.png` 天然排除。挖到就用 `credentialless: true` 描述符回源（宿主允许任意 HTTP(S) host，且**不带渠道 key**，适配签名 CDN 直链）；挖不到才回退官方的 `/content` 端点。

| 文件 | 改动 |
|------|------|
| `plugins/tasks/sora/plugin.js`（M） | 新增 `findVideoUrl` / `isVideoUrl`；`buildContentRequest` 优先返回直链；`meta.version` 随之 bump |

**注意**：只对更新后新产生的任务生效，历史任务不回补。

**合并注意**：官方插件文件是「整文件覆盖」型改动，合并后务必确认 `buildContentRequest` 里的直链分支还在（`grep findVideoUrl plugins/tasks/sora/plugin.js`）。

---

## 功能 5：按次计费任务仍执行 adaptor 差额结算

**背景**：数字人（音频驱动）类模型走 `/v1/videos`，生成时长由音频长度决定，请求里不带 `seconds`，只能在任务完成时按上游返回的实际时长结算。这类渠道通常配「按次计费 + seconds 倍率」，而官方 `settleTaskBillingOnComplete` 里 `PerCallBilling` 会**先于** adaptor 的差额结算直接 return，导致调整永远不执行。

**改动**：`service/task_polling.go` 把 `adaptor.AdjustBillingOnComplete` 提到 `PerCallBilling` 早退**之前**。按次计费仍然跳过 token 重算，只是放行 adaptor 给出的实际额度。

| 文件 | 改动 |
|------|------|
| `service/task_polling.go`（M） | `settleTaskBillingOnComplete` 中 adaptor 调整与 `PerCallBilling` 早退的顺序对调 |
| `service/task_billing_test.go`（M） | 官方 `TestSettle_PerCallBilling_SkipsAdaptorAdjust` 断言的是旧契约，改写为 `TestSettle_PerCallBilling_HonorsAdaptorAdjust` |

**已被官方覆盖的部分**：原先二开还改了 sora 适配器（`defaultEstimateSeconds = 60` 预扣 + `AdjustBillingOnComplete` 按实际秒数多退少补）。上游 #7076 插件化后，`plugins/tasks/sora/plugin.js` 自带 `extractUsageOnComplete`（从完成响应读 `seconds`，钳制到 3600）并配合用量表达式结算重算额度，二开那套已删除。**前提是模型价格用「用量表达式」配置**（会生成 `TieredSnapshot`）；若仍用旧的固定价格，完成时不会重算，只保留预扣。

**合并注意**：`service/task_polling.go` 是官方高频改动文件，合并后务必确认 adaptor 调整仍在 `PerCallBilling` 早退**之前**。

---

## 功能 6：fish.audio 私有端点透传（声音克隆 / 原生 TTS）

**背景**：fish.audio 的 TTS 已经用 OpenAI 兼容格式跑在普通渠道上，但「创建音色模型（声音克隆）」是 fish 私有端点 `POST https://api.fish.audio/model`（multipart），官方网关没有对应路由。官方的「高级自定义渠道」只支持固定白名单端点（chat / responses / messages / rerank / images / embeddings / gemini，见 `relaykit/dto/channel_settings.go` 的 `advancedCustomEndpointTypeFromIncomingPath`），**无法在后台自行配置任意路径透传**，所以只能走代码适配。

**改动**：新增三条网关路由，请求体原样透传到 fish 的同名私有端点，响应（含 Content-Type）原样回写：

| 网关路由 | 上游端点 | 请求体 | 计费维度 |
|------|------|------|------|
| `POST /v1/audio/voices` | `{base_url}/model` | multipart，原样透传 | 按次（`promptTokens = 1`） |
| `POST /v1/tts` | `{base_url}/v1/tts` | JSON，剔除 `model` 后透传 | `text` 字符数 |
| `POST /v1/tts/stream/with-timestamp` | `{base_url}/v1/tts/stream/with-timestamp` | 同上 | 同上（同一份合成工作，只是多回了时间轴） |

带时间戳的端点响应是 **SSE 流**，透传时逐块 flush 并设 `X-Accel-Buffering: no`，避免默认 4KB 缓冲把整条流攒到结束才吐给客户端。

| 文件 | 改动 |
|------|------|
| `relay/fish_handler.go`（A，新增） | `FishVoiceCloneHelper` / `FishTTSHelper` / `fishPassthrough` / `flushingWriter`：走渠道代理设置，成功后 `PostTextConsumeQuota` 结算 |
| `relay/constant/relay_mode.go`（M） | 新增 `RelayModeAudioVoiceClone` / `RelayModeFishTTS`；`Path2RelayMode` 新增 `/v1/audio/voices` 与 `/v1/tts` 分支（`/v1/tts` 是前缀匹配，自动覆盖 `/v1/tts/stream/with-timestamp`） |
| `relay/constant/relay_mode_test.go`（M） | 补 `/v1/audio/voices`、`/v1/tts`、`/v1/tts/stream/with-timestamp` 的前缀区分用例 |
| `router/relay-router.go`（M） | 注册 `/audio/voices`、`/tts`、`/tts/stream/with-timestamp` 三条 `httpRouter.POST`，均复用 `types.RelayFormatOpenAIAudio` |
| `controller/relay.go`（M） | `relayHandler` 新增 `case RelayModeAudioVoiceClone` / `case RelayModeFishTTS` |
| `middleware/distributor.go`（M） | `/v1/audio` 分支新增 `/v1/audio/voices`：multipart 请求从表单里取 `model`，并设置 `relay_mode` |

**后台配置方式**：
1. 建一个渠道（类型「自定义渠道 Custom」或 OpenAI 兼容均可），Base URL 填 `https://api.fish.audio`，Key 填 fish 的 API Key。
2. 该渠道模型列表里加一个克隆用的模型名，如 `fish-voice-clone`。
3. 「系统设置 → 模型固定价格」给 `fish-voice-clone` 配一个单价 —— 这就是一次克隆的价格。**不配固定价格会退回按 token 计费，等于几乎不收钱。**
4. 同一个渠道可以继续用 `/v1/audio/speech` 跑 fish 的 OpenAI 兼容 TTS。
5. TTS 走字符数计费：给合成模型（如 `s2.1-pro-free`）配模型倍率即可；也可以配固定价格变成按次。

**调用方式**：客户端把 fish 官方 `POST /model` 的 multipart 字段（`voices` 音频文件、`title`、`train_mode`、`visibility`、`type` 等）原样发到本网关 `POST /v1/audio/voices`，额外带一个 `model=fish-voice-clone` 表单字段用于选渠道和计价。

**TTS 调用方式**：

```
POST {base}/v1/tts/stream/with-timestamp
Authorization: Bearer <网关的 sk- key>
Content-Type: application/json

{ "model": "s2.1-pro-free", "text": "...", "format": "mp3",
  "prosody": { "speed": 1, "volume": 0 }, "reference_id": "<音色id,可选>" }
```

`model` 只用于选渠道和计价，转发前会被 `sjson.DeleteBytes` 摘掉；其余字段原样透传。响应是 fish 原生 SSE 流，网关不改造格式。

**已知取舍**：声音克隆的 multipart 请求体是**原样透传**的，`model` 字段会一并发给 fish（fish 的表单解析会忽略未知字段）。若实测 fish 报错，改成重建 multipart 并剔除 `model` 即可。

---

## 功能 7：任务插件枚举维度不拦提交

**背景**：上游 #7076 插件化后，每个任务插件在 `meta.usageSchema` 里声明计费维度，其中枚举型字段（sora 的 `size`、doubao/vidu/hailuo 等的 `resolution`、jimeng 的 `product`）会被 `validateResolvedUsageRequest` 拿去**校验原始请求体**：请求体里只要出现同名键且值不在官方白名单里，提交直接 400 `plugin usage enum is not an allowed value`。

问题在于**校验发生在归一化之前**。插件自己其实认得更宽的写法——doubao 的 `normalizeResolution` 能把 `1920x1080`、`2560x1440` 折算到档位，`extractUsageOnComplete` 也只挑白名单内的值上报——但请求体校验先一步把任务毙了。第三方中转普遍用官方清单之外的写法（如 seedance 2.x 的 `2k`、自定义 `WxH`），更新后全部下不了单。

**改动**：

1. `relay/channel/task/jsplugin/adaptor.go` 的 `validateResolvedUsageValue` 跳过**枚举型**维度。枚举只是定价维度，不是计费安全边界：命中不了就不参与倍率，走基础价，不会产生负费用。数值上限（`seconds` / `n`，即 `canonicalUsageLimit` 那条分支）仍然强校验，插件回报的计费事实也照旧走 `validatedUsageRatios` 严格校验，AGENTS.md 的计费安全不变量不受影响。
2. `plugins/tasks/sora/plugin.js` 的 `extractUsage` 补上 `size` 白名单判断，与同文件 `extractUsageOnComplete` 对齐。上游这里是不对称的：完成时挑白名单、提交时原样上报，于是放开请求体校验后会改从 `EstimateBillingValidated` 二次报错。

| 文件 | 改动 |
|------|------|
| `relay/channel/task/jsplugin/adaptor.go`（M） | `validateResolvedUsageValue` 对 `len(schema.Enum) > 0` 的维度不做请求体校验 |
| `relay/channel/task/jsplugin/adaptor_test.go`（M） | 官方 `TestTaskAdaptorBoundsNativeUsageBeforeQuotaCalculation` 的 `declared enum in resolved metadata` 断言的是旧契约，改成正向用例「未列出的枚举值放行」，数值上限用例原样保留 |
| `plugins/tasks/sora/plugin.js`（M） | `extractUsage` 只上报白名单内的 `size` |

**代价**：用了白名单外分辨率的请求，不再按该分辨率的倍率计价，退回基础价。要精确计价就把实际用的值加进对应插件的 `usageSchema.enum`（后台「任务插件」里传自定义插件覆盖内置的即可，不必重新出镜像）。

**合并注意**：`relay/channel/task/jsplugin/adaptor.go` 是官方核心文件，合并后确认 `validateResolvedUsageValue` 里的 `len(schema.Enum) == 0` 判断还在。

---

## 基础设施：自建 CI（推自己的 Docker Hub）

| 文件 | 改动 |
|------|------|
| `.github/workflows/deploy-main.yml`（A，新增） | push `main` 或手动触发时，构建 **amd64** 镜像推到 `${{ secrets.DOCKERHUB_USERNAME }}/new-api`（`:latest` + `:main-日期-sha`）。需配置仓库 secret `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN` |

- 官方的 `docker-build.yml` / `docker-image-branch.yml` 推 `calciumion/new-api` 且带 cosign 签名，**fork 里别用**（只在打 tag / 手动 dispatch 时触发，平时不跑）。
- 服务器更新：`docker pull yuanzc188/new-api:latest && docker compose up -d`。

---

## 全部改动文件清单

新增（A）2 个，修改（M）16 个，共 18 个：

```
A  .github/workflows/deploy-main.yml                                     # 功能: CI
A  relay/fish_handler.go                                                 # 功能 6
M  common/constants.go                                                   # 功能 3
M  controller/relay.go                                                   # 功能 6
M  controller/token.go                                                   # 功能 3
M  middleware/distributor.go                                             # 功能 6
M  model/option.go                                                       # 功能 3
M  plugins/tasks/sora/plugin.js                                          # 功能 4/7
M  relay/channel/task/jsplugin/adaptor.go                                # 功能 7
M  relay/channel/task/jsplugin/adaptor_test.go                           # 功能 7
M  relay/constant/relay_mode.go                                          # 功能 6
M  relay/constant/relay_mode_test.go                                     # 功能 6
M  relay/relay_task.go                                                   # 功能 1
M  router/relay-router.go                                                # 功能 6
M  service/task_billing_test.go                                          # 功能 5
M  service/task_polling.go                                               # 功能 5
M  web/src/features/system-settings/general/system-behavior-section.tsx  # 功能 3
M  web/src/features/system-settings/operations/index.tsx                 # 功能 3
M  web/src/features/system-settings/operations/section-registry.tsx      # 功能 3
M  web/src/features/system-settings/types.ts                             # 功能 3
M  web/src/i18n/locales/en.json                                          # 功能 3 词条
M  web/src/i18n/locales/zh.json                                          # 功能 3 词条
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
   grep -rn "applyTaskParamOverride\|TokenKeyMaskEnabled\|buildMaskedTokenResponse" --include="*.go" .
   grep -n "findVideoUrl\|credentialless" plugins/tasks/sora/plugin.js
   grep -n "len(schema.Enum) == 0" relay/channel/task/jsplugin/adaptor.go
   grep -rn "FishVoiceCloneHelper\|FishTTSHelper\|RelayModeFishTTS" --include="*.go" .
   # settleTaskBillingOnComplete 里 adaptor 调整必须在 PerCallBilling 早退之前
   grep -n "AdjustBillingOnComplete" -A 6 service/task_polling.go
   ```
5. **验证**（缺一不可）：
   - 后端业务包编译（绕开 main 的 `go:embed`，本地无 dist 会报 `web/*/dist: no matching files`，属正常）：
     ```bash
     go build ./relay/... ./controller/... ./model/... ./common/... ./service/... ./middleware/... ./setting/... ./pkg/... ./dto/... ./constant/... ./router/... ./plugins/...
     ```
   - relaykit 独立编译：`cd relaykit && GOWORK=off go build ./...`
   - 相关测试：`go test ./relay/constant/ ./service/ ./plugins/`
   - **Docker 完整构建**（唯一能验证前端语义的方式，因 `web/` 用 bun `catalog:` 依赖，npm/pnpm 装不了）：`docker build -t new-api:merge .`
6. **推送**：`git push origin main`（自动触发 CI 出新镜像）。
7. **实测**：更新服务器后跑一个视频任务（确认视频链接、计费）、一次 `/v1/tts/stream/with-timestamp`（确认 SSE 逐块下发、按字数扣费）、以及掩码开关。

> 官方偶有 DB migration（新表/字段），更新会自动迁移；**更新前建议备份数据库**。

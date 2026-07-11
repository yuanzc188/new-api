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

**背景**：官方 `web/default`（现代主题）的任务日志详情列，视频任务只做「新标签打开 `/content` 代理端点」，且判定用 `fail_reason.startsWith('http')`，不够准确、无内嵌播放。后端 `result_url` 字段其实已返回。

**改动**：详情列改为优先用 `result_url`（回退 `fail_reason` 兼容旧数据），点击弹出内嵌 `<video>` 播放器，带错误回退（新标签打开 / 复制链接）。

| 文件 | 改动 |
|------|------|
| `web/default/src/features/usage-logs/types.ts`（M） | `TaskLog` 接口新增 `result_url?: string` |
| `web/default/src/features/usage-logs/components/columns/task-logs-columns.tsx`（M） | 新增 `VideoPreviewCell`；`DetailsCell` 视频分支改用 `result_url`（回退 `fail_reason`），渲染视频弹窗 |
| `web/default/src/features/usage-logs/components/dialogs/video-preview-dialog.tsx`（A，新增） | 新组件 `VideoPreviewDialog`，`<video controls>` 内嵌播放 + 错误回退 |
| `web/default/src/i18n/locales/zh.json` / `en.json`（M） | 新增词条 `Video Preview`、`Video playback failed`（另 `API Key Masking` 等见功能 3） |

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
| `web/default/src/features/system-settings/types.ts`（M） | `OperationsSettings` 新增 `TokenKeyMaskEnabled: boolean` |
| `web/default/src/features/system-settings/operations/index.tsx`（M） | `defaultOperationsSettings` 新增 `TokenKeyMaskEnabled: true` |
| `web/default/src/features/system-settings/operations/section-registry.tsx`（M） | `SystemBehaviorSection` 的 `defaultValues` 传入 `TokenKeyMaskEnabled` |
| `web/default/src/features/system-settings/general/system-behavior-section.tsx`（M） | zod schema 加字段；新增「API Key 掩码」`Switch` |
| `web/default/src/i18n/locales/zh.json` / `en.json`（M） | 词条 `API Key Masking`、`When enabled, API keys are masked in the token list; turn off to show full keys` |

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

## 基础设施：自建 CI（推自己的 Docker Hub）

| 文件 | 改动 |
|------|------|
| `.github/workflows/deploy-main.yml`（A，新增） | push `main` 或手动触发时，构建 **amd64** 镜像推到 `${{ secrets.DOCKERHUB_USERNAME }}/new-api`（`:latest` + `:main-日期-sha`）。需配置仓库 secret `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN` |

- 官方的 `docker-build.yml` / `docker-image-branch.yml` 推 `calciumion/new-api` 且带 cosign 签名，**fork 里别用**（只在打 tag / 手动 dispatch 时触发，平时不跑）。
- 服务器更新：`docker pull yuanzc188/new-api:latest && docker compose up -d`。

---

## 全部改动文件清单

新增（A）4 个，修改（M）13 个，共 17 个：

```
A  .github/workflows/deploy-main.yml                                                  # 功能: CI
M  common/constants.go                                                                # 功能 3
M  controller/token.go                                                                # 功能 3
M  model/option.go                                                                    # 功能 3
M  relay/relay_task.go                                                                # 功能 1
M  relay/channel/task/sora/adaptor.go                                                 # 功能 4
A  relay/channel/task/sora/video_url.go                                               # 功能 4
A  relay/channel/task/sora/video_url_test.go                                          # 功能 4
M  web/default/src/features/system-settings/general/system-behavior-section.tsx       # 功能 3
M  web/default/src/features/system-settings/operations/index.tsx                      # 功能 3
M  web/default/src/features/system-settings/operations/section-registry.tsx           # 功能 3
M  web/default/src/features/system-settings/types.ts                                  # 功能 3
M  web/default/src/features/usage-logs/components/columns/task-logs-columns.tsx        # 功能 2
A  web/default/src/features/usage-logs/components/dialogs/video-preview-dialog.tsx     # 功能 2
M  web/default/src/features/usage-logs/types.ts                                       # 功能 2
M  web/default/src/i18n/locales/en.json                                               # 功能 2/3 词条
M  web/default/src/i18n/locales/zh.json                                               # 功能 2/3 词条
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
   ls web/default/src/features/usage-logs/components/dialogs/video-preview-dialog.tsx
   grep -c result_url web/default/src/features/usage-logs/types.ts
   ```
5. **验证**（缺一不可）：
   - 后端业务包编译（绕开 main 的 `go:embed`，本地无 dist 会报 `web/*/dist: no matching files`，属正常）：
     ```bash
     go build ./relay/... ./controller/... ./model/... ./common/... ./service/... ./middleware/... ./setting/... ./pkg/... ./dto/... ./constant/...
     ```
   - sora 测试：`go test ./relay/channel/task/sora/`
   - **Docker 完整构建**（唯一能验证前端语义的方式，因 `web/default` 用 bun `catalog:` 依赖，npm/pnpm 装不了）：`docker build -t new-api:merge .`
6. **推送**：`git push origin main`（自动触发 CI 出新镜像）。
7. **实测**：更新服务器后跑一个视频任务，确认视频链接、计费、掩码开关正常。

> 官方偶有 DB migration（新表/字段），更新会自动迁移；**更新前建议备份数据库**。

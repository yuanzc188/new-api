# 二开自维护插件（上传型）

这里的任务插件**不随镜像内置**（`plugins/embed.go` 只嵌入 `plugins/tasks/`）。
上游对 `plugins/tasks/` 下的内置插件有硬契约——必须声明 `openai_responses`
协议并实现 `decodeRequest` / `renderEvents` / `renderFinal`（见
`plugins/builtin_plugins_test.go`）。不走 Responses 的插件放这里，通过后台
「任务插件」页上传，源码在仓库里留档以便版本管理。

## 本地校验

```bash
go build -o /tmp/new-api .        # 需要 web/dist，没有就先 mkdir -p web/dist && echo > web/dist/index.html
/tmp/new-api plugin lint plugins/contrib/<key>/plugin.js
/tmp/new-api plugin test plugins/contrib/<key>/plugin.js --fixture plugins/contrib/<key>/golden.json
```

## 清单

| 目录 | 用途 |
|------|------|
| `qwen-asr-filetrans/` | 阿里云百炼非实时语音识别（录音文件转写），DashScope 异步协议 |
| `video-generate/` | 第三方 seedance 风格视频生成，`POST /v1/video/generate` content 数组协议 |

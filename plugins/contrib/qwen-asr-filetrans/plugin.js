export const meta = {
  apiVersion: 1,
  key: "qwen-asr-filetrans",
  name: "Qwen ASR Filetrans",
  description: {
    en: "Alibaba Model Studio non-realtime speech recognition (async file transcription)",
    zh: "阿里云百炼非实时语音识别（录音文件转写，异步任务）",
  },
  version: "1.0.0",
  author: { name: "yuanzc188" },
  // 走 DashScope 原生异步协议，没有官方渠道类型与之对应：建「任务插件」渠道并选中本插件，
  // Base URL 填你的工作空间地址（如 https://{WorkspaceId}.cn-beijing.maas.aliyuncs.com）。
  models: ["qwen-audio-3.0-asr-flash-filetrans", "qwen3-asr-flash-filetrans"],
  fetchMode: "per_task",
  usageSchema: {
    // 音频时长，1 = 1 秒。这里没有用 unit:"second"，因为宿主会把「秒」维度硬卡在
    // MaxTaskDurationSeconds(3600)，而录音文件转写本来就是拿来跑超过一小时的长音频的，
    // 卡上限会让长文件静默少计费。unit:"credit" 走的是额度饱和上限，够用且不会溢出。
    audio_seconds: {
      type: "number",
      unit: "credit",
      description: { en: "Recognized audio duration in seconds", zh: "识别音频时长（秒）" },
    },
  },
  usageExamples: [
    { label: "1 分钟音频", facts: { audio_seconds: 60 } },
    { label: "30 分钟音频", facts: { audio_seconds: 1800 } },
    { label: "2 小时音频", facts: { audio_seconds: 7200 } },
  ],
  // 路径照搬 DashScope 原样，客户端把 base URL 指到网关就能用原来的 SDK。
  routes: [
    {
      method: "POST",
      path: "/dashscope/api/v1/services/audio/asr/transcription",
      type: "submit",
      action: "transcription",
      decode: "createTask",
      render: "taskCreated",
    },
    {
      method: "GET",
      path: "/dashscope/api/v1/tasks/:task_id",
      type: "query",
      taskIdParam: "task_id",
      render: "taskStatus",
    },
  ],
  auth: "api_key",
};

// 上游 usage.duration 是外部输入，先钳到 24 小时再上报，避免异常值直接变成计费乘数。
const MAX_AUDIO_SECONDS = 86400;
// 提交时拿不到真实时长，先按这个值预扣，完成时用实际时长多退少补。
const DEFAULT_ESTIMATE_SECONDS = 300;

function trimmed(value) {
  return String(value == null ? "" : value).trim();
}

function boundedSeconds(value) {
  const seconds = Number(value);
  if (!Number.isFinite(seconds) || seconds <= 0) return 0;
  return Math.min(Math.round(seconds), MAX_AUDIO_SECONDS);
}

// 接受 DashScope 原生的 input.file_urls，也接受 file_urls / file_url 的扁平写法。
function fileUrls(req) {
  const input = req.input && typeof req.input === "object" && !Array.isArray(req.input) ? req.input : {};
  const candidates = [].concat(input.file_urls || [], req.file_urls || [], input.file_url || [], req.file_url || []);
  const urls = [];
  for (const candidate of candidates) {
    const url = trimmed(candidate);
    if (url && !urls.includes(url)) urls.push(url);
  }
  return urls;
}

function parameters(req) {
  const value = req.parameters;
  return value && typeof value === "object" && !Array.isArray(value) ? value : {};
}

function resultList(data) {
  const output = (data && typeof data === "object" && data.output) || {};
  return Array.isArray(output.results) ? output.results : [];
}

function artifactKey(index) {
  return "transcription-" + (index + 1);
}

export const native = {
  createTask: function (ctx) {
    if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
    const req = ctx.body.value;
    if (!req || typeof req !== "object" || Array.isArray(req)) throw new Error("request body must be an object");
    const model = trimmed(req.model);
    if (!model) throw new Error("model is required");
    const urls = fileUrls(req);
    if (!urls.length) throw new Error("input.file_urls is required");
    for (const url of urls) {
      if (!/^(https?|oss):\/\//i.test(url)) throw new Error("file url must be an http(s) or oss url");
    }
    const requestBody = { model: model, input: { file_urls: urls }, parameters: parameters(req) };
    // 可选的时长提示，只影响预扣额度，不发给上游。
    const hint = boundedSeconds(req.audio_seconds === undefined ? req.duration : req.audio_seconds);
    if (hint) requestBody.audio_seconds = hint;
    return { kind: "submit", model: model, action: "transcription", requestBody: requestBody };
  },

  taskCreated: function (ctx, task) {
    return { output: { task_id: task.task_id, task_status: "PENDING" }, request_id: task.task_id };
  },

  taskStatus: function (ctx, task) {
    const statuses = {
      NOT_START: "PENDING",
      SUBMITTED: "PENDING",
      QUEUED: "PENDING",
      IN_PROGRESS: "RUNNING",
      SUCCESS: "SUCCEEDED",
      FAILURE: "FAILED",
    };
    const data = task.data && typeof task.data === "object" && !Array.isArray(task.data) ? task.data : {};
    const output = Object.assign({}, data.output || {}, { task_id: task.task_id });
    if (!output.task_status) output.task_status = statuses[task.status] || "PENDING";
    if (task.fail_reason && output.task_status === "FAILED" && !output.message) output.message = task.fail_reason;
    return Object.assign({}, data, { output: output, request_id: task.task_id });
  },
};

export function buildSubmitRequest(ctx) {
  const req = ctx.requestBody || {};
  const urls = fileUrls(req);
  if (!urls.length) throw new Error("input.file_urls is required");
  return {
    url: ctx.baseUrl + "/api/v1/services/audio/asr/transcription",
    method: "POST",
    headers: {
      Authorization: "Bearer " + ctx.apiKey,
      "Content-Type": "application/json",
      // 缺这个头会被当成同步调用，长音频必超时。
      "X-DashScope-Async": "enable",
    },
    // parameters 即使为空也必须带上，缺了会「提交成功但识别失败」。
    body: { model: ctx.upstreamModel, input: { file_urls: urls }, parameters: parameters(req) },
  };
}

export function parseSubmitResponse(ctx, resp) {
  const body = resp.body || {};
  const taskId = trimmed((body.output || {}).task_id);
  if (!taskId) throw new Error("task_id is empty");
  return { taskId: taskId, taskData: body };
}

export function buildQueryRequest(ctx) {
  return {
    url: ctx.baseUrl + "/api/v1/tasks/" + encodeURIComponent(ctx.taskId),
    method: "GET",
    headers: { Authorization: "Bearer " + ctx.apiKey },
  };
}

export function parseTaskResult(ctx, body) {
  const output = (body || {}).output || {};
  const statuses = {
    PENDING: "QUEUED",
    PRE_PROCESSING: "IN_PROGRESS",
    RUNNING: "IN_PROGRESS",
    SUCCEEDED: "SUCCESS",
    FAILED: "FAILURE",
    CANCELED: "FAILURE",
  };
  const mapped = statuses[trimmed(output.task_status).toUpperCase()];
  if (!mapped) return { status: "UNKNOWN", reason: "unrecognized task_status: " + trimmed(output.task_status) };
  if (mapped === "FAILURE") {
    return { status: "FAILURE", reason: trimmed(output.message) || trimmed(output.code) || "task failed" };
  }
  if (mapped !== "SUCCESS") return { status: mapped };
  // 整单 SUCCEEDED 但每个子任务都失败时，实际什么都没转写出来，按失败结算让用户拿到退款。
  const results = resultList(body);
  const succeeded = results.filter(function (item) {
    return item && trimmed(item.subtask_status).toUpperCase() === "SUCCEEDED" && trimmed(item.transcription_url);
  });
  if (results.length && !succeeded.length) {
    const first = results[0] || {};
    return { status: "FAILURE", reason: trimmed(first.message) || trimmed(first.code) || "all subtasks failed" };
  }
  return { status: "SUCCESS" };
}

export function extractUsage(ctx) {
  const req = ctx.requestBody || {};
  const hint = boundedSeconds(req.audio_seconds);
  const files = fileUrls(req).length || 1;
  // 没给提示就按默认值 × 文件数预扣，完成时按上游返回的实际时长重算。
  return { audio_seconds: hint || DEFAULT_ESTIMATE_SECONDS * files };
}

export function extractUsageOnComplete(task, taskResult, body) {
  const seconds = boundedSeconds(((body || {}).usage || {}).duration);
  return seconds ? { audio_seconds: seconds } : {};
}

export function listArtifacts(task) {
  if (task.status !== "SUCCESS") return [];
  const artifacts = [];
  resultList(task.data).forEach(function (item, index) {
    if (item && trimmed(item.transcription_url)) {
      artifacts.push({ key: artifactKey(index), type: "file", mimeType: "application/json" });
    }
  });
  return artifacts;
}

export function buildContentRequest(ctx) {
  const results = resultList(ctx.data);
  for (let index = 0; index < results.length; index++) {
    if (artifactKey(index) !== ctx.artifactKey) continue;
    const url = trimmed((results[index] || {}).transcription_url);
    if (!url) break;
    // 转写结果放在 OSS 签名直链上（24 小时有效），不能带渠道 key 回源。
    return { url: url, method: ctx.clientRequest.method, credentialless: true };
  }
  throw new Error("artifact_not_found");
}

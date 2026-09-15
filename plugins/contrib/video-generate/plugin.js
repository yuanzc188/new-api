export const meta = {
  apiVersion: 1,
  key: "video-generate",
  name: "Video Generate (content 协议)",
  description: {
    en: "Third-party seedance-style video generation: POST /v1/video/generate with a content array",
    zh: "第三方 seedance 风格视频生成：POST /v1/video/generate，content 数组协议",
  },
  version: "1.0.0",
  author: { name: "yuanzc188" },
  // 上游是第三方中转，没有官方渠道类型：建「任务插件」渠道并选中本插件，
  // Base URL 填中转地址，模型列表填下面 models 里的名字（或在后台配任务模型别名）。
  models: ["doubao-seedance-2-0-260128", "seedance-2.0", "seedance-2.0-fast"],
  fetchMode: "per_task",
  usageSchema: {
    seconds: {
      type: "number",
      unit: "second",
      description: { en: "Generated video duration", zh: "生成视频时长" },
    },
    resolution: {
      enum: ["480p", "720p", "1080p"],
      enumLabels: {
        "480p": { en: "480p", zh: "480p" },
        "720p": { en: "720p", zh: "720p" },
        "1080p": { en: "1080p", zh: "1080p" },
      },
      description: { en: "Output video resolution", zh: "输出视频分辨率" },
    },
  },
  usageExamples: [
    { label: "720p · 5s", facts: { seconds: 5, resolution: "720p" } },
    { label: "1080p · 10s", facts: { seconds: 10, resolution: "1080p" } },
  ],
  // 路径照搬上游原样，客户端把 base URL 指到网关就能直接用。
  routes: [
    { method: "POST", path: "/v1/video/generate", type: "submit", decode: "createTask", render: "taskCreated" },
    { method: "GET", path: "/v1/video/tasks/:task_id", type: "query", taskIdParam: "task_id", render: "taskStatus" },
  ],
  auth: "api_key",
};

const RESOLUTIONS = ["480p", "720p", "1080p"];
const DEFAULT_RESOLUTION = "720p";
const DEFAULT_DURATION = 5;
// 时长是计费乘数，跟宿主对 seconds 维度的上限保持一致。
const MAX_DURATION = 3600;
// 请求里原样透传给上游的可选字段，网关不解释语义。
const PASSTHROUGH_KEYS = ["resolution", "duration", "ratio", "watermark", "seed", "callback_url"];

function trimmed(value) {
  return String(value == null ? "" : value).trim();
}

function normalizedResolution(value) {
  const resolution = trimmed(value).toLowerCase();
  return RESOLUTIONS.includes(resolution) ? resolution : "";
}

function boundedDuration(value) {
  const duration = Number(value);
  if (!Number.isFinite(duration) || duration <= 0) return 0;
  return Math.min(Math.round(duration), MAX_DURATION);
}

function contentItems(req) {
  return Array.isArray(req.content) ? req.content : [];
}

// content 数组里带参考视频的是续写，带图片的是图生视频，都没有就是文生视频。
function actionFor(items) {
  let hasImage = false;
  for (const item of items) {
    if (!item || typeof item !== "object" || Array.isArray(item)) continue;
    if (item.type === "video_url") return "reference_to_video";
    if (item.type === "image_url") hasImage = true;
  }
  return hasImage ? "image_to_video" : "text_to_video";
}

function videoUrl(data) {
  const body = data && typeof data === "object" && !Array.isArray(data) ? data : {};
  const content = body.content && typeof body.content === "object" && !Array.isArray(body.content) ? body.content : {};
  return trimmed(content.video_url);
}

export const native = {
  createTask: function (ctx) {
    if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
    const req = ctx.body.value;
    if (!req || typeof req !== "object" || Array.isArray(req)) throw new Error("request body must be an object");
    const model = trimmed(req.model);
    if (!model) throw new Error("model is required");
    const items = contentItems(req);
    if (!items.length) throw new Error("content is required");

    let firstFrames = 0;
    let hasPrompt = false;
    for (const item of items) {
      if (!item || typeof item !== "object" || Array.isArray(item)) throw new Error("content items must be objects");
      if (item.role === "first_frame") firstFrames++;
      if (item.type === "text" && trimmed(item.text)) hasPrompt = true;
      if (item.type === "image_url" && !trimmed((item.image_url || {}).url)) throw new Error("image_url.url is required");
      if (item.type === "video_url" && !trimmed((item.video_url || {}).url)) throw new Error("video_url.url is required");
    }
    if (firstFrames > 1) throw new Error("only one content item may use role first_frame");
    if (!hasPrompt && actionFor(items) === "text_to_video") throw new Error("text content is required");

    if (req.resolution !== undefined && !normalizedResolution(req.resolution))
      throw new Error("resolution must be one of 480p / 720p / 1080p");
    if (req.duration !== undefined) {
      // 时长是计费乘数，超界直接拒掉而不是悄悄钳住，避免客户端以为按它发的时长出片。
      const duration = Number(req.duration);
      if (!Number.isFinite(duration) || duration <= 0 || duration > MAX_DURATION)
        throw new Error("duration must be between 1 and " + MAX_DURATION);
    }

    const requestBody = { model: model, content: items };
    for (const key of PASSTHROUGH_KEYS) {
      if (Object.prototype.hasOwnProperty.call(req, key)) requestBody[key] = req[key];
    }
    return { kind: "submit", model: model, action: actionFor(items), requestBody: requestBody };
  },

  taskCreated: function (ctx, task) {
    const data = task.data && typeof task.data === "object" && !Array.isArray(task.data) ? task.data : {};
    // 上游返回体原样回写，只把上游任务 id 换成网关的，客户端拿到的 id 才能回来查。
    return Object.assign({}, data, { task_id: task.task_id });
  },

  taskStatus: function (ctx, task) {
    const statuses = {
      NOT_START: "queued",
      SUBMITTED: "queued",
      QUEUED: "queued",
      IN_PROGRESS: "running",
      SUCCESS: "succeeded",
      FAILURE: "failed",
    };
    const data = task.data && typeof task.data === "object" && !Array.isArray(task.data) ? task.data : {};
    const rendered = Object.assign({}, data, { id: task.task_id });
    delete rendered.task_id;
    if (!rendered.status) rendered.status = statuses[task.status] || "queued";
    if (task.fail_reason && rendered.status === "failed" && !rendered.error) rendered.error = task.fail_reason;
    return rendered;
  },
};

export function buildSubmitRequest(ctx) {
  const req = ctx.requestBody || {};
  const body = { model: ctx.upstreamModel, content: contentItems(req) };
  for (const key of PASSTHROUGH_KEYS) {
    if (Object.prototype.hasOwnProperty.call(req, key)) body[key] = req[key];
  }
  return {
    url: ctx.baseUrl + "/v1/video/generate",
    method: "POST",
    headers: { Authorization: "Bearer " + ctx.apiKey, "Content-Type": "application/json" },
    body: body,
  };
}

export function parseSubmitResponse(ctx, resp) {
  const body = resp.body || {};
  const taskId = trimmed(body.task_id || body.id);
  if (!taskId) throw new Error("task_id is empty");
  return { taskId: taskId, taskData: body };
}

export function buildQueryRequest(ctx) {
  return {
    url: ctx.baseUrl + "/v1/video/tasks/" + encodeURIComponent(ctx.taskId),
    method: "GET",
    headers: { Authorization: "Bearer " + ctx.apiKey },
  };
}

export function parseTaskResult(ctx, body) {
  const statuses = {
    queued: "QUEUED",
    pending: "QUEUED",
    running: "IN_PROGRESS",
    processing: "IN_PROGRESS",
    succeeded: "SUCCESS",
    failed: "FAILURE",
    cancelled: "FAILURE",
    canceled: "FAILURE",
  };
  const raw = trimmed((body || {}).status).toLowerCase();
  const mapped = statuses[raw];
  if (!mapped) return { status: "UNKNOWN", reason: "unrecognized status: " + raw };
  if (mapped === "FAILURE") {
    const error = (body || {}).error;
    const reason = typeof error === "string" ? error : trimmed((error || {}).message);
    return { status: "FAILURE", reason: reason || "task failed" };
  }
  return { status: mapped };
}

export function extractUsage(ctx) {
  const req = ctx.requestBody || {};
  const facts = { seconds: boundedDuration(req.duration) || DEFAULT_DURATION };
  // 上游白名单之外的写法不上报，否则整个提交会被计费事实校验打回。
  const resolution = normalizedResolution(req.resolution) || DEFAULT_RESOLUTION;
  if (resolution) facts.resolution = resolution;
  return facts;
}

export function extractUsageOnComplete(task, taskResult, body) {
  const facts = {};
  const seconds = boundedDuration(((body || {}).usage || {}).duration);
  if (seconds) facts.seconds = seconds;
  const resolution = normalizedResolution((body || {}).resolution);
  if (resolution) facts.resolution = resolution;
  return facts;
}

export function listArtifacts(task) {
  return task.status === "SUCCESS" && videoUrl(task.data) ? [{ key: "video", type: "video" }] : [];
}

export function buildContentRequest(ctx) {
  if (ctx.artifactKey !== "video") throw new Error("artifact_not_found");
  const url = videoUrl(ctx.data);
  if (!url) throw new Error("artifact_not_found");
  // 上游给的是 CDN 直链，不能带渠道 key 回源。
  return { url: url, method: ctx.clientRequest.method, credentialless: true };
}

package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// fish.audio 的私有端点。它们的请求/响应结构都不是 OpenAI 那套，
// 网关只做原样透传，不解析也不重建。
const (
	fishVoiceClonePath = "/model"                        // 创建音色模型（声音克隆），multipart
	fishTTSPath        = "/v1/tts"                       // 语音合成，JSON
	fishTTSTimestamped = "/v1/tts/stream/with-timestamp" // 语音合成 + 时间轴，SSE
)

// FishVoiceCloneHelper 处理声音克隆：网关 POST /v1/audio/voices → 上游 POST {base_url}/model。
// multipart 请求体原样透传（含 model 字段，fish 的表单解析会忽略它）。
// 按次计费：给模型配「模型固定价格」即为一次克隆的单价。
func FishVoiceCloneHelper(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	return fishPassthrough(c, info, fishVoiceClonePath, common.NewReplayableBodyReader(storage), 1)
}

// FishTTSHelper 处理语音合成：网关路径原样映射到上游同名端点
// （/v1/tts，以及带时间轴的 SSE 变体 /v1/tts/stream/with-timestamp）。
// 客户端发 fish 原生字段（text / reference_id / format …），额外带一个 model 字段
// 用于选渠道和计价；转发前会把 model 摘掉，其余字段原样透传。
// 两个端点是同一份合成工作，计费维度都取 text 的字符数，因此固定价格（按次）
// 和模型倍率（按量）两种配置都能用。
func FishTTSHelper(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	rawBody, err := storage.Bytes()
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	text := gjson.GetBytes(rawBody, "text").String()
	if strings.TrimSpace(text) == "" {
		return types.NewErrorWithStatusCode(fmt.Errorf("field text is required"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	upstreamBody, err := sjson.DeleteBytes(rawBody, "model")
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	upstreamPath := fishTTSPath
	if c.FullPath() == "/v1"+fishTTSTimestamped {
		upstreamPath = fishTTSTimestamped
	}
	return fishPassthrough(c, info, upstreamPath, bytes.NewReader(upstreamBody), utf8.RuneCountInString(text))
}

// fishPassthrough 把 body 原样转发到 {base_url}+upstreamPath，响应（含 Content-Type）原样回写，
// 成功后按 promptTokens 结算。
func fishPassthrough(c *gin.Context, info *relaycommon.RelayInfo, upstreamPath string, body io.Reader, promptTokens int) *types.NewAPIError {
	info.InitChannelMeta(c)

	requestURL := strings.TrimSuffix(info.ChannelBaseUrl, "/") + upstreamPath
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, requestURL, body)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	channel.ApplyUpstreamBodyMetadata(req, body)
	req.Header.Set("Authorization", "Bearer "+info.ApiKey)
	req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))

	client, err := service.GetHttpClientWithProxySettings(info.ChannelSetting.Proxy, info.ChannelSetting)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	logger.LogDebug(c, "fish passthrough request url: %s", relaycommon.SanitizeURLForLog(requestURL))
	resp, err := client.Do(req)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		newAPIError := service.RelayErrorHandler(c.Request.Context(), resp, true)
		service.ResetStatusCode(newAPIError, c.GetString("status_code_mapping"))
		return newAPIError
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		c.Writer.Header().Set("Content-Type", contentType)
	}
	// SSE 必须逐块下发：默认的 4KB 缓冲会把整条时间轴流攒到结束才吐给客户端。
	var dst io.Writer = c.Writer
	if strings.HasPrefix(contentType, "text/event-stream") {
		c.Writer.Header().Set("X-Accel-Buffering", "no")
		dst = flushingWriter{c.Writer}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err = io.Copy(dst, resp.Body); err != nil {
		return types.NewOpenAIError(fmt.Errorf("copy response body failed: %w", err), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	service.PostTextConsumeQuota(c, info, &dto.Usage{PromptTokens: promptTokens, TotalTokens: promptTokens}, nil)
	return nil
}

// flushingWriter 每写一块就 flush，用于把上游 SSE 实时透传给客户端。
type flushingWriter struct {
	writer gin.ResponseWriter
}

func (w flushingWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	w.writer.Flush()
	return n, err
}

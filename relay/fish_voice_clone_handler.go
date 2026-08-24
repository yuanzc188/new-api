package relay

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// fishVoiceCloneUpstreamPath 是 fish.audio 创建音色模型（声音克隆）的上游路径。
const fishVoiceCloneUpstreamPath = "/model"

// FishVoiceCloneHelper 处理 fish.audio 的声音克隆请求：
// 网关 POST /v1/audio/voices → 上游 POST {base_url}/model，请求体（multipart）原样透传。
//
// 计费走标准同步链路：请求里的 model 字段决定渠道分发与价格，
// 需要在「模型固定价格」中给该模型配置单价，即为一次克隆的价格（按次计费）。
func FishVoiceCloneHelper(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	info.InitChannelMeta(c)

	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	requestURL := strings.TrimSuffix(info.ChannelBaseUrl, "/") + fishVoiceCloneUpstreamPath
	requestBody := common.NewReplayableBodyReader(storage)
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, requestURL, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	channel.ApplyUpstreamBodyMetadata(req, requestBody)
	req.Header.Set("Authorization", "Bearer "+info.ApiKey)
	req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))

	client, err := service.GetHttpClientWithProxySettings(info.ChannelSetting.Proxy, info.ChannelSetting)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	logger.LogDebug(c, "fish voice clone request url: %s", relaycommon.SanitizeURLForLog(requestURL))
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

	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		c.Writer.Header().Set("Content-Type", contentType)
	}
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err = io.Copy(c.Writer, resp.Body); err != nil {
		return types.NewOpenAIError(fmt.Errorf("copy response body failed: %w", err), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	// 一次克隆算一次调用；固定价格模型下额度即为配置的单价。
	service.PostTextConsumeQuota(c, info, &dto.Usage{PromptTokens: 1, TotalTokens: 1}, nil)
	return nil
}

package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 两条 fish TTS 路由必须各自转发到上游的同名端点。带时间戳那条曾经因为常量拼接
// 错误（"/v1"+常量）永远匹配不上，静默降级成普通合成，客户端拿不到时间轴。
func TestFishTTSUpstreamPathFollowsRegisteredRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		// route 与 router/relay-router.go 中的注册路径保持一致（含 /v1 组前缀）。
		route string
		want  string
	}{
		{route: "/v1/tts", want: "/v1/tts"},
		{route: "/v1/tts/stream/with-timestamp", want: "/v1/tts/stream/with-timestamp"},
	}
	for _, testCase := range tests {
		t.Run(testCase.route, func(t *testing.T) {
			var got string
			engine := gin.New()
			engine.POST(testCase.route, func(c *gin.Context) {
				got = fishTTSUpstreamPath(c.FullPath())
			})
			engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, testCase.route, nil))
			require.NotEmpty(t, got)
			assert.Equal(t, testCase.want, got)
		})
	}

	// 未注册的路由回落到普通合成，不会拼出不存在的上游地址。
	assert.Equal(t, "/v1/tts", fishTTSUpstreamPath(""))
}

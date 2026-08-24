package sora

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 自建数字人上游的真实完成响应：seconds/duration 是数字而非字符串，
// 且视频直链在顶层 url 字段。
const digitalHumanCompletedBody = `{
  "id": "562d69cf1c5a480aa00c8b2c1f439ba4",
  "task_id": "562d69cf1c5a480aa00c8b2c1f439ba4",
  "object": "video",
  "model": "digital-human-audio",
  "status": "completed",
  "progress": 100,
  "created_at": 1787567985,
  "completed_at": 1787568051,
  "url": "https://cdn.example.com/outputs/562d69cf.mp4",
  "format": "mp4",
  "seconds": 12,
  "duration": 12,
  "metadata": { "seconds": 12, "duration": 12 },
  "error": null
}`

// 上游把 seconds 返回成数字时，整个响应必须仍能解析——否则任务永远停在
// 进行中，既不出结果也不结算。
func TestParseTaskResultAcceptsNumericSeconds(t *testing.T) {
	result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(digitalHumanCompletedBody))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, result.Status)
	assert.Equal(t, "https://cdn.example.com/outputs/562d69cf.mp4", result.Url)
}

func TestAdjustBillingOnCompleteWithDigitalHumanBody(t *testing.T) {
	task := &model.Task{Quota: 3000, Data: []byte(digitalHumanCompletedBody)}
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		OtherRatios:    map[string]float64{"seconds": defaultEstimateSeconds, "size": 1},
		PerCallBilling: true,
	}
	// 预扣 60 秒 3000，实际 12 秒 → 600，退回 2400
	assert.Equal(t, 600, (&TaskAdaptor{}).AdjustBillingOnComplete(task, &relaycommon.TaskInfo{}))
}

// 数字人（音频驱动）视频提交时无法得知真实时长，按 defaultEstimateSeconds 预扣，
// 完成时必须按上游返回的 seconds 多退少补。
func TestAdjustBillingOnCompleteScalesQuotaByActualSeconds(t *testing.T) {
	newTask := func(quota int, estimated float64, data string) *model.Task {
		task := &model.Task{Quota: quota, Data: []byte(data)}
		task.PrivateData.BillingContext = &model.TaskBillingContext{
			OtherRatios:    map[string]float64{"seconds": estimated, "size": 1},
			PerCallBilling: true,
		}
		return task
	}

	cases := []struct {
		name string
		task *model.Task
		want int
	}{
		{"实际时长更短则退款", newTask(3000, 60, `{"status":"completed","seconds":"12"}`), 600},
		{"实际时长更长则补扣", newTask(3000, 60, `{"status":"completed","seconds":"90"}`), 4500},
		{"上游返回数字类型同样生效", newTask(3000, 60, `{"status":"completed","seconds":12}`), 600},
		{"与预扣一致则不结算", newTask(3000, 60, `{"status":"completed","seconds":"60"}`), 0},
		{"上游不返回时长则保持预扣", newTask(3000, 60, `{"status":"completed"}`), 0},
		{"时长为 0 则保持预扣", newTask(3000, 60, `{"status":"completed","seconds":"0"}`), 0},
		{"超出上限的时长被钳制", newTask(60, 60, `{"status":"completed","seconds":99999999}`), 3600},
	}

	adaptor := &TaskAdaptor{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, adaptor.AdjustBillingOnComplete(tc.task, &relaycommon.TaskInfo{}))
		})
	}
}

func TestAdjustBillingOnCompleteWithoutBillingContext(t *testing.T) {
	adaptor := &TaskAdaptor{}
	task := &model.Task{Quota: 3000, Data: []byte(`{"seconds":"12"}`)}
	assert.Equal(t, 0, adaptor.AdjustBillingOnComplete(task, &relaycommon.TaskInfo{}))
}

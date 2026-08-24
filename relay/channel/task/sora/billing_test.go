package sora

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/assert"
)

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

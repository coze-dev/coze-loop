// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/webhook/notifications"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// envWebhookEnabled 兜底 feature flag（configer 未落地前使用）。
// EVALUATION_WEBHOOK_ENABLED=false / 0 / off 时短路,对齐 test_case 11。
// 缺省值 true —— 保持向后兼容,由部署侧显式关闭。
const envWebhookEnabledKey = "EVALUATION_WEBHOOK_ENABLED"

// IDispatcher webhook fan-out 抽象;与 pkg/webhook/dispatcher.Dispatcher.Dispatch 签名一致。
// 使用最小接口避免 domain/service 强依赖 pkg/webhook 包内部实现。
type IDispatcher interface {
	Dispatch(ctx context.Context, workspaceID, experimentID int64, event entity.NotificationTrigger, body []byte, rules []entity.NotificationRule) error
}

// IWebhookLifecycleHook 实验状态迁移后 webhook 通道触发钩子。
type IWebhookLifecycleHook interface {
	OnStatusChange(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) error
}

// WebhookLifecycleHook 将 ExptLifecycleEvent 转换为 webhook Dispatch 调用。
// 全字段可 nil / 零值 → 静默 no-op,保证 iter_21 前 wire 阶段不阻塞。
type WebhookLifecycleHook struct {
	dispatcher     IDispatcher
	featureEnabled func(ctx context.Context, workspaceID int64) bool
}

// NewWebhookLifecycleHook 供 wire DI 构造;dispatcher 可为 nil(下轮再注真实 impl)。
func NewWebhookLifecycleHook(dispatcher IDispatcher) *WebhookLifecycleHook {
	return &WebhookLifecycleHook{
		dispatcher:     dispatcher,
		featureEnabled: envFeatureEnabled,
	}
}

// OnStatusChange 主流程调用;nil-safe + 内部 goroutine 异步 dispatch,失败不阻塞主 status transition。
func (h *WebhookLifecycleHook) OnStatusChange(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) error {
	if h == nil || h.dispatcher == nil || event == nil || expt == nil {
		return nil
	}
	trigger, fire := statusToEvent(event.FromStatus, event.ToStatus)
	if !fire {
		return nil
	}
	if h.featureEnabled != nil && !h.featureEnabled(ctx, expt.SpaceID) {
		return nil
	}
	if expt.SpaceID == 0 {
		logs.CtxWarn(ctx, "webhook lifecycle hook skip: workspace_id=0 expt_id=%v", expt.ID)
		return nil
	}
	rules := expt.Notifications
	if rules == nil {
		rules = notifications.PRDDefault()
	}
	body, err := marshalCanonicalBody(expt, trigger)
	if err != nil {
		logs.CtxWarn(ctx, "webhook lifecycle hook marshal body failed: expt=%v err=%v", expt.ID, err)
		return nil
	}
	go func(workspaceID, experimentID int64, ev entity.NotificationTrigger, payload []byte, rr []entity.NotificationRule) {
		defer func() {
			if r := recover(); r != nil {
				logs.CtxError(context.Background(), "webhook dispatch panic: expt=%v event=%v recover=%v", experimentID, ev, r)
			}
		}()
		bgCtx := context.Background()
		if derr := h.dispatcher.Dispatch(bgCtx, workspaceID, experimentID, ev, payload, rr); derr != nil {
			logs.CtxError(bgCtx, "webhook dispatch failed: expt=%v event=%v err=%v", experimentID, ev, derr)
		}
	}(expt.SpaceID, expt.ID, trigger, body, rules)
	return nil
}

// statusToEvent 把实验状态迁移映射为 NotificationTrigger。
//   - Idle/Pending → Processing 首次进入: started (test_case 7)
//   - Processing → Processing 重入: 不 fire (test_case 8)
//   - * → Success: succeeded
//   - * → Failed: failed
//   - * → Terminated / SystemTerminated: terminated 合并 (test_case 7)
//   - 其它: 不 fire
func statusToEvent(from, to entity.ExptStatus) (entity.NotificationTrigger, bool) {
	switch to {
	case entity.ExptStatus_Processing:
		if from == entity.ExptStatus_Processing {
			return entity.NotificationTrigger_Unknown, false
		}
		return entity.NotificationTrigger_Started, true
	case entity.ExptStatus_Success:
		return entity.NotificationTrigger_Succeeded, true
	case entity.ExptStatus_Failed:
		return entity.NotificationTrigger_Failed, true
	case entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated:
		return entity.NotificationTrigger_Terminated, true
	default:
		return entity.NotificationTrigger_Unknown, false
	}
}

// marshalCanonicalBody 组装 webhook body(canonical JSON,test_case 29)。
// 字段顺序由 map 的 key 排序决定 —— encoding/json 对 map[string]* 按 key 升序输出。
func marshalCanonicalBody(expt *entity.Experiment, ev entity.NotificationTrigger) ([]byte, error) {
	payload := map[string]interface{}{
		"event":         eventNameForBody(ev),
		"experiment_id": strconv.FormatInt(expt.ID, 10),
		"status":        int64(expt.Status),
		"timestamp":     time.Now().Unix(),
		"workspace_id":  strconv.FormatInt(expt.SpaceID, 10),
	}
	if expt.Name != "" {
		payload["name"] = expt.Name
	}
	if expt.CreatedBy != "" {
		payload["created_by"] = expt.CreatedBy
	}
	return json.Marshal(payload)
}

// eventNameForBody 与 dispatcher.EventName 输出保持一致;此处独立实现避免 domain/service 引入 dispatcher 包。
func eventNameForBody(e entity.NotificationTrigger) string {
	switch e {
	case entity.NotificationTrigger_Started:
		return "started"
	case entity.NotificationTrigger_Succeeded:
		return "succeeded"
	case entity.NotificationTrigger_Failed:
		return "failed"
	case entity.NotificationTrigger_Terminated:
		return "terminated"
	default:
		return ""
	}
}

func envFeatureEnabled(_ context.Context, _ int64) bool {
	v := strings.TrimSpace(os.Getenv(envWebhookEnabledKey))
	if v == "" {
		return true
	}
	switch strings.ToLower(v) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}

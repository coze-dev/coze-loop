// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strconv"
	"strings"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

// 通知配置领域类型直接复用 IDL（kitex）生成类型，保证 BLOB 序列化形态与 IDL JSON 完全一致：
// {"rules":[{"condition":{"filters":{...}},"actions":[{"type":1,"webhook":{"urls":[...]}}]}]}
// 不引入内部中间表示，避免序列化漂移（见 api-design-semantics.md「BLOB/JSON 字段序列化格式」）。
type (
	NotificationConfig      = expt.NotificationConfig
	NotificationRule        = expt.NotificationRule
	NotificationCondition   = expt.NotificationCondition
	NotificationAction      = expt.NotificationAction
	WebhookAction           = expt.WebhookAction
	NotificationChannelType = expt.NotificationChannelType
)

const (
	NotificationChannelType_Unknown = expt.NotificationChannelType_Unknown
	NotificationChannelType_Webhook = expt.NotificationChannelType_Webhook
	NotificationChannelType_Feishu  = expt.NotificationChannelType_Feishu
)

// FilterField 中标识「实验状态」的字段类型。条件匹配时 field_type 等于该值表示按实验状态匹配。
const NotificationFilterFieldExptStatus = expt.FieldType_ExptStatus

// NotificationEvent 对外回调事件语义（payload `event` 字段取值）。
type NotificationEvent string

const (
	NotificationEventStarted    NotificationEvent = "started"
	NotificationEventSucceeded  NotificationEvent = "succeeded"
	NotificationEventFailed     NotificationEvent = "failed"
	NotificationEventTerminated NotificationEvent = "terminated"
)

// MapExptStatusToNotificationEvent 将实验状态映射为对外通知事件：
// processing→started / success→succeeded / failed→failed / terminated|system_terminated→terminated。
// 其余状态返回空字符串（不触发通知）。
func MapExptStatusToNotificationEvent(status ExptStatus) (NotificationEvent, bool) {
	switch status {
	case ExptStatus_Processing:
		return NotificationEventStarted, true
	case ExptStatus_Success:
		return NotificationEventSucceeded, true
	case ExptStatus_Failed:
		return NotificationEventFailed, true
	case ExptStatus_Terminated, ExptStatus_SystemTerminated:
		return NotificationEventTerminated, true
	default:
		return "", false
	}
}

// DefaultNotificationConfig 默认通知配置（向前兼容兜底）：
// 通知条件 = 实验状态 / 包含(In) / [started(processing), failed, success, terminated, system_terminated]，
// 动作 = 飞书✅、webhook◻️。
//
// 说明：PRD 默认配置字面为 [started, failed, success]；但既有飞书链路（design.md Context / R1
// 「默认配置严格对齐现状」）在 finished 时对 Success/Failed/Terminated/SystemTerminated 均发卡片。
// 为不破坏既有 terminated 飞书语义（向前兼容），默认条件值在 PRD 三态基础上补充 terminated/system_terminated，
// 二者归一为对外 event=terminated（见 MapExptStatusToNotificationEvent）。
func DefaultNotificationConfig() *NotificationConfig {
	statusValues := []int64{
		int64(ExptStatus_Processing),
		int64(ExptStatus_Failed),
		int64(ExptStatus_Success),
		int64(ExptStatus_Terminated),
		int64(ExptStatus_SystemTerminated),
	}
	valBytes, _ := json.Marshal(statusValues)
	return &NotificationConfig{
		Rules: []*NotificationRule{
			{
				Condition: &NotificationCondition{
					Filters: &expt.Filters{
						FilterConditions: []*expt.FilterCondition{
							{
								Field:    &expt.FilterField{FieldType: NotificationFilterFieldExptStatus},
								Operator: expt.FilterOperatorType_In,
								Value:    string(valBytes),
							},
						},
					},
				},
				Actions: []*NotificationAction{
					{Type: ptrChannel(NotificationChannelType_Feishu)},
				},
			},
		},
	}
}

func ptrChannel(t NotificationChannelType) *NotificationChannelType { return &t }

// parseStatusValues 解析 FilterCondition.Value 为状态值列表。
// 兼容两种形态：JSON 数组（`[3,11,12]` 或 `["3","11"]`）、逗号分隔（`3,11,12`）。
func parseStatusValues(value string) []int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	// 优先尝试 JSON 数组（int 或 string 元素）。
	var ints []int64
	if err := json.Unmarshal([]byte(value), &ints); err == nil {
		return ints
	}
	var strs []string
	if err := json.Unmarshal([]byte(value), &strs); err == nil {
		res := make([]int64, 0, len(strs))
		for _, s := range strs {
			if v, e := strconv.ParseInt(strings.TrimSpace(s), 10, 64); e == nil {
				res = append(res, v)
			}
		}
		return res
	}
	// 回退到逗号分隔。
	parts := strings.Split(value, ",")
	res := make([]int64, 0, len(parts))
	for _, p := range parts {
		if v, e := strconv.ParseInt(strings.TrimSpace(p), 10, 64); e == nil {
			res = append(res, v)
		}
	}
	return res
}

// MatchNotificationCondition 判断给定实验状态是否命中通知条件。
// 本期 field 固定为「实验状态」，operator ∈ {In, NotIn}，value 为状态多选。
// 多个 filter_condition 之间按 AND 处理（本期默认单条件，AND 不影响）。
// 空 condition / 空 filters 视为「无条件命中」。
func MatchNotificationCondition(cond *NotificationCondition, status ExptStatus) bool {
	if cond == nil || cond.Filters == nil || len(cond.Filters.FilterConditions) == 0 {
		return true
	}
	for _, fc := range cond.Filters.FilterConditions {
		if fc == nil {
			continue
		}
		// 本期仅支持实验状态字段；其他已知字段类型忽略（不参与匹配，向前兼容）。
		// FieldType 为 0(Unknown) 时按实验状态处理（兼容未显式设置 field 的配置）。
		if fc.Field != nil && fc.Field.FieldType != expt.FieldType_Unknown && fc.Field.FieldType != NotificationFilterFieldExptStatus {
			continue
		}
		values := parseStatusValues(fc.Value)
		contains := false
		for _, v := range values {
			if v == int64(status) {
				contains = true
				break
			}
		}
		switch fc.Operator {
		case expt.FilterOperatorType_In:
			if !contains {
				return false
			}
		case expt.FilterOperatorType_NotIn:
			if contains {
				return false
			}
		default:
			// 未知 operator：本期不支持，按不命中处理（向前兼容 default 分支不 panic）。
			return false
		}
	}
	return true
}

// NotificationProgress webhook 回调 payload 中的 progress（turn 维度，见 design.md D9）。
type NotificationProgress struct {
	Total     int64 `json:"total"`
	Succeeded int64 `json:"succeeded"`
	Failed    int64 `json:"failed"`
}

// NotificationExperiment webhook 回调 payload 中的 experiment 字段。
type NotificationExperiment struct {
	ID       string                `json:"id"`
	Name     string                `json:"name"`
	Status   string                `json:"status"`
	Progress *NotificationProgress `json:"progress"`
}

// WebhookPayload webhook 回调 JSON body（仅基础字段，无 metrics / result_url，见决策1）。
type WebhookPayload struct {
	DeliveryID string                  `json:"delivery_id"`
	Event      string                  `json:"event"`
	Timestamp  string                  `json:"timestamp"` // ISO8601
	Experiment *NotificationExperiment `json:"experiment"`
}

// WebhookDeliveryEvent webhook 投递 MQ 消息体（首发与重试共用，attempt 区分）。
// delivery_id 在重试时保持不变（at-least-once + 业务幂等，见决策6 / spec）。
type WebhookDeliveryEvent struct {
	DeliveryID string          `json:"delivery_id"`
	SpaceID    int64           `json:"space_id"`
	ExptID     int64           `json:"expt_id"`
	URL        string          `json:"url"`     // 单个目标 URL（多 URL 各自独立投递/重试）
	Payload    *WebhookPayload `json:"payload"` // 投递 body；重试时不变
	Attempt    int32           `json:"attempt"` // 当前投递次数：0=首发，1/2/3=第 N 次重试
}

const (
	// WebhookMaxAttempt 最大重试次数（不含首发）：共 4 次投递（首发 + 3 次重试）。
	WebhookMaxAttempt int32 = 3
)

// WebhookRetryDelaySeconds 返回第 attempt 次重试的退避秒数：1min → 5min → 30min。
// attempt 入参为「即将进行的重试序号」（1/2/3）。返回 0 表示无对应退避（不应重试）。
func WebhookRetryDelaySeconds(attempt int32) int64 {
	switch attempt {
	case 1:
		return 60
	case 2:
		return 5 * 60
	case 3:
		return 30 * 60
	default:
		return 0
	}
}

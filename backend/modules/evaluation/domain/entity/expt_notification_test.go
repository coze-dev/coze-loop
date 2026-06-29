// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// statusValueIn 包含运算符的条件构造助手。
func filterIn(values ...NotificationStatusValue) *NotificationFilterCondition {
	return &NotificationFilterCondition{
		FieldType: FieldType_ExptStatus,
		Operator:  NotificationFilterOperatorType_In,
		Values:    values,
	}
}

func filterNotIn(values ...NotificationStatusValue) *NotificationFilterCondition {
	return &NotificationFilterCondition{
		FieldType: FieldType_ExptStatus,
		Operator:  NotificationFilterOperatorType_NotIn,
		Values:    values,
	}
}

// ----- 条件值 <-> 状态映射（含「被终止」合并 Terminated + SystemTerminated）-----

func TestNotificationStatusValueToExptStatuses_TerminatedMerged(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []ExptStatus{ExptStatus_Processing}, notificationStatusValueToExptStatuses(NotificationStatusValue_Started))
	assert.Equal(t, []ExptStatus{ExptStatus_Success}, notificationStatusValueToExptStatuses(NotificationStatusValue_Succeeded))
	assert.Equal(t, []ExptStatus{ExptStatus_Failed}, notificationStatusValueToExptStatuses(NotificationStatusValue_Failed))
	// 被终止：手动终止 + 系统终止合并。
	assert.ElementsMatch(t,
		[]ExptStatus{ExptStatus_Terminated, ExptStatus_SystemTerminated},
		notificationStatusValueToExptStatuses(NotificationStatusValue_Terminated))
	// 未知条件值 -> nil。
	assert.Nil(t, notificationStatusValueToExptStatuses(NotificationStatusValue(999)))
}

func TestExptStatusToNotificationStatusValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		status   ExptStatus
		wantVal  NotificationStatusValue
		wantOk   bool
	}{
		{ExptStatus_Processing, NotificationStatusValue_Started, true},
		{ExptStatus_Success, NotificationStatusValue_Succeeded, true},
		{ExptStatus_Failed, NotificationStatusValue_Failed, true},
		{ExptStatus_Terminated, NotificationStatusValue_Terminated, true},
		{ExptStatus_SystemTerminated, NotificationStatusValue_Terminated, true},
		// 中间过渡态无对应条件值。
		{ExptStatus_Pending, 0, false},
		{ExptStatus_Terminating, 0, false},
		{ExptStatus_Draining, 0, false},
	}
	for _, c := range cases {
		gotVal, gotOk := ExptStatusToNotificationStatusValue(c.status)
		assert.Equal(t, c.wantOk, gotOk, "status=%d", c.status)
		assert.Equal(t, c.wantVal, gotVal, "status=%d", c.status)
	}
}

// ----- NotificationFilterCondition.Match -----

func TestFilterCondition_Match_In(t *testing.T) {
	t.Parallel()

	cond := filterIn(NotificationStatusValue_Succeeded, NotificationStatusValue_Failed)
	assert.True(t, cond.Match(ExptStatus_Success))
	assert.True(t, cond.Match(ExptStatus_Failed))
	assert.False(t, cond.Match(ExptStatus_Processing))
	// 中间过渡态：In 视为不命中。
	assert.False(t, cond.Match(ExptStatus_Terminating))
}

func TestFilterCondition_Match_In_TerminatedCoversBoth(t *testing.T) {
	t.Parallel()

	cond := filterIn(NotificationStatusValue_Terminated)
	assert.True(t, cond.Match(ExptStatus_Terminated))
	assert.True(t, cond.Match(ExptStatus_SystemTerminated))
	assert.False(t, cond.Match(ExptStatus_Success))
}

func TestFilterCondition_Match_NotIn(t *testing.T) {
	t.Parallel()

	cond := filterNotIn(NotificationStatusValue_Succeeded)
	assert.False(t, cond.Match(ExptStatus_Success))
	assert.True(t, cond.Match(ExptStatus_Failed))
	// 中间过渡态：NotIn 视为命中（不在集合内）。
	assert.True(t, cond.Match(ExptStatus_Terminating))
}

func TestFilterCondition_Match_NilAndUnknownOperator(t *testing.T) {
	t.Parallel()

	var nilCond *NotificationFilterCondition
	assert.False(t, nilCond.Match(ExptStatus_Success))

	unknown := &NotificationFilterCondition{
		FieldType: FieldType_ExptStatus,
		Operator:  NotificationFilterOperatorType(999),
		Values:    []NotificationStatusValue{NotificationStatusValue_Succeeded},
	}
	assert.False(t, unknown.Match(ExptStatus_Success))
}

// ----- DefaultNotificationConf / null-safe 默认值 -----

func TestDefaultNotificationConf(t *testing.T) {
	t.Parallel()

	def := DefaultNotificationConf()
	assert.NotNil(t, def.Filter)
	assert.Equal(t, FieldType_ExptStatus, def.Filter.FieldType)
	assert.Equal(t, NotificationFilterOperatorType_In, def.Filter.Operator)
	assert.ElementsMatch(t,
		[]NotificationStatusValue{
			NotificationStatusValue_Started,
			NotificationStatusValue_Succeeded,
			NotificationStatusValue_Failed,
		}, def.Filter.Values)
	// 飞书开启、Webhook 关闭（向后兼容现有飞书行为）。
	assert.True(t, def.Feishu.Enable)
	assert.False(t, def.Webhook.Enable)
}

func TestGetNotificationConfOrDefault_Nil(t *testing.T) {
	t.Parallel()

	var conf *NotificationConf
	got := conf.GetNotificationConfOrDefault()
	assert.NotNil(t, got)
	assert.True(t, got.Feishu.Enable)
	assert.False(t, got.Webhook.Enable)
}

func TestGetNotificationConfOrDefault_PartialFieldsBackfilled(t *testing.T) {
	t.Parallel()

	// 仅配置 webhook，filter / feishu 为 nil -> 应被默认值回填。
	conf := &NotificationConf{
		Webhook: &WebhookNotificationConf{Enable: true, URLs: []string{"https://x"}},
	}
	got := conf.GetNotificationConfOrDefault()
	assert.NotNil(t, got.Filter)
	assert.NotNil(t, got.Feishu)
	assert.True(t, got.Feishu.Enable)
	// 自有 webhook 字段保留。
	assert.True(t, got.Webhook.Enable)
	assert.Equal(t, []string{"https://x"}, got.Webhook.URLs)
}

// ----- ShouldNotifyWebhook -----

func TestShouldNotifyWebhook_NilConfDefaultsToDisabled(t *testing.T) {
	t.Parallel()

	var conf *NotificationConf
	// 默认 webhook 关闭：任何状态都不投递。
	assert.False(t, conf.ShouldNotifyWebhook(ExptStatus_Success))
	assert.False(t, conf.ShouldNotifyWebhook(ExptStatus_Processing))
}

func TestShouldNotifyWebhook_EnabledAndMatched(t *testing.T) {
	t.Parallel()

	conf := &NotificationConf{
		Filter:  filterIn(NotificationStatusValue_Started, NotificationStatusValue_Succeeded),
		Webhook: &WebhookNotificationConf{Enable: true, URLs: []string{"https://x"}},
	}
	assert.True(t, conf.ShouldNotifyWebhook(ExptStatus_Processing))
	assert.True(t, conf.ShouldNotifyWebhook(ExptStatus_Success))
	// 未在条件集合内 -> 不投递。
	assert.False(t, conf.ShouldNotifyWebhook(ExptStatus_Failed))
}

func TestShouldNotifyWebhook_EnabledButNotMatched(t *testing.T) {
	t.Parallel()

	conf := &NotificationConf{
		Filter:  filterIn(NotificationStatusValue_Failed),
		Webhook: &WebhookNotificationConf{Enable: true, URLs: []string{"https://x"}},
	}
	assert.False(t, conf.ShouldNotifyWebhook(ExptStatus_Success))
}

func TestShouldNotifyWebhook_DisabledChannel(t *testing.T) {
	t.Parallel()

	conf := &NotificationConf{
		Filter:  filterIn(NotificationStatusValue_Succeeded),
		Webhook: &WebhookNotificationConf{Enable: false, URLs: []string{"https://x"}},
	}
	assert.False(t, conf.ShouldNotifyWebhook(ExptStatus_Success))
}

// ----- ShouldNotifyFeishu（条件化 + 向后兼容）-----

func TestShouldNotifyFeishu_NilConfBackwardCompatible(t *testing.T) {
	t.Parallel()

	// 历史实验（notification_conf == nil）：默认飞书开启 + filter 含 Started/Succeeded/Failed。
	var conf *NotificationConf
	assert.True(t, conf.ShouldNotifyFeishu(ExptStatus_Success))
	assert.True(t, conf.ShouldNotifyFeishu(ExptStatus_Failed))
	assert.True(t, conf.ShouldNotifyFeishu(ExptStatus_Processing))
	// Terminated 不在默认 filter 内 -> 条件层面不命中（飞书发卡片的终态收口由 base handler 控制）。
	assert.False(t, conf.ShouldNotifyFeishu(ExptStatus_Terminated))
}

func TestShouldNotifyFeishu_ConditionDriven(t *testing.T) {
	t.Parallel()

	conf := &NotificationConf{
		Filter: filterIn(NotificationStatusValue_Terminated),
		Feishu: &FeishuNotificationConf{Enable: true},
	}
	assert.True(t, conf.ShouldNotifyFeishu(ExptStatus_Terminated))
	assert.True(t, conf.ShouldNotifyFeishu(ExptStatus_SystemTerminated))
	assert.False(t, conf.ShouldNotifyFeishu(ExptStatus_Success))
}

func TestShouldNotifyFeishu_DisabledChannel(t *testing.T) {
	t.Parallel()

	conf := &NotificationConf{
		Filter: filterIn(NotificationStatusValue_Succeeded),
		Feishu: &FeishuNotificationConf{Enable: false},
	}
	assert.False(t, conf.ShouldNotifyFeishu(ExptStatus_Success))
}

// ----- Validate（Webhook 启用须有非空 URL）-----

func TestNotificationConf_Validate(t *testing.T) {
	t.Parallel()

	// nil 配置恒合法。
	var nilConf *NotificationConf
	assert.NoError(t, nilConf.Validate())

	// webhook 未启用恒合法（即使无 URL）。
	assert.NoError(t, (&NotificationConf{Webhook: &WebhookNotificationConf{Enable: false}}).Validate())

	// 启用但 URL 全空 -> 报错。
	err := (&NotificationConf{Webhook: &WebhookNotificationConf{Enable: true, URLs: []string{"", "   "}}}).Validate()
	assert.ErrorIs(t, err, ErrWebhookURLRequired)

	// 启用但 URLs 为空切片 -> 报错。
	err = (&NotificationConf{Webhook: &WebhookNotificationConf{Enable: true}}).Validate()
	assert.ErrorIs(t, err, ErrWebhookURLRequired)

	// 启用且至少一个非空 URL -> 合法。
	assert.NoError(t, (&NotificationConf{Webhook: &WebhookNotificationConf{Enable: true, URLs: []string{"  ", "https://ok"}}}).Validate())
}

func TestWebhookNotificationConf_Validate_Nil(t *testing.T) {
	t.Parallel()

	var w *WebhookNotificationConf
	assert.NoError(t, w.Validate())
}

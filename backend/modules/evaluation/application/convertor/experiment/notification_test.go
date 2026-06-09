// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestNotificationTriggerToStatusValues(t *testing.T) {
	tests := []struct {
		name    string
		trigger string
		want    []entity.ExptStatus
	}{
		{name: "started", trigger: "started", want: []entity.ExptStatus{entity.ExptStatus_Processing}},
		{name: "succeeded", trigger: "succeeded", want: []entity.ExptStatus{entity.ExptStatus_Success}},
		{name: "failed", trigger: "failed", want: []entity.ExptStatus{entity.ExptStatus_Failed}},
		{name: "terminated covers both", trigger: "terminated", want: []entity.ExptStatus{entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated}},
		{name: "case-insensitive + trim", trigger: "  Succeeded  ", want: []entity.ExptStatus{entity.ExptStatus_Success}},
		{name: "unknown returns nil", trigger: "bogus", want: nil},
		{name: "empty returns nil", trigger: "", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NotificationTriggerToStatusValues(tt.trigger))
		})
	}
}

func TestEncodeExptStatusValues(t *testing.T) {
	assert.Equal(t, "", EncodeExptStatusValues(nil))
	assert.Equal(t, "", EncodeExptStatusValues([]entity.ExptStatus{}))
	assert.Equal(t, "11", EncodeExptStatusValues([]entity.ExptStatus{entity.ExptStatus_Success}))
	assert.Equal(t, "13,14", EncodeExptStatusValues([]entity.ExptStatus{entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated}))
}

func TestNotificationConfDTO2DO(t *testing.T) {
	t.Run("nil dto returns nil", func(t *testing.T) {
		do, err := NotificationConfDTO2DO(nil)
		assert.NoError(t, err)
		assert.Nil(t, do)
	})

	t.Run("ExptStatus In condition + webhook + feishu actions", func(t *testing.T) {
		dto := &domain_expt.ExptNotificationConf{Rules: []*domain_expt.NotificationRule{
			{
				Filters: &domain_expt.Filters{
					LogicOp: gptr.Of(domain_expt.FilterLogicOp_And),
					FilterConditions: []*domain_expt.FilterCondition{
						{
							Field:    &domain_expt.FilterField{FieldType: domain_expt.FieldType_ExptStatus},
							Operator: domain_expt.FilterOperatorType_In,
							Value:    "11",
						},
					},
				},
				Actions: []*domain_expt.NotificationAction{
					{Type: domain_expt.NotificationActionType_Webhook, Webhook: &domain_expt.WebhookNotificationConf{Urls: []string{"https://a.com/h"}}},
					{Type: domain_expt.NotificationActionType_Feishu, Feishu: &domain_expt.FeishuNotificationConf{}},
				},
			},
		}}
		do, err := NotificationConfDTO2DO(dto)
		assert.NoError(t, err)
		assert.Len(t, do.Rules, 1)
		rule := do.Rules[0]
		assert.Len(t, rule.Conditions, 1)
		assert.Equal(t, entity.NotificationFilterOperator_In, rule.Conditions[0].Operator)
		assert.Equal(t, []entity.ExptStatus{entity.ExptStatus_Success}, rule.Conditions[0].StatusValues)
		assert.Len(t, rule.Actions, 2)
		assert.Equal(t, entity.NotificationActionType_Webhook, rule.Actions[0].Type)
		assert.Equal(t, []string{"https://a.com/h"}, rule.Actions[0].Webhook.URLs)
		assert.Equal(t, entity.NotificationActionType_Feishu, rule.Actions[1].Type)
		assert.NotNil(t, rule.Actions[1].Feishu)
	})

	t.Run("non-ExptStatus field skipped", func(t *testing.T) {
		dto := &domain_expt.ExptNotificationConf{Rules: []*domain_expt.NotificationRule{
			{
				Filters: &domain_expt.Filters{
					FilterConditions: []*domain_expt.FilterCondition{
						{Field: &domain_expt.FilterField{FieldType: domain_expt.FieldType_CreatorBy}, Operator: domain_expt.FilterOperatorType_In, Value: "1"},
					},
				},
			},
		}}
		do, err := NotificationConfDTO2DO(dto)
		assert.NoError(t, err)
		assert.Len(t, do.Rules, 1)
		assert.Empty(t, do.Rules[0].Conditions)
	})

	t.Run("unknown operator skipped", func(t *testing.T) {
		dto := &domain_expt.ExptNotificationConf{Rules: []*domain_expt.NotificationRule{
			{
				Filters: &domain_expt.Filters{
					FilterConditions: []*domain_expt.FilterCondition{
						{Field: &domain_expt.FilterField{FieldType: domain_expt.FieldType_ExptStatus}, Operator: domain_expt.FilterOperatorType_Equal, Value: "11"},
					},
				},
			},
		}}
		do, err := NotificationConfDTO2DO(dto)
		assert.NoError(t, err)
		assert.Empty(t, do.Rules[0].Conditions)
	})

	t.Run("webhook nil conf yields empty conf", func(t *testing.T) {
		dto := &domain_expt.ExptNotificationConf{Rules: []*domain_expt.NotificationRule{
			{Actions: []*domain_expt.NotificationAction{{Type: domain_expt.NotificationActionType_Webhook}}},
		}}
		do, err := NotificationConfDTO2DO(dto)
		assert.NoError(t, err)
		assert.NotNil(t, do.Rules[0].Actions[0].Webhook)
		assert.Empty(t, do.Rules[0].Actions[0].Webhook.URLs)
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		dto := &domain_expt.ExptNotificationConf{Rules: []*domain_expt.NotificationRule{
			{
				Filters: &domain_expt.Filters{
					FilterConditions: []*domain_expt.FilterCondition{
						{Field: &domain_expt.FilterField{FieldType: domain_expt.FieldType_ExptStatus}, Operator: domain_expt.FilterOperatorType_In, Value: "not-int"},
					},
				},
			},
		}}
		_, err := NotificationConfDTO2DO(dto)
		assert.Error(t, err)
	})
}

func TestNotificationConfDO2DTO(t *testing.T) {
	t.Run("nil do returns nil", func(t *testing.T) {
		assert.Nil(t, NotificationConfDO2DTO(nil))
	})

	t.Run("round-trip DTO->DO->DTO preserves status/actions", func(t *testing.T) {
		do := &entity.NotificationConf{Rules: []*entity.NotificationRule{
			{
				Conditions: []*entity.NotificationFilterCondition{
					{Operator: entity.NotificationFilterOperator_In, StatusValues: []entity.ExptStatus{entity.ExptStatus_Success}},
				},
				Actions: []*entity.NotificationAction{
					{Type: entity.NotificationActionType_Webhook, Webhook: &entity.NotificationWebhookConf{URLs: []string{"https://a.com/h"}}},
					{Type: entity.NotificationActionType_Feishu, Feishu: &entity.NotificationFeishuConf{}},
				},
			},
		}}
		dto := NotificationConfDO2DTO(do)
		assert.Len(t, dto.Rules, 1)
		assert.Equal(t, domain_expt.FilterLogicOp_And, dto.Rules[0].Filters.GetLogicOp())
		assert.Len(t, dto.Rules[0].Filters.FilterConditions, 1)
		c := dto.Rules[0].Filters.FilterConditions[0]
		assert.Equal(t, domain_expt.FieldType_ExptStatus, c.Field.FieldType)
		assert.Equal(t, domain_expt.FilterOperatorType_In, c.Operator)
		assert.Equal(t, "11", c.Value)
		assert.Equal(t, []string{"https://a.com/h"}, dto.Rules[0].Actions[0].Webhook.Urls)
		assert.NotNil(t, dto.Rules[0].Actions[1].Feishu)

		// reverse back
		back, err := NotificationConfDTO2DO(dto)
		assert.NoError(t, err)
		assert.Equal(t, do.Rules[0].Conditions[0].StatusValues, back.Rules[0].Conditions[0].StatusValues)
	})

	t.Run("rule without conditions has nil filters", func(t *testing.T) {
		do := &entity.NotificationConf{Rules: []*entity.NotificationRule{
			{Actions: []*entity.NotificationAction{{Type: entity.NotificationActionType_Feishu, Feishu: &entity.NotificationFeishuConf{}}}},
		}}
		dto := NotificationConfDO2DTO(do)
		assert.Nil(t, dto.Rules[0].Filters)
	})
}

func TestMapFilterOperator(t *testing.T) {
	assert.Equal(t, entity.NotificationFilterOperator_In, mapFilterOperatorDTO2DO(domain_expt.FilterOperatorType_In))
	assert.Equal(t, entity.NotificationFilterOperator_NotIn, mapFilterOperatorDTO2DO(domain_expt.FilterOperatorType_NotIn))
	assert.Equal(t, entity.NotificationFilterOperator_Unknown, mapFilterOperatorDTO2DO(domain_expt.FilterOperatorType_Equal))

	assert.Equal(t, domain_expt.FilterOperatorType_In, mapFilterOperatorDO2DTO(entity.NotificationFilterOperator_In))
	assert.Equal(t, domain_expt.FilterOperatorType_NotIn, mapFilterOperatorDO2DTO(entity.NotificationFilterOperator_NotIn))
	assert.Equal(t, domain_expt.FilterOperatorType_Unknown, mapFilterOperatorDO2DTO(entity.NotificationFilterOperator_Unknown))
}

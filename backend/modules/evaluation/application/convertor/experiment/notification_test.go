// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/stretchr/testify/assert"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func sampleNotificationConf() *entity.NotificationConf {
	return &entity.NotificationConf{
		Filter: &entity.NotificationFilterCondition{
			FieldType: entity.FieldType_ExptStatus,
			Operator:  entity.NotificationFilterOperatorType_NotIn,
			Values: []entity.NotificationStatusValue{
				entity.NotificationStatusValue_Started,
				entity.NotificationStatusValue_Succeeded,
				entity.NotificationStatusValue_Failed,
			},
		},
		Webhook: &entity.WebhookNotificationConf{Enable: true, URLs: []string{"https://a.example", "https://b.example"}},
		Feishu:  &entity.FeishuNotificationConf{Enable: true},
	}
}

func TestNotificationConf_DomainRoundTrip(t *testing.T) {
	in := sampleNotificationConf()
	dto := NotificationConfDO2DTO(in)
	assert.NotNil(t, dto)
	assert.True(t, dto.GetWebhook().GetEnable())
	assert.Equal(t, []string{"https://a.example", "https://b.example"}, dto.GetWebhook().GetUrls())
	assert.True(t, dto.GetFeishu().GetEnable())
	// filter condition: ExptStatus + NotIn + value 编码
	conds := dto.GetFilter().GetFilterConditions()
	assert.Len(t, conds, 1)
	assert.Equal(t, domain_expt.FieldType_ExptStatus, conds[0].GetField().GetFieldType())
	assert.Equal(t, domain_expt.FilterOperatorType_NotIn, conds[0].GetOperator())
	assert.Equal(t, "1,2,3", conds[0].GetValue())

	out := NotificationConfDTO2DO(dto)
	assert.Equal(t, in.Filter.Operator, out.Filter.Operator)
	assert.Equal(t, in.Filter.Values, out.Filter.Values)
	assert.Equal(t, in.Webhook.Enable, out.Webhook.Enable)
	assert.Equal(t, in.Webhook.URLs, out.Webhook.URLs)
	assert.Equal(t, in.Feishu.Enable, out.Feishu.Enable)
}

func TestNotificationConf_OpenAPIRoundTrip(t *testing.T) {
	in := sampleNotificationConf()
	dto := OpenAPINotificationConfDO2DTO(in)
	assert.NotNil(t, dto)
	conds := dto.GetFilter().GetFilterConditions()
	assert.Len(t, conds, 1)
	assert.Equal(t, notificationFieldKeyExptStatus, conds[0].GetField().GetFieldType())
	assert.Equal(t, notificationOperatorNotIn, conds[0].GetOperator())
	assert.Equal(t, "1,2,3", conds[0].GetValue())

	out := OpenAPINotificationConfDTO2DO(dto)
	assert.Equal(t, in.Filter.Operator, out.Filter.Operator)
	assert.Equal(t, in.Filter.Values, out.Filter.Values)
	assert.Equal(t, in.Webhook.Enable, out.Webhook.Enable)
	assert.Equal(t, in.Webhook.URLs, out.Webhook.URLs)
	assert.Equal(t, in.Feishu.Enable, out.Feishu.Enable)
}

func TestNotificationConf_OpenAPIToDomainBridge(t *testing.T) {
	openapiDTO := &openapiExperiment.ExptNotificationConf{
		Filter: &openapiExperiment.Filters{
			FilterConditions: []*openapiExperiment.FilterCondition{
				{
					Field:    &openapiExperiment.FilterField{FieldType: ptrStr(notificationFieldKeyExptStatus)},
					Operator: ptrStr(notificationOperatorIn),
					Value:    ptrStr("2,4"),
				},
			},
		},
		Webhook: &openapiExperiment.WebhookNotificationConf{Enable: ptrBool(true), Urls: []string{"https://x.example"}},
	}
	domainDTO := OpenAPINotificationConfDTO2DomainDTO(openapiDTO)
	assert.NotNil(t, domainDTO)
	conds := domainDTO.GetFilter().GetFilterConditions()
	assert.Len(t, conds, 1)
	assert.Equal(t, domain_expt.FieldType_ExptStatus, conds[0].GetField().GetFieldType())
	assert.Equal(t, domain_expt.FilterOperatorType_In, conds[0].GetOperator())
	assert.Equal(t, "2,4", conds[0].GetValue())
	assert.True(t, domainDTO.GetWebhook().GetEnable())
	assert.Equal(t, []string{"https://x.example"}, domainDTO.GetWebhook().GetUrls())
	// 未配置 feishu -> nil（向后兼容由 entity 层兜底）
	assert.Nil(t, domainDTO.GetFeishu())
}

func TestNotificationConf_NilSafe(t *testing.T) {
	assert.Nil(t, NotificationConfDO2DTO(nil))
	assert.Nil(t, NotificationConfDTO2DO(nil))
	assert.Nil(t, OpenAPINotificationConfDO2DTO(nil))
	assert.Nil(t, OpenAPINotificationConfDTO2DO(nil))
	assert.Nil(t, OpenAPINotificationConfDTO2DomainDTO(nil))
	// 空 filter（无 ExptStatus condition）-> filter nil
	dto := NotificationConfDO2DTO(&entity.NotificationConf{Webhook: &entity.WebhookNotificationConf{Enable: false}})
	assert.NotNil(t, dto)
	assert.Nil(t, dto.GetFilter())
}

func ptrStr(s string) *string { return &s }
func ptrBool(b bool) *bool    { return &b }

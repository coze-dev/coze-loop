// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	domainExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ---------------------------------------------------------------------------
// TestSubmitExperimentRequest_NotificationConf
// ---------------------------------------------------------------------------

func TestSubmitExperimentRequest_NotificationConf(t *testing.T) {
	t.Parallel()

	t.Run("normal: full config with filter, webhook and feishu", func(t *testing.T) {
		t.Parallel()
		logicOp := domainExpt.FilterLogicOp_And
		conf := &domainExpt.ExptNotificationConf{
			Filter: &domainExpt.Filters{
				LogicOp: &logicOp,
				FilterConditions: []*domainExpt.FilterCondition{
					{
						Field: &domainExpt.FilterField{
							FieldType: domainExpt.FieldType(3), // ExptStatus
							FieldKey:  gptr.Of("status"),
						},
						Operator: domainExpt.FilterOperatorType(7), // In
						Value:    `["11","12"]`,
					},
				},
			},
			Webhook: &domainExpt.WebhookNotificationConf{
				Enable: true,
				Urls:   gptr.Of("https://example.com/webhook"),
			},
			FeishuNotification: &domainExpt.FeishuNotificationConf{
				Enable: true,
				UserID: gptr.Of("ou_xxx"),
			},
		}

		got, err := NotificationConfDTO2DO(conf)
		assert.NoError(t, err)
		if assert.NotNil(t, got) {
			if assert.NotNil(t, got.Filter) {
				if assert.NotNil(t, got.Filter.LogicOp) {
					assert.Equal(t, entity.FilterLogicOp_And, *got.Filter.LogicOp)
				}
				if assert.Len(t, got.Filter.FilterConditions, 1) {
					cond := got.Filter.FilterConditions[0]
					assert.Equal(t, entity.NotificationOperatorType_In, cond.Operator)
					assert.Equal(t, `["11","12"]`, cond.Value)
					if assert.NotNil(t, cond.Field) {
						assert.Equal(t, entity.NotificationFieldType_ExptStatus, cond.Field.FieldType)
						assert.Equal(t, "status", *cond.Field.FieldKey)
					}
				}
			}
			if assert.NotNil(t, got.Webhook) {
				assert.True(t, got.Webhook.Enable)
				assert.Equal(t, "https://example.com/webhook", *got.Webhook.Urls)
			}
			if assert.NotNil(t, got.FeishuNotification) {
				assert.True(t, got.FeishuNotification.Enable)
				assert.Equal(t, "ou_xxx", *got.FeishuNotification.UserID)
			}
		}
	})

	t.Run("normal: partial config with only webhook enabled", func(t *testing.T) {
		t.Parallel()
		conf := &domainExpt.ExptNotificationConf{
			Webhook: &domainExpt.WebhookNotificationConf{
				Enable: true,
				Urls:   gptr.Of("https://example.com/hook"),
			},
		}

		got, err := NotificationConfDTO2DO(conf)
		assert.NoError(t, err)
		if assert.NotNil(t, got) {
			assert.Nil(t, got.Filter)
			assert.Nil(t, got.FeishuNotification)
			if assert.NotNil(t, got.Webhook) {
				assert.True(t, got.Webhook.Enable)
				assert.Equal(t, "https://example.com/hook", *got.Webhook.Urls)
			}
		}
	})

	t.Run("normal: nil NotificationConf returns nil", func(t *testing.T) {
		t.Parallel()
		got, err := NotificationConfDTO2DO(nil)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("abnormal: invalid operator 99", func(t *testing.T) {
		t.Parallel()
		conf := &domainExpt.ExptNotificationConf{
			Filter: &domainExpt.Filters{
				FilterConditions: []*domainExpt.FilterCondition{
					{
						Field: &domainExpt.FilterField{
							FieldType: domainExpt.FieldType(3),
						},
						Operator: domainExpt.FilterOperatorType(99),
						Value:    `["11"]`,
					},
				},
			},
		}
		got, err := NotificationConfDTO2DO(conf)
		assert.Nil(t, got)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported operator")
	})

	t.Run("abnormal: invalid field_type 99", func(t *testing.T) {
		t.Parallel()
		conf := &domainExpt.ExptNotificationConf{
			Filter: &domainExpt.Filters{
				FilterConditions: []*domainExpt.FilterCondition{
					{
						Field: &domainExpt.FilterField{
							FieldType: domainExpt.FieldType(99),
						},
						Operator: domainExpt.FilterOperatorType(7),
						Value:    `["11"]`,
					},
				},
			},
		}
		got, err := NotificationConfDTO2DO(conf)
		assert.Nil(t, got)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported field_type")
	})

	t.Run("abnormal: invalid value not JSON array", func(t *testing.T) {
		t.Parallel()
		conf := &domainExpt.ExptNotificationConf{
			Filter: &domainExpt.Filters{
				FilterConditions: []*domainExpt.FilterCondition{
					{
						Field: &domainExpt.FilterField{
							FieldType: domainExpt.FieldType(3),
						},
						Operator: domainExpt.FilterOperatorType(7),
						Value:    "not-a-json-array",
					},
				},
			},
		}
		got, err := NotificationConfDTO2DO(conf)
		assert.Nil(t, got)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "value must be a JSON string array")
	})

	t.Run("abnormal: empty value", func(t *testing.T) {
		t.Parallel()
		conf := &domainExpt.ExptNotificationConf{
			Filter: &domainExpt.Filters{
				FilterConditions: []*domainExpt.FilterCondition{
					{
						Field: &domainExpt.FilterField{
							FieldType: domainExpt.FieldType(3),
						},
						Operator: domainExpt.FilterOperatorType(7),
						Value:    "",
					},
				},
			},
		}
		got, err := NotificationConfDTO2DO(conf)
		assert.Nil(t, got)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "value is empty")
	})
}

// ---------------------------------------------------------------------------
// TestSubmitExperimentOApi_NotificationConf
// ---------------------------------------------------------------------------

func TestSubmitExperimentOApi_NotificationConf(t *testing.T) {
	t.Parallel()

	t.Run("normal: full config with OpenAPI string types", func(t *testing.T) {
		t.Parallel()
		conf := &openapiExperiment.ExptNotificationConf{
			Filter: &openapiExperiment.Filters{
				LogicOp: gptr.Of("and"), // only "and" is supported
				FilterConditions: []*openapiExperiment.FilterCondition{
					{
						Field: &openapiExperiment.FilterField{
							FieldType: gptr.Of("3"), // ExptStatus
							FieldKey:  gptr.Of("status_key"),
						},
						Operator: gptr.Of("7"), // In
						Value:    gptr.Of(`["11","12"]`),
					},
				},
			},
			Webhook: &openapiExperiment.WebhookNotificationConf{
				Enable: gptr.Of(true),
				Urls:   gptr.Of("https://hook.example.com"),
			},
			FeishuNotification: &openapiExperiment.FeishuNotificationConf{
				Enable: gptr.Of(true),
				UserID: gptr.Of("ou_abc"),
			},
		}

		domainConf, err := OpenAPINotificationConfDTO2Domain(conf)
		assert.NoError(t, err)
		if assert.NotNil(t, domainConf) {
			if assert.NotNil(t, domainConf.Filter) {
				if assert.NotNil(t, domainConf.Filter.LogicOp) {
					assert.Equal(t, domainExpt.FilterLogicOp_And, *domainConf.Filter.LogicOp)
				}
				if assert.Len(t, domainConf.Filter.FilterConditions, 1) {
					fc := domainConf.Filter.FilterConditions[0]
					assert.Equal(t, domainExpt.FilterOperatorType(7), fc.Operator)
					assert.Equal(t, `["11","12"]`, fc.Value)
					if assert.NotNil(t, fc.Field) {
						assert.Equal(t, domainExpt.FieldType(3), fc.Field.FieldType)
						assert.Equal(t, "status_key", *fc.Field.FieldKey)
					}
				}
			}
			if assert.NotNil(t, domainConf.Webhook) {
				assert.True(t, domainConf.Webhook.Enable)
				assert.Equal(t, "https://hook.example.com", *domainConf.Webhook.Urls)
			}
			if assert.NotNil(t, domainConf.FeishuNotification) {
				assert.True(t, domainConf.FeishuNotification.Enable)
				assert.Equal(t, "ou_abc", *domainConf.FeishuNotification.UserID)
			}
		}
	})

	t.Run("normal: only webhook enabled", func(t *testing.T) {
		t.Parallel()
		conf := &openapiExperiment.ExptNotificationConf{
			Webhook: &openapiExperiment.WebhookNotificationConf{
				Enable: gptr.Of(true),
				Urls:   gptr.Of("https://hook.example.com/only"),
			},
		}

		domainConf, err := OpenAPINotificationConfDTO2Domain(conf)
		assert.NoError(t, err)
		if assert.NotNil(t, domainConf) {
			assert.Nil(t, domainConf.Filter)
			assert.Nil(t, domainConf.FeishuNotification)
			if assert.NotNil(t, domainConf.Webhook) {
				assert.True(t, domainConf.Webhook.Enable)
				assert.Equal(t, "https://hook.example.com/only", *domainConf.Webhook.Urls)
			}
		}
	})

	t.Run("normal: nil returns nil", func(t *testing.T) {
		t.Parallel()
		domainConf, err := OpenAPINotificationConfDTO2Domain(nil)
		assert.NoError(t, err)
		assert.Nil(t, domainConf)
	})

	t.Run("abnormal: invalid operator string 99 passes OpenAPI layer but fails NotificationConfDTO2DO", func(t *testing.T) {
		t.Parallel()
		conf := &openapiExperiment.ExptNotificationConf{
			Filter: &openapiExperiment.Filters{
				FilterConditions: []*openapiExperiment.FilterCondition{
					{
						Field: &openapiExperiment.FilterField{
							FieldType: gptr.Of("3"),
						},
						Operator: gptr.Of("99"),
						Value:    gptr.Of(`["11"]`),
					},
				},
			},
		}
		// OpenAPI layer parses "99" as int64, no error at this level
		domainConf, err := OpenAPINotificationConfDTO2Domain(conf)
		assert.NoError(t, err)
		assert.NotNil(t, domainConf)
		// But downstream NotificationConfDTO2DO will catch the invalid operator
		entityConf, err := NotificationConfDTO2DO(domainConf)
		assert.Nil(t, entityConf)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported operator")
	})

	t.Run("abnormal: invalid field_type string 99 passes OpenAPI layer but fails NotificationConfDTO2DO", func(t *testing.T) {
		t.Parallel()
		conf := &openapiExperiment.ExptNotificationConf{
			Filter: &openapiExperiment.Filters{
				FilterConditions: []*openapiExperiment.FilterCondition{
					{
						Field: &openapiExperiment.FilterField{
							FieldType: gptr.Of("99"),
						},
						Operator: gptr.Of("7"),
						Value:    gptr.Of(`["11"]`),
					},
				},
			},
		}
		// OpenAPI layer parses "99" as int64, no error at this level
		domainConf, err := OpenAPINotificationConfDTO2Domain(conf)
		assert.NoError(t, err)
		assert.NotNil(t, domainConf)
		// But downstream NotificationConfDTO2DO will catch the invalid field_type
		entityConf, err := NotificationConfDTO2DO(domainConf)
		assert.Nil(t, entityConf)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported field_type")
	})

	t.Run("abnormal: non-JSON value string abc passes OpenAPI layer but fails NotificationConfDTO2DO", func(t *testing.T) {
		t.Parallel()
		conf := &openapiExperiment.ExptNotificationConf{
			Filter: &openapiExperiment.Filters{
				FilterConditions: []*openapiExperiment.FilterCondition{
					{
						Field: &openapiExperiment.FilterField{
							FieldType: gptr.Of("3"),
						},
						Operator: gptr.Of("7"),
						Value:    gptr.Of("abc"),
					},
				},
			},
		}
		// OpenAPI layer doesn't validate value format
		domainConf, err := OpenAPINotificationConfDTO2Domain(conf)
		assert.NoError(t, err)
		assert.NotNil(t, domainConf)
		// But downstream NotificationConfDTO2DO catches the bad value
		entityConf, err := NotificationConfDTO2DO(domainConf)
		assert.Nil(t, entityConf)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "value must be a JSON string array")
	})
}

// ---------------------------------------------------------------------------
// TestUpdateExperimentRequest_NotificationConf
// ---------------------------------------------------------------------------

func TestUpdateExperimentRequest_NotificationConf(t *testing.T) {
	t.Parallel()

	t.Run("normal: update with new filter using NotEqual operator", func(t *testing.T) {
		t.Parallel()
		logicOp := domainExpt.FilterLogicOp_And
		conf := &domainExpt.ExptNotificationConf{
			Filter: &domainExpt.Filters{
				LogicOp: &logicOp,
				FilterConditions: []*domainExpt.FilterCondition{
					{
						Field: &domainExpt.FilterField{
							FieldType: domainExpt.FieldType(3),
							FieldKey:  gptr.Of("status"),
						},
						Operator: domainExpt.FilterOperatorType(2), // NotEqual
						Value:    `["3"]`,
					},
				},
			},
		}

		got, err := NotificationConfDTO2DO(conf)
		assert.NoError(t, err)
		if assert.NotNil(t, got) {
			if assert.NotNil(t, got.Filter) {
				if assert.Len(t, got.Filter.FilterConditions, 1) {
					cond := got.Filter.FilterConditions[0]
					assert.Equal(t, entity.NotificationOperatorType_NotEqual, cond.Operator)
					assert.Equal(t, `["3"]`, cond.Value)
				}
			}
		}
	})

	t.Run("normal: update to disable webhook", func(t *testing.T) {
		t.Parallel()
		conf := &domainExpt.ExptNotificationConf{
			Webhook: &domainExpt.WebhookNotificationConf{
				Enable: false,
				Urls:   gptr.Of("https://example.com/webhook"),
			},
		}

		got, err := NotificationConfDTO2DO(conf)
		assert.NoError(t, err)
		if assert.NotNil(t, got) {
			if assert.NotNil(t, got.Webhook) {
				assert.False(t, got.Webhook.Enable)
				assert.Equal(t, "https://example.com/webhook", *got.Webhook.Urls)
			}
		}
	})

	t.Run("normal: nil means no update to notification", func(t *testing.T) {
		t.Parallel()
		got, err := NotificationConfDTO2DO(nil)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("abnormal: empty value array JSON", func(t *testing.T) {
		t.Parallel()
		conf := &domainExpt.ExptNotificationConf{
			Filter: &domainExpt.Filters{
				FilterConditions: []*domainExpt.FilterCondition{
					{
						Field: &domainExpt.FilterField{
							FieldType: domainExpt.FieldType(3),
						},
						Operator: domainExpt.FilterOperatorType(7), // In
						Value:    `[]`,
					},
				},
			},
		}
		got, err := NotificationConfDTO2DO(conf)
		assert.Nil(t, got)
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// TestCreateExptTemplateRequest_NotificationConf
// ---------------------------------------------------------------------------

func TestCreateExptTemplateRequest_NotificationConf(t *testing.T) {
	t.Parallel()

	t.Run("normal: full config with Or logic and multiple conditions", func(t *testing.T) {
		t.Parallel()
		logicOp := domainExpt.FilterLogicOp_Or
		conf := &domainExpt.ExptNotificationConf{
			Filter: &domainExpt.Filters{
				LogicOp: &logicOp,
				FilterConditions: []*domainExpt.FilterCondition{
					{
						Field: &domainExpt.FilterField{
							FieldType: domainExpt.FieldType(3),
							FieldKey:  gptr.Of("status"),
						},
						Operator: domainExpt.FilterOperatorType(1), // Equal
						Value:    `["11"]`,
					},
					{
						Field: &domainExpt.FilterField{
							FieldType: domainExpt.FieldType(3),
							FieldKey:  gptr.Of("status"),
						},
						Operator: domainExpt.FilterOperatorType(8), // NotIn
						Value:    `["12","13"]`,
					},
				},
			},
			Webhook: &domainExpt.WebhookNotificationConf{
				Enable: true,
				Urls:   gptr.Of("https://template.example.com/hook"),
			},
			FeishuNotification: &domainExpt.FeishuNotificationConf{
				Enable: true,
				UserID: gptr.Of("ou_tpl"),
			},
		}

		got, err := NotificationConfDTO2DO(conf)
		assert.NoError(t, err)
		if assert.NotNil(t, got) {
			if assert.NotNil(t, got.Filter) {
				if assert.NotNil(t, got.Filter.LogicOp) {
					assert.Equal(t, entity.FilterLogicOp_Or, *got.Filter.LogicOp)
				}
				if assert.Len(t, got.Filter.FilterConditions, 2) {
					assert.Equal(t, entity.NotificationOperatorType_Equal, got.Filter.FilterConditions[0].Operator)
					assert.Equal(t, entity.NotificationOperatorType_NotIn, got.Filter.FilterConditions[1].Operator)
				}
			}
			if assert.NotNil(t, got.Webhook) {
				assert.True(t, got.Webhook.Enable)
			}
			if assert.NotNil(t, got.FeishuNotification) {
				assert.True(t, got.FeishuNotification.Enable)
				assert.Equal(t, "ou_tpl", *got.FeishuNotification.UserID)
			}
		}
	})

	t.Run("normal: only feishu notification", func(t *testing.T) {
		t.Parallel()
		conf := &domainExpt.ExptNotificationConf{
			FeishuNotification: &domainExpt.FeishuNotificationConf{
				Enable: true,
				UserID: gptr.Of("ou_feishu_only"),
			},
		}

		got, err := NotificationConfDTO2DO(conf)
		assert.NoError(t, err)
		if assert.NotNil(t, got) {
			assert.Nil(t, got.Filter)
			assert.Nil(t, got.Webhook)
			if assert.NotNil(t, got.FeishuNotification) {
				assert.True(t, got.FeishuNotification.Enable)
				assert.Equal(t, "ou_feishu_only", *got.FeishuNotification.UserID)
			}
		}
	})

	t.Run("abnormal: mixed valid and nil conditions in array skips nil", func(t *testing.T) {
		t.Parallel()
		logicOp := domainExpt.FilterLogicOp_And
		conf := &domainExpt.ExptNotificationConf{
			Filter: &domainExpt.Filters{
				LogicOp: &logicOp,
				FilterConditions: []*domainExpt.FilterCondition{
					nil,
					{
						Field: &domainExpt.FilterField{
							FieldType: domainExpt.FieldType(3),
							FieldKey:  gptr.Of("status"),
						},
						Operator: domainExpt.FilterOperatorType(7), // In
						Value:    `["11"]`,
					},
					nil,
				},
			},
		}

		got, err := NotificationConfDTO2DO(conf)
		assert.NoError(t, err)
		if assert.NotNil(t, got) {
			if assert.NotNil(t, got.Filter) {
				assert.Len(t, got.Filter.FilterConditions, 1)
				assert.Equal(t, entity.NotificationOperatorType_In, got.Filter.FilterConditions[0].Operator)
			}
		}
	})

	t.Run("abnormal: condition without field (field=nil)", func(t *testing.T) {
		t.Parallel()
		conf := &domainExpt.ExptNotificationConf{
			Filter: &domainExpt.Filters{
				FilterConditions: []*domainExpt.FilterCondition{
					{
						Field:    nil,
						Operator: domainExpt.FilterOperatorType(1), // Equal
						Value:    `["11"]`,
					},
				},
			},
		}

		got, err := NotificationConfDTO2DO(conf)
		assert.NoError(t, err)
		if assert.NotNil(t, got) {
			if assert.NotNil(t, got.Filter) {
				if assert.Len(t, got.Filter.FilterConditions, 1) {
					assert.Nil(t, got.Filter.FilterConditions[0].Field)
					assert.Equal(t, entity.NotificationOperatorType_Equal, got.Filter.FilterConditions[0].Operator)
				}
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TestUpdateExptTemplateRequest_NotificationConf
// ---------------------------------------------------------------------------

func TestUpdateExptTemplateRequest_NotificationConf(t *testing.T) {
	t.Parallel()

	t.Run("normal: replace notification config entirely", func(t *testing.T) {
		t.Parallel()
		logicOp := domainExpt.FilterLogicOp_And
		conf := &domainExpt.ExptNotificationConf{
			Filter: &domainExpt.Filters{
				LogicOp: &logicOp,
				FilterConditions: []*domainExpt.FilterCondition{
					{
						Field: &domainExpt.FilterField{
							FieldType: domainExpt.FieldType(3),
							FieldKey:  gptr.Of("new_status"),
						},
						Operator: domainExpt.FilterOperatorType(7), // In
						Value:    `["11","12","13"]`,
					},
				},
			},
			Webhook: &domainExpt.WebhookNotificationConf{
				Enable: true,
				Urls:   gptr.Of("https://new-webhook.example.com"),
			},
			FeishuNotification: &domainExpt.FeishuNotificationConf{
				Enable: true,
				UserID: gptr.Of("ou_new_user"),
			},
		}

		got, err := NotificationConfDTO2DO(conf)
		assert.NoError(t, err)
		if assert.NotNil(t, got) {
			if assert.NotNil(t, got.Filter) {
				assert.Equal(t, entity.FilterLogicOp_And, *got.Filter.LogicOp)
				if assert.Len(t, got.Filter.FilterConditions, 1) {
					assert.Equal(t, `["11","12","13"]`, got.Filter.FilterConditions[0].Value)
					assert.Equal(t, "new_status", *got.Filter.FilterConditions[0].Field.FieldKey)
				}
			}
			if assert.NotNil(t, got.Webhook) {
				assert.True(t, got.Webhook.Enable)
				assert.Equal(t, "https://new-webhook.example.com", *got.Webhook.Urls)
			}
			if assert.NotNil(t, got.FeishuNotification) {
				assert.True(t, got.FeishuNotification.Enable)
				assert.Equal(t, "ou_new_user", *got.FeishuNotification.UserID)
			}
		}
	})

	t.Run("normal: disable all notifications", func(t *testing.T) {
		t.Parallel()
		conf := &domainExpt.ExptNotificationConf{
			Webhook: &domainExpt.WebhookNotificationConf{
				Enable: false,
			},
			FeishuNotification: &domainExpt.FeishuNotificationConf{
				Enable: false,
			},
		}

		got, err := NotificationConfDTO2DO(conf)
		assert.NoError(t, err)
		if assert.NotNil(t, got) {
			if assert.NotNil(t, got.Webhook) {
				assert.False(t, got.Webhook.Enable)
			}
			if assert.NotNil(t, got.FeishuNotification) {
				assert.False(t, got.FeishuNotification.Enable)
			}
		}
	})

	t.Run("abnormal: operator=0 unknown is invalid", func(t *testing.T) {
		t.Parallel()
		conf := &domainExpt.ExptNotificationConf{
			Filter: &domainExpt.Filters{
				FilterConditions: []*domainExpt.FilterCondition{
					{
						Field: &domainExpt.FilterField{
							FieldType: domainExpt.FieldType(3),
						},
						Operator: domainExpt.FilterOperatorType(0), // Unknown
						Value:    `["11"]`,
					},
				},
			},
		}
		got, err := NotificationConfDTO2DO(conf)
		assert.Nil(t, got)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported operator")
	})
}

// ---------------------------------------------------------------------------
// TestGetExperiment_NotificationConf_Response
// ---------------------------------------------------------------------------

func TestGetExperiment_NotificationConf_Response(t *testing.T) {
	t.Parallel()

	t.Run("normal: full entity config maps correctly to OpenAPI", func(t *testing.T) {
		t.Parallel()
		logicOp := entity.FilterLogicOp_And
		conf := &entity.ExptNotificationConf{
			Filter: &entity.NotificationFilter{
				LogicOp: &logicOp,
				FilterConditions: []*entity.NotificationFilterCondition{
					{
						Field: &entity.NotificationFilterField{
							FieldType: entity.NotificationFieldType_ExptStatus, // 3
							FieldKey:  gptr.Of("status"),
						},
						Operator: entity.NotificationOperatorType_In, // 7
						Value:    `["11","12"]`,
					},
				},
			},
			Webhook: &entity.WebhookNotificationConf{
				Enable: true,
				Urls:   gptr.Of("https://example.com/webhook"),
			},
			FeishuNotification: &entity.FeishuNotificationConf{
				Enable: true,
				UserID: gptr.Of("ou_resp"),
			},
		}

		got := entityNotificationConfToOpenAPI(conf)
		if assert.NotNil(t, got) {
			if assert.NotNil(t, got.Filter) {
				if assert.NotNil(t, got.Filter.LogicOp) {
					assert.Equal(t, "and", *got.Filter.LogicOp) // And
				}
				if assert.Len(t, got.Filter.FilterConditions, 1) {
					fc := got.Filter.FilterConditions[0]
					assert.Equal(t, "in", *fc.Operator)                 // In
					assert.Equal(t, "expt_status", *fc.Field.FieldType) // ExptStatus
					assert.Equal(t, `["success","failed"]`, *fc.Value)
					assert.Equal(t, "status", *fc.Field.FieldKey)
				}
			}
			if assert.NotNil(t, got.Webhook) {
				assert.Equal(t, true, *got.Webhook.Enable)
				assert.Equal(t, "https://example.com/webhook", *got.Webhook.Urls)
			}
			if assert.NotNil(t, got.FeishuNotification) {
				assert.Equal(t, true, *got.FeishuNotification.Enable)
				assert.Equal(t, "ou_resp", *got.FeishuNotification.UserID)
			}
		}
	})

	t.Run("normal: nil returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, entityNotificationConfToOpenAPI(nil))
	})

	t.Run("normal: empty webhook with enable=false and urls=nil", func(t *testing.T) {
		t.Parallel()
		conf := &entity.ExptNotificationConf{
			Webhook: &entity.WebhookNotificationConf{
				Enable: false,
				Urls:   nil,
			},
		}

		got := entityNotificationConfToOpenAPI(conf)
		if assert.NotNil(t, got) {
			assert.Nil(t, got.Filter)
			assert.Nil(t, got.FeishuNotification)
			if assert.NotNil(t, got.Webhook) {
				assert.Equal(t, false, *got.Webhook.Enable)
				assert.Nil(t, got.Webhook.Urls)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TestGetExptTemplate_NotificationConf_Response
// ---------------------------------------------------------------------------

func TestGetExptTemplate_NotificationConf_Response(t *testing.T) {
	t.Parallel()

	t.Run("normal: full entity config round-trip via notificationConfDO2DTO", func(t *testing.T) {
		t.Parallel()
		logicOp := entity.FilterLogicOp_Or
		conf := &entity.ExptNotificationConf{
			Filter: &entity.NotificationFilter{
				LogicOp: &logicOp,
				FilterConditions: []*entity.NotificationFilterCondition{
					{
						Field: &entity.NotificationFilterField{
							FieldType: entity.NotificationFieldType_ExptStatus,
							FieldKey:  gptr.Of("tpl_status"),
						},
						Operator: entity.NotificationOperatorType_NotEqual,
						Value:    `["3"]`,
					},
				},
			},
			Webhook: &entity.WebhookNotificationConf{
				Enable: true,
				Urls:   gptr.Of("https://template.example.com"),
			},
			FeishuNotification: &entity.FeishuNotificationConf{
				Enable: true,
				UserID: gptr.Of("ou_tpl_user"),
			},
		}

		dto := notificationConfDO2DTO(conf)
		if assert.NotNil(t, dto) {
			if assert.NotNil(t, dto.Filter) {
				if assert.NotNil(t, dto.Filter.LogicOp) {
					assert.Equal(t, domainExpt.FilterLogicOp_Or, *dto.Filter.LogicOp)
				}
				if assert.Len(t, dto.Filter.FilterConditions, 1) {
					fc := dto.Filter.FilterConditions[0]
					assert.Equal(t, domainExpt.FilterOperatorType(2), fc.Operator) // NotEqual
					assert.Equal(t, `["3"]`, fc.Value)
					if assert.NotNil(t, fc.Field) {
						assert.Equal(t, domainExpt.FieldType(3), fc.Field.FieldType)
						assert.Equal(t, "tpl_status", *fc.Field.FieldKey)
					}
				}
			}
			if assert.NotNil(t, dto.Webhook) {
				assert.True(t, dto.Webhook.Enable)
				assert.Equal(t, "https://template.example.com", *dto.Webhook.Urls)
			}
			if assert.NotNil(t, dto.FeishuNotification) {
				assert.True(t, dto.FeishuNotification.Enable)
				assert.Equal(t, "ou_tpl_user", *dto.FeishuNotification.UserID)
			}
		}

		// Also verify entityNotificationConfToOpenAPI for the same config
		oapi := entityNotificationConfToOpenAPI(conf)
		if assert.NotNil(t, oapi) {
			assert.Equal(t, "or", *oapi.Filter.LogicOp)                             // Or
			assert.Equal(t, "not_equal", *oapi.Filter.FilterConditions[0].Operator) // NotEqual
			assert.Equal(t, "expt_status", *oapi.Filter.FilterConditions[0].Field.FieldType)
		}
	})

	t.Run("normal: template with only filter, no webhook or feishu", func(t *testing.T) {
		t.Parallel()
		logicOp := entity.FilterLogicOp_And
		conf := &entity.ExptNotificationConf{
			Filter: &entity.NotificationFilter{
				LogicOp: &logicOp,
				FilterConditions: []*entity.NotificationFilterCondition{
					{
						Field: &entity.NotificationFilterField{
							FieldType: entity.NotificationFieldType_ExptStatus,
						},
						Operator: entity.NotificationOperatorType_In,
						Value:    `["11"]`,
					},
				},
			},
		}

		dto := notificationConfDO2DTO(conf)
		if assert.NotNil(t, dto) {
			assert.NotNil(t, dto.Filter)
			assert.Nil(t, dto.Webhook)
			assert.Nil(t, dto.FeishuNotification)
			if assert.Len(t, dto.Filter.FilterConditions, 1) {
				assert.Equal(t, domainExpt.FilterOperatorType(7), dto.Filter.FilterConditions[0].Operator)
			}
		}

		oapi := entityNotificationConfToOpenAPI(conf)
		if assert.NotNil(t, oapi) {
			assert.NotNil(t, oapi.Filter)
			assert.Nil(t, oapi.Webhook)
			assert.Nil(t, oapi.FeishuNotification)
		}
	})
}

// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	domainExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ---------------------------------------------------------------------------
// TestNotificationConfDTO2DO_BitsUT
// ---------------------------------------------------------------------------

func TestNotificationConfDTO2DO_BitsUT(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns nil, nil", func(t *testing.T) {
		t.Parallel()
		got, err := NotificationConfDTO2DO(nil)
		assert.Nil(t, got)
		assert.Nil(t, err)
	})

	t.Run("valid full config with filter, webhook and feishu", func(t *testing.T) {
		t.Parallel()
		logicOp := domainExpt.FilterLogicOp_And
		conf := &domainExpt.ExptNotificationConf{
			Filter: &domainExpt.Filters{
				LogicOp: &logicOp,
				FilterConditions: []*domainExpt.FilterCondition{
					{
						Field: &domainExpt.FilterField{
							FieldType: domainExpt.FieldType(3), // ExptStatus
							FieldKey:  gptr.Of("status_key"),
						},
						Operator: domainExpt.FilterOperatorType(7), // In
						Value:    `["11","12"]`,
					},
				},
			},
			Webhook: &domainExpt.WebhookNotificationConf{
				Enable: true,
				Urls:   gptr.Of("http://x"),
			},
			FeishuNotification: &domainExpt.FeishuNotificationConf{
				Enable: true,
				UserID: gptr.Of("user123"),
			},
		}

		got, err := NotificationConfDTO2DO(conf)
		assert.NoError(t, err)
		if assert.NotNil(t, got) {
			// Filter
			if assert.NotNil(t, got.Filter) {
				if assert.NotNil(t, got.Filter.LogicOp) {
					assert.Equal(t, entity.FilterLogicOp(1), *got.Filter.LogicOp) // And = 1
				}
				if assert.Len(t, got.Filter.FilterConditions, 1) {
					cond := got.Filter.FilterConditions[0]
					assert.Equal(t, entity.NotificationOperatorType_In, cond.Operator)
					assert.Equal(t, `["11","12"]`, cond.Value)
					if assert.NotNil(t, cond.Field) {
						assert.Equal(t, entity.NotificationFieldType_ExptStatus, cond.Field.FieldType)
						assert.Equal(t, "status_key", *cond.Field.FieldKey)
					}
				}
			}
			// Webhook
			if assert.NotNil(t, got.Webhook) {
				assert.True(t, got.Webhook.Enable)
				assert.Equal(t, "http://x", *got.Webhook.Urls)
			}
			// Feishu
			if assert.NotNil(t, got.FeishuNotification) {
				assert.True(t, got.FeishuNotification.Enable)
				assert.Equal(t, "user123", *got.FeishuNotification.UserID)
			}
		}
	})

	t.Run("validation error: unsupported operator", func(t *testing.T) {
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

	t.Run("validation error: unsupported field_type", func(t *testing.T) {
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

	t.Run("validation error: empty value string", func(t *testing.T) {
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

	t.Run("validation error: non-JSON value", func(t *testing.T) {
		t.Parallel()
		conf := &domainExpt.ExptNotificationConf{
			Filter: &domainExpt.Filters{
				FilterConditions: []*domainExpt.FilterCondition{
					{
						Field: &domainExpt.FilterField{
							FieldType: domainExpt.FieldType(3),
						},
						Operator: domainExpt.FilterOperatorType(7),
						Value:    "not-json",
					},
				},
			},
		}
		got, err := NotificationConfDTO2DO(conf)
		assert.Nil(t, got)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "value must be a JSON string array")
	})
}

// ---------------------------------------------------------------------------
// TestNotificationConfDO2DTO_BitsUT
// ---------------------------------------------------------------------------

func TestNotificationConfDO2DTO_BitsUT(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, notificationConfDO2DTO(nil))
	})

	t.Run("full config round-trip: entity to domain DTO", func(t *testing.T) {
		t.Parallel()
		logicOp := entity.FilterLogicOp_And
		conf := &entity.ExptNotificationConf{
			Filter: &entity.NotificationFilter{
				LogicOp: &logicOp,
				FilterConditions: []*entity.NotificationFilterCondition{
					{
						Field: &entity.NotificationFilterField{
							FieldType: entity.NotificationFieldType_ExptStatus,
							FieldKey:  gptr.Of("key1"),
						},
						Operator: entity.NotificationOperatorType_In,
						Value:    `["11","12"]`,
					},
				},
			},
			Webhook: &entity.WebhookNotificationConf{
				Enable: true,
				Urls:   gptr.Of("http://example.com"),
			},
			FeishuNotification: &entity.FeishuNotificationConf{
				Enable: true,
				UserID: gptr.Of("uid"),
			},
		}

		got := notificationConfDO2DTO(conf)
		if assert.NotNil(t, got) {
			// Filter
			if assert.NotNil(t, got.Filter) {
				if assert.NotNil(t, got.Filter.LogicOp) {
					assert.Equal(t, domainExpt.FilterLogicOp_And, *got.Filter.LogicOp)
				}
				if assert.Len(t, got.Filter.FilterConditions, 1) {
					fc := got.Filter.FilterConditions[0]
					assert.Equal(t, domainExpt.FilterOperatorType(7), fc.Operator) // In = 7
					assert.Equal(t, `["11","12"]`, fc.Value)
					if assert.NotNil(t, fc.Field) {
						assert.Equal(t, domainExpt.FieldType(3), fc.Field.FieldType) // ExptStatus = 3
						assert.Equal(t, "key1", *fc.Field.FieldKey)
					}
				}
			}
			// Webhook
			if assert.NotNil(t, got.Webhook) {
				assert.True(t, got.Webhook.Enable)
				assert.Equal(t, "http://example.com", *got.Webhook.Urls)
			}
			// Feishu
			if assert.NotNil(t, got.FeishuNotification) {
				assert.True(t, got.FeishuNotification.Enable)
				assert.Equal(t, "uid", *got.FeishuNotification.UserID)
			}
		}
	})

	t.Run("partial config: only webhook set", func(t *testing.T) {
		t.Parallel()
		conf := &entity.ExptNotificationConf{
			Webhook: &entity.WebhookNotificationConf{
				Enable: false,
				Urls:   gptr.Of("http://hook"),
			},
		}

		got := notificationConfDO2DTO(conf)
		if assert.NotNil(t, got) {
			assert.Nil(t, got.Filter)
			assert.Nil(t, got.FeishuNotification)
			if assert.NotNil(t, got.Webhook) {
				assert.False(t, got.Webhook.Enable)
				assert.Equal(t, "http://hook", *got.Webhook.Urls)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TestEntityNotificationConfToOpenAPI_BitsUT
// ---------------------------------------------------------------------------

func TestEntityNotificationConfToOpenAPI_BitsUT(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, entityNotificationConfToOpenAPI(nil))
	})

	t.Run("full config: int64 fields converted to string in OpenAPI", func(t *testing.T) {
		t.Parallel()
		logicOp := entity.FilterLogicOp_Or // 2
		conf := &entity.ExptNotificationConf{
			Filter: &entity.NotificationFilter{
				LogicOp: &logicOp,
				FilterConditions: []*entity.NotificationFilterCondition{
					{
						Field: &entity.NotificationFilterField{
							FieldType: entity.NotificationFieldType_ExptStatus, // 3
							FieldKey:  gptr.Of("k"),
						},
						Operator: entity.NotificationOperatorType_In, // 7
						Value:    `["v"]`,
					},
				},
			},
			Webhook: &entity.WebhookNotificationConf{
				Enable: true,
				Urls:   gptr.Of("http://w"),
			},
			FeishuNotification: &entity.FeishuNotificationConf{
				Enable: true,
				UserID: gptr.Of("u1"),
			},
		}

		got := entityNotificationConfToOpenAPI(conf)
		if assert.NotNil(t, got) {
			// Filter — all numeric fields are string in OpenAPI
			if assert.NotNil(t, got.Filter) {
				if assert.NotNil(t, got.Filter.LogicOp) {
					assert.Equal(t, "2", *got.Filter.LogicOp) // Or = 2
				}
				if assert.Len(t, got.Filter.FilterConditions, 1) {
					fc := got.Filter.FilterConditions[0]
					assert.Equal(t, "7", *fc.Operator) // In
					assert.Equal(t, `["v"]`, *fc.Value)
					if assert.NotNil(t, fc.Field) {
						assert.Equal(t, "3", *fc.Field.FieldType) // ExptStatus
						assert.Equal(t, "k", *fc.Field.FieldKey)
					}
				}
			}
			// Webhook
			if assert.NotNil(t, got.Webhook) {
				assert.Equal(t, true, *got.Webhook.Enable)
				assert.Equal(t, "http://w", *got.Webhook.Urls)
			}
			// Feishu
			if assert.NotNil(t, got.FeishuNotification) {
				assert.Equal(t, true, *got.FeishuNotification.Enable)
				assert.Equal(t, "u1", *got.FeishuNotification.UserID)
			}
		}
	})

	t.Run("partial config: only FeishuNotification set", func(t *testing.T) {
		t.Parallel()
		conf := &entity.ExptNotificationConf{
			FeishuNotification: &entity.FeishuNotificationConf{
				Enable: false,
				UserID: gptr.Of("u2"),
			},
		}

		got := entityNotificationConfToOpenAPI(conf)
		if assert.NotNil(t, got) {
			assert.Nil(t, got.Filter)
			assert.Nil(t, got.Webhook)
			if assert.NotNil(t, got.FeishuNotification) {
				assert.Equal(t, false, *got.FeishuNotification.Enable)
				assert.Equal(t, "u2", *got.FeishuNotification.UserID)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TestDomainNotificationConfToOpenAPI_BitsUT
// ---------------------------------------------------------------------------

func TestDomainNotificationConfToOpenAPI_BitsUT(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, domainNotificationConfToOpenAPI(nil))
	})

	t.Run("full config: domain DTO to OpenAPI with type conversions", func(t *testing.T) {
		t.Parallel()
		logicOp := domainExpt.FilterLogicOp_Or // 2
		conf := &domainExpt.ExptNotificationConf{
			Filter: &domainExpt.Filters{
				LogicOp: &logicOp,
				FilterConditions: []*domainExpt.FilterCondition{
					{
						Field: &domainExpt.FilterField{
							FieldType: domainExpt.FieldType(3), // ExptStatus
							FieldKey:  gptr.Of("fk"),
						},
						Operator: domainExpt.FilterOperatorType(7), // In
						Value:    `["a","b"]`,
					},
				},
			},
			Webhook: &domainExpt.WebhookNotificationConf{
				Enable: true,
				Urls:   gptr.Of("http://hook"),
			},
			FeishuNotification: &domainExpt.FeishuNotificationConf{
				Enable: false,
				UserID: gptr.Of("uid"),
			},
		}

		got := domainNotificationConfToOpenAPI(conf)
		if assert.NotNil(t, got) {
			// Filter
			if assert.NotNil(t, got.Filter) {
				if assert.NotNil(t, got.Filter.LogicOp) {
					assert.Equal(t, "2", *got.Filter.LogicOp)
				}
				if assert.Len(t, got.Filter.FilterConditions, 1) {
					fc := got.Filter.FilterConditions[0]
					assert.Equal(t, "7", *fc.Operator)
					assert.Equal(t, `["a","b"]`, *fc.Value)
					if assert.NotNil(t, fc.Field) {
						assert.Equal(t, "3", *fc.Field.FieldType)
						assert.Equal(t, "fk", *fc.Field.FieldKey)
					}
				}
			}
			// Webhook
			if assert.NotNil(t, got.Webhook) {
				assert.Equal(t, true, *got.Webhook.Enable)
				assert.Equal(t, "http://hook", *got.Webhook.Urls)
			}
			// Feishu
			if assert.NotNil(t, got.FeishuNotification) {
				assert.Equal(t, false, *got.FeishuNotification.Enable)
				assert.Equal(t, "uid", *got.FeishuNotification.UserID)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TestTemplateToSubmitExperimentRequest_NotificationConf_BitsUT
// ---------------------------------------------------------------------------

func TestTemplateToSubmitExperimentRequest_NotificationConf_BitsUT(t *testing.T) {
	t.Parallel()

	t.Run("template with NotificationConf: req.NotificationConf is set", func(t *testing.T) {
		t.Parallel()
		template := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:       1,
				ExptType: entity.ExptType_Offline,
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID:        100,
				EvalSetVersionID: 101,
			},
			NotificationConf: &entity.ExptNotificationConf{
				Webhook: &entity.WebhookNotificationConf{
					Enable: true,
					Urls:   gptr.Of("http://x"),
				},
			},
		}

		req := TemplateToSubmitExperimentRequest(template, "test-exp", 100)
		if assert.NotNil(t, req) {
			if assert.NotNil(t, req.NotificationConf) {
				if assert.NotNil(t, req.NotificationConf.Webhook) {
					assert.True(t, req.NotificationConf.Webhook.Enable)
					assert.Equal(t, "http://x", *req.NotificationConf.Webhook.Urls)
				}
			}
		}
	})

	t.Run("template without NotificationConf: req.NotificationConf is nil", func(t *testing.T) {
		t.Parallel()
		template := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:       2,
				ExptType: entity.ExptType_Offline,
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID:        200,
				EvalSetVersionID: 201,
			},
		}

		req := TemplateToSubmitExperimentRequest(template, "test-exp2", 200)
		if assert.NotNil(t, req) {
			assert.Nil(t, req.NotificationConf)
		}
	})

	t.Run("template without TripleConfig: early return, NotificationConf not set", func(t *testing.T) {
		t.Parallel()
		template := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:       3,
				ExptType: entity.ExptType_Offline,
			},
			NotificationConf: &entity.ExptNotificationConf{
				Webhook: &entity.WebhookNotificationConf{Enable: true},
			},
		}

		req := TemplateToSubmitExperimentRequest(template, "test-exp3", 300)
		if assert.NotNil(t, req) {
			// Early return path: NotificationConf is NOT populated
			assert.Nil(t, req.NotificationConf)
		}
	})
}

// ---------------------------------------------------------------------------
// TestToExptTemplateDTO_NotificationConf_BitsUT
// ---------------------------------------------------------------------------

func TestToExptTemplateDTO_NotificationConf_BitsUT(t *testing.T) {
	t.Parallel()

	t.Run("template entity with NotificationConf: dto.NotificationConf is set", func(t *testing.T) {
		t.Parallel()
		logicOp := entity.FilterLogicOp_And
		template := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:       10,
				Name:     "tpl",
				ExptType: entity.ExptType_Offline,
			},
			NotificationConf: &entity.ExptNotificationConf{
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
				FeishuNotification: &entity.FeishuNotificationConf{
					Enable: true,
				},
			},
		}

		dto := ToExptTemplateDTO(template)
		if assert.NotNil(t, dto) {
			if assert.NotNil(t, dto.NotificationConf) {
				assert.NotNil(t, dto.NotificationConf.Filter)
				assert.NotNil(t, dto.NotificationConf.FeishuNotification)
				assert.True(t, dto.NotificationConf.FeishuNotification.Enable)
			}
		}
	})

	t.Run("template entity without NotificationConf: dto.NotificationConf is nil", func(t *testing.T) {
		t.Parallel()
		template := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:       20,
				Name:     "tpl2",
				ExptType: entity.ExptType_Offline,
			},
		}

		dto := ToExptTemplateDTO(template)
		if assert.NotNil(t, dto) {
			assert.Nil(t, dto.NotificationConf)
		}
	})
}

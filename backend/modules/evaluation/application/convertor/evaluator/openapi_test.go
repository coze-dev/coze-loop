// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	openapiCommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	openapiEvaluator "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/evaluator"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/assert"
)

func TestOpenAPIEvaluatorDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIEvaluatorDO2DTO(nil))
	})

	t.Run("normal input", func(t *testing.T) {
		do := &entity.Evaluator{
			ID:            1,
			Name:          "test",
			Description:   "desc",
			EvaluatorType: entity.EvaluatorTypePrompt,
			PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
				ID:      10,
				Version: "v1",
			},
		}
		dto := OpenAPIEvaluatorDO2DTO(do)
		assert.NotNil(t, dto)
		assert.Equal(t, int64(1), *dto.ID)
		assert.Equal(t, "test", *dto.Name)
	})
}

func TestOpenAPIEvaluatorDO2DTOs(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		assert.Nil(t, OpenAPIEvaluatorDO2DTOs(nil))
		assert.Nil(t, OpenAPIEvaluatorDO2DTOs([]*entity.Evaluator{}))
	})

	t.Run("normal input", func(t *testing.T) {
		dos := []*entity.Evaluator{
			{ID: 1},
			nil,
			{ID: 2},
		}
		dtos := OpenAPIEvaluatorDO2DTOs(dos)
		assert.Equal(t, 2, len(dtos))
		assert.Equal(t, int64(1), *dtos[0].ID)
		assert.Equal(t, int64(2), *dtos[1].ID)
	})
}

func TestOpenAPIEvaluatorTypeDO2DTO(t *testing.T) {
	assert.Equal(t, openapiEvaluator.EvaluatorTypePrompt, *OpenAPIEvaluatorTypeDO2DTO(entity.EvaluatorTypePrompt))
	assert.Equal(t, openapiEvaluator.EvaluatorTypeCode, *OpenAPIEvaluatorTypeDO2DTO(entity.EvaluatorTypeCode))
	assert.Equal(t, openapiEvaluator.EvaluatorTypeCustomRPC, *OpenAPIEvaluatorTypeDO2DTO(entity.EvaluatorTypeCustomRPC))
	assert.Nil(t, OpenAPIEvaluatorTypeDO2DTO(entity.EvaluatorType(999)))
}

func TestOpenAPIEvaluatorVersionDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIEvaluatorVersionDO2DTO(nil))
	})

	t.Run("prompt type", func(t *testing.T) {
		do := &entity.Evaluator{
			EvaluatorType: entity.EvaluatorTypePrompt,
			PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
				ID:      1,
				Version: "v1",
			},
		}
		dto := OpenAPIEvaluatorVersionDO2DTO(do)
		assert.NotNil(t, dto)
		assert.Equal(t, int64(1), *dto.ID)
	})

	t.Run("code type", func(t *testing.T) {
		do := &entity.Evaluator{
			EvaluatorType: entity.EvaluatorTypeCode,
			CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
				ID:      2,
				Version: "v2",
			},
		}
		dto := OpenAPIEvaluatorVersionDO2DTO(do)
		assert.NotNil(t, dto)
		assert.Equal(t, int64(2), *dto.ID)
	})

	t.Run("custom rpc type", func(t *testing.T) {
		do := &entity.Evaluator{
			EvaluatorType: entity.EvaluatorTypeCustomRPC,
			CustomRPCEvaluatorVersion: &entity.CustomRPCEvaluatorVersion{
				ID:      3,
				Version: "v3",
			},
		}
		dto := OpenAPIEvaluatorVersionDO2DTO(do)
		assert.NotNil(t, dto)
		assert.Equal(t, int64(3), *dto.ID)
	})

	t.Run("empty version", func(t *testing.T) {
		do := &entity.Evaluator{
			EvaluatorType: entity.EvaluatorTypePrompt,
		}
		assert.Nil(t, OpenAPIEvaluatorVersionDO2DTO(do))
	})
}

func TestOpenAPIEvaluatorVersionDO2DTOs(t *testing.T) {
	dos := []*entity.Evaluator{
		{EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: 1, Version: "v1"}},
		nil,
		{EvaluatorType: entity.EvaluatorTypePrompt},
	}
	dtos := OpenAPIEvaluatorVersionDO2DTOs(dos)
	assert.Equal(t, 1, len(dtos))
	assert.Equal(t, int64(1), *dtos[0].ID)
}

func TestOpenAPILanguageTypeDO2DTO(t *testing.T) {
	assert.Equal(t, openapiEvaluator.LanguageTypePython, *OpenAPILanguageTypeDO2DTO(entity.LanguageTypePython))
	assert.Equal(t, openapiEvaluator.LanguageTypeJS, *OpenAPILanguageTypeDO2DTO(entity.LanguageTypeJS))
	assert.Nil(t, OpenAPILanguageTypeDO2DTO(entity.LanguageType("999")))
}

func TestOpenAPIEvaluatorRecordDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIEvaluatorRecordDO2DTO(nil))
	})

	t.Run("normal input", func(t *testing.T) {
		do := &entity.EvaluatorRecord{
			ID:                 1,
			EvaluatorVersionID: 10,
			ItemID:             100,
			TurnID:             1000,
			Status:             entity.EvaluatorRunStatusSuccess,
		}
		dto := OpenAPIEvaluatorRecordDO2DTO(do)
		assert.NotNil(t, dto)
		assert.Equal(t, int64(1), *dto.ID)
		assert.Equal(t, openapiEvaluator.EvaluatorRunStatusSuccess, *dto.Status)
	})
}

func TestOpenAPIEvaluatorRecordDO2DTOs(t *testing.T) {
	dos := []*entity.EvaluatorRecord{
		{ID: 1},
		nil,
	}
	dtos := OpenAPIEvaluatorRecordDO2DTOs(dos)
	assert.Equal(t, 1, len(dtos))
	assert.Equal(t, int64(1), *dtos[0].ID)
}

func TestOpenAPIEvaluatorRunStatusDO2DTO(t *testing.T) {
	assert.Equal(t, openapiEvaluator.EvaluatorRunStatusSuccess, *OpenAPIEvaluatorRunStatusDO2DTO(entity.EvaluatorRunStatusSuccess))
	assert.Equal(t, openapiEvaluator.EvaluatorRunStatusFailed, *OpenAPIEvaluatorRunStatusDO2DTO(entity.EvaluatorRunStatusFail))
	assert.Equal(t, openapiEvaluator.EvaluatorRunStatusUnknown, *OpenAPIEvaluatorRunStatusDO2DTO(entity.EvaluatorRunStatusUnknown))
	assert.Equal(t, openapiEvaluator.EvaluatorRunStatusProcessing, *OpenAPIEvaluatorRunStatusDO2DTO(entity.EvaluatorRunStatus(999)))
}

func TestOpenAPIEvaluatorOutputDataDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIEvaluatorOutputDataDO2DTO(nil))
	})

	t.Run("normal input", func(t *testing.T) {
		do := &entity.EvaluatorOutputData{
			TimeConsumingMS: 100,
			Stdout:          "output",
			EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(float64(5))},
		}
		dto := OpenAPIEvaluatorOutputDataDO2DTO(do)
		assert.NotNil(t, dto)
		assert.Equal(t, int64(100), *dto.TimeConsumingMs)
		assert.Equal(t, "output", *dto.Stdout)
		assert.Equal(t, float64(5), *dto.EvaluatorResult_.Score)
	})
}

func TestOpenAPIEvaluatorResultDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIEvaluatorResultDO2DTO(nil))
	})

	t.Run("normal input", func(t *testing.T) {
		do := &entity.EvaluatorResult{
			Score:     gptr.Of(float64(5)),
			Reasoning: "good",
			Correction: &entity.Correction{
				Score: gptr.Of(float64(4)),
			},
		}
		dto := OpenAPIEvaluatorResultDO2DTO(do)
		assert.NotNil(t, dto)
		assert.Equal(t, float64(5), *dto.Score)
		assert.Equal(t, "good", *dto.Reasoning)
		assert.Equal(t, float64(4), *dto.Correction.Score)
	})
}

func TestOpenAPICorrectionDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPICorrectionDO2DTO(nil))
	})

	t.Run("normal input", func(t *testing.T) {
		do := &entity.Correction{
			Score:     gptr.Of(float64(4)),
			Explain:   "better",
			UpdatedBy: "user1",
		}
		dto := OpenAPICorrectionDO2DTO(do)
		assert.NotNil(t, dto)
		assert.Equal(t, float64(4), *dto.Score)
		assert.Equal(t, "better", *dto.Explain)
		assert.Equal(t, "user1", *dto.UpdatedBy)
	})
}

func TestOpenAPIEvaluatorUsageDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIEvaluatorUsageDO2DTO(nil))
	})

	t.Run("normal input", func(t *testing.T) {
		do := &entity.EvaluatorUsage{
			InputTokens:  10,
			OutputTokens: 20,
		}
		dto := OpenAPIEvaluatorUsageDO2DTO(do)
		assert.NotNil(t, dto)
		assert.Equal(t, int64(10), *dto.InputTokens)
		assert.Equal(t, int64(20), *dto.OutputTokens)
	})
}

func TestOpenAPIEvaluatorRunErrorDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIEvaluatorRunErrorDO2DTO(nil))
	})

	t.Run("normal input", func(t *testing.T) {
		do := &entity.EvaluatorRunError{
			Code:    500,
			Message: "error",
		}
		dto := OpenAPIEvaluatorRunErrorDO2DTO(do)
		assert.NotNil(t, dto)
		assert.Equal(t, int32(500), *dto.Code)
		assert.Equal(t, "error", *dto.Message)
	})
}

func TestOpenAPIEvaluatorInputDataDTO2DO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIEvaluatorInputDataDTO2DO(nil))
	})

	t.Run("normal input", func(t *testing.T) {
		dto := &openapiEvaluator.EvaluatorInputData{
			HistoryMessages: []*openapiCommon.Message{
				{Role: gptr.Of("user"), Content: &openapiCommon.Content{Text: gptr.Of("hello")}},
			},
		}
		do := OpenAPIEvaluatorInputDataDTO2DO(dto)
		assert.NotNil(t, do)
		assert.Equal(t, 1, len(do.HistoryMessages))
	})
}

func TestOpenAPIEvaluatorRunConfigDTO2DO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIEvaluatorRunConfigDTO2DO(nil))
	})

	t.Run("normal input", func(t *testing.T) {
		dto := &openapiEvaluator.EvaluatorRunConfig{
			Env: gptr.Of("test"),
		}
		do := OpenAPIEvaluatorRunConfigDTO2DO(dto)
		assert.NotNil(t, do)
		assert.Equal(t, "test", *do.Env)
	})
}

func TestOpenAPICorrectionDTO2DO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPICorrectionDTO2DO(nil))
	})

	t.Run("normal input", func(t *testing.T) {
		dto := &openapiEvaluator.Correction{
			Score:   gptr.Of(float64(4)),
			Explain: gptr.Of("better"),
		}
		do := OpenAPICorrectionDTO2DO(dto)
		assert.NotNil(t, do)
		assert.Equal(t, float64(4), *do.Score)
		assert.Equal(t, "better", do.Explain)
	})
}

func TestOpenAPIEvaluatorFiltersDTO2DO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIEvaluatorFiltersDTO2DO(nil))
	})

	t.Run("complex filters", func(t *testing.T) {
		dto := &openapiEvaluator.EvaluatorFilters{
			LogicOp: gptr.Of(openapiEvaluator.EvaluatorFilterLogicOpAnd),
			FilterConditions: []*openapiEvaluator.EvaluatorFilterCondition{
				{
					TagKey:   gptr.Of("key1"),
					Operator: gptr.Of("EQUAL"),
					Value:    gptr.Of("val1"),
				},
				nil,
			},
			SubFilters: []*openapiEvaluator.EvaluatorFilters{
				{
					LogicOp: gptr.Of(openapiEvaluator.EvaluatorFilterLogicOpOr),
				},
				nil,
			},
		}
		do := OpenAPIEvaluatorFiltersDTO2DO(dto)
		assert.NotNil(t, do)
		assert.Equal(t, entity.FilterLogicOp_And, *do.LogicOp)
		assert.Equal(t, 1, len(do.FilterConditions))
		assert.Equal(t, 1, len(do.SubFilters))
	})
}

func TestOpenAPIEvaluatorFilterLogicOpDTO2DO(t *testing.T) {
	assert.Equal(t, entity.FilterLogicOp_And, OpenAPIEvaluatorFilterLogicOpDTO2DO(gptr.Of(openapiEvaluator.EvaluatorFilterLogicOpAnd)))
	assert.Equal(t, entity.FilterLogicOp_Or, OpenAPIEvaluatorFilterLogicOpDTO2DO(gptr.Of(openapiEvaluator.EvaluatorFilterLogicOpOr)))
	assert.Equal(t, entity.FilterLogicOp_Unknown, OpenAPIEvaluatorFilterLogicOpDTO2DO(nil))
	assert.Equal(t, entity.FilterLogicOp_Unknown, OpenAPIEvaluatorFilterLogicOpDTO2DO(gptr.Of(openapiEvaluator.EvaluatorFilterLogicOp("999"))))
}

func TestOpenAPIEvaluatorFilterOperatorTypeDTO2DO(t *testing.T) {
	assert.Equal(t, entity.EvaluatorFilterOperatorType_Equal, OpenAPIEvaluatorFilterOperatorTypeDTO2DO("EQUAL"))
	assert.Equal(t, entity.EvaluatorFilterOperatorType_NotEqual, OpenAPIEvaluatorFilterOperatorTypeDTO2DO("NOT_EQUAL"))
	assert.Equal(t, entity.EvaluatorFilterOperatorType_In, OpenAPIEvaluatorFilterOperatorTypeDTO2DO("IN"))
	assert.Equal(t, entity.EvaluatorFilterOperatorType_In, OpenAPIEvaluatorFilterOperatorTypeDTO2DO("In")) // Pascal case from client
	assert.Equal(t, entity.EvaluatorFilterOperatorType_NotIn, OpenAPIEvaluatorFilterOperatorTypeDTO2DO("NOT_IN"))
	assert.Equal(t, entity.EvaluatorFilterOperatorType_Like, OpenAPIEvaluatorFilterOperatorTypeDTO2DO("LIKE"))
	assert.Equal(t, entity.EvaluatorFilterOperatorType_IsNull, OpenAPIEvaluatorFilterOperatorTypeDTO2DO("IS_NULL"))
	assert.Equal(t, entity.EvaluatorFilterOperatorType_IsNotNull, OpenAPIEvaluatorFilterOperatorTypeDTO2DO("IS_NOT_NULL"))
	assert.Equal(t, entity.EvaluatorFilterOperatorType_Unknown, OpenAPIEvaluatorFilterOperatorTypeDTO2DO("UNKNOWN"))
}

func TestOpenAPIEvaluatorFilterOptionDTO2DO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIEvaluatorFilterOptionDTO2DO(nil))
	})

	t.Run("normal input", func(t *testing.T) {
		dto := &openapiEvaluator.EvaluatorFilterOption{
			SearchKeyword: gptr.Of("test"),
			Filters: &openapiEvaluator.EvaluatorFilters{
				LogicOp: gptr.Of(openapiEvaluator.EvaluatorFilterLogicOpAnd),
			},
		}
		do := OpenAPIEvaluatorFilterOptionDTO2DO(dto)
		assert.NotNil(t, do)
		assert.Equal(t, "test", *do.SearchKeyword)
	})
}

func TestOpenAPIEvaluatorContentDTO2DO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		do, err := OpenAPIEvaluatorContentDTO2DO(nil, entity.EvaluatorTypePrompt)
		assert.NoError(t, err)
		assert.Nil(t, do)
	})

	t.Run("prompt type", func(t *testing.T) {
		dto := &openapiEvaluator.EvaluatorContent{
			IsReceiveChatHistory: gptr.Of(true),
			PromptEvaluator: &openapiEvaluator.PromptEvaluator{
				Messages: []*openapiCommon.Message{
					{Role: gptr.Of("user"), Content: &openapiCommon.Content{Text: gptr.Of("hi")}},
				},
			},
		}
		do, err := OpenAPIEvaluatorContentDTO2DO(dto, entity.EvaluatorTypePrompt)
		assert.NoError(t, err)
		assert.NotNil(t, do)
		assert.True(t, *do.PromptEvaluatorVersion.ReceiveChatHistory)
		assert.Equal(t, 1, len(do.PromptEvaluatorVersion.MessageList))
	})

	t.Run("code type", func(t *testing.T) {
		dto := &openapiEvaluator.EvaluatorContent{
			CodeEvaluator: &openapiEvaluator.CodeEvaluator{
				LanguageType: gptr.Of(openapiEvaluator.LanguageTypePython),
				CodeContent:  gptr.Of("print(1)"),
			},
		}
		do, err := OpenAPIEvaluatorContentDTO2DO(dto, entity.EvaluatorTypeCode)
		assert.NoError(t, err)
		assert.NotNil(t, do)
		assert.Equal(t, entity.LanguageTypePython, do.CodeEvaluatorVersion.LanguageType)
		assert.Equal(t, "print(1)", do.CodeEvaluatorVersion.CodeContent)
	})

	t.Run("custom rpc type", func(t *testing.T) {
		dto := &openapiEvaluator.EvaluatorContent{
			CustomRPCEvaluator: &openapiEvaluator.CustomRPCEvaluator{
				ServiceName: gptr.Of("svc"),
				Cluster:     gptr.Of("cls"),
			},
		}
		do, err := OpenAPIEvaluatorContentDTO2DO(dto, entity.EvaluatorTypeCustomRPC)
		assert.NoError(t, err)
		assert.NotNil(t, do)
		assert.Equal(t, "svc", *do.CustomRPCEvaluatorVersion.ServiceName)
		assert.Equal(t, "cls", *do.CustomRPCEvaluatorVersion.Cluster)
	})
}

func TestOpenAPILanguageTypeDTO2DO(t *testing.T) {
	assert.Equal(t, entity.LanguageTypePython, OpenAPILanguageTypeDTO2DO(gptr.Of(openapiEvaluator.LanguageTypePython)))
	assert.Equal(t, entity.LanguageTypeJS, OpenAPILanguageTypeDTO2DO(gptr.Of(openapiEvaluator.LanguageTypeJS)))
	assert.Equal(t, entity.LanguageTypePython, OpenAPILanguageTypeDTO2DO(nil))
	assert.Equal(t, entity.LanguageTypePython, OpenAPILanguageTypeDTO2DO(gptr.Of(openapiEvaluator.LanguageType("999"))))
}

func TestOpenAPIEvaluatorDTO2DO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		do, err := OpenAPIEvaluatorDTO2DO(nil)
		assert.NoError(t, err)
		assert.Nil(t, do)
	})

	t.Run("normal input", func(t *testing.T) {
		dto := &openapiEvaluator.Evaluator{
			ID:            gptr.Of(int64(1)),
			Name:          gptr.Of("name"),
			EvaluatorType: gptr.Of(openapiEvaluator.EvaluatorTypePrompt),
			CurrentVersion: &openapiEvaluator.EvaluatorVersion{
				Version: gptr.Of("v1"),
				EvaluatorContent: &openapiEvaluator.EvaluatorContent{
					IsReceiveChatHistory: gptr.Of(true),
				},
			},
		}
		do, err := OpenAPIEvaluatorDTO2DO(dto)
		assert.NoError(t, err)
		assert.NotNil(t, do)
		assert.Equal(t, int64(1), do.ID)
		assert.Equal(t, "v1", do.GetVersion())
	})
}

func TestOpenAPIEvaluatorTypeDTO2DO(t *testing.T) {
	assert.Equal(t, entity.EvaluatorTypePrompt, OpenAPIEvaluatorTypeDTO2DO(gptr.Of(openapiEvaluator.EvaluatorTypePrompt)))
	assert.Equal(t, entity.EvaluatorTypeCode, OpenAPIEvaluatorTypeDTO2DO(gptr.Of(openapiEvaluator.EvaluatorTypeCode)))
	assert.Equal(t, entity.EvaluatorTypeCustomRPC, OpenAPIEvaluatorTypeDTO2DO(gptr.Of(openapiEvaluator.EvaluatorTypeCustomRPC)))
	assert.Equal(t, entity.EvaluatorTypePrompt, OpenAPIEvaluatorTypeDTO2DO(nil))
	assert.Equal(t, entity.EvaluatorTypePrompt, OpenAPIEvaluatorTypeDTO2DO(gptr.Of(openapiEvaluator.EvaluatorType("999"))))
}

func TestOpenAPIEvaluatorContentDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OpenAPIEvaluatorContentDO2DTO(nil))
	})

	t.Run("prompt type", func(t *testing.T) {
		do := &entity.Evaluator{
			EvaluatorType: entity.EvaluatorTypePrompt,
			PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
				ReceiveChatHistory: gptr.Of(true),
			},
		}
		dto := OpenAPIEvaluatorContentDO2DTO(do)
		assert.NotNil(t, dto)
		assert.True(t, *dto.IsReceiveChatHistory)
	})

	t.Run("code type", func(t *testing.T) {
		do := &entity.Evaluator{
			EvaluatorType: entity.EvaluatorTypeCode,
			CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypePython,
				CodeContent:  "print(1)",
			},
		}
		dto := OpenAPIEvaluatorContentDO2DTO(do)
		assert.NotNil(t, dto)
		assert.Equal(t, openapiEvaluator.LanguageTypePython, *dto.CodeEvaluator.LanguageType)
	})

	t.Run("custom rpc type", func(t *testing.T) {
		do := &entity.Evaluator{
			EvaluatorType: entity.EvaluatorTypeCustomRPC,
			CustomRPCEvaluatorVersion: &entity.CustomRPCEvaluatorVersion{
				ServiceName: gptr.Of("svc"),
			},
		}
		dto := OpenAPIEvaluatorContentDO2DTO(do)
		assert.NotNil(t, dto)
		assert.Equal(t, "svc", dto.CustomRPCEvaluator.GetServiceName())
	})
}

// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	openapiCommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	openapiEvalTarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/eval_target"
	openapiEvaluator "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/evaluator"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	openapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"

	domainCommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domaindoEvalTarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	domainEvaluator "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	domainExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestOpenAPIColumnEvalTargetDO2DTOs(t *testing.T) {
	t.Parallel()

	label := gptr.Of("label-1")
	from := []*entity.ColumnEvalTarget{
		{
			Name:  "col-1",
			Desc:  "desc-1",
			Label: label,
		},
		nil, // 应跳过
		{
			Name: "col-2",
			Desc: "desc-2",
		},
	}

	got := OpenAPIColumnEvalTargetDO2DTOs(from)

	if assert.Len(t, got, 2) {
		assert.Equal(t, "col-1", gptr.Indirect(got[0].Name))
		assert.Equal(t, "desc-1", gptr.Indirect(got[0].Description))
		assert.Same(t, label, got[0].Label)

		assert.Equal(t, "col-2", gptr.Indirect(got[1].Name))
		assert.Equal(t, "desc-2", gptr.Indirect(got[1].Description))
		assert.Nil(t, got[1].Label)
	}

	assert.Nil(t, OpenAPIColumnEvalTargetDO2DTOs(nil))
	assert.Nil(t, OpenAPIColumnEvalTargetDO2DTOs([]*entity.ColumnEvalTarget{}))
}

func TestOpenAPITargetFieldMappingDTO2Domain(t *testing.T) {
	t.Parallel()

	fieldName := "target"
	fromField := "source"
	dto := &openapiExperiment.TargetFieldMapping{
		FromEvalSet: []*openapiExperiment.FieldMapping{
			{FieldName: &fieldName, FromFieldName: &fromField},
			nil,
		},
	}

	converted := OpenAPITargetFieldMappingDTO2Domain(dto)
	assert.NotNil(t, converted)
	if assert.Len(t, converted.FromEvalSet, 1) {
		assert.Equal(t, fieldName, gptr.Indirect(converted.FromEvalSet[0].FieldName))
		assert.Equal(t, fromField, gptr.Indirect(converted.FromEvalSet[0].FromFieldName))
	}
	assert.Nil(t, OpenAPITargetFieldMappingDTO2Domain(nil))
}

func TestOpenAPIEvaluatorFieldMappingDTO2Domain(t *testing.T) {
	t.Parallel()

	fieldEval := "score"
	fromEval := "eval_score"
	fieldTarget := "input"
	fromTarget := "source_input"

	mapping := &openapiExperiment.EvaluatorFieldMapping{
		EvaluatorID: gptr.Of(int64(1)),
		Version:     gptr.Of("v1"),
		FromEvalSet: []*openapiExperiment.FieldMapping{{FieldName: &fieldEval, FromFieldName: &fromEval}},
		FromTarget:  []*openapiExperiment.FieldMapping{{FieldName: &fieldTarget, FromFieldName: &fromTarget}},
	}

	result := OpenAPIEvaluatorFieldMappingDTO2Domain([]*openapiExperiment.EvaluatorFieldMapping{mapping}, map[string]int64{"1_v1": 99})
	if assert.Len(t, result, 1) {
		assert.Equal(t, int64(99), result[0].EvaluatorVersionID)
		if assert.Len(t, result[0].FromEvalSet, 1) {
			assert.Equal(t, fieldEval, gptr.Indirect(result[0].FromEvalSet[0].FieldName))
			assert.Equal(t, fromEval, gptr.Indirect(result[0].FromEvalSet[0].FromFieldName))
		}
		if assert.Len(t, result[0].FromTarget, 1) {
			assert.Equal(t, fieldTarget, gptr.Indirect(result[0].FromTarget[0].FieldName))
			assert.Equal(t, fromTarget, gptr.Indirect(result[0].FromTarget[0].FromFieldName))
		}
	}

	assert.Nil(t, OpenAPIEvaluatorFieldMappingDTO2Domain(nil, nil))
	assert.Nil(t, OpenAPIEvaluatorFieldMappingDTO2Domain([]*openapiExperiment.EvaluatorFieldMapping{}, nil))
}

func TestOpenAPICreateEvalTargetParamDTO2Domain(t *testing.T) {
	t.Parallel()

	evalType := openapiEvalTarget.EvalTargetTypeCozeBot
	botInfo := openapiEvalTarget.CozeBotInfoTypeProductBot
	region := openapiEvalTarget.RegionCN
	param := &openapi.SubmitExperimentEvalTargetParam{
		SourceTargetID:      gptr.Of("123"),
		SourceTargetVersion: gptr.Of("2"),
		BotPublishVersion:   gptr.Of("456"),
		Env:                 gptr.Of("prod"),
		EvalTargetType:      &evalType,
		BotInfoType:         &botInfo,
		Region:              &region,
		CustomEvalTarget: &openapiEvalTarget.CustomEvalTarget{
			ID:   gptr.Of("1"),
			Name: gptr.Of("name"),
		},
	}

	converted := OpenAPICreateEvalTargetParamDTO2Domain(param)
	if assert.NotNil(t, converted) {
		assert.Equal(t, "123", gptr.Indirect(converted.SourceTargetID))
		assert.Equal(t, "456", gptr.Indirect(converted.BotPublishVersion))
		if assert.NotNil(t, converted.EvalTargetType) {
			assert.Equal(t, domaindoEvalTarget.EvalTargetType_CozeBot, *converted.EvalTargetType)
		}
		if assert.NotNil(t, converted.BotInfoType) {
			assert.Equal(t, domaindoEvalTarget.CozeBotInfoType_ProductBot, *converted.BotInfoType)
		}
		if assert.NotNil(t, converted.Region) {
			assert.Equal(t, domaindoEvalTarget.RegionCN, *converted.Region)
		}
		if assert.NotNil(t, converted.CustomEvalTarget) {
			assert.Equal(t, "1", converted.CustomEvalTarget.GetID())
		}
	}

	invalidType := openapiEvalTarget.EvalTargetType("invalid")
	assert.Nil(t, OpenAPICreateEvalTargetParamDTO2Domain(&openapi.SubmitExperimentEvalTargetParam{EvalTargetType: &invalidType}))
	invalidRegion := openapiEvalTarget.Region("invalid")
	assert.Nil(t, OpenAPICreateEvalTargetParamDTO2Domain(&openapi.SubmitExperimentEvalTargetParam{Region: &invalidRegion}))
}

func TestParseOpenAPIEvaluatorVersions(t *testing.T) {
	t.Parallel()

	ids, err := ParseOpenAPIEvaluatorVersions([]string{"1", "2"})
	assert.NoError(t, err)
	assert.Equal(t, []int64{1, 2}, ids)

	ids, err = ParseOpenAPIEvaluatorVersions(nil)
	assert.NoError(t, err)
	assert.Nil(t, ids)

	_, err = ParseOpenAPIEvaluatorVersions([]string{"abc"})
	assert.Error(t, err)
}

func TestDomainExperimentDTO2OpenAPI(t *testing.T) {
	t.Parallel()

	status := domainExpt.ExptStatus_Success
	start := int64(100)
	end := int64(200)
	itemConcur := int32(3)
	fieldName := "field"
	fromField := "from"
	jsonValue := "{}"

	domainExperiment := &domainExpt.Experiment{
		ID:            gptr.Of(int64(1)),
		Name:          gptr.Of("experiment"),
		ItemConcurNum: &itemConcur,
		Status:        &status,
		StartTime:     &start,
		EndTime:       &end,
		TargetFieldMapping: &domainExpt.TargetFieldMapping{
			FromEvalSet: []*domainExpt.FieldMapping{{FieldName: &fieldName, FromFieldName: &fromField}},
		},
		EvaluatorFieldMapping: []*domainExpt.EvaluatorFieldMapping{{
			EvaluatorVersionID: 11,
			FromEvalSet:        []*domainExpt.FieldMapping{{FieldName: &fieldName}},
			FromTarget:         []*domainExpt.FieldMapping{{FieldName: &fieldName}},
		}},
		TargetRuntimeParam: &domainCommon.RuntimeParam{JSONValue: &jsonValue},
	}
	domainExperiment.Evaluators = []*domainEvaluator.Evaluator{
		{
			EvaluatorID: gptr.Of(int64(5)),
			CurrentVersion: &domainEvaluator.EvaluatorVersion{
				ID:      gptr.Of(int64(11)),
				Version: gptr.Of("v1"),
			},
		},
	}

	converted := DomainExperimentDTO2OpenAPI(domainExperiment)
	if assert.NotNil(t, converted) {
		assert.Equal(t, domainExperiment.GetID(), converted.GetID())
		assert.Equal(t, openapiExperiment.ExperimentStatusSuccess, converted.GetStatus())
		assert.Equal(t, itemConcur, converted.GetItemConcurNum())
		if assert.NotNil(t, converted.TargetFieldMapping) && assert.Len(t, converted.TargetFieldMapping.FromEvalSet, 1) {
			assert.Equal(t, fieldName, converted.TargetFieldMapping.FromEvalSet[0].GetFieldName())
		}
		if assert.NotNil(t, converted.TargetRuntimeParam) {
			assert.Equal(t, jsonValue, converted.TargetRuntimeParam.GetJSONValue())
		}
		if assert.Len(t, converted.EvaluatorFieldMapping, 1) {
			assert.Equal(t, int64(5), converted.EvaluatorFieldMapping[0].GetEvaluatorID())
			assert.Equal(t, "v1", converted.EvaluatorFieldMapping[0].GetVersion())
		}
	}
	assert.Nil(t, DomainExperimentDTO2OpenAPI(nil))
}

func TestOpenAPIAggregatorResultsDO2DTOs(t *testing.T) {
	t.Parallel()

	value := 0.75
	results := []*entity.AggregatorResult{{
		AggregatorType: entity.Average,
		Data: &entity.AggregateData{
			DataType: entity.Double,
			Value:    &value,
		},
	}}

	converted := OpenAPIAggregatorResultsDO2DTOs(results)
	if assert.Len(t, converted, 1) {
		assert.Equal(t, openapiExperiment.AggregatorTypeAverage, converted[0].GetAggregatorType())
		if assert.NotNil(t, converted[0].Data) {
			assert.Equal(t, openapiExperiment.DataTypeDouble, converted[0].Data.GetDataType())
			assert.InDelta(t, value, converted[0].Data.GetValue(), 1e-9)
		}
	}
	assert.Nil(t, OpenAPIAggregatorResultsDO2DTOs(nil))
}

func TestMapOpenAPIEvalTargetType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   openapiEvalTarget.EvalTargetType
		want    domaindoEvalTarget.EvalTargetType
		wantErr bool
	}{
		{"coze_bot", openapiEvalTarget.EvalTargetTypeCozeBot, domaindoEvalTarget.EvalTargetType_CozeBot, false},
		{"loop_prompt", openapiEvalTarget.EvalTargetTypeCozeLoopPrompt, domaindoEvalTarget.EvalTargetType_CozeLoopPrompt, false},
		{"trace", openapiEvalTarget.EvalTargetTypeTrace, domaindoEvalTarget.EvalTargetType_Trace, false},
		{"workflow", openapiEvalTarget.EvalTargetTypeCozeWorkflow, domaindoEvalTarget.EvalTargetType_CozeWorkflow, false},
		{"volcengine", openapiEvalTarget.EvalTargetTypeVolcengineAgent, domaindoEvalTarget.EvalTargetType_VolcengineAgent, false},
		{"rpc", openapiEvalTarget.EvalTargetTypeCustomRPCServer, domaindoEvalTarget.EvalTargetType_CustomRPCServer, false},
		{"invalid", openapiEvalTarget.EvalTargetType("invalid"), 0, true},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapOpenAPIEvalTargetType(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMapOpenAPIRegion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   openapiEvalTarget.Region
		want    domaindoEvalTarget.Region
		wantErr bool
	}{
		{"boe", openapiEvalTarget.RegionBOE, domaindoEvalTarget.RegionBOE, false},
		{"cn", openapiEvalTarget.RegionCN, domaindoEvalTarget.RegionCN, false},
		{"i18n", openapiEvalTarget.RegionI18N, domaindoEvalTarget.RegionI18N, false},
		{"invalid", openapiEvalTarget.Region("invalid"), "", true},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapOpenAPIRegion(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDomainExperimentStatsDTO2OpenAPI(t *testing.T) {
	t.Parallel()

	stats := &domainExpt.ExptStatistics{
		PendingTurnCnt:    gptr.Of(int32(1)),
		SuccessTurnCnt:    gptr.Of(int32(2)),
		FailTurnCnt:       gptr.Of(int32(3)),
		TerminatedTurnCnt: gptr.Of(int32(4)),
		ProcessingTurnCnt: gptr.Of(int32(5)),
	}

	converted := DomainExperimentStatsDTO2OpenAPI(stats)
	if assert.NotNil(t, converted) {
		assert.Equal(t, int32(1), converted.GetPendingTurnCount())
		assert.Equal(t, int32(2), converted.GetSuccessTurnCount())
		assert.Equal(t, int32(3), converted.GetFailedTurnCount())
		assert.Equal(t, int32(4), converted.GetTerminatedTurnCount())
		assert.Equal(t, int32(5), converted.GetProcessingTurnCount())
	}
	assert.Nil(t, DomainExperimentStatsDTO2OpenAPI(nil))
}

func TestDomainBaseInfoDTO2OpenAPI(t *testing.T) {
	t.Parallel()

	createdAt := int64(10)
	updatedAt := int64(20)
	info := &domainCommon.BaseInfo{
		CreatedBy: &domainCommon.UserInfo{UserID: gptr.Of("creator"), Name: gptr.Of("name")},
		UpdatedBy: &domainCommon.UserInfo{UserID: gptr.Of("updater")},
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}

	converted := DomainBaseInfoDTO2OpenAPI(info)
	if assert.NotNil(t, converted) {
		assert.Equal(t, "creator", converted.GetCreatedBy().GetUserID())
		assert.Equal(t, "updater", converted.GetUpdatedBy().GetUserID())
		assert.Equal(t, createdAt, converted.GetCreatedAt())
		assert.Equal(t, updatedAt, converted.GetUpdatedAt())
	}
	assert.Nil(t, DomainBaseInfoDTO2OpenAPI(nil))
}

func TestDomainUserInfoDTO2OpenAPI(t *testing.T) {
	t.Parallel()

	info := &domainCommon.UserInfo{
		UserID:    gptr.Of("user"),
		Name:      gptr.Of("name"),
		AvatarURL: gptr.Of("avatar"),
		Email:     gptr.Of("mail"),
	}

	converted := DomainUserInfoDTO2OpenAPI(info)
	if assert.NotNil(t, converted) {
		assert.Equal(t, "user", converted.GetUserID())
		assert.Equal(t, "name", converted.GetName())
		assert.Equal(t, "avatar", converted.GetAvatarURL())
		assert.Equal(t, "mail", converted.GetEmail())
	}
	assert.Nil(t, DomainUserInfoDTO2OpenAPI(nil))
}

func TestMapExperimentStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input *domainExpt.ExptStatus
		want  openapiExperiment.ExperimentStatus
	}{
		{"pending", gptr.Of(domainExpt.ExptStatus_Pending), openapiExperiment.ExperimentStatusPending},
		{"processing", gptr.Of(domainExpt.ExptStatus_Processing), openapiExperiment.ExperimentStatusProcessing},
		{"success", gptr.Of(domainExpt.ExptStatus_Success), openapiExperiment.ExperimentStatusSuccess},
		{"failed", gptr.Of(domainExpt.ExptStatus_Failed), openapiExperiment.ExperimentStatusFailed},
		{"terminated", gptr.Of(domainExpt.ExptStatus_Terminated), openapiExperiment.ExperimentStatusTerminated},
		{"system_terminated", gptr.Of(domainExpt.ExptStatus_SystemTerminated), openapiExperiment.ExperimentStatusSystemTerminated},
		{"draining", gptr.Of(domainExpt.ExptStatus_Draining), openapiExperiment.ExperimentStatusDraining},
		{"unknown", gptr.Of(domainExpt.ExptStatus(999)), ""},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			converted := mapExperimentStatus(tt.input)
			if tt.input == nil {
				assert.Nil(t, converted)
				return
			}
			if assert.NotNil(t, converted) {
				assert.Equal(t, tt.want, *converted)
			}
		})
	}
	assert.Nil(t, mapExperimentStatus(nil))
}

func TestOpenAPIExptDO2DTO(t *testing.T) {
	t.Parallel()

	start := time.Unix(100, 0)
	end := time.Unix(200, 0)
	runtimeJSON := "{\"foo\":\"bar\"}"

	experiment := &entity.Experiment{
		ID:          10,
		Name:        "exp",
		Description: "desc",
		CreatedBy:   "creator",
		Status:      entity.ExptStatus_Success,
		StartAt:     &start,
		EndAt:       &end,
		Stats: &entity.ExptStats{
			PendingItemCnt:    1,
			SuccessItemCnt:    2,
			FailItemCnt:       3,
			TerminatedItemCnt: 4,
			ProcessingItemCnt: 5,
		},
		EvalConf: &entity.EvaluationConfiguration{
			ItemConcurNum: gptr.Of(3),
			ConnectorConf: entity.Connector{
				TargetConf: &entity.TargetConf{
					IngressConf: &entity.TargetIngressConf{
						EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "output", FromField: "input"}}},
						CustomConf:     &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam, Value: runtimeJSON}}},
					},
				},
				EvaluatorsConf: &entity.EvaluatorsConf{
					EvaluatorConf: []*entity.EvaluatorConf{{
						EvaluatorVersionID: 7,
						IngressConf: &entity.EvaluatorIngressConf{
							EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "score", FromField: "eval_score"}}},
							TargetAdapter:  &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "target", FromField: "target_field"}}},
						},
					}},
				},
			},
		},
		Evaluators: []*entity.Evaluator{{
			ID:            88,
			EvaluatorType: entity.EvaluatorTypePrompt,
			PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
				ID:          7,
				EvaluatorID: 99,
				Version:     "v1",
			},
		}},
	}

	converted := OpenAPIExptDO2DTO(experiment)
	if assert.NotNil(t, converted) {
		assert.Equal(t, experiment.ID, gptr.Indirect(converted.ID))
		assert.Equal(t, experiment.Name, gptr.Indirect(converted.Name))
		assert.Equal(t, "desc", gptr.Indirect(converted.Description))
		assert.Equal(t, openapiExperiment.ExperimentStatusSuccess, converted.GetStatus())
		assert.Equal(t, int32(3), converted.GetItemConcurNum())
		assert.Equal(t, start.Unix(), gptr.Indirect(converted.StartedAt))
		assert.Equal(t, end.Unix(), gptr.Indirect(converted.EndedAt))
		if assert.NotNil(t, converted.TargetFieldMapping) {
			assert.Equal(t, "output", converted.TargetFieldMapping.FromEvalSet[0].GetFieldName())
			assert.Equal(t, "input", converted.TargetFieldMapping.FromEvalSet[0].GetFromFieldName())
		}
		if assert.NotNil(t, converted.TargetRuntimeParam) {
			assert.Equal(t, runtimeJSON, converted.TargetRuntimeParam.GetJSONValue())
		}
		if assert.Len(t, converted.EvaluatorFieldMapping, 1) {
			assert.Equal(t, int64(88), converted.EvaluatorFieldMapping[0].GetEvaluatorID())
			assert.Equal(t, "v1", converted.EvaluatorFieldMapping[0].GetVersion())
			assert.Equal(t, "score", converted.EvaluatorFieldMapping[0].FromEvalSet[0].GetFieldName())
			assert.Equal(t, "target", converted.EvaluatorFieldMapping[0].FromTarget[0].GetFieldName())
		}
		if assert.NotNil(t, converted.ExptStats) {
			assert.Equal(t, int32(1), converted.ExptStats.GetPendingTurnCount())
		}
		if assert.NotNil(t, converted.BaseInfo) && assert.NotNil(t, converted.BaseInfo.CreatedBy) {
			assert.Equal(t, "creator", converted.BaseInfo.CreatedBy.GetUserID())
		}
	}
	assert.Nil(t, OpenAPIExptDO2DTO(nil))
}

func TestOpenAPIExperimentStatusDO2DTO(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input entity.ExptStatus
		want  *openapiExperiment.ExperimentStatus
	}{
		{"pending", entity.ExptStatus_Pending, gptr.Of(openapiExperiment.ExperimentStatusPending)},
		{"processing", entity.ExptStatus_Processing, gptr.Of(openapiExperiment.ExperimentStatusProcessing)},
		{"success", entity.ExptStatus_Success, gptr.Of(openapiExperiment.ExperimentStatusSuccess)},
		{"failed", entity.ExptStatus_Failed, gptr.Of(openapiExperiment.ExperimentStatusFailed)},
		{"terminated", entity.ExptStatus_Terminated, gptr.Of(openapiExperiment.ExperimentStatusTerminated)},
		{"system_terminated", entity.ExptStatus_SystemTerminated, gptr.Of(openapiExperiment.ExperimentStatusSystemTerminated)},
		{"draining", entity.ExptStatus_Draining, gptr.Of(openapiExperiment.ExperimentStatusDraining)},
		{"unknown", entity.ExptStatus(999), nil},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			converted := OpenAPIExperimentStatusDO2DTO(tt.input)
			if tt.want == nil {
				assert.Nil(t, converted)
				return
			}
			if assert.NotNil(t, converted) {
				assert.Equal(t, *tt.want, *converted)
			}
		})
	}
}

func TestExtractTargetIngressInfo(t *testing.T) {
	t.Parallel()

	runtimeJSON := "{\"key\":1}"
	mapping, param := extractTargetIngressInfo(&entity.TargetConf{
		IngressConf: &entity.TargetIngressConf{
			EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "field", FromField: "source"}}},
			CustomConf:     &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam, Value: runtimeJSON}}},
		},
	})
	if assert.NotNil(t, mapping) {
		assert.Equal(t, "field", mapping.FromEvalSet[0].GetFieldName())
	}
	if assert.NotNil(t, param) {
		assert.Equal(t, runtimeJSON, param.GetJSONValue())
	}

	m, p := extractTargetIngressInfo(nil)
	assert.Nil(t, m)
	assert.Nil(t, p)
}

func TestOpenAPIExperimentStatsDO2DTO(t *testing.T) {
	t.Parallel()

	stats := &entity.ExptStats{
		PendingItemCnt:    1,
		SuccessItemCnt:    2,
		FailItemCnt:       3,
		TerminatedItemCnt: 4,
		ProcessingItemCnt: 5,
	}

	converted := openAPIExperimentStatsDO2DTO(stats)
	if assert.NotNil(t, converted) {
		assert.Equal(t, int32(1), gptr.Indirect(converted.PendingTurnCount))
		assert.Equal(t, int32(2), gptr.Indirect(converted.SuccessTurnCount))
		assert.Equal(t, int32(3), gptr.Indirect(converted.FailedTurnCount))
		assert.Equal(t, int32(4), gptr.Indirect(converted.TerminatedTurnCount))
		assert.Equal(t, int32(5), gptr.Indirect(converted.ProcessingTurnCount))
	}
	assert.Nil(t, openAPIExperimentStatsDO2DTO(nil))
}

func TestItemRunStateDO2DTO(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input entity.ItemRunState
		want  *openapiExperiment.ItemRunState
	}{
		{"queueing", entity.ItemRunState_Queueing, gptr.Of(openapiExperiment.ItemRunStateQueueing)},
		{"processing", entity.ItemRunState_Processing, gptr.Of(openapiExperiment.ItemRunStateProcessing)},
		{"success", entity.ItemRunState_Success, gptr.Of(openapiExperiment.ItemRunStateSuccess)},
		{"fail", entity.ItemRunState_Fail, gptr.Of(openapiExperiment.ItemRunStateFail)},
		{"terminal", entity.ItemRunState_Terminal, gptr.Of(openapiExperiment.ItemRunStateTerminal)},
		{"unknown", entity.ItemRunState(-1), nil},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			converted := ItemRunStateDO2DTO(tt.input)
			if tt.want == nil {
				assert.Nil(t, converted)
				return
			}
			if assert.NotNil(t, converted) {
				assert.Equal(t, *tt.want, *converted)
			}
		})
	}
}

func TestTurnRunStateDO2DTO(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input entity.TurnRunState
		want  *openapiExperiment.TurnRunState
	}{
		{"queueing", entity.TurnRunState_Queueing, gptr.Of(openapiExperiment.TurnRunStateQueueing)},
		{"processing", entity.TurnRunState_Processing, gptr.Of(openapiExperiment.TurnRunStateProcessing)},
		{"success", entity.TurnRunState_Success, gptr.Of(openapiExperiment.TurnRunStateSuccess)},
		{"fail", entity.TurnRunState_Fail, gptr.Of(openapiExperiment.TurnRunStateFail)},
		{"terminal", entity.TurnRunState_Terminal, gptr.Of(openapiExperiment.TurnRunStateTerminal)},
		{"invalid", entity.TurnRunState(-1), nil},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			converted := TurnRunStateDO2DTO(tt.input)
			if tt.want == nil {
				assert.Nil(t, converted)
				return
			}
			if assert.NotNil(t, converted) {
				assert.Equal(t, *tt.want, *converted)
			}
		})
	}
}

func TestOpenAPITurnResultsDO2DTOs(t *testing.T) {
	t.Parallel()

	turnID := int64(123)
	result := &entity.TurnResult{
		TurnID: turnID,
		ExperimentResults: []*entity.ExperimentResult{{
			Payload: &entity.ExperimentTurnPayload{
				EvalSet: &entity.TurnEvalSet{Turn: &entity.Turn{ID: 456}},
			},
		}},
	}

	converted := openAPITurnResultsDO2DTOs([]*entity.TurnResult{result, nil})
	if assert.Len(t, converted, 1) {
		assert.Equal(t, strconv.FormatInt(turnID, 10), converted[0].GetTurnID())
		if assert.NotNil(t, converted[0].Payload) {
			assert.NotNil(t, converted[0].Payload.EvalSetTurn)
		}
	}
	assert.Nil(t, openAPITurnResultsDO2DTOs(nil))
}

func TestOpenAPIResultPayloadDO2DTO(t *testing.T) {
	t.Parallel()

	score := 0.9
	reasoning := "reason"
	correctedScore := 0.95
	correctedReason := "corrected"
	payload := &entity.ExperimentTurnPayload{
		EvalSet: &entity.TurnEvalSet{Turn: &entity.Turn{ID: 1}},
		EvaluatorOutput: &entity.TurnEvaluatorOutput{
			EvaluatorRecords: map[int64]*entity.EvaluatorRecord{
				1: {
					ID:                 100,
					EvaluatorVersionID: 200,
					ItemID:             300,
					TurnID:             400,
					Status:             entity.EvaluatorRunStatusSuccess,
					LogID:              "log",
					TraceID:            "trace",
					BaseInfo:           &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: gptr.Of("creator")}},
					EvaluatorOutputData: &entity.EvaluatorOutputData{
						EvaluatorResult: &entity.EvaluatorResult{
							Score: &score,
							Correction: &entity.Correction{
								Score:   &correctedScore,
								Explain: correctedReason,
							},
							Reasoning: reasoning,
						},
						EvaluatorUsage:    &entity.EvaluatorUsage{InputTokens: 10, OutputTokens: 20},
						EvaluatorRunError: &entity.EvaluatorRunError{Code: 1, Message: "error"},
						TimeConsumingMS:   30,
					},
				},
			},
		},
		TargetOutput: &entity.TurnTargetOutput{
			EvalTargetRecord: &entity.EvalTargetRecord{
				ID:              500,
				TargetID:        600,
				TargetVersionID: 700,
				ItemID:          800,
				TurnID:          900,
				LogID:           "target-log",
				TraceID:         "target-trace",
				EvalTargetOutputData: &entity.EvalTargetOutputData{
					OutputFields: map[string]*entity.Content{
						"field": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("text")},
					},
					EvalTargetUsage:    &entity.EvalTargetUsage{InputTokens: 1, OutputTokens: 2},
					EvalTargetRunError: &entity.EvalTargetRunError{Code: 2, Message: "target-error"},
					TimeConsumingMS:    gptr.Of(int64(40)),
				},
				Status:   gptr.Of(entity.EvalTargetRunStatusSuccess),
				BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: gptr.Of("target_creator")}},
			},
		},
		SystemInfo: &entity.TurnSystemInfo{TurnRunState: entity.TurnRunState_Success},
	}

	converted := openAPIResultPayloadDO2DTO(&entity.ExperimentResult{Payload: payload})
	if assert.NotNil(t, converted) {
		assert.NotNil(t, converted.EvalSetTurn)
		if assert.Len(t, converted.EvaluatorRecords, 1) {
			assert.Equal(t, int64(100), converted.EvaluatorRecords[0].GetID())
		}
		if assert.NotNil(t, converted.TargetRecord) {
			assert.Equal(t, int64(500), converted.TargetRecord.GetID())
			assert.Equal(t, int64(600), converted.TargetRecord.GetTargetID())
		}
		if assert.NotNil(t, converted.SystemInfo) {
			assert.Equal(t, openapiExperiment.TurnRunStateSuccess, converted.SystemInfo.GetTurnRunState())
		}
	}
	assert.Nil(t, openAPIResultPayloadDO2DTO(nil))
	assert.Nil(t, openAPIResultPayloadDO2DTO(&entity.ExperimentResult{Payload: &entity.ExperimentTurnPayload{}}))

	payload.SystemInfo = nil
	payload.TargetOutput = nil
	payload.EvaluatorOutput = nil
	converted = openAPIResultPayloadDO2DTO(&entity.ExperimentResult{Payload: payload})
	assert.NotNil(t, converted)

	payload.EvalSet = nil
	converted = openAPIResultPayloadDO2DTO(&entity.ExperimentResult{Payload: payload})
	assert.Nil(t, converted)
}

func TestOpenAPIEvaluatorRecordDO2DTO(t *testing.T) {
	t.Parallel()

	score := 1.0
	record := &entity.EvaluatorRecord{
		ID:                 1,
		EvaluatorVersionID: 2,
		ItemID:             3,
		TurnID:             4,
		Status:             entity.EvaluatorRunStatusSuccess,
		LogID:              "log",
		TraceID:            "trace",
		BaseInfo:           &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: gptr.Of("user")}},
		EvaluatorOutputData: &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{Score: &score, Reasoning: "reason"},
			EvaluatorUsage:  &entity.EvaluatorUsage{InputTokens: 10, OutputTokens: 20},
			EvaluatorRunError: &entity.EvaluatorRunError{
				Code:    1,
				Message: "err",
			},
			TimeConsumingMS: 30,
		},
	}

	converted := openAPIEvaluatorRecordDO2DTO(record)
	if assert.NotNil(t, converted) {
		assert.Equal(t, int64(1), converted.GetID())
		assert.Equal(t, openapiEvaluator.EvaluatorRunStatusSuccess, converted.GetStatus())
		if assert.NotNil(t, converted.EvaluatorOutputData) {
			assert.Equal(t, score, gptr.Indirect(converted.EvaluatorOutputData.EvaluatorResult_.Score))
			assert.Equal(t, int64(10), gptr.Indirect(converted.EvaluatorOutputData.EvaluatorUsage.InputTokens))
			assert.Equal(t, int32(1), gptr.Indirect(converted.EvaluatorOutputData.EvaluatorRunError.Code))
		}
	}
	assert.Nil(t, openAPIEvaluatorRecordDO2DTO(nil))
}

func TestOpenAPIAggregatorTypeDO2DTO(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input entity.AggregatorType
		want  *openapiExperiment.AggregatorType
	}{
		{"average", entity.Average, gptr.Of(openapiExperiment.AggregatorTypeAverage)},
		{"sum", entity.Sum, gptr.Of(openapiExperiment.AggregatorTypeSum)},
		{"max", entity.Max, gptr.Of(openapiExperiment.AggregatorTypeMax)},
		{"min", entity.Min, gptr.Of(openapiExperiment.AggregatorTypeMin)},
		{"distribution", entity.Distribution, gptr.Of(openapiExperiment.AggregatorTypeDistribution)},
		{"unknown", entity.AggregatorType(999), nil},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			converted := openAPIAggregatorTypeDO2DTO(tt.input)
			if tt.want == nil {
				assert.Nil(t, converted)
				return
			}
			if assert.NotNil(t, converted) {
				assert.Equal(t, *tt.want, *converted)
			}
		})
	}
}

func TestOpenAPIScoreDistributionDO2DTO(t *testing.T) {
	t.Parallel()

	// 测试正常情况
	score1 := "0.8"
	count1 := int64(10)
	percentage1 := 0.25
	score2 := "0.9"
	count2 := int64(20)
	percentage2 := 0.75

	data := &entity.ScoreDistributionData{
		ScoreDistributionItems: []*entity.ScoreDistributionItem{
			{Score: score1, Count: count1, Percentage: percentage1},
			{Score: score2, Count: count2, Percentage: percentage2},
		},
	}

	converted := openAPIScoreDistributionDO2DTO(data)
	if assert.NotNil(t, converted) {
		if assert.Len(t, converted.ScoreDistributionItems, 2) {
			assert.Equal(t, score1, gptr.Indirect(converted.ScoreDistributionItems[0].Score))
			assert.Equal(t, count1, gptr.Indirect(converted.ScoreDistributionItems[0].Count))
			assert.Equal(t, percentage1, gptr.Indirect(converted.ScoreDistributionItems[0].Percentage))
			assert.Equal(t, score2, gptr.Indirect(converted.ScoreDistributionItems[1].Score))
			assert.Equal(t, count2, gptr.Indirect(converted.ScoreDistributionItems[1].Count))
			assert.Equal(t, percentage2, gptr.Indirect(converted.ScoreDistributionItems[1].Percentage))
		}
	}

	// 测试空数据
	assert.Nil(t, openAPIScoreDistributionDO2DTO(nil))

	// 测试空项目列表
	emptyData := &entity.ScoreDistributionData{
		ScoreDistributionItems: []*entity.ScoreDistributionItem{},
	}
	assert.Nil(t, openAPIScoreDistributionDO2DTO(emptyData))

	// 测试包含nil项目
	dataWithNil := &entity.ScoreDistributionData{
		ScoreDistributionItems: []*entity.ScoreDistributionItem{
			{Score: score1, Count: count1, Percentage: percentage1},
			nil,
			{Score: score2, Count: count2, Percentage: percentage2},
		},
	}
	convertedWithNil := openAPIScoreDistributionDO2DTO(dataWithNil)
	if assert.NotNil(t, convertedWithNil) {
		// nil项目应该被跳过，只剩2个有效项目
		assert.Len(t, convertedWithNil.ScoreDistributionItems, 2)
	}
}

func TestOpenAPIEvalTargetDO2DTO(t *testing.T) {
	t.Parallel()

	// Case 1: nil input
	assert.Nil(t, OpenAPIEvalTargetDO2DTO(nil))

	// Case 2: valid input with version and base info
	targetDO := &entity.EvalTarget{
		ID:             1,
		SourceTargetID: "2",
		EvalTargetType: entity.EvalTargetTypeCozeBot,
		EvalTargetVersion: &entity.EvalTargetVersion{
			ID:                  10,
			TargetID:            1,
			SourceTargetVersion: "v1",
			InputSchema:         []*entity.ArgsSchema{{Key: gptr.Of("input")}},
			OutputSchema:        []*entity.ArgsSchema{{Key: gptr.Of("output")}},
			BaseInfo:            &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: gptr.Of("user1")}},
		},
		BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: gptr.Of("user1")}},
	}

	got := OpenAPIEvalTargetDO2DTO(targetDO)
	if assert.NotNil(t, got) {
		assert.Equal(t, targetDO.ID, gptr.Indirect(got.ID))
		assert.Equal(t, targetDO.SourceTargetID, gptr.Indirect(got.SourceTargetID))
		assert.Equal(t, openapiEvalTarget.EvalTargetTypeCozeBot, gptr.Indirect(got.EvalTargetType))
		if assert.NotNil(t, got.EvalTargetVersion) {
			assert.Equal(t, targetDO.EvalTargetVersion.ID, gptr.Indirect(got.EvalTargetVersion.ID))
			assert.Equal(t, targetDO.EvalTargetVersion.TargetID, gptr.Indirect(got.EvalTargetVersion.TargetID))
			assert.Equal(t, targetDO.EvalTargetVersion.SourceTargetVersion, gptr.Indirect(got.EvalTargetVersion.SourceTargetVersion))
			if assert.NotNil(t, got.EvalTargetVersion.EvalTargetContent) {
				assert.Len(t, got.EvalTargetVersion.EvalTargetContent.InputSchemas, 1)
				assert.Len(t, got.EvalTargetVersion.EvalTargetContent.OutputSchemas, 1)
			}
		}
		if assert.NotNil(t, got.BaseInfo) {
			assert.Equal(t, "user1", got.BaseInfo.CreatedBy.GetUserID())
		}
	}

	// Case 3: input with Prompt type
	promptTargetDO := &entity.EvalTarget{
		ID:             3,
		EvalTargetType: entity.EvalTargetTypeLoopPrompt,
		EvalTargetVersion: &entity.EvalTargetVersion{
			ID: 30,
			Prompt: &entity.LoopPrompt{
				PromptID:     300,
				Version:      "v3",
				Name:         "prompt-3",
				PromptKey:    "key-3",
				SubmitStatus: entity.SubmitStatus_Submitted,
				Description:  "desc-3",
			},
		},
	}

	gotPrompt := OpenAPIEvalTargetDO2DTO(promptTargetDO)
	if assert.NotNil(t, gotPrompt) {
		assert.Equal(t, openapiEvalTarget.EvalTargetTypeCozeLoopPrompt, gptr.Indirect(gotPrompt.EvalTargetType))
		if assert.NotNil(t, gotPrompt.EvalTargetVersion) && assert.NotNil(t, gotPrompt.EvalTargetVersion.EvalTargetContent) {
			promptDTO := gotPrompt.EvalTargetVersion.EvalTargetContent.Prompt
			if assert.NotNil(t, promptDTO) {
				assert.Equal(t, int64(300), gptr.Indirect(promptDTO.PromptID))
				assert.Equal(t, "v3", gptr.Indirect(promptDTO.Version))
				assert.Equal(t, openapiEvalTarget.SubmitStatusSubmitted, gptr.Indirect(promptDTO.SubmitStatus))
			}
		}
	}

	// Case 4: input with CustomRPCServer type
	rpcTargetDO := &entity.EvalTarget{
		ID:             4,
		EvalTargetType: entity.EvalTargetTypeCustomRPCServer,
		EvalTargetVersion: &entity.EvalTargetVersion{
			ID: 40,
			CustomRPCServer: &entity.CustomRPCServer{
				ID:             400,
				ServerName:     "rpc-server",
				AccessProtocol: entity.AccessProtocolRPC,
				Regions:        []entity.Region{entity.RegionCN},
			},
		},
	}

	gotRPC := OpenAPIEvalTargetDO2DTO(rpcTargetDO)
	if assert.NotNil(t, gotRPC) {
		assert.Equal(t, openapiEvalTarget.EvalTargetTypeCustomRPCServer, gptr.Indirect(gotRPC.EvalTargetType))
		if assert.NotNil(t, gotRPC.EvalTargetVersion) && assert.NotNil(t, gotRPC.EvalTargetVersion.EvalTargetContent) {
			rpcDTO := gotRPC.EvalTargetVersion.EvalTargetContent.CustomRPCServer
			if assert.NotNil(t, rpcDTO) {
				assert.Equal(t, int64(400), gptr.Indirect(rpcDTO.ID))
				assert.Equal(t, "rpc-server", gptr.Indirect(rpcDTO.ServerName))
				assert.Equal(t, openapiEvalTarget.AccessProtocolRPC, gptr.Indirect(rpcDTO.AccessProtocol))
			}
		}
	}
}

func TestMapEntitySubmitStatusToOpenAPI(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  entity.SubmitStatus
		expect openapiEvalTarget.SubmitStatus
	}{
		{"unsubmit", entity.SubmitStatus_UnSubmit, openapiEvalTarget.SubmitStatusUnSubmit},
		{"submitted", entity.SubmitStatus_Submitted, openapiEvalTarget.SubmitStatusSubmitted},
		{"unknown", entity.SubmitStatus(999), ""},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := mapEntitySubmitStatusToOpenAPI(tt.input)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestConvertEntityEvalTargetTypeToOpenAPI(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  entity.EvalTargetType
		expect openapiEvalTarget.EvalTargetType
	}{
		{"coze_bot", entity.EvalTargetTypeCozeBot, openapiEvalTarget.EvalTargetTypeCozeBot},
		{"loop_prompt", entity.EvalTargetTypeLoopPrompt, openapiEvalTarget.EvalTargetTypeCozeLoopPrompt},
		{"loop_trace", entity.EvalTargetTypeLoopTrace, openapiEvalTarget.EvalTargetTypeTrace},
		{"workflow", entity.EvalTargetTypeCozeWorkflow, openapiEvalTarget.EvalTargetTypeCozeWorkflow},
		{"volcengine", entity.EvalTargetTypeVolcengineAgent, openapiEvalTarget.EvalTargetTypeVolcengineAgent},
		{"rpc_server", entity.EvalTargetTypeCustomRPCServer, openapiEvalTarget.EvalTargetTypeCustomRPCServer},
		{"unknown", entity.EvalTargetType(999), ""},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := convertEntityEvalTargetTypeToOpenAPI(tt.input)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestOpenAPIHTTPInfoDO2DTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, OpenAPIHTTPInfoDO2DTO(nil))

	do := &entity.HTTPInfo{
		Method: "POST",
		Path:   "/api/v1/invoke",
	}

	got := OpenAPIHTTPInfoDO2DTO(do)
	if assert.NotNil(t, got) {
		assert.Equal(t, "POST", got.GetMethod())
		assert.Equal(t, "/api/v1/invoke", got.GetPath())
	}
}

func TestOpenAPICustomEvalTargetDO2DTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, OpenAPICustomEvalTargetDO2DTO(nil))

	do := &entity.CustomEvalTarget{
		ID:        gptr.Of("123"),
		Name:      gptr.Of("custom-target"),
		AvatarURL: gptr.Of("http://avatar.url"),
		Ext:       map[string]string{"foo": "bar"},
	}

	got := OpenAPICustomEvalTargetDO2DTO(do)
	if assert.NotNil(t, got) {
		assert.Equal(t, "123", got.GetID())
		assert.Equal(t, "custom-target", got.GetName())
		assert.Equal(t, "http://avatar.url", got.GetAvatarURL())
		assert.Equal(t, map[string]string{"foo": "bar"}, got.GetExt())
	}
}

func TestOpenAPIExptTemplateDO2DTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, OpenAPIExptTemplateDO2DTO(nil))

	template := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          1,
			WorkspaceID: 10,
			Name:        "test",
			ExptType:    entity.ExptType_Offline,
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID: 100,
			EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{
				{EvaluatorVersionID: 500, Version: "v1"},
			},
		},
		FieldMappingConfig: &entity.ExptFieldMapping{
			ItemConcurNum: gptr.Of(3),
			TargetFieldMapping: &entity.TargetFieldMapping{
				FromEvalSet: []*entity.ExptTemplateFieldMapping{
					{FieldName: "f1", FromFieldName: "s1"},
				},
			},
		},
	}

	got := OpenAPIExptTemplateDO2DTO(template)
	if assert.NotNil(t, got) {
		assert.Equal(t, int64(1), got.Meta.GetID())
		assert.Equal(t, int64(100), got.TripleConfig.GetEvalSetID())
		assert.Len(t, got.TripleConfig.EvaluatorVersions, 1)
		assert.Equal(t, int32(3), got.FieldMappingConfig.GetItemConcurNum())
	}
}

func TestOpenAPICreateExptTemplateReq2Domain(t *testing.T) {
	t.Parallel()

	req := &openapi.CreateExptTemplateOApiRequest{
		WorkspaceID: gptr.Of(int64(10)),
		Meta: &openapiExperiment.ExptTemplateMeta{
			Name:     gptr.Of("test"),
			ExptType: gptr.Of(openapiExperiment.ExperimentTypeOffline),
		},
		TripleConfig: &openapiExperiment.ExptTuple{
			EvalSetID: gptr.Of(int64(100)),
		},
	}

	got, err := OpenAPICreateExptTemplateReq2Domain(req)
	assert.NoError(t, err)
	if assert.NotNil(t, got) {
		assert.Equal(t, int64(10), got.SpaceID)
		assert.Equal(t, "test", got.Name)
		assert.Equal(t, int64(100), got.EvalSetID)
	}

	// Case 2: full request
	fullReq := &openapi.CreateExptTemplateOApiRequest{
		WorkspaceID: gptr.Of(int64(10)),
		Meta: &openapiExperiment.ExptTemplateMeta{
			Name:        gptr.Of("test-full"),
			Description: gptr.Of("desc"),
			ExptType:    gptr.Of(openapiExperiment.ExperimentTypeOnline),
		},
		TripleConfig: &openapiExperiment.ExptTuple{
			EvalSetID:        gptr.Of(int64(100)),
			EvalSetVersionID: gptr.Of(int64(101)),
			TargetID:         gptr.Of(int64(200)),
			TargetVersionID:  gptr.Of(int64(201)),
			EvaluatorVersions: []*openapiEvaluator.EvaluatorVersion{
				{ID: gptr.Of(int64(300)), Version: gptr.Of("v1")},
				nil,
			},
		},
		FieldMappingConfig: &openapiExperiment.ExptFieldMapping{
			ItemConcurNum: gptr.Of(int32(5)),
			TargetFieldMapping: &openapiExperiment.TargetFieldMapping{
				FromEvalSet: []*openapiExperiment.FieldMapping{
					{FieldName: gptr.Of("f1"), FromFieldName: gptr.Of("s1")},
				},
			},
			TargetRuntimeParam: &openapiCommon.RuntimeParam{
				JSONValue: gptr.Of("{}"),
			},
			EvaluatorFieldMapping: []*openapiExperiment.EvaluatorFieldMapping{
				{
					EvaluatorID: gptr.Of(int64(300)),
					Version:     gptr.Of("v1"),
					FromEvalSet: []*openapiExperiment.FieldMapping{
						{FieldName: gptr.Of("ef1"), FromFieldName: gptr.Of("es1")},
					},
					FromTarget: []*openapiExperiment.FieldMapping{
						{FieldName: gptr.Of("tf1"), FromFieldName: gptr.Of("ts1")},
					},
				},
				nil,
			},
		},
		DefaultEvaluatorsConcurNum: gptr.Of(int32(10)),
	}

	gotFull, err := OpenAPICreateExptTemplateReq2Domain(fullReq)
	assert.NoError(t, err)
	if assert.NotNil(t, gotFull) {
		assert.Equal(t, "test-full", gotFull.Name)
		assert.Equal(t, entity.ExptType_Online, gotFull.ExptType)
		assert.Equal(t, int64(300), gotFull.EvaluatorIDVersionItems[0].EvaluatorVersionID)
		if assert.NotNil(t, gotFull.TemplateConf) {
			assert.Equal(t, 5, *gotFull.TemplateConf.ItemConcurNum)
			assert.Equal(t, 10, *gotFull.TemplateConf.EvaluatorsConcurNum)
			if assert.NotNil(t, gotFull.TemplateConf.ConnectorConf.TargetConf) && assert.NotNil(t, gotFull.TemplateConf.ConnectorConf.TargetConf.IngressConf) {
				assert.Equal(t, "f1", gotFull.TemplateConf.ConnectorConf.TargetConf.IngressConf.EvalSetAdapter.FieldConfs[0].FieldName)
			}
			if assert.NotNil(t, gotFull.TemplateConf.ConnectorConf.EvaluatorsConf) {
				assert.Len(t, gotFull.TemplateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf, 1)
			}
		}
	}
}

func TestOpenAPIUpdateExptTemplateReq2Domain(t *testing.T) {
	t.Parallel()

	req := &openapi.UpdateExptTemplateOApiRequest{
		TemplateID:  gptr.Of(int64(1)),
		WorkspaceID: gptr.Of(int64(10)),
		Meta: &openapiExperiment.ExptTemplateMeta{
			Name:     gptr.Of("updated"),
			ExptType: gptr.Of(openapiExperiment.ExperimentTypeOffline),
		},
		TripleConfig: &openapiExperiment.ExptTuple{
			EvalSetVersionID: gptr.Of(int64(102)),
		},
		FieldMappingConfig: &openapiExperiment.ExptFieldMapping{
			ItemConcurNum: gptr.Of(int32(2)),
		},
	}

	got, err := OpenAPIUpdateExptTemplateReq2Domain(req)
	assert.NoError(t, err)
	if assert.NotNil(t, got) {
		assert.Equal(t, int64(1), got.TemplateID)
		assert.Equal(t, "updated", got.Name)
		assert.Equal(t, int64(102), got.EvalSetVersionID)
		if assert.NotNil(t, got.TemplateConf) {
			assert.Equal(t, 2, *got.TemplateConf.ItemConcurNum)
		}
	}

	gotNil, errNil := OpenAPIUpdateExptTemplateReq2Domain(nil)
	assert.Nil(t, gotNil)
	assert.Nil(t, errNil)
}

func TestOpenAPIRuntimeParamDTO2Domain(t *testing.T) {
	t.Parallel()

	assert.Nil(t, OpenAPIRuntimeParamDTO2Domain(nil))

	p1 := &openapiCommon.RuntimeParam{}
	assert.NotNil(t, OpenAPIRuntimeParamDTO2Domain(p1))

	jsonVal := "{}"
	p2 := &openapiCommon.RuntimeParam{JSONValue: &jsonVal}
	got := OpenAPIRuntimeParamDTO2Domain(p2)
	assert.Equal(t, jsonVal, *got.JSONValue)
}

func TestOpenAPIColumnEvalSetFieldsDO2DTOs(t *testing.T) {
	t.Parallel()

	assert.Nil(t, OpenAPIColumnEvalSetFieldsDO2DTOs(nil))

	from := []*entity.ColumnEvalSetField{
		{
			Key:         gptr.Of("k1"),
			Name:        gptr.Of("n1"),
			ContentType: entity.ContentTypeText,
		},
		nil,
	}
	got := OpenAPIColumnEvalSetFieldsDO2DTOs(from)
	if assert.Len(t, got, 1) {
		assert.Equal(t, "k1", *got[0].Key)
		assert.Equal(t, openapiCommon.ContentTypeText, *got[0].ContentType)
	}

	assert.Nil(t, OpenAPIColumnEvalSetFieldsDO2DTOs([]*entity.ColumnEvalSetField{nil}))
}

func TestOpenAPIColumnEvaluatorsDO2DTOs(t *testing.T) {
	t.Parallel()

	assert.Nil(t, OpenAPIColumnEvaluatorsDO2DTOs(nil))

	from := []*entity.ColumnEvaluator{
		{
			EvaluatorID:   1,
			Name:          gptr.Of("e1"),
			EvaluatorType: entity.EvaluatorTypePrompt,
		},
		nil,
	}
	got := OpenAPIColumnEvaluatorsDO2DTOs(from)
	if assert.Len(t, got, 1) {
		assert.Equal(t, int64(1), *got[0].EvaluatorID)
		assert.Equal(t, openapiEvaluator.EvaluatorTypePrompt, *got[0].EvaluatorType)
	}

	assert.Nil(t, OpenAPIColumnEvaluatorsDO2DTOs([]*entity.ColumnEvaluator{nil}))
}

func TestOpenAPIItemResultsDO2DTOs(t *testing.T) {
	t.Parallel()

	assert.Nil(t, OpenAPIItemResultsDO2DTOs(nil))

	from := []*entity.ItemResult{
		{
			ItemID:     1,
			SystemInfo: &entity.ItemSystemInfo{RunState: entity.ItemRunState_Success},
		},
		nil,
	}
	got := OpenAPIItemResultsDO2DTOs(from)
	if assert.Len(t, got, 1) {
		assert.Equal(t, int64(1), *got[0].ItemID)
		assert.Equal(t, openapiExperiment.ItemRunStateSuccess, *got[0].SystemInfo.RunState)
	}

	assert.Nil(t, OpenAPIItemResultsDO2DTOs([]*entity.ItemResult{nil}))
}

func TestConvertEntityContentTypeToOpenAPI(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input  entity.ContentType
		expect *openapiCommon.ContentType
	}{
		{entity.ContentTypeText, gptr.Of(openapiCommon.ContentTypeText)},
		{entity.ContentTypeImage, gptr.Of(openapiCommon.ContentTypeImage)},
		{entity.ContentTypeAudio, gptr.Of(openapiCommon.ContentTypeAudio)},
		{entity.ContentTypeMultipart, gptr.Of(openapiCommon.ContentTypeMultiPart)},
		{entity.ContentTypeMultipartVariable, gptr.Of(openapiCommon.ContentTypeMultiPart)},
		{entity.ContentType("unknown"), nil},
	}

	for _, tt := range cases {
		got := convertEntityContentTypeToOpenAPI(tt.input)
		if tt.expect == nil {
			assert.Nil(t, got)
		} else {
			assert.Equal(t, *tt.expect, *got)
		}
	}
}

func TestConvertEntityEvaluatorTypeToOpenAPI(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input  entity.EvaluatorType
		expect *openapiEvaluator.EvaluatorType
	}{
		{entity.EvaluatorTypePrompt, gptr.Of(openapiEvaluator.EvaluatorTypePrompt)},
		{entity.EvaluatorTypeCode, gptr.Of(openapiEvaluator.EvaluatorTypeCode)},
		{entity.EvaluatorType(999), nil},
	}

	for _, tt := range cases {
		got := convertEntityEvaluatorTypeToOpenAPI(tt.input)
		if tt.expect == nil {
			assert.Nil(t, got)
		} else {
			assert.Equal(t, *tt.expect, *got)
		}
	}
}

func TestOpenTargetAggrResultDO2DTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, OpenTargetAggrResultDO2DTO(nil))

	do := &entity.EvalTargetMtrAggrResult{
		TargetID: 1,
		LatencyAggrResults: []*entity.AggregatorResult{
			{AggregatorType: entity.Average, Data: &entity.AggregateData{DataType: entity.Double, Value: gptr.Of(0.5)}},
		},
	}
	got := OpenTargetAggrResultDO2DTO(do)
	if assert.NotNil(t, got) {
		assert.Equal(t, int64(1), *got.TargetID)
		assert.Len(t, got.Latency, 1)
	}
}

func TestTargetAggrResultDO2DTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, TargetAggrResultDO2DTO(nil))

	do := &entity.EvalTargetMtrAggrResult{
		TargetID: 1,
	}
	got := TargetAggrResultDO2DTO(do)
	if assert.NotNil(t, got) {
		assert.Equal(t, int64(1), *got.TargetID)
	}
}

func TestOpenAPIEvaluatorParamsDTO2Domain(t *testing.T) {
	t.Parallel()

	assert.Nil(t, OpenAPIEvaluatorParamsDTO2Domain(nil))

	dtos := []*openapi.SubmitExperimentEvaluatorParam{
		{EvaluatorID: gptr.Of(int64(1))},
		nil,
	}
	got := OpenAPIEvaluatorParamsDTO2Domain(dtos)
	assert.Len(t, got, 1)
	assert.Equal(t, int64(1), *got[0].EvaluatorID)
}

func TestOpenAPIEvaluatorRunConfigDTO2Domain(t *testing.T) {
	t.Parallel()

	assert.Nil(t, OpenAPIEvaluatorRunConfigDTO2Domain(nil))

	dto := &openapiEvaluator.EvaluatorRunConfig{
		Env: gptr.Of("test"),
	}
	got := OpenAPIEvaluatorRunConfigDTO2Domain(dto)
	assert.Equal(t, "test", *got.Env)
}

func TestOpenAPIExptTemplateDO2DTOs(t *testing.T) {
	t.Parallel()

	assert.Nil(t, OpenAPIExptTemplateDO2DTOs(nil))

	from := []*entity.ExptTemplate{{Meta: &entity.ExptTemplateMeta{ID: 1}}}
	got := OpenAPIExptTemplateDO2DTOs(from)
	assert.Len(t, got, 1)
}

func TestOpenAPIExptTypeDO2DTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, OpenAPIExptTypeDO2DTO(entity.ExptType(999)))
	assert.Equal(t, openapiExperiment.ExperimentTypeOffline, *OpenAPIExptTypeDO2DTO(entity.ExptType_Offline))
	assert.Equal(t, openapiExperiment.ExperimentTypeOnline, *OpenAPIExptTypeDO2DTO(entity.ExptType_Online))
}

func TestOpenAPIExptTypeDTO2DO(t *testing.T) {
	t.Parallel()

	assert.Equal(t, entity.ExptType_Offline, OpenAPIExptTypeDTO2DO(nil))
	assert.Equal(t, entity.ExptType_Offline, OpenAPIExptTypeDTO2DO(gptr.Of(openapiExperiment.ExperimentTypeOffline)))
	assert.Equal(t, entity.ExptType_Online, OpenAPIExptTypeDTO2DO(gptr.Of(openapiExperiment.ExperimentTypeOnline)))
	assert.Equal(t, entity.ExptType_Offline, OpenAPIExptTypeDTO2DO(gptr.Of(openapiExperiment.ExperimentType("invalid"))))
}

func TestOpenAPICreateEvalTargetParamDTO2DomainV2(t *testing.T) {
	t.Parallel()

	assert.Nil(t, OpenAPICreateEvalTargetParamDTO2DomainV2(nil))

	param := &openapi.SubmitExperimentEvalTargetParam{
		SourceTargetID:   gptr.Of("123"),
		EvalTargetType:   gptr.Of(openapiEvalTarget.EvalTargetTypeCozeBot),
		BotInfoType:      gptr.Of(openapiEvalTarget.CozeBotInfoTypeProductBot),
		Region:           gptr.Of(openapiEvalTarget.RegionCN),
		CustomEvalTarget: &openapiEvalTarget.CustomEvalTarget{ID: gptr.Of("id")},
	}

	got := OpenAPICreateEvalTargetParamDTO2DomainV2(param)
	if assert.NotNil(t, got) {
		assert.Equal(t, "123", *got.SourceTargetID)
		assert.Equal(t, entity.EvalTargetTypeCozeBot, *got.EvalTargetType)
		assert.Equal(t, entity.CozeBotInfoTypeProductBot, *got.BotInfoType)
		assert.Equal(t, entity.RegionCN, *got.Region)
		assert.Equal(t, "id", *got.CustomEvalTarget.ID)
	}

	// Case 2: DraftBot and invalid type
	botDraft := openapiEvalTarget.CozeBotInfoTypeDraftBot
	paramDraft := &openapi.SubmitExperimentEvalTargetParam{
		BotInfoType: &botDraft,
	}
	gotDraft := OpenAPICreateEvalTargetParamDTO2Domain(paramDraft)
	assert.Equal(t, domaindoEvalTarget.CozeBotInfoType_DraftBot, *gotDraft.BotInfoType)

	invalidBot := openapiEvalTarget.CozeBotInfoType("invalid")
	assert.Nil(t, OpenAPICreateEvalTargetParamDTO2Domain(&openapi.SubmitExperimentEvalTargetParam{BotInfoType: &invalidBot}))
}

func TestDomainRuntimeParamDTO2OpenAPI(t *testing.T) {
	t.Parallel()

	assert.Nil(t, DomainRuntimeParamDTO2OpenAPI(nil))

	p1 := &domainCommon.RuntimeParam{}
	assert.NotNil(t, DomainRuntimeParamDTO2OpenAPI(p1))

	jsonVal := "{}"
	p2 := &domainCommon.RuntimeParam{JSONValue: &jsonVal}
	got := DomainRuntimeParamDTO2OpenAPI(p2)
	assert.Equal(t, jsonVal, *got.JSONValue)
}

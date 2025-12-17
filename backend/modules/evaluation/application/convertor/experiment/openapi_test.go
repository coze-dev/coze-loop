// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

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

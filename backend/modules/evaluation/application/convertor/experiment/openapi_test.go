package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	openapiEvalTarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/eval_target"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	openapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"

	domainCommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domaindoEvalTarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	domainEvaluator "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	domainExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

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

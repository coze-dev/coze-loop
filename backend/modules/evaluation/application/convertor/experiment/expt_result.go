// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/common"
	evalsetconv "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluation_set"
	evaluatorconv "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluator"
	targetconv "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/target"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func ColumnEvalSetFieldsDO2DTOs(from []*entity.ColumnEvalSetField) []*domain_expt.ColumnEvalSetField {
	fields := make([]*domain_expt.ColumnEvalSetField, 0, len(from))
	for _, f := range from {
		fields = append(fields, ColumnEvalSetFieldsDO2DTO(f))
	}
	return fields
}

func ColumnEvalSetFieldsDO2DTO(from *entity.ColumnEvalSetField) *domain_expt.ColumnEvalSetField {
	contentType := common.ConvertContentTypeDO2DTO(from.ContentType)
	return &domain_expt.ColumnEvalSetField{
		Key:         from.Key,
		Name:        from.Name,
		Description: from.Description,
		ContentType: &contentType,
		TextSchema:  from.TextSchema,
	}
}

func ColumnEvaluatorsDO2DTOs(from []*entity.ColumnEvaluator) []*domain_expt.ColumnEvaluator {
	evaluators := make([]*domain_expt.ColumnEvaluator, 0, len(from))
	for _, f := range from {
		evaluators = append(evaluators, ColumnEvaluatorsDO2DTO(f))
	}
	return evaluators
}

func ColumnEvaluatorsDO2DTO(from *entity.ColumnEvaluator) *domain_expt.ColumnEvaluator {
	return &domain_expt.ColumnEvaluator{
		EvaluatorVersionID: from.EvaluatorVersionID,
		EvaluatorID:        from.EvaluatorID,
		EvaluatorType:      evaluator.EvaluatorType(from.EvaluatorType),
		Name:               from.Name,
		Version:            from.Version,
		Description:        from.Description,
	}
}

func ItemResultsDO2DTOs(from []*entity.ItemResult) []*domain_expt.ItemResult_ {
	results := make([]*domain_expt.ItemResult_, 0, len(from))
	for _, f := range from {
		results = append(results, ItemResultsDO2DTO(f))
	}
	return results
}

func ItemResultsDO2DTO(from *entity.ItemResult) *domain_expt.ItemResult_ {
	return &domain_expt.ItemResult_{
		ItemID:      from.ItemID,
		TurnResults: TurnResultsDO2DTOs(from.TurnResults),
		SystemInfo:  ItemSystemInfoDO2DTO(from.SystemInfo),
		ItemIndex:   from.ItemIndex,
	}
}

func TurnResultsDO2DTOs(from []*entity.TurnResult) []*domain_expt.TurnResult_ {
	results := make([]*domain_expt.TurnResult_, 0, len(from))
	for _, f := range from {
		results = append(results, TurnResultsDO2DTO(f))
	}
	return results
}

func TurnResultsDO2DTO(from *entity.TurnResult) *domain_expt.TurnResult_ {
	return &domain_expt.TurnResult_{
		TurnID:            from.TurnID,
		ExperimentResults: ExperimentResultsDO2DTOs(from.ExperimentResults),
		TurnIndex:         from.TurnIndex,
	}
}

func ExperimentResultsDO2DTOs(from []*entity.ExperimentResult) []*domain_expt.ExperimentResult_ {
	results := make([]*domain_expt.ExperimentResult_, 0, len(from))
	for _, f := range from {
		results = append(results, ExperimentResultsDO2DTO(f))
	}
	return results
}

func ExperimentResultsDO2DTO(from *entity.ExperimentResult) *domain_expt.ExperimentResult_ {
	return &domain_expt.ExperimentResult_{
		ExperimentID: from.ExperimentID,
		Payload:      ExperimentTurnPayloadDO2DTO(from.Payload),
	}
}

func ExperimentTurnPayloadDO2DTO(from *entity.ExperimentTurnPayload) *domain_expt.ExperimentTurnPayload {
	return &domain_expt.ExperimentTurnPayload{
		TurnID:          from.TurnID,
		EvalSet:         TurnEvalSetDO2DTO(from.EvalSet),
		TargetOutput:    TurnTargetOutputDO2DTO(from.TargetOutput),
		EvaluatorOutput: TurnEvaluatorOutputDO2DTO(from.EvaluatorOutput),
		SystemInfo:      TurnSystemInfoDO2DTO(from.SystemInfo),
	}
}

func TurnEvaluatorOutputDO2DTO(from *entity.TurnEvaluatorOutput) *domain_expt.TurnEvaluatorOutput {
	if from == nil {
		return &domain_expt.TurnEvaluatorOutput{}
	}
	evaluatorRecords := make(map[int64]*evaluator.EvaluatorRecord)
	for k, v := range from.EvaluatorRecords {
		evaluatorRecords[k] = evaluatorconv.ConvertEvaluatorRecordDO2DTO(v)
	}
	return &domain_expt.TurnEvaluatorOutput{
		EvaluatorRecords: evaluatorRecords,
	}
}

func TurnTargetOutputDO2DTO(from *entity.TurnTargetOutput) *domain_expt.TurnTargetOutput {
	if from == nil {
		return &domain_expt.TurnTargetOutput{}
	}
	return &domain_expt.TurnTargetOutput{
		EvalTargetRecord: targetconv.EvalTargetRecordDO2DTO(from.EvalTargetRecord),
	}
}

func TurnEvalSetDO2DTO(from *entity.TurnEvalSet) *domain_expt.TurnEvalSet {
	if from == nil {
		return &domain_expt.TurnEvalSet{}
	}
	return &domain_expt.TurnEvalSet{
		Turn: evalsetconv.TurnDO2DTO(from.Turn),
	}
}

func TurnSystemInfoDO2DTO(from *entity.TurnSystemInfo) *domain_expt.TurnSystemInfo {
	if from == nil {
		return &domain_expt.TurnSystemInfo{}
	}
	return &domain_expt.TurnSystemInfo{
		TurnRunState: domain_expt.TurnRunStatePtr(domain_expt.TurnRunState(from.TurnRunState)),
		LogID:        from.LogID,
		Error:        RunErrorDO2DTO(from.Error),
	}
}

func RunErrorDO2DTO(from *entity.RunError) *domain_expt.RunError {
	if from == nil {
		return nil
	}
	return &domain_expt.RunError{
		Code:    from.Code,
		Message: from.Message,
		Detail:  from.Detail,
	}
}

func ItemSystemInfoDO2DTO(from *entity.ItemSystemInfo) *domain_expt.ItemSystemInfo {
	if from == nil {
		return &domain_expt.ItemSystemInfo{}
	}
	return &domain_expt.ItemSystemInfo{
		RunState: domain_expt.ItemRunStatePtr(domain_expt.ItemRunState(from.RunState)),
		LogID:    from.LogID,
		Error:    RunErrorDO2DTO(from.Error),
	}
}

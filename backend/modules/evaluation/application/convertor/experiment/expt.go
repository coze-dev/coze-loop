// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"fmt"

	"github.com/bytedance/gg/gcond"
	"github.com/bytedance/gg/gptr"

	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluation_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluator"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/target"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/maps"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func NewEvalConfConvert() *EvalConfConvert {
	return &EvalConfConvert{}
}

type EvalConfConvert struct{}

func (e *EvalConfConvert) ConvertToEntity(cer *expt.CreateExperimentRequest) (*entity.EvaluationConfiguration, error) {
	if cer == nil || cer.TargetFieldMapping == nil || cer.EvaluatorFieldMapping == nil {
		return nil, fmt.Errorf("invalid EvaluationConfiguration")
	}
	return &entity.EvaluationConfiguration{
		ConnectorConf: entity.Connector{
			TargetConf: &entity.TargetConf{
				TargetVersionID: cer.GetTargetVersionID(),
				IngressConf:     toTargetFieldMappingDO(cer.GetTargetFieldMapping()),
			},
			EvaluatorsConf: &entity.EvaluatorsConf{
				EvaluatorConcurNum: ptr.ConvIntPtr[int32, int](cer.EvaluatorsConcurNum),
				EvaluatorConf:      toEvaluatorFieldMappingDo(cer.GetEvaluatorFieldMapping()),
			},
		},
		ItemConcurNum: ptr.ConvIntPtr[int32, int](cer.ItemConcurNum),
	}, nil
}

func toTargetFieldMappingDO(mapping *domain_expt.TargetFieldMapping) *entity.TargetIngressConf {
	fc := make([]*entity.FieldConf, 0, len(mapping.GetFromEvalSet()))
	for _, fm := range mapping.GetFromEvalSet() {
		fc = append(fc, &entity.FieldConf{
			FieldName: fm.GetFieldName(),
			FromField: fm.GetFromFieldName(),
			Value:     fm.GetConstValue(),
		})
	}
	return &entity.TargetIngressConf{
		EvalSetAdapter: &entity.FieldAdapter{
			FieldConfs: fc,
		},
	}
}

func toEvaluatorFieldMappingDo(mapping []*domain_expt.EvaluatorFieldMapping) []*entity.EvaluatorConf {
	ec := make([]*entity.EvaluatorConf, 0, len(mapping))
	for _, fm := range mapping {
		esf := make([]*entity.FieldConf, 0, len(fm.GetFromEvalSet()))
		for _, fes := range fm.GetFromEvalSet() {
			esf = append(esf, &entity.FieldConf{
				FieldName: fes.GetFieldName(),
				FromField: fes.GetFromFieldName(),
				Value:     fes.GetConstValue(),
			})
		}
		tf := make([]*entity.FieldConf, 0, len(fm.GetFromTarget()))
		for _, ft := range fm.GetFromTarget() {
			tf = append(tf, &entity.FieldConf{
				FieldName: ft.GetFieldName(),
				FromField: ft.GetFromFieldName(),
				Value:     ft.GetConstValue(),
			})
		}
		ec = append(ec, &entity.EvaluatorConf{
			EvaluatorVersionID: fm.GetEvaluatorVersionID(),
			IngressConf: &entity.EvaluatorIngressConf{
				EvalSetAdapter: &entity.FieldAdapter{FieldConfs: esf},
				TargetAdapter:  &entity.FieldAdapter{FieldConfs: tf},
			},
		})
	}
	return ec
}

func (e *EvalConfConvert) ConvertEntityToDTO(ec *entity.EvaluationConfiguration) (*domain_expt.TargetFieldMapping, []*domain_expt.EvaluatorFieldMapping) {
	if ec == nil {
		return nil, nil
	}

	var evaluatorMappings []*domain_expt.EvaluatorFieldMapping
	if evaluatorsConf := ec.ConnectorConf.EvaluatorsConf; evaluatorsConf != nil {
		for _, evaluatorConf := range evaluatorsConf.EvaluatorConf {
			if evaluatorConf.IngressConf == nil {
				continue
			}
			m := &domain_expt.EvaluatorFieldMapping{
				EvaluatorVersionID: evaluatorConf.EvaluatorVersionID,
			}
			if evaluatorConf.IngressConf.EvalSetAdapter != nil {
				for _, fc := range evaluatorConf.IngressConf.EvalSetAdapter.FieldConfs {
					m.FromEvalSet = append(m.FromEvalSet, &domain_expt.FieldMapping{
						FieldName:     gptr.Of(fc.FieldName),
						FromFieldName: gptr.Of(fc.FromField),
						ConstValue:    gptr.Of(fc.Value),
					})
				}
			}
			if evaluatorConf.IngressConf.TargetAdapter != nil {
				for _, fc := range evaluatorConf.IngressConf.TargetAdapter.FieldConfs {
					m.FromTarget = append(m.FromTarget, &domain_expt.FieldMapping{
						FieldName:     gptr.Of(fc.FieldName),
						FromFieldName: gptr.Of(fc.FromField),
						ConstValue:    gptr.Of(fc.Value),
					})
				}
			}
			evaluatorMappings = append(evaluatorMappings, m)
		}
	}

	targetMapping := &domain_expt.TargetFieldMapping{}
	if targetConf := ec.ConnectorConf.TargetConf; targetConf != nil && targetConf.IngressConf != nil && targetConf.IngressConf.EvalSetAdapter != nil {
		for _, fc := range targetConf.IngressConf.EvalSetAdapter.FieldConfs {
			targetMapping.FromEvalSet = append(targetMapping.FromEvalSet, &domain_expt.FieldMapping{
				FieldName:     gptr.Of(fc.FieldName),
				FromFieldName: gptr.Of(fc.FromField),
				ConstValue:    gptr.Of(fc.Value),
			})
		}
	}
	return targetMapping, evaluatorMappings
}

func ToExptStatsInfoDTO(experiment *entity.Experiment, stats *entity.ExptStats) *domain_expt.ExptStatsInfo {
	if stats == nil {
		return nil
	}
	return &domain_expt.ExptStatsInfo{
		ExptID:    gptr.Of(experiment.ID),
		SourceID:  gptr.Of(experiment.SourceID),
		ExptStats: ToExptStatsDTO(stats, nil),
	}
}

func ToExptDTOs(experiments []*entity.Experiment) []*domain_expt.Experiment {
	dtos := make([]*domain_expt.Experiment, 0, len(experiments))
	for _, experiment := range experiments {
		dtos = append(dtos, ToExptDTO(experiment))
	}

	return dtos
}

func ToExptDTO(experiment *entity.Experiment) *domain_expt.Experiment {
	evaluatorVersionIDs := make([]int64, 0, len(experiment.EvaluatorVersionRef))
	for _, ref := range experiment.EvaluatorVersionRef {
		evaluatorVersionIDs = append(evaluatorVersionIDs, ref.EvaluatorVersionID)
	}

	tm, ems := NewEvalConfConvert().ConvertEntityToDTO(experiment.EvalConf)

	res := &domain_expt.Experiment{
		ID:                    gptr.Of(experiment.ID),
		Name:                  gptr.Of(experiment.Name),
		Desc:                  gptr.Of(experiment.Description),
		CreatorBy:             gptr.Of(experiment.CreatedBy),
		EvalSetVersionID:      gptr.Of(experiment.EvalSetVersionID),
		TargetVersionID:       gptr.Of(experiment.TargetVersionID),
		EvalSetID:             gptr.Of(experiment.EvalSetID),
		TargetID:              gptr.Of(experiment.TargetID),
		EvaluatorVersionIds:   evaluatorVersionIDs,
		Status:                gptr.Of(domain_expt.ExptStatus(experiment.Status)),
		StatusMessage:         gptr.Of(experiment.StatusMessage),
		ExptStats:             ToExptStatsDTO(experiment.Stats, experiment.AggregateResult),
		TargetFieldMapping:    tm,
		EvaluatorFieldMapping: ems,
		SourceType:            gptr.Of(domain_expt.SourceType(experiment.SourceType)),
		SourceID:              gptr.Of(experiment.SourceID),
		ExptType:              gptr.Of(domain_expt.ExptType(experiment.ExptType)),
		MaxAliveTime:          gptr.Of(experiment.MaxAliveTime),
	}

	if experiment.StartAt != nil {
		res.StartTime = gptr.Of(experiment.StartAt.Unix())
	}
	if experiment.EndAt != nil {
		res.EndTime = gptr.Of(experiment.EndAt.Unix())
	}

	res.EvalTarget = target.EvalTargetDO2DTO(experiment.Target)
	if experiment.ExptType != entity.ExptType_Online {
		res.EvalSet = evaluation_set.EvaluationSetDO2DTO(experiment.EvalSet)
	}
	res.Evaluators = make([]*evaluatordto.Evaluator, 0, len(experiment.Evaluators))
	for _, evaluatorDO := range experiment.Evaluators {
		res.Evaluators = append(res.Evaluators, evaluator.ConvertEvaluatorDO2DTO(evaluatorDO))
	}
	return res
}

func ToExptStatsDTO(stats *entity.ExptStats, aggrResult *entity.ExptAggregateResult) *domain_expt.ExptStatistics {
	if stats == nil {
		return nil
	}
	exptStatistics := &domain_expt.ExptStatistics{
		PendingTurnCnt:    gcond.If(stats.PendingItemCnt > 0, gptr.Of(stats.PendingItemCnt), gptr.Of(int32(0))),
		SuccessTurnCnt:    gcond.If(stats.SuccessItemCnt > 0, gptr.Of(stats.SuccessItemCnt), gptr.Of(int32(0))),
		FailTurnCnt:       gcond.If(stats.FailItemCnt > 0, gptr.Of(stats.FailItemCnt), gptr.Of(int32(0))),
		ProcessingTurnCnt: gcond.If(stats.ProcessingItemCnt > 0, gptr.Of(stats.ProcessingItemCnt), gptr.Of(int32(0))),
		TerminatedTurnCnt: gcond.If(stats.TerminatedItemCnt > 0, gptr.Of(stats.TerminatedItemCnt), gptr.Of(int32(0))),
		CreditCost:        gptr.Of(stats.CreditCost),
		TokenUsage: &domain_expt.TokenUsage{
			InputTokens:  gptr.Of(stats.InputTokenCost),
			OutputTokens: gptr.Of(stats.OutputTokenCost),
		},
	}

	if aggrResult != nil {
		aggrResultDTO := ExptAggregateResultDOToDTO(aggrResult)
		exptStatistics.EvaluatorAggregateResults = append(exptStatistics.EvaluatorAggregateResults, maps.ToSlice(aggrResultDTO.GetEvaluatorResults(), func(k int64, v *domain_expt.EvaluatorAggregateResult_) *domain_expt.EvaluatorAggregateResult_ {
			return v
		})...)
	}

	return exptStatistics
}

func CreateEvalTargetParamDTO2DO(param *eval_target.CreateEvalTargetParam, exptType domain_expt.ExptType) *entity.CreateEvalTargetParam {
	res := &entity.CreateEvalTargetParam{
		SourceTargetID:      param.SourceTargetID,
		SourceTargetVersion: param.SourceTargetVersion,
		BotPublishVersion:   param.BotPublishVersion,
	}
	if param.EvalTargetType != nil {
		res.EvalTargetType = gptr.Of(entity.EvalTargetType(*param.EvalTargetType))
	}
	if param.BotInfoType != nil {
		res.BotInfoType = gptr.Of(entity.CozeBotInfoType(*param.BotInfoType))
	}

	return res
}

func ExptType2EvalMode(exptType domain_expt.ExptType) entity.ExptRunMode {
	exptMode := entity.EvaluationModeSubmit
	if exptType == domain_expt.ExptType_Online {
		exptMode = entity.EvaluationModeAppend
	}
	return exptMode
}

func ConvertCreateReq(cer *expt.CreateExperimentRequest) (param *entity.CreateExptParam, err error) {
	param = &entity.CreateExptParam{
		WorkspaceID:           cer.WorkspaceID,
		EvalSetVersionID:      cer.GetEvalSetVersionID(),
		TargetVersionID:       cer.GetTargetVersionID(),
		EvaluatorVersionIds:   cer.GetEvaluatorVersionIds(),
		Name:                  cer.GetName(),
		Desc:                  cer.GetDesc(),
		EvalSetID:             cer.GetEvalSetID(),
		TargetID:              cer.TargetID,
		CreateEvalTargetParam: CreateEvalTargetParamDTO2DO(cer.GetCreateEvalTargetParam(), cer.GetExptType()),
		ExptType:              entity.ExptType(cer.GetExptType()),
		MaxAliveTime:          cer.GetMaxAliveTime(),
		SourceType:            entity.SourceType(cer.GetSourceType()),
		SourceID:              cer.GetSourceID(),
		ExptConf:              nil,
	}
	evaluationConfiguration, err := NewEvalConfConvert().ConvertToEntity(cer)
	if err != nil {
		return nil, err
	}
	param.ExptConf = evaluationConfiguration

	return param, nil
}

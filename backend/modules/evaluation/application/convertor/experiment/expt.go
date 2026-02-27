// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"github.com/bytedance/gg/gcond"
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluation_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluator"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/target"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/maps"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
)

func NewEvalConfConvert() *EvalConfConvert {
	return &EvalConfConvert{}
}

type EvalConfConvert struct{}

func (e *EvalConfConvert) ConvertToEntity(cer *expt.CreateExperimentRequest, evaluatorVersionRunConfigs map[int64]*evaluatordto.EvaluatorRunConfig) (*entity.EvaluationConfiguration, error) {
	ec := &entity.EvaluationConfiguration{
		ItemConcurNum: ptr.ConvIntPtr[int32, int](cer.ItemConcurNum),
	}

	ec.ConnectorConf.TargetConf = &entity.TargetConf{
		TargetVersionID: cer.GetTargetVersionID(),
		IngressConf:     toTargetFieldMappingDO(cer.GetTargetFieldMapping(), cer.GetTargetRuntimeParam()),
	}
	if cer.GetEvaluatorFieldMapping() != nil {
		evalsConf := &entity.EvaluatorsConf{
			EvaluatorConcurNum: ptr.ConvIntPtr[int32, int](cer.EvaluatorsConcurNum),
			EvaluatorConf:      toEvaluatorConfDO(cer.GetEvaluatorFieldMapping(), evaluatorVersionRunConfigs),
		}
		// 将请求中的 evaluator_score_weights 下沉到 EvaluatorConf.ScoreWeight
		if weights := cer.GetEvaluatorScoreWeights(); len(weights) > 0 {
			for _, conf := range evalsConf.EvaluatorConf {
				if conf == nil {
					continue
				}
				if w, ok := weights[conf.EvaluatorVersionID]; ok && w > 0 {
					conf.ScoreWeight = gptr.Of(w)
				}
			}
		}
		ec.ConnectorConf.EvaluatorsConf = evalsConf
	}
	if cer.GetItemRetryNum() > 0 {
		ec.ItemRetryNum = gptr.Of(int(cer.GetItemRetryNum()))
	}
	return ec, nil
}

func toTargetFieldMappingDO(mapping *domain_expt.TargetFieldMapping, rtp *common.RuntimeParam) *entity.TargetIngressConf {
	tic := &entity.TargetIngressConf{EvalSetAdapter: &entity.FieldAdapter{}}

	if mapping != nil {
		fc := make([]*entity.FieldConf, 0, len(mapping.GetFromEvalSet()))
		for _, fm := range mapping.GetFromEvalSet() {
			fc = append(fc, &entity.FieldConf{
				FieldName: fm.GetFieldName(),
				FromField: fm.GetFromFieldName(),
				Value:     fm.GetConstValue(),
			})
		}
		tic.EvalSetAdapter.FieldConfs = fc
	}

	if rtp != nil && len(rtp.GetJSONValue()) > 0 {
		tic.CustomConf = &entity.FieldAdapter{
			FieldConfs: []*entity.FieldConf{{
				FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
				Value:     rtp.GetJSONValue(),
			}},
		}
	}
	return tic
}

func toEvaluatorConfDO(mapping []*domain_expt.EvaluatorFieldMapping, runConfigMap map[int64]*evaluatordto.EvaluatorRunConfig) []*entity.EvaluatorConf {
	if mapping == nil {
		return nil
	}
	ec := make([]*entity.EvaluatorConf, 0, len(mapping))
	for _, fm := range mapping {
		if fm == nil {
			continue
		}
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

		// 从 EvaluatorIDVersionItem 中提取信息，如果不存在则使用 EvaluatorVersionID
		var evaluatorID int64
		var version string
		evaluatorVersionID := fm.GetEvaluatorVersionID()

		if fm.IsSetEvaluatorIDVersionItem() {
			item := fm.GetEvaluatorIDVersionItem()
			if item != nil {
				if item.IsSetEvaluatorID() {
					evaluatorID = item.GetEvaluatorID()
				}
				if item.IsSetVersion() {
					version = item.GetVersion()
				}
				if item.IsSetEvaluatorVersionID() {
					evaluatorVersionID = item.GetEvaluatorVersionID()
				}
			}
		}

		var runConf *evaluatordto.EvaluatorRunConfig = nil
		if len(runConfigMap) > 0 {
			runConf = runConfigMap[fm.GetEvaluatorVersionID()]
		}
		ec = append(ec, &entity.EvaluatorConf{
			EvaluatorVersionID: evaluatorVersionID,
			EvaluatorID:        evaluatorID,
			Version:            version,
			IngressConf: &entity.EvaluatorIngressConf{
				EvalSetAdapter: &entity.FieldAdapter{FieldConfs: esf},
				TargetAdapter:  &entity.FieldAdapter{FieldConfs: tf},
			},
			RunConf: evaluator.ConvertEvaluatorRunConfDTO2DO(runConf),
		})
	}
	return ec
}

func (e *EvalConfConvert) ConvertEntityToDTO(ec *entity.EvaluationConfiguration) (*domain_expt.TargetFieldMapping, []*domain_expt.EvaluatorFieldMapping, *common.RuntimeParam, map[int64]*evaluatordto.EvaluatorRunConfig) {
	if ec == nil {
		return nil, nil, nil, nil
	}

	var evaluatorMappings []*domain_expt.EvaluatorFieldMapping
	evaluatorVersionRunConfMap := make(map[int64]*evaluatordto.EvaluatorRunConfig)
	if evaluatorsConf := ec.ConnectorConf.EvaluatorsConf; evaluatorsConf != nil {
		for _, evaluatorConf := range evaluatorsConf.EvaluatorConf {
			if evaluatorConf.IngressConf == nil {
				continue
			}
			m := &domain_expt.EvaluatorFieldMapping{
				EvaluatorVersionID: evaluatorConf.EvaluatorVersionID,
			}

			// 构建 EvaluatorIDVersionItem
			if evaluatorConf.EvaluatorID > 0 || evaluatorConf.Version != "" || evaluatorConf.EvaluatorVersionID > 0 {
				item := &evaluatordto.EvaluatorIDVersionItem{}
				if evaluatorConf.EvaluatorID > 0 {
					item.SetEvaluatorID(gptr.Of(evaluatorConf.EvaluatorID))
				}
				if evaluatorConf.Version != "" {
					item.SetVersion(gptr.Of(evaluatorConf.Version))
				}
				if evaluatorConf.EvaluatorVersionID > 0 {
					item.SetEvaluatorVersionID(gptr.Of(evaluatorConf.EvaluatorVersionID))
				}
				// 如果 EvaluatorConf 中有 ScoreWeight，也填充到 item 中
				if evaluatorConf.ScoreWeight != nil && *evaluatorConf.ScoreWeight > 0 {
					item.SetScoreWeight(gptr.Of(*evaluatorConf.ScoreWeight))
				}
				m.SetEvaluatorIDVersionItem(item)
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

			if evaluatorConf.RunConf != nil {
				evaluatorVersionRunConfMap[evaluatorConf.EvaluatorVersionID] = evaluator.ConvertEvaluatorRunConfDO2DTO(evaluatorConf.RunConf)
			}
		}
	}

	targetMapping := &domain_expt.TargetFieldMapping{}
	trtp := &common.RuntimeParam{}
	if targetConf := ec.ConnectorConf.TargetConf; targetConf != nil && targetConf.IngressConf != nil {
		if targetConf.IngressConf.EvalSetAdapter != nil {
			for _, fc := range targetConf.IngressConf.EvalSetAdapter.FieldConfs {
				targetMapping.FromEvalSet = append(targetMapping.FromEvalSet, &domain_expt.FieldMapping{
					FieldName:     gptr.Of(fc.FieldName),
					FromFieldName: gptr.Of(fc.FromField),
					ConstValue:    gptr.Of(fc.Value),
				})
			}
		}
		if targetConf.IngressConf.CustomConf != nil {
			for _, fc := range targetConf.IngressConf.CustomConf.FieldConfs {
				if fc.FieldName == consts.FieldAdapterBuiltinFieldNameRuntimeParam {
					trtp.JSONValue = gptr.Of(fc.Value)
				}
			}
		}
	}

	return targetMapping, evaluatorMappings, trtp, evaluatorVersionRunConfMap
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

	// 构建 evaluator_version_id -> score_weight 映射（来自 EvaluatorConf.ScoreWeight）
	evalWeights := make(map[int64]float64)
	if experiment.EvalConf != nil && experiment.EvalConf.ConnectorConf.EvaluatorsConf != nil {
		for _, ec := range experiment.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf {
			if ec == nil || ec.ScoreWeight == nil || *ec.ScoreWeight <= 0 {
				continue
			}
			evalWeights[ec.EvaluatorVersionID] = *ec.ScoreWeight
		}
	}

	// 构建 EvaluatorIDVersionItems 列表
	evaluatorIDVersionItems := make([]*evaluatordto.EvaluatorIDVersionItem, 0)
	// 优先从 Evaluators 中获取完整信息（包含 evaluator_id, version, evaluator_version_id）
	if len(experiment.Evaluators) > 0 {
		for _, evaluator := range experiment.Evaluators {
			if evaluator == nil {
				continue
			}
			evaluatorID := evaluator.GetEvaluatorID()
			version := evaluator.GetVersion()
			evaluatorVersionID := evaluator.GetEvaluatorVersionID()
			if evaluatorID > 0 && evaluatorVersionID > 0 {
				item := &evaluatordto.EvaluatorIDVersionItem{
					EvaluatorID:        gptr.Of(evaluatorID),
					Version:            gptr.Of(version),
					EvaluatorVersionID: gptr.Of(evaluatorVersionID),
				}
				// 如果 EvalConf 中有权重配置，则填充
				if weight, ok := evalWeights[evaluatorVersionID]; ok && weight > 0 {
					item.ScoreWeight = gptr.Of(weight)
				}
				evaluatorIDVersionItems = append(evaluatorIDVersionItems, item)
			}
		}
	} else if len(experiment.EvaluatorVersionRef) > 0 {
		// 如果没有 Evaluators，则从 EvaluatorVersionRef 构建（只有 evaluator_id 和 evaluator_version_id）
		for _, ref := range experiment.EvaluatorVersionRef {
			if ref.EvaluatorID > 0 && ref.EvaluatorVersionID > 0 {
				item := &evaluatordto.EvaluatorIDVersionItem{
					EvaluatorID:        gptr.Of(ref.EvaluatorID),
					EvaluatorVersionID: gptr.Of(ref.EvaluatorVersionID),
				}
				// 如果 EvalConf 中有权重配置，则填充
				if weight, ok := evalWeights[ref.EvaluatorVersionID]; ok && weight > 0 {
					item.ScoreWeight = gptr.Of(weight)
				}
				evaluatorIDVersionItems = append(evaluatorIDVersionItems, item)
			}
		}
	}

	tm, ems, trtp, evrcs := NewEvalConfConvert().ConvertEntityToDTO(experiment.EvalConf)

	evaluatorVersionIDMap := slices.ToMap(experiment.Evaluators, func(evaluator *entity.Evaluator) (int64, *entity.Evaluator) {
		return evaluator.GetEvaluatorVersionID(), evaluator
	})

	evaluatorIDVersionList := make([]*evaluatordto.EvaluatorIDVersionItem, 0, len(experiment.EvaluatorVersionRef))
	for _, evaluatorVersionID := range evaluatorVersionIDs {
		curEvaluatorIDVersionItem := &evaluatordto.EvaluatorIDVersionItem{}
		if len(evaluatorVersionIDMap) > 0 && evaluatorVersionIDMap[evaluatorVersionID] != nil {
			curEvaluatorIDVersionItem.EvaluatorID = gptr.Of(evaluatorVersionIDMap[evaluatorVersionID].GetEvaluatorID())
			curEvaluatorIDVersionItem.Version = gptr.Of(evaluatorVersionIDMap[evaluatorVersionID].GetVersion())
		}
		if len(evrcs) > 0 && evrcs[evaluatorVersionID] != nil {
			curEvaluatorIDVersionItem.RunConfig = evrcs[evaluatorVersionID]
		}
		evaluatorIDVersionList = append(evaluatorIDVersionList, curEvaluatorIDVersionItem)
	}

	res := &domain_expt.Experiment{
		ID:                     gptr.Of(experiment.ID),
		Name:                   gptr.Of(experiment.Name),
		Desc:                   gptr.Of(experiment.Description),
		CreatorBy:              gptr.Of(experiment.CreatedBy),
		EvalSetVersionID:       gptr.Of(experiment.EvalSetVersionID),
		TargetVersionID:        gptr.Of(experiment.TargetVersionID),
		EvalSetID:              gptr.Of(experiment.EvalSetID),
		TargetID:               gptr.Of(experiment.TargetID),
		EvaluatorVersionIds:    evaluatorVersionIDs,
		Status:                 gptr.Of(domain_expt.ExptStatus(experiment.Status)),
		StatusMessage:          gptr.Of(experiment.StatusMessage),
		ExptStats:              ToExptStatsDTO(experiment.Stats, experiment.AggregateResult),
		TargetFieldMapping:     tm,
		EvaluatorFieldMapping:  ems,
		SourceType:             gptr.Of(domain_expt.SourceType(experiment.SourceType)),
		SourceID:               gptr.Of(experiment.SourceID),
		ExptType:               gptr.Of(domain_expt.ExptType(experiment.ExptType)),
		MaxAliveTime:           gptr.Of(experiment.MaxAliveTime),
		TargetRuntimeParam:     trtp,
		EvaluatorIDVersionList: evaluatorIDVersionList,
	}

	// 注意：Experiment DTO 中没有 TripleConfig 字段，如果需要可以通过其他方式传递

	if experiment.StartAt != nil {
		res.StartTime = gptr.Of(experiment.StartAt.Unix())
	}
	if experiment.EndAt != nil {
		res.EndTime = gptr.Of(experiment.EndAt.Unix())
	}
	if experiment.EvalConf != nil {
		if experiment.EvalConf.ItemConcurNum != nil {
			res.ItemConcurNum = gptr.Of(int32(gptr.Indirect(experiment.EvalConf.ItemConcurNum)))
		}
		if experiment.EvalConf.ItemRetryNum != nil {
			res.ItemRetryNum = gptr.Of(int32(gptr.Indirect(experiment.EvalConf.ItemRetryNum)))
		} else {
			res.ItemRetryNum = gptr.Of(int32(0))
		}
	}

	// 填充权重配置（score_weight_config 和 enable_weighted_score）
	enableWeightedScore := len(evalWeights) > 0
	if experiment.EvalConf != nil && experiment.EvalConf.ConnectorConf.EvaluatorsConf != nil {
		enableWeightedScore = enableWeightedScore || experiment.EvalConf.ConnectorConf.EvaluatorsConf.EnableScoreWeight
	}
	if enableWeightedScore {
		res.EnableWeightedScore = gptr.Of(true)
		res.ScoreWeightConfig = &domain_expt.ExptScoreWeight{
			EnableWeightedScore:   gptr.Of(enableWeightedScore),
			EvaluatorScoreWeights: evalWeights,
		}
	}

	// 关联的实验模板（仅在查询时按需填充基础信息）
	if experiment.ExptTemplateMeta != nil {
		res.ExptTemplateMeta = &domain_expt.ExptTemplateMeta{
			ID:          gptr.Of(experiment.ExptTemplateMeta.ID),
			WorkspaceID: gptr.Of(experiment.ExptTemplateMeta.WorkspaceID),
			Name:        gptr.Of(experiment.ExptTemplateMeta.Name),
			Desc:        gptr.Of(experiment.ExptTemplateMeta.Desc),
			ExptType:    gptr.Of(domain_expt.ExptType(experiment.ExptTemplateMeta.ExptType)),
		}
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

func CreateEvalTargetParamDTO2DO(param *eval_target.CreateEvalTargetParam) *entity.CreateEvalTargetParam {
	if param == nil {
		return nil
	}

	res := &entity.CreateEvalTargetParam{
		SourceTargetID:      param.SourceTargetID,
		SourceTargetVersion: param.SourceTargetVersion,
		BotPublishVersion:   param.BotPublishVersion,
		Region:              param.Region,
		Env:                 param.Env,
	}
	if param.EvalTargetType != nil {
		res.EvalTargetType = gptr.Of(entity.EvalTargetType(*param.EvalTargetType))
	}
	if param.BotInfoType != nil {
		res.BotInfoType = gptr.Of(entity.CozeBotInfoType(*param.BotInfoType))
	}
	if param.CustomEvalTarget != nil {
		res.CustomEvalTarget = &entity.CustomEvalTarget{
			ID:        param.CustomEvalTarget.ID,
			Name:      param.CustomEvalTarget.Name,
			AvatarURL: param.CustomEvalTarget.AvatarURL,
			Ext:       param.CustomEvalTarget.Ext,
		}
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

func ConvertCreateReq(cer *expt.CreateExperimentRequest, evaluatorVersionRunConfigs map[int64]*evaluatordto.EvaluatorRunConfig) (param *entity.CreateExptParam, err error) {
	param = &entity.CreateExptParam{
		WorkspaceID:           cer.WorkspaceID,
		EvalSetVersionID:      cer.GetEvalSetVersionID(),
		TargetVersionID:       cer.GetTargetVersionID(),
		EvaluatorVersionIds:   cer.GetEvaluatorVersionIds(),
		Name:                  cer.GetName(),
		Desc:                  cer.GetDesc(),
		EvalSetID:             cer.GetEvalSetID(),
		TargetID:              cer.TargetID,
		CreateEvalTargetParam: CreateEvalTargetParamDTO2DO(cer.GetCreateEvalTargetParam()),
		ExptType:              entity.ExptType(cer.GetExptType()),
		MaxAliveTime:          cer.GetMaxAliveTime(),
		SourceType:            entity.SourceType(cer.GetSourceType()),
		SourceID:              cer.GetSourceID(),
		ExptConf:              nil,
	}
	evaluationConfiguration, err := NewEvalConfConvert().ConvertToEntity(cer, evaluatorVersionRunConfigs)
	if err != nil {
		return nil, err
	}
	param.ExptConf = evaluationConfiguration

	if cer.IsSetExptTemplateID() {
		param.ExptTemplateID = cer.GetExptTemplateID()
	}
	return param, nil
}

func ConvRetryMode(m domain_expt.ExptRetryMode) entity.ExptRunMode {
	switch m {
	case domain_expt.ExptRetryMode_RetryFailure:
		return entity.EvaluationModeFailRetry
	case domain_expt.ExptRetryMode_RetryAll:
		return entity.EvaluationModeRetryAll
	case domain_expt.ExptRetryMode_RetryTargetItems:
		return entity.EvaluationModeRetryItems
	default:
		return entity.EvaluationModeUnknown
	}
}

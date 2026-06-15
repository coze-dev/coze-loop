// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"fmt"
	"strings"

	"github.com/bytedance/gg/gcond"
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	domain_filter "github.com/coze-dev/coze-loop/backend/kitex_gen/stone/fornax/ml_flow/domain/filter"
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
		Ext:           cer.Ext,
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
				if w, ok := weights[conf.EvaluatorVersionID]; ok && w >= 0 {
					conf.ScoreWeight = gptr.Of(w)
				}
			}
		}
		ec.ConnectorConf.EvaluatorsConf = evalsConf
	}
	if cer.GetItemRetryNum() > 0 {
		ec.ItemRetryNum = gptr.Of(int(cer.GetItemRetryNum()))
	}
	if cer.IsSetEnableExtractTrajectory() {
		ec.EnableExtractTrajectory = gptr.Of(cer.GetEnableExtractTrajectory())
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
				if item.IsSetEvaluatorVersionID() && item.GetEvaluatorVersionID() > 0 {
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
				if evaluatorConf.ScoreWeight != nil && *evaluatorConf.ScoreWeight >= 0 {
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
			if ec == nil || ec.ScoreWeight == nil || *ec.ScoreWeight < 0 {
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
		ID:                        gptr.Of(experiment.ID),
		Name:                      gptr.Of(experiment.Name),
		Desc:                      gptr.Of(experiment.Description),
		CreatorBy:                 gptr.Of(experiment.CreatedBy),
		EvalSetVersionID:          gptr.Of(experiment.EvalSetVersionID),
		TargetVersionID:           gptr.Of(experiment.TargetVersionID),
		EvalSetID:                 gptr.Of(experiment.EvalSetID),
		TargetID:                  gptr.Of(experiment.TargetID),
		EvaluatorVersionIds:       evaluatorVersionIDs,
		Status:                    gptr.Of(domain_expt.ExptStatus(experiment.Status)),
		StatusMessage:             gptr.Of(experiment.StatusMessage),
		OfflineExptAnalysisStatus: gptr.Of(domain_expt.OfflineExptAnalysisStatus(experiment.OfflineExptAnalysisStatus)),
		ExptStats:                 ToExptStatsDTO(experiment.Stats, experiment.AggregateResult),
		TargetFieldMapping:        tm,
		EvaluatorFieldMapping:     ems,
		SourceType:                gptr.Of(domain_expt.SourceType(experiment.SourceType)),
		SourceID:                  gptr.Of(experiment.SourceID),
		ExptType:                  gptr.Of(domain_expt.ExptType(experiment.ExptType)),
		MaxAliveTime:              gptr.Of(experiment.MaxAliveTime),
		TargetRuntimeParam:        trtp,
		EvaluatorIDVersionList:    evaluatorIDVersionList,
	}
	if experiment.Visibility == entity.Visibility_Hidden {
		res.Visibility = gptr.Of(domain_expt.VisibilityHidden)
	}
	if experiment.ThreadID != nil {
		res.ThreadID = experiment.ThreadID
	}
	if experiment.TriggerType != "" {
		tt := experiment.TriggerType
		res.TriggerType = &tt
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
		res.EnableExtractTrajectory = experiment.EvalConf.EnableExtractTrajectory
		res.Ext = experiment.EvalConf.Ext
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

	// 关联的实验模板（仅在查询时按需填充基础信息）；在线实验不对外返回模板信息
	if experiment.ExptType != entity.ExptType_Online && experiment.ExptTemplateMeta != nil {
		res.ExptTemplateMeta = &domain_expt.ExptTemplateMeta{
			ID:          gptr.Of(experiment.ExptTemplateMeta.ID),
			WorkspaceID: gptr.Of(experiment.ExptTemplateMeta.WorkspaceID),
			Name:        gptr.Of(experiment.ExptTemplateMeta.Name),
			Desc:        gptr.Of(experiment.ExptTemplateMeta.Desc),
			ExptType:    gptr.Of(domain_expt.ExptType(experiment.ExptTemplateMeta.ExptType)),
		}
		if experiment.ExptTemplateMeta.Visibility == entity.Visibility_Hidden {
			res.ExptTemplateMeta.Visibility = gptr.Of(domain_expt.VisibilityHidden)
		}
	}

	res.EvalTarget = target.EvalTargetDO2DTO(experiment.Target)
	if experiment.EvalSet != nil {
		res.EvalSet = evaluation_set.EvaluationSetDO2DTO(experiment.EvalSet)
	}
	res.Evaluators = make([]*evaluatordto.Evaluator, 0, len(experiment.Evaluators))
	for _, evaluatorDO := range experiment.Evaluators {
		res.Evaluators = append(res.Evaluators, evaluator.ConvertEvaluatorDO2DTO(evaluatorDO))
	}

	// expt_source：查询路径下由 manager 填充（Workflow 时含 span_filter_fields / scheduler / sampler）；否则用一级 source 字段构造
	if es := ExptSourceDO2DTO(experiment.ExptSource); es != nil {
		res.SetExptSource(es)
	} else {
		st := domain_expt.SourceType(experiment.SourceType)
		fallback := &domain_expt.ExptSource{
			SourceType: &st,
			SourceID:   gptr.Of(experiment.SourceID),
		}
		if experiment.EvalConf != nil && experiment.EvalConf.TimeRange != nil {
			fallback.TimeRange = taskTimeRangeDO2DTO(experiment.EvalConf.TimeRange)
		}
		res.SetExptSource(fallback)
	}

	// ★ 新增段位 110~119: 多评测集读视图
	res.EvalSetSourceType = gptr.Of(domain_expt.ExptEvalSetSourceType(experiment.EvalSetSourceType))
	if experiment.EvalConf != nil && len(experiment.EvalConf.EvalSetConfigs) > 0 {
		// eval_set_configs 回显: 直接从 eval_conf 反序列化结果转 DTO (与 Create 入参同构)
		res.EvalSetConfigs = convertEvalSetConfigsDOToDTO(experiment.EvalConf.EvalSetConfigs)
	}
	// evaluators_concur_num 回显: 从 EvaluatorsConf 取
	if experiment.EvalConf != nil && experiment.EvalConf.ConnectorConf.EvaluatorsConf != nil &&
		experiment.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConcurNum != nil {
		res.EvaluatorsConcurNum = gptr.Of(int32(*experiment.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConcurNum))
	}

	return res
}

// convertEvalSetConfigsDOToDTO 将 domain EvalSetConfig 转回 IDL DTO (回显用)
func convertEvalSetConfigsDOToDTO(dos []*entity.EvalSetConfig) []*domain_expt.EvalSetConfig {
	if len(dos) == 0 {
		return nil
	}
	dtos := make([]*domain_expt.EvalSetConfig, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		dto := &domain_expt.EvalSetConfig{
			EvalSetID:        do.EvalSetID,
			EvalSetVersionID: do.EvalSetVersionID,
		}
		// item_filter 回显
		if do.ItemFilter != nil {
			dto.ItemFilter = convertExptFilterDOToDTO(do.ItemFilter)
		}
		for _, ec := range do.EvaluatorConfs {
			if ec == nil {
				continue
			}
			evDTO := &domain_expt.ExptEvaluatorConf{
				EvaluatorID:        ec.EvaluatorID,
				EvaluatorVersionID: ec.EvaluatorVersionID,
				Alias:              gptr.Of(ec.Alias),
				FilterMode:         gptr.Of(ec.FilterMode),
				ScoreWeight:        ec.ScoreWeight,
				FromEvalSet:        fieldConfsToFieldMappingDTO(ec.FromEvalSet),
				FromTarget:         fieldConfsToFieldMappingDTO(ec.FromTarget),
				RuntimeParam:       runtimeParamMapToDTO(ec.RuntimeParam),
			}
			if ec.Filter != nil {
				evDTO.Filter = convertExptFilterDOToDTO(ec.Filter)
			}
			dto.EvaluatorConfs = append(dto.EvaluatorConfs, evDTO)
		}
		// target_confs 回显
		for _, tc := range do.TargetConfs {
			if tc == nil {
				continue
			}
			tDTO := &domain_expt.ExptTargetConf{
				TargetID:        gptr.Of(tc.TargetID),
				TargetVersionID: gptr.Of(tc.TargetVersionID),
				Alias:           gptr.Of(tc.Alias),
				RuntimeParam:    runtimeParamMapToDTO(tc.RuntimeParam),
			}
			if len(tc.FieldMapping) > 0 {
				tDTO.FieldMapping = &domain_expt.TargetFieldMapping{
					FromEvalSet: fieldConfsToFieldMappingDTO(tc.FieldMapping),
				}
			}
			dto.TargetConfs = append(dto.TargetConfs, tDTO)
		}
		dtos = append(dtos, dto)
	}
	return dtos
}

// fieldConfsToFieldMappingDTO 将 domain FieldConf 列表回显为 IDL FieldMapping 列表。
func fieldConfsToFieldMappingDTO(fcs []*entity.FieldConf) []*domain_expt.FieldMapping {
	if len(fcs) == 0 {
		return nil
	}
	out := make([]*domain_expt.FieldMapping, 0, len(fcs))
	for _, fc := range fcs {
		if fc == nil {
			continue
		}
		out = append(out, &domain_expt.FieldMapping{
			FieldName:     gptr.Of(fc.FieldName),
			FromFieldName: gptr.Of(fc.FromField),
		})
	}
	return out
}

// runtimeParamMapToDTO 将 domain 的 runtime_param map 回显为 common.RuntimeParam ({json_value})。
func runtimeParamMapToDTO(m map[string]string) *common.RuntimeParam {
	if len(m) == 0 {
		return nil
	}
	v, ok := m[consts.FieldAdapterBuiltinFieldNameRuntimeParam]
	if !ok || len(v) == 0 {
		return nil
	}
	return &common.RuntimeParam{JSONValue: gptr.Of(v)}
}

// convertExptFilterDOToDTO 将 domain ExptItemFilter 回显为 data filter.Filter。
func convertExptFilterDOToDTO(do *entity.ExptItemFilter) *domain_filter.Filter {
	if do == nil {
		return nil
	}
	out := &domain_filter.Filter{}
	if len(do.QueryAndOr) > 0 {
		out.QueryAndOr = gptr.Of(domain_filter.QueryRelation(do.QueryAndOr))
	}
	for _, ff := range do.FilterFields {
		if ff == nil {
			continue
		}
		field := &domain_filter.FilterField{
			FieldName: ff.FieldName,
			FieldType: domain_filter.FieldType(ff.FieldType),
			Values:    ff.Values,
		}
		if len(ff.QueryType) > 0 {
			field.QueryType = gptr.Of(domain_filter.QueryType(ff.QueryType))
		}
		out.FilterFields = append(out.FilterFields, field)
	}
	return out
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
		SourceTargetID:       param.SourceTargetID,
		SourceTargetVersion:  param.SourceTargetVersion,
		BotPublishVersion:    param.BotPublishVersion,
		Region:               param.Region,
		Env:                  param.Env,
		OperationInstruction: param.OperationInstruction,
		Cluster:              param.Cluster,
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

func ExptType2EvalMode(exptType domain_expt.ExptType, trialRunItemCount *int64) entity.ExptRunMode {
	exptMode := entity.EvaluationModeSubmit
	if trialRunItemCount != nil && *trialRunItemCount > 0 {
		return entity.EvaluationModeTrialRun
	}
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
		TrialRunItemCount:     cer.GetTrialRunItemCount(),
		ExptConf:              nil,
		// ★ 分流依据透传: 不再从 len(EvalSetConfigs) 派生, 唯一以 eval_set_source_type 为准
		EvalSetSourceType: entity.ExptEvalSetSourceType(cer.GetEvalSetSourceType()),
	}
	if cer.IsSetVisibility() {
		if cer.GetVisibility() == domain_expt.VisibilityHidden {
			param.Visibility = gptr.Of(entity.Visibility_Hidden)
		} else {
			param.Visibility = gptr.Of(entity.Visibility(0))
		}
	}
	if cer.IsSetThreadID() {
		param.ThreadID = cer.ThreadID
	}

	// ★ 新路径: 仅当 eval_set_source_type == MultiSetConfig(2) 时转换 EvalSetConfigs (不再用 len 判断)
	if cer.GetEvalSetSourceType() == domain_expt.ExptEvalSetSourceType_MultiSetConfig {
		param.EvalSetConfigs = convertEvalSetConfigsDTOToDO(cer.GetEvalSetConfigs())
		// 顶层身份兜底: 供下游 getExptTupleByID 解析评测集/评测对象详情并做空间归属校验。
		// EvaluatorVersionIds 已在 application 层 (resolveEvaluatorVersionIDsFromEvalSetConfigs) 提取并回填到 cer，
		// 这里通过 cer.GetEvaluatorVersionIds() 透传，无需重复展开。
		fillTopLevelIdentityFromEvalSetConfigs(cer, param)
		// ★ 连接器兜底: 从 eval_set_configs 展开构建 ConnectorConf (EvaluatorsConf + TargetConf)。
		// 新路径权威源是 EvalSetConfigs，但 CreateExpt.CheckRun → CheckConnector 仍按老连接器结构
		// (EvalConf.ConnectorConf) 做同步字段映射校验。此处由 eval_set_configs 的 evaluator_confs
		// 字段映射构建等价的老连接器，使两侧由同一份输入派生、天然一致，校验得以通过。
		param.ExptConf = buildExptConfFromEvalSetConfigs(cer, evaluatorVersionRunConfigs)
	} else {
		// 老路径: 走 EvalConfConvert
		evaluationConfiguration, err := NewEvalConfConvert().ConvertToEntity(cer, evaluatorVersionRunConfigs)
		if err != nil {
			return nil, err
		}
		param.ExptConf = evaluationConfiguration
	}

	if cer.IsSetExptTemplateID() {
		param.ExptTemplateID = cer.GetExptTemplateID()
	}
	if cer.IsSetTriggerType() {
		param.TriggerType = strings.TrimSpace(cer.GetTriggerType())
	}
	return param, nil
}

// fillTopLevelIdentityFromEvalSetConfigs 新路径 (MultiSetConfig) 顶层身份兜底。
// 新路径下评测集/评测对象身份收敛进 eval_set_configs，但下游 CreateExpt.getExptTupleByID 仍按
// 顶层 EvalSetID/EvalSetVersionID/TargetID 解析三元组详情并做权限校验，故在此用 configs 兜底回填：
//   - 评测集: 取首个 set 作为"主集"标签 (与 CreateExpt 内主集兜底一致)；
//   - 评测对象: 顶层未显式传时，取首个 set 的 target_confs[0] (本期 len<=1, 各 set 与实验级一致)。
// 仅在顶层字段缺省时填充，不覆盖调用方显式传入的值。
func fillTopLevelIdentityFromEvalSetConfigs(cer *expt.CreateExperimentRequest, param *entity.CreateExptParam) {
	configs := cer.GetEvalSetConfigs()
	if len(configs) == 0 {
		return
	}

	// 主集标签兜底
	if param.EvalSetID == 0 {
		if first := configs[0]; first != nil {
			param.EvalSetID = first.GetEvalSetID()
			param.EvalSetVersionID = first.GetEvalSetVersionID()
		}
	}

	// 评测对象兜底: 顶层未传 target 时，从首个含 target_confs 的 set 取
	if (param.TargetID == nil || *param.TargetID == 0) && param.TargetVersionID == 0 {
		for _, sc := range configs {
			if sc == nil || len(sc.GetTargetConfs()) == 0 {
				continue
			}
			tc := sc.GetTargetConfs()[0]
			if tc == nil {
				continue
			}
			if tid := tc.GetTargetID(); tid != 0 {
				param.TargetID = gptr.Of(tid)
				param.TargetVersionID = tc.GetTargetVersionID()
				break
			}
		}
	}
}

// buildExptConfFromEvalSetConfigs 新路径 (MultiSetConfig) 连接器兜底构建。
// 把 eval_set_configs[].evaluator_confs / target_confs 的字段映射展平成老连接器结构
// (EvaluationConfiguration.ConnectorConf)，供 CheckConnector 同步字段映射校验使用。
//
// 收敛规则 (本期, 与 ValidateEvalSetConfigs 约束一致):
//   - EvaluatorsConf: 跨 set 聚合全部 evaluator_confs，按 (evaluator_version_id, alias) 去重，
//     每个 conf 的 from_eval_set→EvalSetAdapter、from_target→TargetAdapter；
//   - TargetConf: 顶层未显式传 target 时，取首个含 target_confs 的 set 的 target_confs[0]
//     (本期 len<=1，各 set 与实验级一致) 作为 IngressConf 的 EvalSetAdapter 字段映射；
//   - ScoreWeight / RunConf 复用既有 evaluatorVersionRunConfigs 与 conf.score_weight。
//
// 注: 这里仅为通过同步校验而派生老连接器；EvalSetConfigs 仍是落库与调度的权威源。
func buildExptConfFromEvalSetConfigs(cer *expt.CreateExperimentRequest, runConfigMap map[int64]*evaluatordto.EvaluatorRunConfig) *entity.EvaluationConfiguration {
	configs := cer.GetEvalSetConfigs()
	if len(configs) == 0 {
		return nil
	}

	ec := &entity.EvaluationConfiguration{
		ItemConcurNum: ptr.ConvIntPtr[int32, int](cer.ItemConcurNum),
		Ext:           cer.Ext,
	}
	if cer.GetItemRetryNum() > 0 {
		ec.ItemRetryNum = gptr.Of(int(cer.GetItemRetryNum()))
	}
	if cer.IsSetEnableExtractTrajectory() {
		ec.EnableExtractTrajectory = gptr.Of(cer.GetEnableExtractTrajectory())
	}

	// TargetConf: 取首个含 target_confs 的 set 的 target_confs[0]
	targetVersionID := cer.GetTargetVersionID()
	var targetMapping *domain_expt.TargetFieldMapping
	var targetRuntimeParam *common.RuntimeParam
	for _, sc := range configs {
		if sc == nil || len(sc.GetTargetConfs()) == 0 {
			continue
		}
		tc := sc.GetTargetConfs()[0]
		if tc == nil {
			continue
		}
		if targetVersionID == 0 {
			targetVersionID = tc.GetTargetVersionID()
		}
		targetMapping = tc.GetFieldMapping()
		targetRuntimeParam = tc.GetRuntimeParam()
		break
	}
	// 顶层显式 target_field_mapping 优先 (与老路径语义一致)
	if cer.GetTargetFieldMapping() != nil {
		targetMapping = cer.GetTargetFieldMapping()
	}
	if cer.GetTargetRuntimeParam() != nil {
		targetRuntimeParam = cer.GetTargetRuntimeParam()
	}
	ec.ConnectorConf.TargetConf = &entity.TargetConf{
		TargetVersionID: targetVersionID,
		IngressConf:     toTargetFieldMappingDO(targetMapping, targetRuntimeParam),
	}

	// EvaluatorsConf: 跨 set 聚合 evaluator_confs，按 (version_id, alias) 去重
	evalConfs := make([]*entity.EvaluatorConf, 0)
	seen := make(map[string]struct{})
	for _, sc := range configs {
		if sc == nil {
			continue
		}
		for _, evc := range sc.GetEvaluatorConfs() {
			if evc == nil || evc.GetEvaluatorVersionID() == 0 {
				continue
			}
			key := fmt.Sprintf("%d#%s", evc.GetEvaluatorVersionID(), evc.GetAlias())
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			esf := make([]*entity.FieldConf, 0, len(evc.GetFromEvalSet()))
			for _, fm := range evc.GetFromEvalSet() {
				if fm != nil {
					esf = append(esf, &entity.FieldConf{FieldName: fm.GetFieldName(), FromField: fm.GetFromFieldName()})
				}
			}
			tf := make([]*entity.FieldConf, 0, len(evc.GetFromTarget()))
			for _, fm := range evc.GetFromTarget() {
				if fm != nil {
					tf = append(tf, &entity.FieldConf{FieldName: fm.GetFieldName(), FromField: fm.GetFromFieldName()})
				}
			}

			var runConf *evaluatordto.EvaluatorRunConfig
			if len(runConfigMap) > 0 {
				runConf = runConfigMap[evc.GetEvaluatorVersionID()]
			}
			conf := &entity.EvaluatorConf{
				EvaluatorVersionID: evc.GetEvaluatorVersionID(),
				EvaluatorID:        evc.GetEvaluatorID(),
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{FieldConfs: esf},
					TargetAdapter:  &entity.FieldAdapter{FieldConfs: tf},
				},
				RunConf:     evaluator.ConvertEvaluatorRunConfDTO2DO(runConf),
				ScoreWeight: evc.ScoreWeight,
			}
			evalConfs = append(evalConfs, conf)
		}
	}
	if len(evalConfs) > 0 {
		evalsConf := &entity.EvaluatorsConf{
			EvaluatorConcurNum: ptr.ConvIntPtr[int32, int](cer.EvaluatorsConcurNum),
			EvaluatorConf:      evalConfs,
		}
		ec.ConnectorConf.EvaluatorsConf = evalsConf
	}

	return ec
}

// convertEvalSetConfigsDTOToDO 将 IDL EvalSetConfig 列表转换为 domain EvalSetConfig
func convertEvalSetConfigsDTOToDO(dtos []*domain_expt.EvalSetConfig) []*entity.EvalSetConfig {
	if len(dtos) == 0 {
		return nil
	}
	dos := make([]*entity.EvalSetConfig, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		do := &entity.EvalSetConfig{
			EvalSetID:        dto.GetEvalSetID(),
			EvalSetVersionID: dto.GetEvalSetVersionID(),
		}
		// item_filter
		if dto.IsSetItemFilter() {
			do.ItemFilter = convertExptFilterDTOToDO(dto.ItemFilter)
		}
		// evaluator_confs
		for _, ec := range dto.EvaluatorConfs {
			if ec == nil {
				continue
			}
			evConf := &entity.ExptEvaluatorConf{
				EvaluatorID:        ec.GetEvaluatorID(),
				EvaluatorVersionID: ec.GetEvaluatorVersionID(),
				Alias:              ec.GetAlias(),
				FilterMode:         ec.GetFilterMode(),
				ScoreWeight:        ec.ScoreWeight,
				RuntimeParam:       runtimeParamDTOToMap(ec.GetRuntimeParam()),
			}
			// field mappings
			for _, fm := range ec.FromEvalSet {
				if fm != nil {
					evConf.FromEvalSet = append(evConf.FromEvalSet, &entity.FieldConf{FieldName: fm.GetFieldName(), FromField: fm.GetFromFieldName()})
				}
			}
			for _, fm := range ec.FromTarget {
				if fm != nil {
					evConf.FromTarget = append(evConf.FromTarget, &entity.FieldConf{FieldName: fm.GetFieldName(), FromField: fm.GetFromFieldName()})
				}
			}
			if ec.IsSetFilter() {
				evConf.Filter = convertExptFilterDTOToDO(ec.Filter)
			}
			do.EvaluatorConfs = append(do.EvaluatorConfs, evConf)
		}
		// target_confs
		for _, tc := range dto.GetTargetConfs() {
			if tc == nil {
				continue
			}
			tConf := &entity.ExptTargetConf{
				TargetID:        tc.GetTargetID(),
				TargetVersionID: tc.GetTargetVersionID(),
				Alias:           tc.GetAlias(),
				RuntimeParam:    runtimeParamDTOToMap(tc.GetRuntimeParam()),
			}
			// target field_mapping: 仅 from_eval_set 维度 (DTO TargetFieldMapping)
			if fmap := tc.GetFieldMapping(); fmap != nil {
				for _, fm := range fmap.GetFromEvalSet() {
					if fm != nil {
						tConf.FieldMapping = append(tConf.FieldMapping, &entity.FieldConf{FieldName: fm.GetFieldName(), FromField: fm.GetFromFieldName()})
					}
				}
			}
			do.TargetConfs = append(do.TargetConfs, tConf)
		}
		dos = append(dos, do)
	}
	return dos
}

// runtimeParamDTOToMap 将 common.RuntimeParam ({json_value}) 落回 domain 的 map 表示，
// 与老路径一致用固定 key (builtin_runtime_param) 承载序列化后的运行时参数，避免新增维度。
func runtimeParamDTOToMap(rtp *common.RuntimeParam) map[string]string {
	if rtp == nil || len(rtp.GetJSONValue()) == 0 {
		return nil
	}
	return map[string]string{
		consts.FieldAdapterBuiltinFieldNameRuntimeParam: rtp.GetJSONValue(),
	}
}

// convertExptFilterDTOToDO 将 data filter.Filter 转换为 domain ExptItemFilter
// item 圈选 / evaluator 行级过滤复用 data/domain/filter.thrift 的 Filter/FilterField，
// 这里把生成的枚举 typedef (QueryRelation/FieldType/QueryType，底层均为 string) 落回 domain 的纯 string 表示。
func convertExptFilterDTOToDO(dto *domain_filter.Filter) *entity.ExptItemFilter {
	if dto == nil {
		return nil
	}
	do := &entity.ExptItemFilter{
		QueryAndOr: string(dto.GetQueryAndOr()),
	}
	for _, ff := range dto.FilterFields {
		if ff == nil {
			continue
		}
		do.FilterFields = append(do.FilterFields, &entity.ExptItemFilterField{
			FieldName: ff.GetFieldName(),
			FieldType: string(ff.GetFieldType()),
			Values:    ff.Values,
			QueryType: string(ff.GetQueryType()),
		})
	}
	return do
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

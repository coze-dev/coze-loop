// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"

func newHookExecutionSnapshot(expt *entity.Experiment, log *entity.ExptRunLog, scope string) (*entity.HookExecutionSnapshot, error) {
	mode := entity.ExptRunMode(log.Mode)
	online := mode == entity.EvaluationModeAppend && expt.ExptType == entity.ExptType_Online
	single := expt.EvalSetSourceType == 0 || expt.EvalSetSourceType == entity.ExptEvalSetSourceType_SingleSet
	if !online && (mode == entity.EvaluationModeAppend || expt.ExptType != entity.ExptType_Offline || !entity.HookBoundExecutionMode(mode) || (expt.EvalSetSourceType != entity.ExptEvalSetSourceType_MultiSetConfig && !(single && entity.HookBoundRetryMode(mode)))) {
		return nil, nil
	}
	if online && expt.EvalConf == nil {
		copy := *expt
		copy.EvalConf = &entity.EvaluationConfiguration{}
		expt = &copy
	}
	if expt.EvalConf == nil || (!online && !single && len(expt.EvalConf.EvalSetConfigs) == 0) {
		return nil, entity.ErrHookStoreCorrupt
	}
	d := &entity.HookExecutionSnapshot{
		Version: 1, Key: entity.HookRunKey{WorkspaceID: log.SpaceID, ExperimentID: log.ExptID, RunID: log.ExptRunID}, ExecutionScope: scope, Mode: mode,
		Target:                  entity.HookExecutionTarget{ID: expt.TargetID, VersionID: expt.TargetVersionID, Type: expt.TargetType, SourceSpaceID: expt.TargetSpaceID, Config: expt.EvalConf.ConnectorConf.TargetConf},
		RunModeConfig:           expt.EvalConf.RunModeConfig,
		VerificationConfig:      expt.EvalConf.VerificationConfig,
		EnableExtractTrajectory: expt.EvalConf.EnableExtractTrajectory,
		SkillTOSKeys:            expt.EvalConf.SkillTOSKeys,
		EvaluatorFallback:       &entity.HookExecutionEvaluatorFallback{},
	}
	if conf := expt.EvalConf.ConnectorConf.EvaluatorsConf; conf != nil {
		d.EvaluatorFallback.Confs = conf.EvaluatorConf
		d.EvaluatorFallback.EnableScoreWeight = conf.EnableScoreWeight
	}
	if online {
		d.SingleSet = true
		for _, set := range expt.EvalConf.EvalSetConfigs {
			if set == nil {
				return nil, entity.ErrHookStoreCorrupt
			}
			d.Sets = append(d.Sets, entity.HookExecutionSet{EvalSetID: set.EvalSetID, EvalSetVersionID: set.EvalSetVersionID, SourceSpaceID: set.SourceSpaceID, ItemConfig: &entity.ExptItemConfig{EvalSetSourceSpaceID: set.SourceSpaceID}})
		}
		if len(d.Sets) == 0 && expt.EvalSetID > 0 {
			d.Sets = []entity.HookExecutionSet{{EvalSetID: expt.EvalSetID, EvalSetVersionID: expt.EvalSetVersionID, SourceSpaceID: expt.EvalSetSpaceID, ItemConfig: &entity.ExptItemConfig{EvalSetSourceSpaceID: expt.EvalSetSpaceID}}}
		}
		return d, nil
	}
	if single {
		d.SingleSet = true
		d.Sets = []entity.HookExecutionSet{{EvalSetID: expt.EvalSetID, EvalSetVersionID: expt.EvalSetVersionID, SourceSpaceID: expt.EvalSetSpaceID, ItemConfig: &entity.ExptItemConfig{EvalSetSourceSpaceID: expt.EvalSetSpaceID}}}
		return d, nil
	}
	for _, set := range expt.EvalConf.EvalSetConfigs {
		if set == nil {
			return nil, entity.ErrHookStoreCorrupt
		}
		d.Sets = append(d.Sets, entity.HookExecutionSet{EvalSetID: set.EvalSetID, EvalSetVersionID: set.EvalSetVersionID, SourceSpaceID: set.SourceSpaceID, ItemFilter: set.ItemFilter, ItemConfig: buildItemConfigFromSetConf(set, expt.EvalConf.RunModeConfig)})
	}
	return d, nil
}

// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func (e *ExptMangerImpl) checkResultSetAdmission(ctx context.Context, spaceID int64, conf *entity.EvaluationConfiguration) error {
	if !conf.IsResultSet() {
		return nil
	}
	gate, ok := e.configer.(component.ResultSetAdmission)
	if !ok {
		return fmt.Errorf("result_set is disabled in this deployment")
	}
	versions, err := conf.ResultSetBinaryVersions()
	if err != nil {
		return err
	}
	for _, version := range versions {
		if err := gate.CheckResultSetRuntime(ctx, spaceID, version); err != nil {
			return err
		}
	}
	return nil
}

// Resolve a system default only during creation; retries consume the frozen pin.
func (e *ExptMangerImpl) prepareResultSetCreation(ctx context.Context, req *entity.CreateExptParam) error {
	if err := e.normalizeResultSetApplication(ctx, req); err != nil {
		return err
	}
	if !req.ExptConf.IsResultSet() {
		return nil
	}
	gate, ok := e.configer.(component.ResultSetAdmission)
	if !ok {
		return fmt.Errorf("result_set is disabled in this deployment")
	}
	req.ExptConf.EvalSetConfigs = req.EvalSetConfigs
	versions, err := req.ExptConf.RequestedResultSetBinaryVersions()
	if err != nil {
		return err
	}
	requested := ""
	for _, version := range versions {
		if requested != "" && requested != version {
			return fmt.Errorf("result_set runtime version pins conflict")
		}
		requested = version
	}
	version, err := gate.ResolveResultSetRuntime(ctx, req.WorkspaceID, requested)
	if err != nil {
		return err
	}
	if err := req.ExptConf.FreezeResultSetBinaryVersion(version); err != nil {
		return err
	}
	return e.checkResultSetAdmission(ctx, req.WorkspaceID, req.ExptConf)
}

func (e *ExptMangerImpl) normalizeResultSetApplication(ctx context.Context, req *entity.CreateExptParam) error {
	gate, ok := e.configer.(component.ResultSetApplicationAdmission)
	if !ok {
		return nil
	}
	id := ""
	if !req.CreateEvalTargetParam.IsNull() {
		id = gptr.Indirect(req.CreateEvalTargetParam.SourceTargetID)
	} else if gptr.Indirect(req.TargetID) > 0 {
		target, err := e.evalTargetService.GetEvalTarget(ctx, *req.TargetID)
		if err != nil {
			return err
		}
		if target != nil {
			id = target.SourceTargetID
		}
	}
	if !gate.IsResultSetApplication(ctx, id) {
		return nil
	}
	if !req.CreateEvalTargetParam.IsNull() {
		if gptr.Indirect(req.CreateEvalTargetParam.EvalTargetType) != entity.EvalTargetTypeSandboxAgent {
			return fmt.Errorf("result_set application requires SandboxAgent target type")
		}
		req.CreateEvalTargetParam.SandboxAgent = nil
		if sourceSpaceID := entity.ResultSetApplicationSourceSpace(gptr.Indirect(req.CreateEvalTargetParam.SourceTargetVersion)); sourceSpaceID > 0 && sourceSpaceID != req.WorkspaceID && !req.TargetSharedOption.Enabled() {
			req.TargetSharedOption = &entity.SharedResourceOption{IsShared: true, SourceSpaceID: gptr.Of(sourceSpaceID)}
		}
	}
	for _, set := range req.EvalSetConfigs {
		if set == nil || !set.TargetSharedOption.Enabled() {
			continue
		}
		if !req.TargetSharedOption.Enabled() || *set.TargetSharedOption.SourceSpaceID != *req.TargetSharedOption.SourceSpaceID {
			return fmt.Errorf("result_set requires one authorized application source")
		}
	}
	if req.ExptConf == nil {
		req.ExptConf = &entity.EvaluationConfiguration{}
	}
	if req.EvalSetSourceType != entity.ExptEvalSetSourceType_MultiSetConfig {
		if (req.EvalSetSourceType != 0 && req.EvalSetSourceType != entity.ExptEvalSetSourceType_SingleSet) || len(req.EvalSetConfigs) != 0 {
			return fmt.Errorf("result_set single-set selection conflicts with eval_set_configs")
		}
		if req.EvalSetID <= 0 || req.EvalSetVersionID <= 0 {
			return fmt.Errorf("result_set requires a fixed evaluation set version")
		}
		if !e.configer.GetExptMultiSetWhiteList(ctx).IsSpaceAllowed(req.WorkspaceID) {
			return fmt.Errorf("space %d is not allowed to create multi-set experiments", req.WorkspaceID)
		}
		binding := &entity.ExptTargetConf{TargetID: gptr.Indirect(req.TargetID), TargetVersionID: req.TargetVersionID, RuntimeParam: make(map[string]string)}
		if target := req.ExptConf.ConnectorConf.TargetConf; target != nil && target.IngressConf != nil {
			if adapter := target.IngressConf.EvalSetAdapter; adapter != nil {
				binding.FieldMapping = adapter.FieldConfs
			}
			if custom := target.IngressConf.CustomConf; custom != nil {
				for _, field := range custom.FieldConfs {
					if field != nil {
						binding.RuntimeParam[field.FieldName] = field.Value
					}
				}
			}
		}
		req.EvalSetConfigs = []*entity.EvalSetConfig{{EvalSetID: req.EvalSetID, EvalSetVersionID: req.EvalSetVersionID, SharedOption: req.EvalSetSharedOption, TargetConfs: []*entity.ExptTargetConf{binding}}}
		req.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
	}
	cfg := req.ExptConf.RunModeConfig
	if cfg == nil {
		cfg = &entity.RunModeConfig{}
	}
	copy := *cfg
	if copy.RunMode == entity.RunModeSingleTurn && copy.MaxTurns == 1 {
		copy.MaxTurns = 0
	}
	copy.RunMode = entity.RunModeResultSet
	if copy.MaxRunMinutes == 0 {
		copy.MaxRunMinutes = 30
	}
	req.ExptConf.RunModeConfig = &copy
	req.ResultSetApplication = true
	return nil
}

func (e *ExptMangerImpl) resultSetApplicationIDs(ctx context.Context, target *entity.EvalTarget) []string {
	if target == nil {
		return nil
	}
	if gate, ok := e.configer.(component.ResultSetApplicationAdmission); ok && gate.IsResultSetApplication(ctx, target.SourceTargetID) {
		return []string{target.SourceTargetID}
	}
	return nil
}

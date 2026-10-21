// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"

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

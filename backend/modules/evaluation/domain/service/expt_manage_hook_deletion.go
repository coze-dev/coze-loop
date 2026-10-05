// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewExptManagerWithHookDeletion(base IExptManager, repository repo.IHookDeletionRepo) (*ExptMangerImpl, error) {
	m, ok := base.(*ExptMangerImpl)
	if !ok || m == nil || m.finalization == nil || m.deletion != nil || missingManagerHookDependency(repository) {
		return nil, errors.New("missing hook deletion recovery dependencies")
	}
	copy := *m
	copy.deletion = repository
	return &copy, nil
}

func (e *ExptMangerImpl) deleteHookExperiments(ctx context.Context, ids []int64, spaceID int64, single bool) error {
	var expts []*entity.Experiment
	if single {
		expt, err := e.exptRepo.GetByID(ctx, ids[0], spaceID)
		if err != nil {
			return err
		}
		expts = []*entity.Experiment{expt}
	} else {
		var err error
		expts, err = e.exptRepo.MGetByID(ctx, ids, spaceID)
		if err != nil {
			return err
		}
	}
	_, prepared := e.deletion.(interface{ HookDeletionScope() string })
	// Preserve the ordinary path; prepared deletion may still roll back the whole batch.
	if !prepared {
		for _, expt := range expts {
			e.releaseCentralQuotaForIncompleteItems(ctx, expt, nil, entity.ExptStatus_Terminated)
		}
	}
	deleted, err := e.deletion.DeleteExperiments(ctx, ids, spaceID, e.finalization.ExecutionScope)
	if err != nil {
		return err
	}
	if prepared {
		for _, expt := range deleted {
			if expt == nil {
				continue
			}
			// The committed parent view also identifies a legacy Latest above older managed Runs.
			original := *expt
			runID := original.LatestRunID
			e.releaseCentralQuotaForIncompleteItems(ctx, &original, &runID, entity.ExptStatus_Terminated)
		}
	}
	for _, expt := range deleted {
		if expt == nil || expt.ExptTemplateMeta == nil || expt.ExptTemplateMeta.ID <= 0 || e.templateManager == nil {
			continue
		}
		if err := e.templateManager.UpdateExptInfo(ctx, expt.ExptTemplateMeta.ID, spaceID, expt.ID, expt.Status, -1, nil); err != nil {
			logs.CtxError(ctx, "Hook deletion template update failed, experiment=%d: %v", expt.ID, err)
		}
	}
	return nil
}

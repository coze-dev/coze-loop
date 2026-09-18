// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"strings"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func (e *EvalTargetServiceImpl) GetExecutionContext(ctx context.Context, spaceID, recordID int64) (*entity.EvalTargetExecutionContext, error) {
	if spaceID <= 0 || recordID <= 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}
	record, err := e.evalTargetRepo.GetEvalTargetRecordByIDAndSpaceID(ctx, spaceID, recordID)
	if err != nil {
		return nil, err
	}
	if record == nil || record.ID != recordID || record.SpaceID != spaceID || record.ExperimentRunID <= 0 {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode)
	}
	run, err := e.exptRunLogRepo.GetByRunID(ctx, record.ExperimentRunID)
	if err != nil {
		return nil, err
	}
	if run == nil || run.ExptRunID != record.ExperimentRunID ||
		run.ExptID <= 0 || run.SpaceID <= 0 || strings.TrimSpace(run.CreatedBy) == "" {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode)
	}
	// Shared targets keep records in the target space; the persisted run belongs to the experiment space.
	return &entity.EvalTargetExecutionContext{
		WorkspaceID:           record.SpaceID,
		EvalTargetRecordID:    record.ID,
		ExperimentID:          run.ExptID,
		ExperimentRunID:       run.ExptRunID,
		ExperimentWorkspaceID: run.SpaceID,
		InitiatorUserID:       strings.TrimSpace(run.CreatedBy),
	}, nil
}

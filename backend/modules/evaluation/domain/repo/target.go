// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/target.go -package=mocks . IEvalTargetRepo
type IEvalTargetRepo interface {
	CreateEvalTarget(ctx context.Context, do *entity.EvalTarget) (id, versionID int64, err error)
	GetEvalTarget(ctx context.Context, targetID int64) (do *entity.EvalTarget, err error)
	GetEvalTargetVersion(ctx context.Context, spaceID, versionID int64) (do *entity.EvalTarget, err error)
	BatchGetEvalTargetBySource(ctx context.Context, param *BatchGetEvalTargetBySourceParam) (dos []*entity.EvalTarget, err error)
	BatchGetEvalTargetVersion(ctx context.Context, spaceID int64, versionIDs []int64) (dos []*entity.EvalTarget, err error)

	// target record start
	CreateEvalTargetRecord(ctx context.Context, record *entity.EvalTargetRecord) (int64, error)
	SaveEvalTargetRecord(ctx context.Context, record *entity.EvalTargetRecord) error
	GetEvalTargetRecordByIDAndSpaceID(ctx context.Context, spaceID int64, recordID int64) (*entity.EvalTargetRecord, error)
	ListEvalTargetRecordByIDsAndSpaceID(ctx context.Context, spaceID int64, recordIDs []int64) ([]*entity.EvalTargetRecord, error)
	// target record end
}

type BatchGetEvalTargetBySourceParam struct {
	SpaceID        int64
	SourceTargetID []string
	TargetType     entity.EvalTargetType
}

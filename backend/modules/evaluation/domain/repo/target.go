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
	GetEvalTargetVersionByTarget(ctx context.Context, spaceID, targetID int64, sourceTargetVersion string) (do *entity.EvalTarget, err error)
	GetEvalTargetVersionBySourceTarget(ctx context.Context, spaceID int64, sourceTargetID, sourceTargetVersion string, targetType entity.EvalTargetType) (do *entity.EvalTarget, err error)
	BatchGetEvalTargetBySource(ctx context.Context, param *BatchGetEvalTargetBySourceParam) (dos []*entity.EvalTarget, err error)
	BatchGetEvalTargetVersion(ctx context.Context, spaceID int64, versionIDs []int64) (dos []*entity.EvalTarget, err error)

	// target record start
	CreateEvalTargetRecord(ctx context.Context, record *entity.EvalTargetRecord) (int64, error)
	SaveEvalTargetRecord(ctx context.Context, record *entity.EvalTargetRecord) error
	UpdateEvalTargetRecord(ctx context.Context, record *entity.EvalTargetRecord) error
	GetEvalTargetRecordByIDAndSpaceID(ctx context.Context, spaceID int64, recordID int64) (*entity.EvalTargetRecord, error)
	GetEvalTargetRecordByExperimentRunIDAndItemID(ctx context.Context, spaceID int64, targetID int64, experimentRunID int64, itemID int64, turnID int64) (*entity.EvalTargetRecord, error)
	GetEvalTargetRecordByExperimentRunIDAndItemIDWithoutTargetID(ctx context.Context, spaceID int64, experimentRunID int64, itemID int64, turnID int64) (*entity.EvalTargetRecord, error)
	ListEvalTargetRecordByIDsAndSpaceID(ctx context.Context, spaceID int64, recordIDs []int64) ([]*entity.EvalTargetRecord, error)
	// LoadEvalTargetRecordOutputFields 从 S3 加载 record output 中指定字段的大对象完整内容
	LoadEvalTargetRecordOutputFields(ctx context.Context, record *entity.EvalTargetRecord, fieldKeys []string) error
	// target record end
}

type BatchGetEvalTargetBySourceParam struct {
	SpaceID        int64
	SourceTargetID []string
	TargetType     entity.EvalTargetType
}

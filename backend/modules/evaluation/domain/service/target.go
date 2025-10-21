// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/target.go -package=mocks . IEvalTargetService
type IEvalTargetService interface {
	CreateEvalTarget(ctx context.Context, spaceID int64, sourceTargetID, sourceTargetVersion string, targetType entity.EvalTargetType, opts ...entity.Option) (id, versionID int64, err error)
	GetEvalTarget(ctx context.Context, targetID int64) (do *entity.EvalTarget, err error)
	GetEvalTargetVersion(ctx context.Context, spaceID, versionID int64, needSourceInfo bool) (do *entity.EvalTarget, err error)
	GetEvalTargetVersionBySource(ctx context.Context, spaceID, targetID int64, sourceVersion string, needSourceInfo bool) (do *entity.EvalTarget, err error)
	GetEvalTargetVersionByTarget(ctx context.Context, spaceID, targetID int64, sourceTargetVersion string, needSourceInfo bool) (do *entity.EvalTarget, err error)
	GetEvalTargetVersionBySourceTarget(ctx context.Context, spaceID int64, sourceTargetID, sourceTargetVersion string, targetType entity.EvalTargetType, needSourceInfo bool) (do *entity.EvalTarget, err error)
	BatchGetEvalTargetBySource(ctx context.Context, param *entity.BatchGetEvalTargetBySourceParam) (dos []*entity.EvalTarget, err error)
	BatchGetEvalTargetVersion(ctx context.Context, spaceID int64, versionIDs []int64, needSourceInfo bool) (dos []*entity.EvalTarget, err error)

	ExecuteTarget(ctx context.Context, spaceID int64, targetID int64, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error)
	AsyncExecuteTarget(ctx context.Context, spaceID int64, targetID int64, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) (record *entity.EvalTargetRecord, callee string, err error)
	DebugTarget(ctx context.Context, param *entity.DebugTargetParam) (record *entity.EvalTargetRecord, err error)
	AsyncDebugTarget(ctx context.Context, param *entity.DebugTargetParam) (record *entity.EvalTargetRecord, callee string, err error)
	GetRecordByID(ctx context.Context, spaceID int64, recordID int64) (*entity.EvalTargetRecord, error)
	CreateRecord(ctx context.Context, record *entity.EvalTargetRecord) error
	BatchGetRecordByIDs(ctx context.Context, spaceID int64, recordIDs []int64) ([]*entity.EvalTargetRecord, error)
	ReportInvokeRecords(ctx context.Context, recordID2Params *entity.ReportTargetRecordParam) error
	ValidateRuntimeParam(ctx context.Context, targetType entity.EvalTargetType, runtimeParam string) error
	GenerateMockOutputData(outputSchemas []*entity.ArgsSchema) (map[string]string, error)
}

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

	ExecuteTarget(ctx context.Context, spaceID, targetID, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error)
	AsyncExecuteTarget(ctx context.Context, spaceID, targetID, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) (record *entity.EvalTargetRecord, callee string, err error)
	DebugTarget(ctx context.Context, param *entity.DebugTargetParam) (record *entity.EvalTargetRecord, err error)
	AsyncDebugTarget(ctx context.Context, param *entity.DebugTargetParam) (record *entity.EvalTargetRecord, callee string, err error)
	GetRecordByID(ctx context.Context, spaceID, recordID int64) (*entity.EvalTargetRecord, error)
	GetRecordByRunItemTurn(ctx context.Context, spaceID, runID, itemID, turnID int64) (*entity.EvalTargetRecord, error)
	CreateRecord(ctx context.Context, record *entity.EvalTargetRecord) error
	BatchGetRecordByIDs(ctx context.Context, spaceID int64, recordIDs []int64) ([]*entity.EvalTargetRecord, error)
	// LoadRecordOutputFields 从 TOS 加载 record 中指定 output 字段的完整内容（用于评估器输入需完整 target_output 的场景）
	LoadRecordOutputFields(ctx context.Context, record *entity.EvalTargetRecord, fieldKeys []string) error
	// LoadRecordFullData 从 TOS 加载 record 中所有被省略的大对象完整内容（用于导出等需要完整字段的场景）
	LoadRecordFullData(ctx context.Context, record *entity.EvalTargetRecord) error
	ReportInvokeRecords(ctx context.Context, recordID2Params *entity.ReportTargetRecordParam) error
	// TerminateAsyncRecordsAndDestroySandbox 把仍处于 AsyncInvoking 状态的 SandboxAgent EvalTargetRecord 置为 Fail，
	// 并以 best-effort 方式触发沙箱 Execute 销毁。非 SandboxAgent / 非 AsyncInvoking 的 record 会被忽略。
	// errCode 用于写入 EvalTargetRunError，区分 zombie timeout / 手动取消等场景。
	TerminateAsyncRecordsAndDestroySandbox(ctx context.Context, spaceID int64, recordIDs []int64, errCode int32, errMessage string)
	ValidateRuntimeParam(ctx context.Context, targetType entity.EvalTargetType, runtimeParam string) error
	GenerateMockOutputData(outputSchemas []*entity.ArgsSchema) (map[string]string, error)
	ExtractTrajectory(ctx context.Context, spaceID int64, traceID string, startTimeMS *int64) (*entity.Trajectory, error)
}

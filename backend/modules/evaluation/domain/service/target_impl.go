// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/sonic"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
	"github.com/mohae/deepcopy"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/looptracer"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/jsonmock"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type EvalTargetServiceImpl struct {
	idgen             idgen.IIDGenerator
	metric            metrics.EvalTargetMetrics
	evalTargetRepo    repo.IEvalTargetRepo
	typedOperators    map[entity.EvalTargetType]ISourceEvalTargetOperateService
	trajectoryAdapter rpc.ITrajectoryAdapter
	configer          component.IConfiger
}

func NewEvalTargetServiceImpl(evalTargetRepo repo.IEvalTargetRepo,
	idgen idgen.IIDGenerator,
	metric metrics.EvalTargetMetrics,
	typedOperators map[entity.EvalTargetType]ISourceEvalTargetOperateService,
	trajectoryAdapter rpc.ITrajectoryAdapter,
	configer component.IConfiger,
) IEvalTargetService {
	singletonEvalTargetService := &EvalTargetServiceImpl{
		evalTargetRepo:    evalTargetRepo,
		idgen:             idgen,
		metric:            metric,
		typedOperators:    typedOperators,
		trajectoryAdapter: trajectoryAdapter,
		configer:          configer,
	}
	return singletonEvalTargetService
}

func (e *EvalTargetServiceImpl) CreateEvalTarget(ctx context.Context, spaceID int64, sourceTargetID, sourceTargetVersion string, targetType entity.EvalTargetType, opts ...entity.Option) (id, versionID int64, err error) {
	defer func() {
		e.metric.EmitCreate(spaceID, err)
	}()
	if e.typedOperators[targetType] == nil {
		return 0, 0, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
	}
	do, err := e.typedOperators[targetType].BuildBySource(ctx, spaceID, sourceTargetID, sourceTargetVersion, opts...)
	if err != nil {
		return 0, 0, err
	}

	if do == nil {
		return 0, 0, errorx.NewByCode(errno.CommonInvalidParamCode)
	}

	return e.evalTargetRepo.CreateEvalTarget(ctx, do)
}

func (e *EvalTargetServiceImpl) GetEvalTarget(ctx context.Context, targetID int64) (do *entity.EvalTarget, err error) {
	return e.evalTargetRepo.GetEvalTarget(ctx, targetID)
}

func (e *EvalTargetServiceImpl) GetEvalTargetVersion(ctx context.Context, spaceID, versionID int64, needSourceInfo bool) (do *entity.EvalTarget, err error) {
	do, err = e.evalTargetRepo.GetEvalTargetVersion(ctx, spaceID, versionID)
	if err != nil {
		return nil, err
	}
	// Wrap source info
	if needSourceInfo {
		for _, op := range e.typedOperators {
			err = op.PackSourceVersionInfo(ctx, spaceID, []*entity.EvalTarget{do})
			if err != nil {
				return nil, err
			}
		}
	}
	return do, nil
}

func (e *EvalTargetServiceImpl) GetEvalTargetVersionBySourceTarget(ctx context.Context, spaceID int64, sourceTargetID, sourceTargetVersion string, targetType entity.EvalTargetType, needSourceInfo bool) (do *entity.EvalTarget, err error) {
	do, err = e.evalTargetRepo.GetEvalTargetVersionBySourceTarget(ctx, spaceID, sourceTargetID, sourceTargetVersion, targetType)
	if err != nil {
		return nil, err
	}
	// Wrap source info
	if needSourceInfo {
		for _, op := range e.typedOperators {
			err = op.PackSourceVersionInfo(ctx, spaceID, []*entity.EvalTarget{do})
			if err != nil {
				return nil, err
			}
		}
	}
	return do, nil
}

func (e *EvalTargetServiceImpl) GetEvalTargetVersionBySource(ctx context.Context, spaceID, targetID int64, sourceVersion string, needSourceInfo bool) (do *entity.EvalTarget, err error) {
	// Query version by spaceID, targetID, and sourceVersion
	versions, err := e.evalTargetRepo.BatchGetEvalTargetBySource(ctx, &repo.BatchGetEvalTargetBySourceParam{
		SpaceID:        spaceID,
		SourceTargetID: []string{strconv.FormatInt(targetID, 10)},
	})
	if err != nil {
		return nil, err
	}

	// Iterate through versions to find matching sourceVersion
	for _, version := range versions {
		if version.EvalTargetVersion != nil && version.EvalTargetVersion.SourceTargetVersion == sourceVersion {
			// Wrap source info
			if needSourceInfo {
				for _, op := range e.typedOperators {
					err = op.PackSourceVersionInfo(ctx, spaceID, []*entity.EvalTarget{version})
					if err != nil {
						return nil, err
					}
				}
			}
			return version, nil
		}
	}

	return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("eval target version not found for source version: "+sourceVersion))
}

func (e *EvalTargetServiceImpl) GetEvalTargetVersionByTarget(ctx context.Context, spaceID, targetID int64, sourceTargetVersion string, needSourceInfo bool) (do *entity.EvalTarget, err error) {
	do, err = e.evalTargetRepo.GetEvalTargetVersionByTarget(ctx, spaceID, targetID, sourceTargetVersion)
	if err != nil {
		return nil, err
	}
	// Wrap source info
	if needSourceInfo {
		for _, op := range e.typedOperators {
			err = op.PackSourceVersionInfo(ctx, spaceID, []*entity.EvalTarget{do})
			if err != nil {
				return nil, err
			}
		}
	}
	return do, nil
}

func (e *EvalTargetServiceImpl) BatchGetEvalTargetBySource(ctx context.Context, param *entity.BatchGetEvalTargetBySourceParam) (dos []*entity.EvalTarget, err error) {
	return e.evalTargetRepo.BatchGetEvalTargetBySource(ctx, &repo.BatchGetEvalTargetBySourceParam{
		SpaceID:        param.SpaceID,
		SourceTargetID: param.SourceTargetID,
		TargetType:     param.TargetType,
	})
}

func (e *EvalTargetServiceImpl) BatchGetEvalTargetVersion(ctx context.Context, spaceID int64, versionIDs []int64, needSourceInfo bool) (dos []*entity.EvalTarget, err error) {
	versions, err := e.evalTargetRepo.BatchGetEvalTargetVersion(ctx, spaceID, versionIDs)
	if err != nil {
		return nil, err
	}
	// Wrap source info
	if needSourceInfo {
		for _, op := range e.typedOperators {
			err = op.PackSourceVersionInfo(ctx, spaceID, versions)
			if err != nil {
				return nil, err
			}
		}
	}
	return versions, nil
}

func (e *EvalTargetServiceImpl) ExecuteTarget(ctx context.Context, spaceID, targetID, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) (record *entity.EvalTargetRecord, err error) {
	startTime := time.Now()
	defer func() {
		e.metric.EmitRun(spaceID, err, startTime)
	}()
	if spaceID == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("[ExecuteTarget]space_id is zero"))
	}
	if inputData == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("[ExecuteTarget]inputData is zero"))
	}
	if param == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("[ExecuteTarget]param is zero"))
	}

	var span looptracer.Span
	spanParam := &targetSpanTagsParams{
		Error:    nil,
		ErrCode:  "",
		CallType: "eval_target",
	}

	var outputData *entity.EvalTargetOutputData
	runStatus := entity.EvalTargetRunStatusUnknown

	evalTargetDO, err := e.GetEvalTargetVersion(ctx, spaceID, targetVersionID, false)
	if err != nil {
		return nil, err
	}
	if evalTargetDO == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("[ExecuteTarget]evalTargetDO is nil"))
	}

	defer func() {
		if e := recover(); e != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			logs.CtxError(ctx, "goroutine panic: %s: %s", e, buf)
			err = errorx.New("panic occurred when, reason=%v", e)
		}

		execErr := err
		if execErr != nil {
			logs.CtxError(ctx, "execute target failed, spaceID=%v, targetID=%d, targetVersionID=%d, param=%v, inputData=%v, err=%v",
				spaceID, targetID, targetVersionID, json.Jsonify(param), json.Jsonify(inputData), err)
			spanParam.Error = err
			runStatus = entity.EvalTargetRunStatusFail
			outputData = &entity.EvalTargetOutputData{
				OutputFields:       map[string]*entity.Content{},
				EvalTargetUsage:    &entity.EvalTargetUsage{InputTokens: 0, OutputTokens: 0},
				EvalTargetRunError: &entity.EvalTargetRunError{},
				TimeConsumingMS:    gptr.Of(int64(0)),
			}
			statusErr, ok := errorx.FromStatusError(err)
			if ok {
				outputData.EvalTargetRunError = &entity.EvalTargetRunError{
					Code:    statusErr.Code(),
					Message: statusErr.Error(),
				}
				spanParam.ErrCode = strconv.FormatInt(int64(statusErr.Code()), 10)
			} else {
				outputData.EvalTargetRunError = &entity.EvalTargetRunError{
					Code:    errno.CommonInternalErrorCode,
					Message: err.Error(),
				}
			}
		}

		userIDInContext := session.UserIDInCtxOrEmpty(ctx)

		if span != nil {
			span.SetInput(ctx, Convert2TraceString(spanParam.Inputs))
			span.SetOutput(ctx, Convert2TraceString(spanParam.Outputs))
			span.SetInputTokens(ctx, int(spanParam.InputToken))
			span.SetOutputTokens(ctx, int(spanParam.OutputToken))
			if spanParam.Error != nil {
				span.SetError(ctx, spanParam.Error)
			}
			tags := make(map[string]interface{})
			tags["eval_target_type"] = spanParam.TargetType
			tags["eval_target_id"] = spanParam.TargetID
			tags["eval_target_version"] = spanParam.TargetVersion

			span.SetUserID(ctx, userIDInContext)

			span.SetTags(ctx, tags)
			span.Finish(ctx)
		}

		if execErr == nil && evalTargetDO.EvalTargetType.SupptTrajectory() {
			time.Sleep(e.configer.GetTargetTrajectoryConf(ctx).GetExtractInterval(spaceID))
			trajectory, err := e.ExtractTrajectory(ctx, spaceID, span.GetTraceID(), nil)
			if err != nil {
				logs.CtxError(ctx, "ExtractTrajectory fail, space_id: %v, target_id: %v, target_version_id: %v, trace_id: %v, err: %v",
					spaceID, targetID, targetVersionID, span.GetTraceID(), err)
			} else {
				if outputData.OutputFields == nil {
					outputData.OutputFields = make(map[string]*entity.Content)
				}
				outputData.OutputFields[consts.EvalTargetOutputFieldKeyTrajectory] = trajectory.ToContent(ctx)
			}
		}

		recordID, err1 := e.idgen.GenID(ctx)
		if err1 != nil {
			err = err1
			return
		}
		logID := logs.GetLogID(ctx)

		record = &entity.EvalTargetRecord{
			ID:                   recordID,
			SpaceID:              spaceID,
			TargetID:             targetID,
			TargetVersionID:      targetVersionID,
			ExperimentRunID:      gptr.Indirect(param.ExperimentRunID),
			ItemID:               param.ItemID,
			TurnID:               param.TurnID,
			TraceID:              span.GetTraceID(),
			LogID:                logID,
			EvalTargetInputData:  inputData,
			EvalTargetOutputData: outputData,
			Status:               &runStatus,
			BaseInfo: &entity.BaseInfo{
				CreatedBy: &entity.UserInfo{
					UserID: gptr.Of(userIDInContext),
				},
				UpdatedBy: &entity.UserInfo{
					UserID: gptr.Of(userIDInContext),
				},
				CreatedAt: gptr.Of(time.Now().UnixMilli()),
				UpdatedAt: gptr.Of(time.Now().UnixMilli()),
			},
		}

		_, errCreate := e.evalTargetRepo.CreateEvalTargetRecord(ctx, record)
		if errCreate != nil {
			return
		}
		err = nil
	}()

	ctx, span = looptracer.GetTracer().StartSpan(ctx, "EvalTarget", "eval_target", looptracer.WithStartNewTrace(), looptracer.WithSpanWorkspaceID(strconv.FormatInt(spaceID, 10)))
	span.SetCallType("EvalTarget")
	ctx = looptracer.GetTracer().Inject(ctx)
	if e.typedOperators[evalTargetDO.EvalTargetType] == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
	}
	err = e.typedOperators[evalTargetDO.EvalTargetType].ValidateInput(ctx, spaceID, evalTargetDO.EvalTargetVersion.InputSchema, inputData)
	if err != nil {
		return nil, err
	}
	outputData, runStatus, err = e.typedOperators[evalTargetDO.EvalTargetType].Execute(ctx, spaceID, &entity.ExecuteEvalTargetParam{
		ExptID:              gptr.Indirect(param.ExperimentID),
		TargetID:            targetID,
		VersionID:           targetVersionID,
		SourceTargetID:      evalTargetDO.SourceTargetID,
		SourceTargetVersion: evalTargetDO.EvalTargetVersion.SourceTargetVersion,
		Input:               inputData,
		TargetType:          evalTargetDO.EvalTargetType,
		EvalTarget:          evalTargetDO,
	})
	if err != nil {
		return nil, err
	}

	if outputData == nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("[ExecuteTarget]outputData is nil"))
	}
	// setSpan
	spanParam.TargetType = evalTargetDO.EvalTargetType.String()
	spanParam.TargetID = strconv.FormatInt(targetID, 10)
	spanParam.TargetVersion = strconv.FormatInt(targetVersionID, 10)
	if outputData.EvalTargetRunError != nil {
		span.SetError(ctx, errors.New(outputData.EvalTargetRunError.Message))
	}
	setSpanInputOutput(ctx, spanParam, inputData, outputData)

	return record, nil
}

func (e *EvalTargetServiceImpl) ExtractTrajectory(ctx context.Context, spaceID int64, traceID string, startTimeMS *int64) (*entity.Trajectory, error) {
	if len(traceID) == 0 {
		return nil, errorx.New("ExtractTrajectory with null traceID")
	}
	trajectories, err := e.trajectoryAdapter.ListTrajectory(ctx, spaceID, []string{traceID}, startTimeMS)
	if err != nil {
		return nil, err
	}
	if len(trajectories) == 0 {
		return nil, nil
	}
	return trajectories[0], nil
}

func (e *EvalTargetServiceImpl) AsyncExecuteTarget(ctx context.Context, spaceID int64, targetID int64, targetVersionID int64,
	param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData,
) (record *entity.EvalTargetRecord, callee string, err error) {
	if inputData == nil || param == nil {
		return nil, "", errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("AsyncExecuteTarget with invalid param"))
	}

	evalTargetDO, err := e.GetEvalTargetVersion(ctx, spaceID, targetVersionID, false)
	if err != nil {
		return nil, "", err
	}

	return e.asyncExecuteTarget(ctx, spaceID, evalTargetDO, param, inputData)
}

func (e *EvalTargetServiceImpl) asyncExecuteTarget(ctx context.Context, spaceID int64, target *entity.EvalTarget, param *entity.ExecuteTargetCtx,
	inputData *entity.EvalTargetInputData,
) (record *entity.EvalTargetRecord, callee string, err error) {
	defer func(st time.Time) { e.metric.EmitRun(spaceID, err, st) }(time.Now()) // todo(@liushengyang): mtr
	defer goroutine.Recovery(ctx)

	targetID := target.ID
	targetVersionID := target.EvalTargetVersion.ID

	operator := e.typedOperators[target.EvalTargetType]
	if operator == nil {
		return nil, "", errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
	}

	if err := operator.ValidateInput(ctx, spaceID, target.EvalTargetVersion.InputSchema, inputData); err != nil {
		return nil, "", err
	}

	status := entity.EvalTargetRunStatusAsyncInvoking
	outputData := &entity.EvalTargetOutputData{
		OutputFields:    map[string]*entity.Content{},
		EvalTargetUsage: &entity.EvalTargetUsage{InputTokens: 0, OutputTokens: 0},
		TimeConsumingMS: gptr.Of(int64(0)),
	}

	invokeID, callee, execErr := operator.AsyncExecute(ctx, spaceID, &entity.ExecuteEvalTargetParam{
		TargetID:            targetID,
		VersionID:           targetVersionID,
		SourceTargetID:      target.SourceTargetID,
		SourceTargetVersion: target.EvalTargetVersion.SourceTargetVersion,
		Input:               inputData,
		TargetType:          target.EvalTargetType,
		EvalTarget:          target,
	})
	if execErr != nil {
		// If an asynchronous call fails, return immediately without logging the error or propagating the exception.
		// Avoid triggering a follow-up process via an asynchronous callback after a successful return.
		logs.CtxError(ctx, "async execute target failed, spaceID=%v, targetID=%d, targetVersionID=%d, param=%v, inputData=%v, err=%v",
			spaceID, targetID, targetVersionID, json.Jsonify(param), json.Jsonify(inputData), execErr)
		return nil, callee, execErr
	}

	logs.CtxInfo(ctx, "AsyncExecute with invoke_id %v, callee: %v, target_id: %v, target_version_id: %v", invokeID, callee, targetID, targetVersionID)

	userID := session.UserIDInCtxOrEmpty(ctx)
	record = &entity.EvalTargetRecord{
		ID:                   invokeID,
		SpaceID:              spaceID,
		TargetID:             targetID,
		TargetVersionID:      targetVersionID,
		ExperimentRunID:      gptr.Indirect(param.ExperimentRunID),
		ItemID:               param.ItemID,
		TurnID:               param.TurnID,
		LogID:                logs.GetLogID(ctx),
		EvalTargetInputData:  inputData,
		EvalTargetOutputData: outputData,
		Status:               gptr.Of(status),
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{
				UserID: gptr.Of(userID),
			},
			UpdatedBy: &entity.UserInfo{
				UserID: gptr.Of(userID),
			},
			CreatedAt: gptr.Of(time.Now().UnixMilli()),
			UpdatedAt: gptr.Of(time.Now().UnixMilli()),
		},
	}

	traceID, _ := e.emitTargetTrace(ctx, record, &entity.Session{UserID: userID})
	record.TraceID = traceID

	if _, err := e.evalTargetRepo.CreateEvalTargetRecord(ctx, record); err != nil {
		return nil, callee, err
	}

	return record, callee, nil
}

func (e *EvalTargetServiceImpl) DebugTarget(ctx context.Context, param *entity.DebugTargetParam) (record *entity.EvalTargetRecord, err error) {
	defer func(st time.Time) { e.metric.EmitRun(param.SpaceID, err, st) }(time.Now()) // todo(@liushengyang): mtr
	defer goroutine.Recovery(ctx)

	operator := e.typedOperators[param.PatchyTarget.EvalTargetType]
	if operator == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
	}

	if err := operator.ValidateInput(ctx, param.SpaceID, param.PatchyTarget.EvalTargetVersion.InputSchema, param.InputData); err != nil {
		return nil, err
	}

	outputData, status, execErr := operator.Execute(ctx, param.SpaceID, &entity.ExecuteEvalTargetParam{
		Input:      param.InputData,
		TargetType: param.PatchyTarget.EvalTargetType,
		EvalTarget: param.PatchyTarget,
	})
	if execErr != nil {
		logs.CtxError(ctx, "execute target failed, param=%v, err=%v", json.Jsonify(param), execErr)
		status = entity.EvalTargetRunStatusFail
		outputData = &entity.EvalTargetOutputData{
			OutputFields:       map[string]*entity.Content{},
			EvalTargetUsage:    &entity.EvalTargetUsage{},
			EvalTargetRunError: &entity.EvalTargetRunError{},
			TimeConsumingMS:    gptr.Of(int64(0)),
		}
		statusErr, ok := errorx.FromStatusError(execErr)
		if ok {
			outputData.EvalTargetRunError = &entity.EvalTargetRunError{
				Code:    statusErr.Code(),
				Message: errorx.ErrorWithoutStack(execErr),
			}
		} else {
			outputData.EvalTargetRunError = &entity.EvalTargetRunError{
				Code:    errno.CommonInternalErrorCode,
				Message: execErr.Error(),
			}
		}
	}

	userID := session.UserIDInCtxOrEmpty(ctx)
	recordID, err := e.idgen.GenID(ctx)
	if err != nil {
		return nil, err
	}

	record = &entity.EvalTargetRecord{
		ID:                   recordID,
		SpaceID:              param.SpaceID,
		LogID:                logs.GetLogID(ctx),
		EvalTargetInputData:  param.InputData,
		EvalTargetOutputData: outputData,
		Status:               gptr.Of(status),
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{
				UserID: gptr.Of(userID),
			},
			UpdatedBy: &entity.UserInfo{
				UserID: gptr.Of(userID),
			},
			CreatedAt: gptr.Of(time.Now().UnixMilli()),
			UpdatedAt: gptr.Of(time.Now().UnixMilli()),
		},
	}
	if _, err := e.evalTargetRepo.CreateEvalTargetRecord(ctx, record); err != nil {
		return nil, err
	}

	return record, nil
}

func (e *EvalTargetServiceImpl) AsyncDebugTarget(ctx context.Context, param *entity.DebugTargetParam) (record *entity.EvalTargetRecord, callee string, err error) {
	return e.asyncExecuteTarget(ctx, param.SpaceID, param.PatchyTarget, &entity.ExecuteTargetCtx{}, param.InputData)
}

func (e *EvalTargetServiceImpl) CreateRecord(ctx context.Context, record *entity.EvalTargetRecord) error {
	_, err := e.evalTargetRepo.CreateEvalTargetRecord(ctx, record)
	return err
}

func (e *EvalTargetServiceImpl) GetRecordByID(ctx context.Context, spaceID int64, recordID int64) (*entity.EvalTargetRecord, error) {
	return e.evalTargetRepo.GetEvalTargetRecordByIDAndSpaceID(ctx, spaceID, recordID)
}

func (e *EvalTargetServiceImpl) BatchGetRecordByIDs(ctx context.Context, spaceID int64, recordIDs []int64) ([]*entity.EvalTargetRecord, error) {
	if spaceID == 0 || len(recordIDs) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}

	return e.evalTargetRepo.ListEvalTargetRecordByIDsAndSpaceID(ctx, spaceID, recordIDs)
}

func (e *EvalTargetServiceImpl) ReportInvokeRecords(ctx context.Context, param *entity.ReportTargetRecordParam) error {
	record, err := e.evalTargetRepo.GetEvalTargetRecordByIDAndSpaceID(ctx, param.SpaceID, param.RecordID)
	if err != nil {
		return err
	}

	if record == nil {
		return errorx.NewByCode(errno.CommonBadRequestCode, errorx.WithExtraMsg(fmt.Sprintf("target record not found %d, space_id %d", param.RecordID, param.SpaceID)))
	}

	if status := gptr.Indirect(record.Status); status != entity.EvalTargetRunStatusAsyncInvoking {
		return errorx.NewByCode(errno.CommonBadRequestCode, errorx.WithExtraMsg(fmt.Sprintf("unexpected target result status %d", status)))
	}

	record.EvalTargetOutputData = param.OutputData
	record.Status = gptr.Of(param.Status)
	if err := e.evalTargetRepo.SaveEvalTargetRecord(ctx, record); err != nil {
		return err
	}

	// traceID, err := e.emitTargetTrace(logs.SetLogID(ctx, record.LogID), record, param.Session)
	// if err != nil {
	//	logs.CtxError(ctx, "emitTargetTrace fail, target_id: %v, target_version_id: %v, record_id: %v, err: %v",
	//		record.TargetID, record.TargetVersionID, record.ID, err)
	// }

	recordTrajectory := func() error {
		trajectory, err := e.ExtractTrajectory(ctx, param.SpaceID, record.TraceID, nil)
		if err != nil {
			return errorx.Wrapf(err, "ExtractTrajectory fail, space_id: %v, trace_id: %v", param.SpaceID, record.TraceID)
		}
		od, ok := deepcopy.Copy(param.OutputData).(*entity.EvalTargetOutputData)
		if !ok {
			return errorx.New("EvalTargetOutputData deepcopy fail")
		}
		if od == nil {
			od = &entity.EvalTargetOutputData{}
		}
		if od.OutputFields == nil {
			od.OutputFields = map[string]*entity.Content{}
		}
		od.OutputFields[consts.EvalTargetOutputFieldKeyTrajectory] = trajectory.ToContent(ctx)
		return e.evalTargetRepo.UpdateEvalTargetRecord(ctx, &entity.EvalTargetRecord{
			ID:                   record.ID,
			TraceID:              record.TraceID,
			EvalTargetOutputData: od,
		})
	}

	goroutine.Go(ctx, func() {
		time.Sleep(e.configer.GetTargetTrajectoryConf(ctx).GetExtractInterval(param.SpaceID))
		if err := recordTrajectory(); err != nil {
			logs.CtxError(ctx, "extract and record trajectory fail, record_id: %v, err: %v", record.ID, err)
		}
	})

	return nil
}

func (e *EvalTargetServiceImpl) emitTargetTrace(ctx context.Context, record *entity.EvalTargetRecord, session *entity.Session) (string, error) {
	if record.EvalTargetOutputData == nil {
		logs.CtxInfo(ctx, "emitTargetTrace with null data")
		return "", nil
	}

	ctx, span := looptracer.GetTracer().StartSpan(ctx, "EvalTarget", "eval_target", looptracer.WithStartNewTrace(), looptracer.WithSpanWorkspaceID(strconv.FormatInt(record.SpaceID, 10)))
	span.SetCallType("EvalTarget")
	ctx = looptracer.GetTracer().Inject(ctx)

	spanParam := &targetSpanTagsParams{
		Error:         nil,
		ErrCode:       "",
		CallType:      "eval_target",
		TargetID:      strconv.FormatInt(record.TargetID, 10),
		TargetVersion: strconv.FormatInt(record.TargetVersionID, 10),
	}
	setSpanInputOutput(ctx, spanParam, record.EvalTargetInputData, record.EvalTargetOutputData)

	if record.TargetVersionID > 0 {
		evalTargetDO, err := e.GetEvalTargetVersion(ctx, record.SpaceID, record.TargetVersionID, false)
		if err != nil {
			return "", err
		}
		spanParam.TargetType = evalTargetDO.EvalTargetType.String()
	}

	if record.EvalTargetOutputData.EvalTargetRunError != nil {
		span.SetError(ctx, fmt.Errorf("code: %v, msg: %v", record.EvalTargetOutputData.EvalTargetRunError.Code, record.EvalTargetOutputData.EvalTargetRunError.Message))
	}
	span.SetInput(ctx, Convert2TraceString(spanParam.Inputs))
	span.SetOutput(ctx, Convert2TraceString(spanParam.Outputs))
	span.SetInputTokens(ctx, int(spanParam.InputToken))
	span.SetOutputTokens(ctx, int(spanParam.OutputToken))
	span.SetUserID(ctx, session.UserID)
	span.SetTags(ctx, map[string]any{
		"eval_target_type":    spanParam.TargetType,
		"eval_target_id":      spanParam.TargetID,
		"eval_target_version": spanParam.TargetVersion,
	})
	span.Finish(ctx)

	return span.GetTraceID(), nil
}

func (e *EvalTargetServiceImpl) ValidateRuntimeParam(ctx context.Context, targetType entity.EvalTargetType, runtimeParam string) error {
	if len(runtimeParam) == 0 {
		return nil
	}

	so, err := e.sourceTargetOperator(targetType)
	if err != nil {
		return err
	}

	_, err = so.RuntimeParam().ParseFromJSON(runtimeParam)
	return err
}

func (e *EvalTargetServiceImpl) sourceTargetOperator(targetType entity.EvalTargetType) (ISourceEvalTargetOperateService, error) {
	o, ok := e.typedOperators[targetType]
	if !ok || o == nil {
		return nil, errorx.New("target %v operator not found", targetType)
	}
	return o, nil
}

func setSpanInputOutput(ctx context.Context, spanParam *targetSpanTagsParams, inputData *entity.EvalTargetInputData, outputData *entity.EvalTargetOutputData) {
	if inputData != nil {
		spanParam.Inputs = map[string][]*tracespec.ModelMessagePart{}
		for key, content := range inputData.InputFields {
			spanParam.Inputs[key] = toTraceParts(ctx, content)
		}
	}
	if outputData != nil {
		spanParam.Outputs = map[string][]*tracespec.ModelMessagePart{}
		for key, content := range outputData.OutputFields {
			spanParam.Outputs[key] = toTraceParts(ctx, content)
		}
		if outputData.EvalTargetUsage != nil {
			spanParam.InputToken = outputData.EvalTargetUsage.InputTokens
			spanParam.OutputToken = outputData.EvalTargetUsage.OutputTokens
		}
	}
}

func toTraceParts(ctx context.Context, content *entity.Content) []*tracespec.ModelMessagePart {
	switch content.GetContentType() {
	case entity.ContentTypeText:
		return []*tracespec.ModelMessagePart{{
			Text: content.GetText(),
			Type: tracespec.ModelMessagePartType(content.GetContentType()),
		}}
	case entity.ContentTypeImage:
		return []*tracespec.ModelMessagePart{{
			ImageURL: &tracespec.ModelImageURL{
				Name: gptr.Indirect(content.Image.Name),
				URL:  gptr.Indirect(content.Image.URL),
			},
			Type: tracespec.ModelMessagePartType(content.GetContentType()),
		}}
	case entity.ContentTypeMultipart:
		parts := make([]*tracespec.ModelMessagePart, 0, len(content.MultiPart))
		for _, sub := range content.MultiPart {
			parts = append(parts, toTraceParts(ctx, sub)...)
		}
		return parts
	default:
		logs.CtxInfo(ctx, "toTraceParts with unsupported content type %s", content.GetContentType())
		return []*tracespec.ModelMessagePart{{
			Text: content.GetText(),
			Type: tracespec.ModelMessagePartType(content.GetContentType()),
		}}
	}
}

type targetSpanTagsParams struct {
	Inputs  map[string][]*tracespec.ModelMessagePart
	Outputs map[string][]*tracespec.ModelMessagePart
	Error   error
	ErrCode string

	CallType      string
	TargetType    string
	TargetID      string
	TargetVersion string
	InputToken    int64
	OutputToken   int64
}

func Convert2TraceString(input any) string {
	if input == nil {
		return ""
	}
	str, err := sonic.MarshalString(input)
	if err != nil {
		return ""
	}

	return str
}

// GenerateMockOutputData generates mock data according to output schema
func (e *EvalTargetServiceImpl) GenerateMockOutputData(outputSchemas []*entity.ArgsSchema) (map[string]string, error) {
	if len(outputSchemas) == 0 {
		return map[string]string{}, nil
	}

	result := make(map[string]string)

	for _, schema := range outputSchemas {
		if schema.Key != nil && schema.JsonSchema != nil {
			// Use jsonmock to generate independent mock data for each schema
			mockData, err := jsonmock.GenerateMockData(*schema.JsonSchema)
			if err != nil {
				// If generation fails, use default value
				result[*schema.Key] = "{}"
			} else {
				result[*schema.Key] = mockData
			}
		}
	}

	return result, nil
}

// buildPageByCursor some interfaces do not have rolling pagination, need to adapt with page manually
func buildPageByCursor(cursor *string) (page int32, err error) {
	if cursor == nil {
		page = 1
	} else {
		pageParse, err := strconv.ParseInt(gptr.Indirect(cursor), 10, 32)
		if err != nil {
			return 0, err
		}
		page = int32(pageParse)
	}
	return page, nil
}

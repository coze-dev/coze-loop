// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/sonic"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
	"github.com/mohae/deepcopy"

	"github.com/coze-dev/coze-loop/backend/infra/backoff"
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

	// SandboxAgent 评测对象单行执行完成后用于销毁沙箱执行
	sandboxSchedulerAdapter rpc.ISandboxSchedulerAdapter
	exptRunLogRepo          repo.IExptRunLogRepo

	// sandboxDestroyRetryBudget 是 destroySandboxExecute 重试的总时间预算；<=0 时取
	// defaultSandboxDestroyRetryBudget。做成字段而非常量纯为可测：一条"Destroy 恒失败"的用例
	// 若按线上预算跑就要真等十秒，压到毫秒级才能把"重试到用尽"确定性地断言完。
	// 生产路径（wire 注入）从不设置它，走默认值。
	sandboxDestroyRetryBudget time.Duration
}

const evalTargetRecordPersistTimeout = 5 * time.Second

// defaultSandboxDestroyRetryBudget 是 destroySandboxExecute 重试的默认总时间预算。
// 十秒的取舍见 destroySandboxExecute 注释「用 RetryTenSeconds 而非更长」。
const defaultSandboxDestroyRetryBudget = 10 * time.Second

// trajectoryStartTimeBufferMS 抽取 trajectory 时，时间下界额外向前预留的 buffer(1 分钟)，
// 用于吸收请求发起时间与实际 span 上报时间之间可能的时钟/延迟误差，避免漏掉最早的 span。
const trajectoryStartTimeBufferMS = int64(60 * 1000)

func NewEvalTargetServiceImpl(evalTargetRepo repo.IEvalTargetRepo,
	idgen idgen.IIDGenerator,
	metric metrics.EvalTargetMetrics,
	typedOperators map[entity.EvalTargetType]ISourceEvalTargetOperateService,
	trajectoryAdapter rpc.ITrajectoryAdapter,
	configer component.IConfiger,
	sandboxSchedulerAdapter rpc.ISandboxSchedulerAdapter,
	exptRunLogRepo repo.IExptRunLogRepo,
) IEvalTargetService {
	singletonEvalTargetService := &EvalTargetServiceImpl{
		evalTargetRepo:          evalTargetRepo,
		idgen:                   idgen,
		metric:                  metric,
		typedOperators:          typedOperators,
		trajectoryAdapter:       trajectoryAdapter,
		configer:                configer,
		sandboxSchedulerAdapter: sandboxSchedulerAdapter,
		exptRunLogRepo:          exptRunLogRepo,
	}
	return singletonEvalTargetService
}

func (e *EvalTargetServiceImpl) CreateEvalTarget(ctx context.Context, spaceID int64, sourceTargetID, sourceTargetVersion string, targetType entity.EvalTargetType, opts ...entity.Option) (id, versionID int64, err error) {
	defer func() {
		e.metric.EmitCreate(spaceID, err)
	}()

	srcID := strings.TrimSpace(sourceTargetID)
	srcVer := strings.TrimSpace(sourceTargetVersion)
	// 仅记录型（*Online）：无业务 source 时落库占位对象。版本为空或与在线实验默认占位版本一致时均走此路径（避免 CreateExpt 注入 0.0.1 后误走 BuildBySource）
	recordOnlyPlaceholder := targetType.IsRecordOnlyType() && srcID == "" &&
		(srcVer == "" || srcVer == consts.DefaultSourceTargetVersion)
	if recordOnlyPlaceholder {
		do := e.newRecordOnlyEvalTargetWithoutSource(ctx, spaceID, targetType)
		if do == nil {
			return 0, 0, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
		}
		return e.evalTargetRepo.CreateEvalTarget(ctx, do)
	}

	// 仅记录型复用对应基础类型的 operator 构建，再覆盖为记录型
	buildType := targetType
	if baseType, ok := targetType.RecordOnlyTypeToBaseType(); ok {
		if e.typedOperators[baseType] == nil {
			return 0, 0, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
		}
		buildType = baseType
	}
	if e.typedOperators[buildType] == nil {
		return 0, 0, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
	}
	do, err := e.typedOperators[buildType].BuildBySource(ctx, spaceID, sourceTargetID, sourceTargetVersion, opts...)
	if err != nil {
		return 0, 0, err
	}

	if do == nil {
		return 0, 0, errorx.NewByCode(errno.CommonInvalidParamCode)
	}

	// 仅记录型：覆盖为请求的 targetType，保证存储的是 *Online 类型
	if buildType != targetType {
		do.EvalTargetType = targetType
		if do.EvalTargetVersion != nil {
			do.EvalTargetVersion.EvalTargetType = targetType
		}
	}

	return e.evalTargetRepo.CreateEvalTarget(ctx, do)
}

// newRecordOnlyEvalTargetWithoutSource 构建无业务 source 的仅记录型评测对象（与 CreateExperiment 在线场景默认版本对齐）
func (e *EvalTargetServiceImpl) newRecordOnlyEvalTargetWithoutSource(ctx context.Context, spaceID int64, targetType entity.EvalTargetType) *entity.EvalTarget {
	if !targetType.IsRecordOnlyType() {
		return nil
	}
	userID := session.UserIDInCtxOrEmpty(ctx)
	now := time.Now().UnixMilli()
	return &entity.EvalTarget{
		SpaceID:        spaceID,
		SourceTargetID: "",
		EvalTargetType: targetType,
		EvalTargetVersion: &entity.EvalTargetVersion{
			SpaceID:             spaceID,
			SourceTargetVersion: consts.DefaultSourceTargetVersion,
			EvalTargetType:      targetType,
			BaseInfo: &entity.BaseInfo{
				CreatedBy: &entity.UserInfo{UserID: gptr.Of(userID)},
				UpdatedBy: &entity.UserInfo{UserID: gptr.Of(userID)},
				CreatedAt: gptr.Of(now),
				UpdatedAt: gptr.Of(now),
			},
		},
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{UserID: gptr.Of(userID)},
			UpdatedBy: &entity.UserInfo{UserID: gptr.Of(userID)},
			CreatedAt: gptr.Of(now),
			UpdatedAt: gptr.Of(now),
		},
	}
}

func (e *EvalTargetServiceImpl) GetEvalTarget(ctx context.Context, targetID int64) (do *entity.EvalTarget, err error) {
	return e.evalTargetRepo.GetEvalTarget(ctx, targetID)
}

func (e *EvalTargetServiceImpl) GetEvalTargetVersion(ctx context.Context, spaceID, versionID int64, needSourceInfo bool) (do *entity.EvalTarget, err error) {
	do, err = e.evalTargetRepo.GetEvalTargetVersion(ctx, spaceID, versionID)
	if err != nil {
		return nil, err
	}
	if do == nil {
		return nil, nil
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
	e.fillSharedInfo(spaceID, do, entity.BuildSharedResourceInfo(spaceID, do.SpaceID, consts.Read, "versioned"))
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

func (e *EvalTargetServiceImpl) fillSharedInfo(consumerSpaceID int64, target *entity.EvalTarget, sharedInfo *entity.SharedResourceInfo) {
	if target == nil {
		return
	}
	target.SharedInfo = sharedInfo
	if target.EvalTargetVersion != nil {
		target.EvalTargetVersion.SharedInfo = sharedInfo
	}
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
	for _, version := range versions {
		e.fillSharedInfo(spaceID, version, entity.BuildSharedResourceInfo(spaceID, version.SpaceID, consts.Read, "versioned"))
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
					Message: errorx.ErrorWithoutStack(err),
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

		if execErr == nil && evalTargetDO.EvalTargetType.SupptTrajectory() && (param.EnableExtractTrajectory == nil || *param.EnableExtractTrajectory) {
			time.Sleep(e.configer.GetTargetTrajectoryConf(ctx).GetExtractInterval(spaceID))
			trajectory, err := e.ExtractTrajectory(ctx, spaceID, span.GetTraceID(), gptr.Of(startTime.UnixMilli()))
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

		recordCtx, recordCancel := context.WithTimeout(context.WithoutCancel(ctx), evalTargetRecordPersistTimeout)
		defer recordCancel()

		recordID, err1 := e.idgen.GenID(recordCtx)
		if err1 != nil {
			err = err1
			return
		}
		logID := logs.GetLogID(recordCtx)

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
			Ext:                  inputData.GetExt(),
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
		e.convEvalTargetRunErr(recordCtx, record)

		_, errCreate := e.evalTargetRepo.CreateEvalTargetRecord(recordCtx, record, nil)
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
		EvalSetItemID:       gptr.Of(param.ItemID),
		EvalSetTurnID:       gptr.Of(param.TurnID),
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
	// 时间下界默认向前多减 1 分钟 buffer:防御请求发起时间与实际 span 时间之间可能存在的时钟/上报误差,
	// 避免因下界略偏晚而漏掉最早的 span 导致轨迹拉不全。trace 查询按 traceID 精确匹配,放宽下界不会引入无关数据。
	if startTimeMS != nil {
		startTimeMS = gptr.Of(*startTimeMS - trajectoryStartTimeBufferMS)
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

func (e *EvalTargetServiceImpl) AsyncExecuteTarget(ctx context.Context, spaceID, targetID, targetVersionID int64,
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

	ctx, span := looptracer.GetTracer().StartSpan(ctx, "EvalTarget", "eval_target", looptracer.WithStartNewTrace(), looptracer.WithSpanWorkspaceID(strconv.FormatInt(spaceID, 10)))
	span.SetCallType("EvalTarget")
	ctx = looptracer.GetTracer().Inject(ctx)

	invokeID, callee, ext, execErr := operator.AsyncExecute(ctx, spaceID, &entity.ExecuteEvalTargetParam{
		ExptID:              gptr.Indirect(param.ExperimentID),
		TargetID:            targetID,
		VersionID:           targetVersionID,
		SourceTargetID:      target.SourceTargetID,
		SourceTargetVersion: target.EvalTargetVersion.SourceTargetVersion,
		Input:               inputData,
		TargetType:          target.EvalTargetType,
		EvalTarget:          target,
		EvalSetItemID:       gptr.Of(param.ItemID),
		EvalSetTurnID:       gptr.Of(param.TurnID),
		LogID:               param.LogID,
		ItemMeta:            param.ItemMeta,
		ExptGroupKey:        param.ExptGroupKey,
	})
	if execErr != nil {
		// If an asynchronous call fails, return immediately without logging the error or propagating the exception.
		// Avoid triggering a follow-up process via an asynchronous callback after a successful return.
		logs.CtxError(ctx, "async execute target failed, spaceID=%v, targetID=%d, targetVersionID=%d, param=%v, inputData=%v, err=%v",
			spaceID, targetID, targetVersionID, json.Jsonify(param), json.Jsonify(inputData), execErr)
		return nil, callee, execErr
	}

	logs.CtxInfo(ctx, "AsyncExecute with invoke_id %v, callee: %v, target_id: %v, target_version_id: %v", invokeID, callee, targetID, targetVersionID)
	outputData.Ext = ext
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
		Ext:                  inputData.GetExt(),
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

	traceID, _ := e.emitTargetTrace(ctx, span, record, &entity.Session{UserID: userID})
	record.TraceID = traceID

	// 仅 DebugTarget 传入 TruncateLargeContent，其他场景 nil 默认剪裁
	truncateLargeContent := param.TruncateLargeContent
	if _, err := e.evalTargetRepo.CreateEvalTargetRecord(ctx, record, truncateLargeContent); err != nil {
		// ⚠️ 这里是**唯一的清理机会**：此刻 operator 已成功返回，沙箱确定在跑，ext 也已算好
		// 落在 outputData.Ext 里；但 record 没落库，而平台侧三条兜底销毁链
		// (destroySandboxExecuteIfNeeded / TerminateAsyncRecordsAndDestroySandbox /
		// CheckSandboxTerminated) **全部以"按 recordID 从库里查出 record 再读 ext"为前提** ——
		// 库里没这行，一条都命中不到。就这么 return 掉，沙箱只能等平台侧 patrol，
		// 期间并发名额一直不归还 (配额耗尽后新 item 直接撞 601300702 且该错误不重试)。
		//
		// 用内存里已有的 record 直接销毁：sandboxExecuteIDsOf 只读 record.EvalTargetOutputData.Ext
		// 与 record.ID，是纯内存操作、不查库，所以落库失败也拿得到正确的 execute id。
		// taskID 同理不走 resolveSandboxTaskIDByRunID (它查 expt_run_log，多一跳且可能返回 "")
		// —— 此刻手上直接有 param.ExperimentID，operator 侧的 taskID 也正是它。
		//
		// 类型守卫用手上的 target 直接判，比 destroySandboxExecuteIfNeeded 那种"查库反查
		// version"更直接可靠 (destroySandboxExecute 自己不检查 target 类型，漏判会把非沙箱
		// 场景也发一次 Destroy)。
		if target.EvalTargetType == entity.EvalTargetTypeSandboxAgent {
			executeIDs := sandboxExecuteIDsOf(ctx, record)
			logs.CtxError(ctx, "[SandboxDestroy] create eval target record fail, destroying sandbox executes to avoid leak, invoke_id=%d, expt_id=%d, execute_ids=%v, err=%v",
				record.ID, gptr.Indirect(param.ExperimentID), executeIDs, err)
			e.destroySandboxExecute(ctx, strconv.FormatInt(gptr.Indirect(param.ExperimentID), 10), spaceID, executeIDs, false)
		}
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
	e.convEvalTargetRunErr(ctx, record)

	if _, err := e.evalTargetRepo.CreateEvalTargetRecord(ctx, record, param.TruncateLargeContent); err != nil {
		return nil, err
	}

	return record, nil
}

func (e *EvalTargetServiceImpl) convEvalTargetRunErr(ctx context.Context, record *entity.EvalTargetRecord) {
	if record == nil || record.EvalTargetOutputData == nil || record.EvalTargetOutputData.EvalTargetRunError == nil {
		return
	}
	if record.EvalTargetOutputData.EvalTargetRunError.Code == int32(errno.CustomEvalTargetInvokeFailCode) {
		return
	}
	if len(record.EvalTargetOutputData.EvalTargetRunError.Message) > 0 {
		record.EvalTargetOutputData.EvalTargetRunError.Message = e.configer.GetErrCtrl(ctx).ConvertErrMsg(record.EvalTargetOutputData.EvalTargetRunError.Message)
	}
}

func (e *EvalTargetServiceImpl) AsyncDebugTarget(ctx context.Context, param *entity.DebugTargetParam) (record *entity.EvalTargetRecord, callee string, err error) {
	return e.asyncExecuteTarget(ctx, param.SpaceID, param.PatchyTarget, &entity.ExecuteTargetCtx{TruncateLargeContent: param.TruncateLargeContent}, param.InputData)
}

func (e *EvalTargetServiceImpl) CreateRecord(ctx context.Context, record *entity.EvalTargetRecord) error {
	_, err := e.evalTargetRepo.CreateEvalTargetRecord(ctx, record, nil)
	return err
}

func (e *EvalTargetServiceImpl) GetRecordByID(ctx context.Context, spaceID, recordID int64) (*entity.EvalTargetRecord, error) {
	return e.evalTargetRepo.GetEvalTargetRecordByIDAndSpaceID(ctx, spaceID, recordID)
}

func (e *EvalTargetServiceImpl) GetRecordByRunItemTurn(ctx context.Context, spaceID, runID, itemID, turnID int64) (*entity.EvalTargetRecord, error) {
	return e.evalTargetRepo.GetEvalTargetRecordByRunItemTurn(ctx, spaceID, runID, itemID, turnID)
}

func (e *EvalTargetServiceImpl) BatchGetRecordByIDs(ctx context.Context, spaceID int64, recordIDs []int64) ([]*entity.EvalTargetRecord, error) {
	if spaceID == 0 || len(recordIDs) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}

	return e.evalTargetRepo.ListEvalTargetRecordByIDsAndSpaceID(ctx, spaceID, recordIDs)
}

func (e *EvalTargetServiceImpl) LoadRecordOutputFields(ctx context.Context, record *entity.EvalTargetRecord, fieldKeys []string) error {
	if record == nil || len(fieldKeys) == 0 {
		return nil
	}
	return e.evalTargetRepo.LoadEvalTargetRecordOutputFields(ctx, record, fieldKeys)
}

func (e *EvalTargetServiceImpl) LoadRecordFullData(ctx context.Context, record *entity.EvalTargetRecord) error {
	if record == nil {
		return nil
	}
	return e.evalTargetRepo.LoadEvalTargetRecordFullData(ctx, record)
}

// destroySandboxExecuteIfNeeded 在 SandboxAgent 评测对象单行执行完成后销毁该次沙箱执行。
// 仅做 best-effort：任何步骤失败仅记录日志，不阻断上层调用。
// 若 record.EvalTargetOutputData.Ext 里带 SandboxAgentExtKeyExtraExecuteID（双沙箱模式的从沙箱），
// 会追加销毁一次；避免从沙箱只能等 session TTL 到期后被 patrol 兼底回收。
func (e *EvalTargetServiceImpl) destroySandboxExecuteIfNeeded(ctx context.Context, record *entity.EvalTargetRecord) {
	if e.sandboxSchedulerAdapter == nil || record == nil {
		// 静默跳过等于沙箱泄漏 + 并发名额不归还，必须留痕（adapter 未接线是配置问题，不该沉默）。
		logs.CtxWarn(ctx, "[SandboxDestroy] skip: adapter_nil=%t record_nil=%t", e.sandboxSchedulerAdapter == nil, record == nil)
		return
	}
	// 仅 SandboxAgent 评测对象需要销毁沙箱执行
	targetVersion, err := e.evalTargetRepo.GetEvalTargetVersion(ctx, record.SpaceID, record.TargetVersionID)
	if err != nil {
		logs.CtxWarn(ctx, "[SandboxDestroy] get eval target version fail, space_id=%d, version_id=%d, err=%v",
			record.SpaceID, record.TargetVersionID, err)
		return
	}
	if targetVersion == nil || targetVersion.EvalTargetType != entity.EvalTargetTypeSandboxAgent {
		// 类型不匹配就不销 —— 但双沙箱 item 若因 version 查不到/类型读错落到这里，沙箱同样泄漏，
		// 而原来这条路径完全无日志，排查时无法区分「不该销」和「该销却没销」。
		gotType := entity.EvalTargetType(-1)
		if targetVersion != nil {
			gotType = targetVersion.EvalTargetType
		}
		logs.CtxWarn(ctx, "[SandboxDestroy] skip: not a SandboxAgent target, record_id=%d, space_id=%d, version_id=%d, version_nil=%t, got_type=%d",
			record.ID, record.SpaceID, record.TargetVersionID, targetVersion == nil, gotType)
		return
	}

	taskID := e.resolveSandboxTaskIDByRunID(ctx, record.ExperimentRunID)
	executeIDs := sandboxExecuteIDsOf(ctx, record)
	logs.CtxInfo(ctx, "[SandboxDestroy] destroying sandbox executes, record_id=%d, task_id=%s, expt_run_id=%d, execute_ids=%v",
		record.ID, taskID, record.ExperimentRunID, executeIDs)
	e.destroySandboxExecute(ctx, taskID, record.SpaceID, executeIDs, false)
}

// extractExtraSandboxExecuteID 从 record 的 outputData.Ext 里读取额外沙箱 execute id；
// 缺失/空值返回 ""。
//
// 与 sandboxExecuteIDsOf 的分工: 销毁走 sandboxExecuteIDsOf (取两个 key 的并集, 是本函数的超集);
// 本函数只在需要"单个 extra id 原值"时用 —— 例如把它重新写回 fail record 的 ext 供事后审计。
func extractExtraSandboxExecuteID(record *entity.EvalTargetRecord) string {
	if record == nil || record.EvalTargetOutputData == nil {
		return ""
	}
	return record.EvalTargetOutputData.Ext[entity.SandboxAgentExtKeyExtraExecuteID]
}

// sandboxExecuteIDsOf 取该 record 本次调用实际创建的 sandbox execution id 列表。
//
// 双沙箱一次调用会创建多个 execution, 其 id 命名规则属 operator 实现细节, 平台侧不推断 ——
// 只读 operator 在 AsyncExecute 回传、落在 output ext 里的值。
//
// ⚠️ **必须同时认两个 key**: 双沙箱目前有两套 operator 实现并存 (见 SandboxCountMode 分流),
// 各自写自己的 key ——
//
//	consts.OutputDataExtKeySandboxExecuteIDs   (JSON 字符串数组, 全部 execution)
//	entity.SandboxAgentExtKeyExtraExecuteID    (裸字符串, 仅"额外"那个; 主 execution = record.ID)
//
// 只认一个的后果是**另一条链路静默漏沙箱**: 代码编译通过、自己的测试也过, 但双沙箱每 item 占
// 2 个并发名额, 泄漏累积到配额上限后新 item 直接失败 (601300702), 且该错误不重试。
// 两个 key 都读、取并集, 是这两套实现合并期间唯一安全的形状。
//
// 都缺省时退回 record.ID (单沙箱实现的 executeID 即 record.ID/invokeID)。
func sandboxExecuteIDsOf(ctx context.Context, record *entity.EvalTargetRecord) []string {
	fallback := []string{strconv.FormatInt(record.ID, 10)}
	if record.EvalTargetOutputData == nil || len(record.EvalTargetOutputData.Ext) == 0 {
		return fallback
	}
	ext := record.EvalTargetOutputData.Ext

	seen := make(map[string]bool)
	out := make([]string, 0, 2)
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}

	// 形态一: 完整列表 (我们这条链路)。
	if raw := ext[consts.OutputDataExtKeySandboxExecuteIDs]; raw != "" {
		var ids []string
		if err := json.Unmarshal([]byte(raw), &ids); err != nil {
			logs.CtxWarn(ctx, "[SandboxDestroy] unmarshal sandbox execute ids fail, record_id=%d, raw=%s, err=%v",
				record.ID, raw, err)
		} else {
			for _, id := range ids {
				add(id)
			}
		}
	}

	// 形态二: 主 execution (= record.ID) + 一个额外 id (main 那条链路)。
	// 只有该 key 存在时才补主 id —— 否则会给形态一的结果凭空多加一个。
	if extra := ext[entity.SandboxAgentExtKeyExtraExecuteID]; extra != "" {
		add(strconv.FormatInt(record.ID, 10))
		add(extra)
	}

	if len(out) == 0 {
		return fallback
	}
	return out
}

// resolveSandboxTaskIDByRunID 通过 ExperimentRunID 反查 ExptID 作为 sandbox TaskID。
func (e *EvalTargetServiceImpl) resolveSandboxTaskIDByRunID(ctx context.Context, experimentRunID int64) string {
	if e.exptRunLogRepo == nil || experimentRunID <= 0 {
		return ""
	}
	runLog, err := e.exptRunLogRepo.Get(ctx, 0, experimentRunID)
	if err != nil {
		logs.CtxWarn(ctx, "[SandboxDestroy] get expt_run_log fail, expt_run_id=%d, err=%v", experimentRunID, err)
		return ""
	}
	if runLog == nil || runLog.ExptID <= 0 {
		return ""
	}
	return strconv.FormatInt(runLog.ExptID, 10)
}

// destroySandboxExecute 异步 best-effort 销毁本次调用创建的 sandbox execute（可能多个）。
//
// 列表形态而非单个 int64: 双沙箱一次调用创建多个 execution, 且 id 由 operator 命名 (可能不是
// record.ID 那样的纯数字), 所以这里只接受 operator 给出的字符串 id 列表, 不做格式假定。
// zombieTimeout=true 时透传给下游适配器，由适配器决定是否附带 SandboxAgent 收尾命令。
//
// # 为什么要重试
//
// 这条路径是**正常完成时的主回收口** (ReportInvokeRecords → destroySandboxExecuteIfNeeded),
// 但它此前是"一次 RPC + 失败只 warn": 一次网络抖动 (`remote or network error[remote]`) 就等于
// 沙箱确定性泄漏, 而且**没有任何后续机制会重试** —— record 已被写成终态, 后续 tick 的
// zombie/sweep 都以 `Status == AsyncInvoking` 为前提, 命中不到它。
//
// Destroy 天然幂等: 服务端对不存在/已终态的 executeID 只是不计入 affected_count, 不报错,
// 所以重复销毁无副作用。这与 Run 路径的顾虑正好相反 (那边重试可能起第二个进程, 必须窄判),
// 这里重试的唯一代价是多一次 RPC, 而不重试的代价是沙箱一直挂着 + 并发名额永不归还
// (后续 item 全撞 601300702 且该错误不重试)。
//
// 用 RetryTenSeconds 而非更长: 本函数已在独立 goroutine 里, 拖长不阻塞调用方; 但沙箱侧
// 真的不可用时再久也没用, 十秒足够吸收瞬时抖动。
//
// # 为什么重试要挂在 WithoutCancel 的 ctx 上
//
// 调用方 (ReportInvokeRecords) 是**请求态**: 它一 return, Hertz/Kitex 就取消请求 ctx。
// 而本函数刻意异步 (goroutine.Go), 重试窗口 (十秒) 远长于调用方剩余寿命 —— 若直接用入参 ctx,
// cenk/backoff 的 backOffContext.NextBackOff 见 ctx.Done() 立即返回 Stop, 于是**第一次
// Destroy 失败后一次都不会重试**, 整个重试形同虚设 (恰是本函数要修的那个泄漏)。
// 与 ReportInvokeRecords 里落 record 的 persistCtx 同款处置: 剥离取消信号, 保留 logID 等值。
func (e *EvalTargetServiceImpl) destroySandboxExecute(ctx context.Context, taskID string, spaceID int64, executeIDs []string, zombieTimeout bool) {
	if e.sandboxSchedulerAdapter == nil || len(executeIDs) == 0 {
		return
	}
	// 先剥离取消信号再交给 goroutine: 见上"为什么重试要挂在 WithoutCancel 的 ctx 上"。
	destroyCtx := context.WithoutCancel(ctx)
	goroutine.Go(destroyCtx, func() {
		e.destroySandboxExecuteSync(destroyCtx, taskID, spaceID, executeIDs, zombieTimeout)
	})
}

// destroySandboxExecuteSync 是 destroySandboxExecute 的同步本体（含重试与终局告警）。
//
// 拆出来只为可测: 重试逻辑若只能经 goroutine 触发, 测试就必须靠 channel/sleep 去等一个
// **没有 join 点**的后台 goroutine —— 那正是本次 -race 失败的成因 (等到了第一次 Destroy 就
// 返回, 后续重试落在测试结束之后, 撞 gomock 在已完成的 *testing.T 上 Fatalf → panic
// "Fail in goroutine after ... has completed")。同步入口让"重试到用尽"能被确定性地断言完,
// 不留悬空 goroutine。
func (e *EvalTargetServiceImpl) destroySandboxExecuteSync(ctx context.Context, taskID string, spaceID int64, executeIDs []string, zombieTimeout bool) {
	budget := e.sandboxDestroyRetryBudget
	if budget <= 0 {
		budget = defaultSandboxDestroyRetryBudget
	}
	if err := backoff.RetryWithElapsedTime(ctx, budget, func() error {
		_, derr := e.sandboxSchedulerAdapter.Destroy(ctx, &rpc.SandboxDestroyRequest{
			TaskID:        taskID,
			DestroyType:   rpc.SandboxDestroyTypeExecute,
			ExecuteIDs:    executeIDs,
			WorkspaceID:   spaceID,
			ZombieTimeout: zombieTimeout,
		})
		return derr
	}); err != nil {
		// 重试用尽仍失败 = 沙箱确定性泄漏 + 并发名额不归还, 且没有下一次机会 (见上)。
		// 故打 Error 而非 Warn: 它需要人介入, 不是"可能有问题"。
		logs.CtxError(ctx, "[SandboxDestroy] destroy sandbox execute fail after retries, sandboxes may leak, task_id=%s, execute_ids=%v, err=%v",
			taskID, executeIDs, err)
	}
}

// destroySandboxExtraExecute 异步 best-effort 销毁指定字符串 executeID 的沙箱执行；用于双沙箱
// 从沙箱这类没有 int64 ID 的额外销毁点。zombieTimeout 强制 false，避免向从沙箱下发收尾命令。
func (e *EvalTargetServiceImpl) destroySandboxExtraExecute(ctx context.Context, taskID string, spaceID int64, executeID string) {
	e.destroySandboxExecute(ctx, taskID, spaceID, []string{executeID}, false)
}

// TerminateAsyncRecordsAndDestroySandbox 把仍处于 AsyncInvoking 状态的 SandboxAgent EvalTargetRecord 置为 Fail，
// 并以 best-effort 方式触发沙箱 Execute 销毁。非 SandboxAgent / 非 AsyncInvoking 的 record 会被忽略。
// zombieTimeout=true 时，Destroy 请求会带上 SandboxAgent 收尾命令 EndCmd（含 expt_id/invoke_id）；
// 其余场景（如手动取消）不下发 EndCmd。
func (e *EvalTargetServiceImpl) TerminateAsyncRecordsAndDestroySandbox(ctx context.Context, spaceID int64, recordIDs []int64, errCode int32, errMessage string, zombieTimeout bool) {
	if len(recordIDs) == 0 {
		return
	}
	records, err := e.evalTargetRepo.ListEvalTargetRecordByIDsAndSpaceID(ctx, spaceID, recordIDs)
	if err != nil {
		logs.CtxWarn(ctx, "[SandboxDestroy] batch get eval target records fail, space_id=%d, err=%v", spaceID, err)
		return
	}

	versionIDSet := make(map[int64]struct{})
	for _, r := range records {
		if r == nil || r.TargetVersionID <= 0 {
			continue
		}
		versionIDSet[r.TargetVersionID] = struct{}{}
	}
	if len(versionIDSet) == 0 {
		return
	}
	versionIDs := make([]int64, 0, len(versionIDSet))
	for id := range versionIDSet {
		versionIDs = append(versionIDs, id)
	}
	versions, err := e.evalTargetRepo.BatchGetEvalTargetVersion(ctx, spaceID, versionIDs)
	if err != nil {
		logs.CtxWarn(ctx, "[SandboxDestroy] batch get eval target versions fail, space_id=%d, err=%v", spaceID, err)
		return
	}
	sandboxVersionIDs := make(map[int64]struct{})
	for _, v := range versions {
		if v == nil || v.EvalTargetVersion == nil {
			continue
		}
		if v.EvalTargetType == entity.EvalTargetTypeSandboxAgent {
			sandboxVersionIDs[v.EvalTargetVersion.ID] = struct{}{}
		}
	}
	if len(sandboxVersionIDs) == 0 {
		return
	}

	taskIDCache := make(map[int64]string)
	for _, r := range records {
		if r == nil {
			continue
		}
		if _, ok := sandboxVersionIDs[r.TargetVersionID]; !ok {
			continue
		}
		if gptr.Indirect(r.Status) != entity.EvalTargetRunStatusAsyncInvoking {
			continue
		}
		// ⚠️ 必须在下面用 failOutput 覆盖 EvalTargetOutputData 之前取，否则 ext 里
		// operator 回传的 sandbox execute id 列表会被抹掉，只能退回裸 record.ID。
		// sandboxExecuteIDsOf 同时覆盖两种 ext 形态（完整列表 / 主 id + extra id），是 extra 单值的超集。
		executeIDs := sandboxExecuteIDsOf(ctx, r)
		extraExecuteID := extractExtraSandboxExecuteID(r)
		failOutput := &entity.EvalTargetOutputData{
			EvalTargetRunError: &entity.EvalTargetRunError{
				Code:    errCode,
				Message: errMessage,
			},
		}
		// 保留 extra execute id 在落库的 fail record 上，便于事后审计 / 手动兜底销毁。
		if extraExecuteID != "" {
			failOutput.Ext = map[string]string{entity.SandboxAgentExtKeyExtraExecuteID: extraExecuteID}
		}
		r.Status = gptr.Of(entity.EvalTargetRunStatusFail)
		r.EvalTargetOutputData = failOutput
		if err := e.evalTargetRepo.SaveEvalTargetRecord(ctx, r, nil); err != nil {
			logs.CtxWarn(ctx, "[SandboxDestroy] save terminated target record fail, record_id=%d, err=%v", r.ID, err)
		}

		taskID, ok := taskIDCache[r.ExperimentRunID]
		if !ok {
			taskID = e.resolveSandboxTaskIDByRunID(ctx, r.ExperimentRunID)
			taskIDCache[r.ExperimentRunID] = taskID
		}
		// ⚠️ 主 / 从沙箱必须**分两次**调用销毁，不能合成一次传 2 个 ExecuteIDs：
		// commercial 侧 destroyReqDO2PO 只在 len(ExecuteIDs)==1 时才拼 zombie 收尾 EndCmd
		// （见 infra/rpc/sandbox/sandbox_scheduler.go），合成一次会让 EndCmd 静默不下发。
		// 主沙箱：executeIDs 里除 extra 之外的部分（通常即 record.ID），带 zombieTimeout。
		for _, id := range executeIDs {
			if id == extraExecuteID {
				continue
			}
			e.destroySandboxExecute(ctx, taskID, r.SpaceID, []string{id}, zombieTimeout)
		}
		// 双沙箱：额外销毁从沙箱 execute，语义对齐 destroySandboxExecuteIfNeeded (success 分支)。
		// 从沙箱 zombieTimeout 恒为 false，不需要下发 SandboxAgent EndCmd（EndCmd 只对主沙箱有意义）。
		if extraExecuteID != "" {
			e.destroySandboxExtraExecute(ctx, taskID, r.SpaceID, extraExecuteID)
		}
	}
}

// sandboxStatusCheckConcurrency 单次 sweep 内并发调 sandbox.Get 的上限，避免打爆下游。
const sandboxStatusCheckConcurrency = 8

// CheckSandboxTerminated 参见 IEvalTargetService.CheckSandboxTerminated。
// 实现要点：
//   - 只对 SandboxAgent 且仍处于 AsyncInvoking 的 record 发 sandbox.Get；避免在同步/非 sandbox record 上浪费 RPC。
//   - Get 出错（含开源 stub 的 "not implement" / adapter 未注入）一律 warn + skip 该 record，让 zombie 兜底。
//   - 只把 Failed / Canceled / Finished 视为"结束但没上报"命中；Succeeded 会有毫秒级 in-flight 回调窗口，交给 zombie 或后续 tick 兜底。
//   - 查哪些 execute id 由 sandboxExecuteIDsOf 决定（与销毁同源），**不按 record.ID 约定推断** ——
//     否则双沙箱那种"id 带 operator 后缀"的形态整条记录都判不出终态，只能等 3h zombie。
func (e *EvalTargetServiceImpl) CheckSandboxTerminated(ctx context.Context, spaceID int64, recordIDs []int64) ([]int64, map[int64]string) {
	if e.sandboxSchedulerAdapter == nil || len(recordIDs) == 0 {
		return nil, nil
	}

	records, err := e.evalTargetRepo.ListEvalTargetRecordByIDsAndSpaceID(ctx, spaceID, recordIDs)
	if err != nil {
		logs.CtxWarn(ctx, "[SandboxStatusCheck] batch get eval target records fail, space_id=%d, err=%v", spaceID, err)
		return nil, nil
	}

	versionIDSet := make(map[int64]struct{})
	for _, r := range records {
		if r == nil || r.TargetVersionID <= 0 {
			continue
		}
		if gptr.Indirect(r.Status) != entity.EvalTargetRunStatusAsyncInvoking {
			continue
		}
		versionIDSet[r.TargetVersionID] = struct{}{}
	}
	if len(versionIDSet) == 0 {
		return nil, nil
	}
	versionIDs := make([]int64, 0, len(versionIDSet))
	for id := range versionIDSet {
		versionIDs = append(versionIDs, id)
	}
	versions, err := e.evalTargetRepo.BatchGetEvalTargetVersion(ctx, spaceID, versionIDs)
	if err != nil {
		logs.CtxWarn(ctx, "[SandboxStatusCheck] batch get eval target versions fail, space_id=%d, err=%v", spaceID, err)
		return nil, nil
	}
	sandboxVersionIDs := make(map[int64]struct{})
	for _, v := range versions {
		if v == nil || v.EvalTargetVersion == nil {
			continue
		}
		if v.EvalTargetType == entity.EvalTargetTypeSandboxAgent {
			sandboxVersionIDs[v.EvalTargetVersion.ID] = struct{}{}
		}
	}
	if len(sandboxVersionIDs) == 0 {
		return nil, nil
	}

	// 筛出真正需要查询的 record，避免起没用的 goroutine
	targets := make([]*entity.EvalTargetRecord, 0, len(records))
	for _, r := range records {
		if r == nil {
			continue
		}
		if _, ok := sandboxVersionIDs[r.TargetVersionID]; !ok {
			continue
		}
		if gptr.Indirect(r.Status) != entity.EvalTargetRunStatusAsyncInvoking {
			continue
		}
		targets = append(targets, r)
	}
	if len(targets) == 0 {
		return nil, nil
	}

	sem := make(chan struct{}, sandboxStatusCheckConcurrency)
	var mu sync.Mutex
	terminated := make([]int64, 0)
	statusMap := make(map[int64]string)

	var wg sync.WaitGroup
	for _, r := range targets {
		r := r
		wg.Add(1)
		sem <- struct{}{}
		goroutine.Go(ctx, func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			// execute id 一律走 sandboxExecuteIDsOf（与销毁同一个来源），**不按约定推断**。
			//
			// 这里曾把主沙箱硬编码成 `record.ID`（单沙箱约定），只从 ext 取"额外"那一个。
			// 后果是双沙箱里"完整列表"那种形态（ext 只有 sandbox_execute_ids、没有 extra key，
			// 其 id 带 operator 自己的后缀）**整条记录对本巡检完全不可见**：
			// 主查询拿裸 record.ID 去问，沙箱侧根本没有这个 execution，只回
			// "execution not found"，按 querySandboxTerminalStatus 的契约返回 (false, _)；
			// 而 extra key 不存在，从查询直接跳过。于是 record 永远判不出终态，
			// 这类实验拿不到本巡检的快速兜底，只能等 item 级 zombie 超时（异步默认 3h）。
			//
			// 现在与 destroySandboxExecuteIfNeeded / TerminateAsyncRecordsAndDestroySandbox
			// 共用同一个取值函数：两个 ext key 都认、都缺才退回裸 record.ID。
			// "id 怎么拼"只此一处知道，不再在巡检里复制一份约定。
			//
			// 任一 execution 进终态就算命中——双沙箱是配对关系，一边挂了另一边的产出也没意义。
			ids := sandboxExecuteIDsOf(ctx, r)
			var (
				mainTerminal bool
				mainStatus   rpc.SandboxExecuteStatus
				subTerminal  bool
				subStatus    rpc.SandboxExecuteStatus
			)
			for i, id := range ids {
				// 列表首个即"主"（单沙箱下就是 record.ID；双沙箱下是 operator 回传的第一个），
				// 其余归入"从"。side 只用于 err_msg 与日志可读性，不参与命中判定。
				if i == 0 {
					mainTerminal, mainStatus = e.querySandboxTerminalStatus(ctx, id, spaceID, r.ID, "main")
					continue
				}
				terminal, status := e.querySandboxTerminalStatus(ctx, id, spaceID, r.ID, "subordinate")
				// 多个从沙箱时取**第一个进终态**的那个做标签，避免被后面仍 Running 的覆盖掉。
				if terminal && !subTerminal {
					subTerminal, subStatus = terminal, status
				}
			}
			if !mainTerminal && !subTerminal {
				return
			}
			mu.Lock()
			terminated = append(terminated, r.ID)
			statusMap[r.ID] = combineSandboxStatusLabel(mainTerminal, mainStatus, subTerminal, subStatus)
			mu.Unlock()
		})
	}
	wg.Wait()

	if len(terminated) == 0 {
		return nil, nil
	}
	return terminated, statusMap
}

// querySandboxTerminalStatus 查询单个 sandbox execute 状态；返回 (是否终态, 状态)。
// Get 出错 / adapter 未接入 / 非终态 一律 (false, _) —— warn + skip 该 execute。
func (e *EvalTargetServiceImpl) querySandboxTerminalStatus(ctx context.Context, executeID string, spaceID, recordID int64, side string) (bool, rpc.SandboxExecuteStatus) {
	resp, err := e.sandboxSchedulerAdapter.Get(ctx, &rpc.SandboxGetRequest{
		ExecuteID:   executeID,
		WorkspaceID: spaceID,
	})
	if err != nil {
		logs.CtxWarn(ctx, "[SandboxStatusCheck] sandbox get fail, record_id=%d, execute_id=%s, side=%s, err=%v", recordID, executeID, side, err)
		return false, 0
	}
	if resp == nil || resp.ExecuteInfo == nil {
		return false, 0
	}
	status := resp.ExecuteInfo.Status
	if status != rpc.SandboxExecuteStatusFailed && status != rpc.SandboxExecuteStatusCanceled && status != rpc.SandboxExecuteStatusFinished {
		return false, status
	}
	return true, status
}

// combineSandboxStatusLabel 生成"哪一侧沙箱进入终态"的人类可读标签，写进 err_msg 辅助定位。
//
//	"Failed (main)" / "Canceled (subordinate)" / "Failed (main), Canceled (subordinate)"
func combineSandboxStatusLabel(mainTerminal bool, mainStatus rpc.SandboxExecuteStatus, subTerminal bool, subStatus rpc.SandboxExecuteStatus) string {
	parts := make([]string, 0, 2)
	if mainTerminal {
		parts = append(parts, sandboxStatusText(mainStatus)+" (main)")
	}
	if subTerminal {
		parts = append(parts, sandboxStatusText(subStatus)+" (subordinate)")
	}
	return strings.Join(parts, ", ")
}

func sandboxStatusText(s rpc.SandboxExecuteStatus) string {
	switch s {
	case rpc.SandboxExecuteStatusFailed:
		return "Failed"
	case rpc.SandboxExecuteStatusCanceled:
		return "Canceled"
	case rpc.SandboxExecuteStatusFinished:
		return "Finished"
	case rpc.SandboxExecuteStatusSucceeded:
		return "Succeeded"
	default:
		return strconv.Itoa(int(s))
	}
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
		// 这条 return 会跳过函数尾部的 destroySandboxExecuteIfNeeded —— 双沙箱下就是
		// 「沙箱永不回收 + 并发名额永不归还」，后续 item 全撞 601300702。原来只返回 error，
		// 而上游 ReportEvalTargetInvokeResult 并不打印它，现象是「什么都没发生」，
		// 只能靠「日志里没有 destroy」反推。必须留痕。
		logs.CtxWarn(ctx, "[SandboxDestroy] ReportInvokeRecords skipped: record status is not AsyncInvoking, sandbox will NOT be destroyed, record_id=%d, space_id=%d, got_status=%d, want_status=%d",
			param.RecordID, param.SpaceID, status, entity.EvalTargetRunStatusAsyncInvoking)
		return errorx.NewByCode(errno.CommonBadRequestCode, errorx.WithExtraMsg(fmt.Sprintf("unexpected target result status %d", status)))
	}
	if record.EvalTargetOutputData != nil && len(record.EvalTargetOutputData.Ext) > 0 {
		if param.OutputData == nil {
			param.OutputData = &entity.EvalTargetOutputData{}
		}
		if param.OutputData.Ext == nil {
			param.OutputData.Ext = make(map[string]string)
		}
		for k, v := range record.EvalTargetOutputData.Ext {
			param.OutputData.Ext[k] = v
		}
	}
	// 保留 record 上累积的 EvalTargetSteps（沙箱 agent ReportEvalTargetStepMetric 期间 append 上来的）,
	// 否则整体覆盖 param.OutputData 会抹掉 step 明细。与上方 Ext 保留一致的模式。
	if record.EvalTargetOutputData != nil && len(record.EvalTargetOutputData.EvalTargetSteps) > 0 {
		if param.OutputData == nil {
			param.OutputData = &entity.EvalTargetOutputData{}
		}
		param.OutputData.EvalTargetSteps = record.EvalTargetOutputData.EvalTargetSteps
	}

	record.EvalTargetOutputData = param.OutputData
	record.Status = gptr.Of(param.Status)
	e.convEvalTargetRunErr(ctx, record)

	if err := e.evalTargetRepo.SaveEvalTargetRecord(ctx, record, nil); err != nil {
		return err
	}

	// SandboxAgent 评测对象单行执行完成后，best-effort 销毁沙箱执行（仅告警不阻断）
	e.destroySandboxExecuteIfNeeded(ctx, record)

	// traceID, err := e.emitTargetTrace(logs.SetLogID(ctx, record.LogID), record, param.Session)
	// if err != nil {
	//	logs.CtxError(ctx, "emitTargetTrace fail, target_id: %v, target_version_id: %v, record_id: %v, err: %v",
	//		record.TargetID, record.TargetVersionID, record.ID, err)
	// }

	recordTrajectory := func() error {
		var sms *int64
		// 优先用「请求发起时间」作为抽取 trajectory 的时间下界;它比 record.BaseInfo.CreatedAt(异步返回后才 stamp)
		// 更早,避免漏掉请求发起到返回之间的 span。为 0(未透传)时回退到 CreatedAt,保持向前兼容。
		if param.AsyncUnixMS > 0 {
			sms = gptr.Of(param.AsyncUnixMS)
		} else if record.BaseInfo != nil {
			sms = record.BaseInfo.CreatedAt
		}
		trajectory, err := e.ExtractTrajectory(ctx, param.SpaceID, record.TraceID, sms)
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
		updateRec := &entity.EvalTargetRecord{
			ID:                   record.ID,
			TraceID:              record.TraceID,
			EvalTargetOutputData: od,
		}
		return e.evalTargetRepo.UpdateEvalTargetRecord(ctx, updateRec, nil)
	}

	if param.EnableExtractTrajectory == nil || *param.EnableExtractTrajectory {
		goroutine.Go(ctx, func() {
			time.Sleep(e.configer.GetTargetTrajectoryConf(ctx).GetExtractInterval(param.SpaceID))
			if err := recordTrajectory(); err != nil {
				logs.CtxError(ctx, "extract and record trajectory fail, record_id: %v, err: %v", record.ID, err)
			}
		})
	}

	return nil
}

func (e *EvalTargetServiceImpl) emitTargetTrace(ctx context.Context, span looptracer.Span, record *entity.EvalTargetRecord, session *entity.Session) (string, error) {
	if record.EvalTargetOutputData == nil {
		logs.CtxInfo(ctx, "emitTargetTrace with null data")
		return "", nil
	}

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
		var name, url string
		if content.Image != nil {
			name = gptr.Indirect(content.Image.Name)
			url = gptr.Indirect(content.Image.URL)
		}
		return []*tracespec.ModelMessagePart{{
			ImageURL: &tracespec.ModelImageURL{
				Name: name,
				URL:  url,
			},
			Type: tracespec.ModelMessagePartType(content.GetContentType()),
		}}
	case entity.ContentTypeAudio:
		var name, url string
		if content.Audio != nil {
			name = gptr.Indirect(content.Audio.Name)
			url = gptr.Indirect(content.Audio.URL)
		}
		return []*tracespec.ModelMessagePart{{
			AudioURL: &tracespec.ModelAudioURL{
				Name: name,
				URL:  url,
			},
			Type: tracespec.ModelMessagePartTypeAudio,
		}}
	case entity.ContentTypeVideo:
		var name, url string
		if content.Video != nil {
			name = gptr.Indirect(content.Video.Name)
			url = gptr.Indirect(content.Video.URL)
		}
		return []*tracespec.ModelMessagePart{{
			VideoURL: &tracespec.ModelVideoURL{
				Name: name,
				URL:  url,
			},
			Type: tracespec.ModelMessagePartTypeVideo,
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

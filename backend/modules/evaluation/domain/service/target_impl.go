// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"runtime"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/sonic"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/looptracer"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type EvalTargetServiceImpl struct {
	idgen          idgen.IIDGenerator
	metric         metrics.EvalTargetMetrics
	evalTargetRepo repo.IEvalTargetRepo
	typedOperators map[entity.EvalTargetType]ISourceEvalTargetOperateService
}

func NewEvalTargetServiceImpl(evalTargetRepo repo.IEvalTargetRepo,
	idgen idgen.IIDGenerator,
	metric metrics.EvalTargetMetrics,
	typedOperators map[entity.EvalTargetType]ISourceEvalTargetOperateService,
) IEvalTargetService {
	singletonEvalTargetService := &EvalTargetServiceImpl{
		evalTargetRepo: evalTargetRepo,
		idgen:          idgen,
		metric:         metric,
		typedOperators: typedOperators,
	}
	return singletonEvalTargetService
}

func (e *EvalTargetServiceImpl) CreateEvalTarget(ctx context.Context, spaceID int64, sourceTargetID, sourceTargetVersion string, targetType entity.EvalTargetType, opts ...entity.Option) (id int64, versionID int64, err error) {
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

func (e *EvalTargetServiceImpl) GetEvalTargetVersion(ctx context.Context, spaceID int64, versionID int64, needSourceInfo bool) (do *entity.EvalTarget, err error) {
	do, err = e.evalTargetRepo.GetEvalTargetVersion(ctx, spaceID, versionID)
	if err != nil {
		return nil, err
	}
	// 包装source info信息
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
	// 包装source info信息
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

func (e *EvalTargetServiceImpl) ExecuteTarget(ctx context.Context, spaceID int64, targetID int64, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) (record *entity.EvalTargetRecord, err error) {
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

	defer func() {
		if e := recover(); e != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			logs.CtxError(ctx, "goroutine panic: %s: %s", e, buf)
			err = errorx.New("panic occurred when, reason=%v", e)
		}

		if err != nil {
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
				outputData.EvalTargetRunError.Code = statusErr.Code()
				outputData.EvalTargetRunError.Message = statusErr.Error()
				spanParam.ErrCode = strconv.FormatInt(int64(statusErr.Code()), 10)
			} else {
				outputData.EvalTargetRunError.Code = errno.CommonInternalErrorCode
				outputData.EvalTargetRunError.Message = err.Error()
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
			span.SetCallType("EvalTarget")
			tags := make(map[string]interface{})
			tags["eval_target_type"] = spanParam.TargetType
			tags["eval_target_id"] = spanParam.TargetID
			tags["eval_target_version"] = spanParam.TargetVersion

			span.SetUserID(ctx, userIDInContext)

			span.SetTags(ctx, tags)
			span.Finish(ctx)
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

	evalTargetDO, err := e.GetEvalTargetVersion(ctx, spaceID, targetVersionID, false)
	if err != nil {
		return nil, err
	}
	if evalTargetDO == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("[ExecuteTarget]evalTargetDO is nil"))
	}

	ctx, span = looptracer.GetTracer().StartSpan(ctx, "EvalTarget", "eval_target", looptracer.WithStartNewTrace(), looptracer.WithSpanWorkspaceID(strconv.FormatInt(spaceID, 10)))
	if err != nil {
		logs.CtxWarn(ctx, "start span failed, err=%v", err)
	}

	// inject flow trace
	ctx = looptracer.GetTracer().Inject(ctx)
	if err != nil {
		logs.CtxWarn(ctx, "Inject ctx failed, err=%v", err)
	}
	if e.typedOperators[evalTargetDO.EvalTargetType] == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
	}
	err = e.typedOperators[evalTargetDO.EvalTargetType].ValidateInput(ctx, spaceID, evalTargetDO.EvalTargetVersion.InputSchema, inputData)
	if err != nil {
		return nil, err
	}
	outputData, runStatus, err = e.typedOperators[evalTargetDO.EvalTargetType].Execute(ctx, spaceID, &entity.ExecuteEvalTargetParam{
		TargetID:            targetID,
		VersionID:           targetVersionID,
		SourceTargetID:      evalTargetDO.SourceTargetID,
		SourceTargetVersion: evalTargetDO.EvalTargetVersion.SourceTargetVersion,
		Input:               inputData,
		TargetType:          evalTargetDO.EvalTargetType,
	})
	if err != nil {
		return nil, err
	}

	if outputData == nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("[ExecuteTarget]outputData is nil"))
	}
	// setSpan
	setSpanInputOutput(spanParam, evalTargetDO, inputData, outputData)

	return record, nil
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

func setSpanInputOutput(spanParam *targetSpanTagsParams, do *entity.EvalTarget, inputData *entity.EvalTargetInputData, outputData *entity.EvalTargetOutputData) {
	spanParam.TargetType = do.EvalTargetType.String()
	spanParam.TargetID = do.SourceTargetID
	spanParam.TargetVersion = do.EvalTargetVersion.SourceTargetVersion

	if inputData != nil {
		spanParam.Inputs = map[string]*tracespec.ModelMessagePart{}
		for key, content := range inputData.InputFields {
			// TODO 先只处理text
			spanParam.Inputs[key] = &tracespec.ModelMessagePart{
				Text: content.GetText(),
				Type: tracespec.ModelMessagePartType(content.GetContentType()),
			}
		}
	}
	if outputData != nil {
		spanParam.Outputs = map[string]*tracespec.ModelMessagePart{}
		for key, content := range outputData.OutputFields {
			spanParam.Outputs[key] = &tracespec.ModelMessagePart{
				// TODO 先只处理text
				Text: content.GetText(),
				Type: tracespec.ModelMessagePartType(content.GetContentType()),
			}
		}
		spanParam.InputToken = outputData.EvalTargetUsage.InputTokens
		spanParam.OutputToken = outputData.EvalTargetUsage.OutputTokens
	}
}

type targetSpanTagsParams struct {
	Inputs  map[string]*tracespec.ModelMessagePart
	Outputs map[string]*tracespec.ModelMessagePart
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

// buildPage 有的接口没有滚动分页，需要自己用page适配一下
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

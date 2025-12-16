// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/experimentservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type EvaluationProvider struct {
	client experimentservice.Client
}

func NewEvaluationRPCProvider(client experimentservice.Client) rpc.IEvaluationRPCAdapter {
	return &EvaluationProvider{client: client}
}

func (e *EvaluationProvider) SubmitExperiment(ctx context.Context, param *rpc.SubmitExperimentReq) (exptID, exptRunID int64, err error) {
	if param.WorkspaceID == 0 {
		return 0, 0, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("workspace ID is nil"))
	}
	logs.CtxInfo(ctx, "SubmitExperiment, param: %+v", param)
	resp, err := e.client.SubmitExperiment(ctx, &expt.SubmitExperimentRequest{
		WorkspaceID:           param.WorkspaceID,
		EvalSetVersionID:      param.EvalSetVersionID,
		TargetVersionID:       param.TargetVersionID,
		EvaluatorVersionIds:   param.EvaluatorVersionIds,
		Name:                  param.Name,
		Desc:                  param.Desc,
		EvalSetID:             param.EvalSetID,
		TargetID:              param.TargetID,
		TargetFieldMapping:    param.TargetFieldMapping,
		EvaluatorFieldMapping: param.EvaluatorFieldMapping,
		ItemConcurNum:         param.ItemConcurNum,
		EvaluatorsConcurNum:   param.EvaluatorsConcurNum,
		CreateEvalTargetParam: param.CreateEvalTargetParam,
		ExptType:              param.ExptType,
		MaxAliveTime:          param.MaxAliveTime,
		SourceType:            param.SourceType,
		SourceID:              param.SourceID,
		Session:               param.Session,
	})
	if err != nil {
		logs.CtxError(ctx, "SubmitExperiment failed, err: %v", err)
		return 0, 0, errorx.NewByCode(obErrorx.CommonRPCErrorCode, errorx.WithExtraMsg("SubmitExperiment failed"))
	}
	return resp.GetExperiment().GetID(), resp.GetRunID(), nil
}

func (e *EvaluationProvider) InvokeExperiment(ctx context.Context, param *rpc.InvokeExperimentReq) (addedItems int64, err error) {
	if param.WorkspaceID == 0 {
		return 0, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("workspace ID is nil"))
	}
	if param.EvaluationSetID == 0 {
		return 0, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("evaluation set ID is nil"))
	}
	logs.CtxInfo(ctx, "InvokeExperiment, param: %+v", param)
	resp, err := e.client.InvokeExperiment(ctx, &expt.InvokeExperimentRequest{
		WorkspaceID:      param.WorkspaceID,
		EvaluationSetID:  param.EvaluationSetID,
		Items:            param.Items,
		SkipInvalidItems: param.SkipInvalidItems,
		AllowPartialAdd:  param.AllowPartialAdd,
		ExperimentID:     param.ExperimentID,
		ExperimentRunID:  param.ExperimentRunID,
		Ext:              param.Ext,
		Session:          param.Session,
	})
	if err != nil {
		logs.CtxError(ctx, "InvokeExperiment failed, err: %v", err)
		// 透传下游 BizStatus 错误码（Kitex biz exception），以便上层做精确处理
		if statusErr, ok := errorx.FromStatusError(err); ok {
			return 0, statusErr
		}
		// 其他非 BizStatus 错误保留原始错误作为 cause，并包装为通用 RPC 错误
		return 0, errorx.WrapByCode(err, obErrorx.CommonRPCErrorCode)
	}
	return int64(len(resp.GetAddedItems())), nil
}

func (e *EvaluationProvider) FinishExperiment(ctx context.Context, param *rpc.FinishExperimentReq) (err error) {
	if param.WorkspaceID == 0 {
		return errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("workspace ID is nil"))
	}
	if param.ExperimentID == 0 {
		return errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("experiment ID is nil"))
	}
	if param.ExperimentRunID == 0 {
		return errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("experiment run ID is nil"))
	}
	logs.CtxInfo(ctx, "FinishExperiment, param: %+v", param)
	_, err = e.client.FinishExperiment(ctx, &expt.FinishExperimentRequest{
		WorkspaceID:     ptr.Of(param.WorkspaceID),
		ExperimentID:    ptr.Of(param.ExperimentID),
		ExperimentRunID: ptr.Of(param.ExperimentRunID),
		Session:         param.Session,
	})
	if err != nil {
		logs.CtxError(ctx, "FinishExperiment failed, err: %v", err)
		return errorx.NewByCode(obErrorx.CommonRPCErrorCode, errorx.WithExtraMsg("FinishExperiment failed"))
	}
	return nil
}

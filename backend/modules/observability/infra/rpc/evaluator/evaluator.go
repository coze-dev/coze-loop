// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"
	"strconv"

	"github.com/bytedance/gg/gptr"
	doevaluator "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluator"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluatorservice"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/samber/lo"
)

type EvaluatorRPCAdapter struct {
	client evaluatorservice.Client
}

func NewEvaluatorRPCProvider(client evaluatorservice.Client) rpc.IEvaluatorRPCAdapter {
	return &EvaluatorRPCAdapter{
		client: client,
	}
}

func (r *EvaluatorRPCAdapter) BatchGetEvaluatorVersions(ctx context.Context, param *rpc.BatchGetEvaluatorVersionsParam) ([]*rpc.Evaluator, map[int64]*rpc.Evaluator, error) {
	if len(param.EvaluatorVersionIds) == 0 {
		return nil, nil, nil
	}
	res, err := r.client.BatchGetEvaluatorVersions(ctx, &evaluator.BatchGetEvaluatorVersionsRequest{
		WorkspaceID:         param.WorkspaceID,
		EvaluatorVersionIds: param.EvaluatorVersionIds,
		IncludeDeleted:      ptr.Of(false),
	})
	if err != nil {
		logs.CtxWarn(ctx, "get evaluator info failed: %v", err)
		return nil, nil, err
	}
	evalInfos := make([]*rpc.Evaluator, 0)
	for _, eval := range res.GetEvaluators() {
		evalInfos = append(evalInfos, &rpc.Evaluator{
			EvaluatorVersionID: eval.GetCurrentVersion().GetID(),
			EvaluatorName:      eval.GetName(),
			EvaluatorVersion:   eval.GetCurrentVersion().GetVersion(),
		})
	}
	evalMap := lo.Associate(evalInfos, func(item *rpc.Evaluator) (int64, *rpc.Evaluator) {
		return item.EvaluatorVersionID, item
	})
	return evalInfos, evalMap, nil
}

func (r *EvaluatorRPCAdapter) UpdateEvaluatorRecord(ctx context.Context, param *rpc.UpdateEvaluatorRecordParam) error {
	workspaceID, err := strconv.ParseInt(param.WorkspaceID, 10, 64)
	if err != nil {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace ID"))
	}
	_, err = r.client.UpdateEvaluatorRecord(ctx, &evaluator.UpdateEvaluatorRecordRequest{
		WorkspaceID:       workspaceID,
		EvaluatorRecordID: param.EvaluatorRecordID,
		Correction: &doevaluator.Correction{
			Score:     lo.ToPtr(param.Score),
			Explain:   lo.ToPtr(param.Reasoning),
			UpdatedBy: lo.ToPtr(param.UpdatedBy),
		},
	})
	if err != nil {
		logs.CtxWarn(ctx, "update evaluator record failed: %v", err)
		return err
	}

	return nil
}

func (r *EvaluatorRPCAdapter) ListEvaluators(ctx context.Context, param *rpc.ListEvaluatorsParam) ([]*rpc.Evaluator, error) {
	resp, err := r.client.ListEvaluators(ctx, &evaluator.ListEvaluatorsRequest{
		WorkspaceID: param.WorkspaceID,
		SearchName:  param.Name,
		PageSize:    gptr.Of(int32(500)),
		PageNumber:  gptr.Of(int32(1)),
		WithVersion: gptr.Of(true),
	})
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonRPCErrorCodeCode)
	}
	logs.CtxInfo(ctx, "ListEvaluators: %v", resp.GetEvaluators())
	evalInfos := make([]*rpc.Evaluator, 0)
	for _, eval := range resp.GetEvaluators() {
		evalInfos = append(evalInfos, &rpc.Evaluator{
			EvaluatorVersionID: eval.GetCurrentVersion().GetID(),
			EvaluatorName:      eval.GetName(),
			EvaluatorVersion:   eval.GetCurrentVersion().GetVersion(),
		})
	}
	return evalInfos, nil
}

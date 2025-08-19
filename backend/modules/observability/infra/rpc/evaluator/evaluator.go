// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluator"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluatorservice"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
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

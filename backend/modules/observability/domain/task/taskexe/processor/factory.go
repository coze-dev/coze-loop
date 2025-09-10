// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

var (
	autoEvaluteProc *AutoEvaluteProcessor
	dataReflowProc  *DataReflowProcessor
)

func NewProcessor(ctx context.Context, taskType task.TaskType) (taskexe.Processor, error) {
	switch taskType {
	case task.TaskTypeAutoEval:
		if autoEvaluteProc == nil {
			return nil, errorx.NewByCode(obErrorx.CommonInternalErrorCode, errorx.WithExtraMsg("nil proc of span_eval"))
		}
		return autoEvaluteProc, nil
	case task.TaskTypeAutoDataReflow:
		if dataReflowProc == nil {
			return nil, errorx.NewByCode(obErrorx.CommonInternalErrorCode, errorx.WithExtraMsg("nil proc of span_data_reflow"))
		}
		return dataReflowProc, nil
	default:
		return nil, errorx.NewByCode(obErrorx.CommonInternalErrorCode, errorx.WithExtraMsg(fmt.Sprintf("invalid task_type='%s' when new processor", taskType)))
	}
}

func InitProcessor(
	datasetServiceProvider *service.DatasetServiceAdaptor,
	evalService rpc.IEvaluatorRPCAdapter,
	evaluationService rpc.IEvaluationRPCAdapter,
	taskRepo repo.ITaskRepo) {
	autoEvaluteProc = newAutoEvaluteProcessor(datasetServiceProvider, evalService, evaluationService, taskRepo)
	dataReflowProc = newDataReflowProcessor(datasetServiceProvider, taskRepo)
}

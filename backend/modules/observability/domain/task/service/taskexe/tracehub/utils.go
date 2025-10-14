// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/bytedance/sonic"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	XttEnv = "x_tt_env"
)

type ContextKey string

const (
	CtxKeyEnv ContextKey = "env"
)

func ToJSONString(ctx context.Context, obj interface{}) string {
	if obj == nil {
		return ""
	}
	jsonData, err := sonic.Marshal(obj)
	if err != nil {
		logs.CtxError(ctx, "JSON marshal error: %v", err)
		return ""
	}
	jsonStr := string(jsonData)
	return jsonStr
}

func (h *TraceHubServiceImpl) fillCtx(ctx context.Context) context.Context {

	logID := logs.NewLogID()
	ctx = logs.SetLogID(ctx, logID)

	//todo：是否需要？——eval
	ctx = metainfo.WithPersistentValue(ctx, "LANE_C_FORNAX_APPID", strconv.FormatInt(int64(h.aid), 10))
	if os.Getenv("TCE_HOST_ENV") == "boe" {
		ctx = context.WithValue(ctx, CtxKeyEnv, "boe_auto_task")
	} else {
		ctx = context.WithValue(ctx, CtxKeyEnv, "ppe_auto_task")
	}
	if env := os.Getenv(XttEnv); env != "" {
		ctx = context.WithValue(ctx, CtxKeyEnv, env) //nolint:staticcheck,SA1029
	}
	return ctx
}

func (h *TraceHubServiceImpl) getTenants(ctx context.Context, platform loop_span.PlatformType) ([]string, error) {
	return h.tenantProvider.GetTenantsByPlatformType(ctx, platform)
}
func (h *TraceHubServiceImpl) getSpan(ctx context.Context, tenants []string, spanIds []string, traceId, workspaceId string, startAt, endAt int64) ([]*loop_span.Span, error) {
	if len(spanIds) == 0 || workspaceId == "" {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	var filterFields []*loop_span.FilterField
	filterFields = append(filterFields, &loop_span.FilterField{
		FieldName: loop_span.SpanFieldSpanId,
		FieldType: loop_span.FieldTypeString,
		Values:    spanIds,
		QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
	})
	filterFields = append(filterFields, &loop_span.FilterField{
		FieldName: loop_span.SpanFieldSpaceId,
		FieldType: loop_span.FieldTypeString,
		Values:    []string{workspaceId},
		QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
	})
	if traceId != "" {
		filterFields = append(filterFields, &loop_span.FilterField{
			FieldName: loop_span.SpanFieldTraceId,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{traceId},

			QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
		})
	}
	res, err := h.traceRepo.ListSpans(ctx, &repo.ListSpansParam{
		Tenants: tenants,
		Filters: &loop_span.FilterFields{
			FilterFields: filterFields,
		},
		StartAt:            startAt,
		EndAt:              endAt,
		NotQueryAnnotation: true,
		Limit:              2,
	})
	if err != nil {
		logs.CtxError(ctx, "failed to list span, %v", err)
		return nil, err
	} else if len(res.Spans) == 0 {
		return nil, nil
	}
	return res.Spans, nil
}

// updateTaskRunStatusCount updates the Redis count based on Status
func (h *TraceHubServiceImpl) updateTaskRunDetailsCount(ctx context.Context, taskID int64, turn *entity.OnlineExptTurnEvalResult) error {
	// Retrieve taskRunID from Ext
	taskRunIDStr := turn.Ext["run_id"]
	if taskRunIDStr == "" {
		return fmt.Errorf("task_run_id not found in ext")
	}

	taskRunID, err := strconv.ParseInt(taskRunIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid task_run_id: %s, err: %v", taskRunIDStr, err)
	}
	// Increase the corresponding counter based on Status
	switch turn.Status {
	case entity.EvaluatorRunStatus_Success:
		return h.taskRepo.IncrTaskRunSuccessCount(ctx, taskID, taskRunID)
	case entity.EvaluatorRunStatus_Fail:
		return h.taskRepo.IncrTaskRunFailCount(ctx, taskID, taskRunID)
	default:
		logs.CtxDebug(ctx, "未知的评估状态，跳过计数: taskID=%d, taskRunID=%d, status=%d",
			taskID, taskRunID, turn.Status)
		return nil
	}
}

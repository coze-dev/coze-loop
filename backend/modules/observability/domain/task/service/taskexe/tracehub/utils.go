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
	CtxKeyEnv           = "K_ENV"
	TceEnv              = "TCE_ENV"
	TceCluster          = "TCE_CLUSTER"
	TracehubClusterName = "tracehub_default"
	InjectClusterName   = "ingest_default"
	AppIDKey            = "LANE_C_FORNAX_APPID"
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
	ctx = metainfo.WithPersistentValue(ctx, AppIDKey, strconv.FormatInt(int64(h.aid), 10))
	if env := os.Getenv(TceEnv); env != "" {
		ctx = context.WithValue(ctx, CtxKeyEnv, env) //nolint:staticcheck,SA1029
	}
	return ctx
}

func (h *TraceHubServiceImpl) getTenants(ctx context.Context, platform loop_span.PlatformType) ([]string, error) {
	return h.tenantProvider.GetTenantsByPlatformType(ctx, platform)
}

// todo need platformtype
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
	var spans []*loop_span.Span
	for _, tenant := range tenants {
		res, err := h.traceRepo.ListSpans(ctx, &repo.ListSpansParam{
			WorkSpaceID: workspaceId,
			Tenants:     []string{tenant},
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
			return spans, err
		}
		spans = append(spans, res.Spans...)
	}
	logs.CtxInfo(ctx, "list span, spans: %v", spans)

	return spans, nil
}

// updateTaskRunStatusCount updates the Redis count based on Status
func (h *TraceHubServiceImpl) updateTaskRunDetailsCount(ctx context.Context, taskID int64, turn *entity.OnlineExptTurnEvalResult, ttl int64) error {
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
		return h.taskRepo.IncrTaskRunSuccessCount(ctx, taskID, taskRunID, ttl)
	case entity.EvaluatorRunStatus_Fail:
		return h.taskRepo.IncrTaskRunFailCount(ctx, taskID, taskRunID, ttl)
	default:
		logs.CtxDebug(ctx, "未知的评估状态，跳过计数: taskID=%d, taskRunID=%d, status=%d",
			taskID, taskRunID, turn.Status)
		return nil
	}
}

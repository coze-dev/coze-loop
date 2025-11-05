// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"os"
	"strconv"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/bytedance/sonic"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
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

// todo 看看有没有更好的写法
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

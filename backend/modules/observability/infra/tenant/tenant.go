// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tenant

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewTenantProvider(traceConfig config.ITraceConfig) tenant.ITenantProvider {
	return &TenantProviderImpl{
		traceConfig: traceConfig,
	}
}

type TenantProviderImpl struct {
	traceConfig config.ITraceConfig
}

func (t *TenantProviderImpl) GetIngestTenant(ctx context.Context, spans []*loop_span.Span) string {
	return t.traceConfig.GetDefaultTraceTenant(ctx)
}

func (t *TenantProviderImpl) GetOAPIQueryTenants(ctx context.Context, platformType loop_span.PlatformType) []string {
	tenants, _ := t.GetTenantsByPlatformType(ctx, platformType)
	return tenants
}

func (t *TenantProviderImpl) GetTenantsByPlatformType(ctx context.Context, platform loop_span.PlatformType) ([]string, error) {
	cfg, err := t.traceConfig.GetPlatformTenants(ctx)
	if err != nil {
		logs.CtxError(ctx, "fail to get platform tenants, %v", err)
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInternalErrorCodeCode)
	}
	if tenants, ok := cfg.Config[string(platform)]; ok {
		return tenants, nil
	} else {
		logs.CtxError(ctx, "tenant not found for platform %s", platform)
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("tenant not found for the platform"))
	}
}

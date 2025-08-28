// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tenant

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

//go:generate mockgen -destination=mocks/tenant_provider.go -package=mocks . ITenantProvider
type ITenantProvider interface {
	GetIngestTenant(ctx context.Context, spans []*loop_span.Span) string
	GetOAPIQueryTenants(ctx context.Context, platformType loop_span.PlatformType) []string
	GetTenantsByPlatformType(ctx context.Context, platformType loop_span.PlatformType) ([]string, error)
}

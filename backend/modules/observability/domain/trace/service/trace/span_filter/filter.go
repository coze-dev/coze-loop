// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_filter

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type SpanEnv struct {
	WorkspaceId int64
}

type Factory interface {
	PlatformType() loop_span.PlatformType
	CreateFilter(context.Context) (Filter, error)
}

//go:generate mockgen -destination=mocks/filter.go -package=mocks . Filter
type Filter interface {
	BuildBasicSpanFilter(context.Context, *SpanEnv) ([]*loop_span.FilterField, bool, error)
	BuildRootSpanFilter(context.Context, *SpanEnv) ([]*loop_span.FilterField, error)
	BuildLLMSpanFilter(context.Context, *SpanEnv) ([]*loop_span.FilterField, error)
	BuildALLSpanFilter(context.Context, *SpanEnv) ([]*loop_span.FilterField, error)
}

//go:generate mockgen -destination=mocks/filter_factory.go -package=mocks . PlatformFilterFactory
type PlatformFilterFactory interface {
	GetFilter(context.Context, loop_span.PlatformType) (Filter, error)
}

type PlatformFilterFactoryImpl struct {
	platformFilters map[loop_span.PlatformType]Factory
}

func (p *PlatformFilterFactoryImpl) GetFilter(ctx context.Context, platformType loop_span.PlatformType) (Filter, error) {
	factory, ok := p.platformFilters[platformType]
	if !ok {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("platform not configured"))
	}
	return factory.CreateFilter(ctx)
}

func NewPlatformFilterFactory(factories []Factory) PlatformFilterFactory {
	ret := &PlatformFilterFactoryImpl{
		platformFilters: make(map[loop_span.PlatformType]Factory),
	}
	for _, factory := range factories {
		ret.platformFilters[factory.PlatformType()] = factory
	}
	return ret
}

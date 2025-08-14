// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExpireErrorProcessor struct {
	platformType loop_span.PlatformType
	queryEndTime int64
	workspaceId  int64
	benefitSvc   benefit.IBenefitService
}

func (c *ExpireErrorProcessor) Transform(ctx context.Context, spans loop_span.SpanList) (loop_span.SpanList, error) {
	if len(spans) > 0 {
		return spans, nil
	}
	res, err := c.benefitSvc.CheckTraceBenefit(ctx, &benefit.CheckTraceBenefitParams{
		ConnectorUID: session.UserIDInCtxOrEmpty(ctx),
		SpaceID:      c.workspaceId,
	})
	if err != nil {
		logs.CtxWarn(ctx, "fail to check trace benefit, %v", err)
		return nil, errorx.WrapByCode(err, obErrorx.ExpiredTraceErrorCode)
	} else if res == nil {
		logs.CtxWarn(ctx, "fail to get trace benefit, got nil response")
		return nil, errorx.NewByCode(obErrorx.ExpiredTraceErrorCode)
	}
	earliestTime := time.Now().UnixMilli() - (24 * time.Duration(res.StorageDuration) * time.Hour).Milliseconds()
	if c.queryEndTime < earliestTime {
		return nil, errorx.NewByCode(obErrorx.ExpiredTraceErrorCode)
	}
	return spans, nil
}

type ExpireErrorProcessorFactory struct {
	benefitSvc benefit.IBenefitService
}

func (c *ExpireErrorProcessorFactory) CreateProcessor(ctx context.Context, set Settings) (Processor, error) {
	return &ExpireErrorProcessor{
		benefitSvc:   c.benefitSvc,
		queryEndTime: set.QueryEndTime,
		platformType: set.PlatformType,
		workspaceId:  set.WorkspaceId,
	}, nil
}

func NewExpireErrorProcessorFactory(benefitSvc benefit.IBenefitService) Factory {
	return &ExpireErrorProcessorFactory{
		benefitSvc: benefitSvc,
	}
}

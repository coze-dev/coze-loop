// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	commonentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type RateLimiterImpl struct {
	limiter limiter.IRateLimiter
}

func NewRateLimiterImpl(ctx context.Context, limiterFactory limiter.IRateLimiterFactory, evaluatorConfiger conf.IConfiger) repo.RateLimiter {
	return &RateLimiterImpl{
		limiter: limiterFactory.NewRateLimiter(limiter.WithRules(evaluatorConfiger.GetRateLimiterConf(ctx)...)),
	}
}

func (s *RateLimiterImpl) AllowInvoke(ctx context.Context, spaceID int64) bool {
	tags := []limiter.Tag{
		{K: "space_id", V: spaceID},
	}
	res, err := s.limiter.AllowN(ctx, consts.RateLimitBizKeyEvaluator, 1, limiter.WithTags(tags...))
	if err != nil {
		logs.CtxError(ctx, "allow invoke failed, err=%v", err)
		return true
	}
	if res.Allowed {
		logs.CtxInfo(ctx, "[AllowInvoke] allow invoke")
		return true
	}
	logs.CtxInfo(ctx, "[AllowInvoke] not allow invoke")
	return false
}

type PlainRateLimiterImpl struct {
	limiter limiter.IPlainRateLimiter
}

func NewPlainRateLimiterImpl(limiterFactory limiter.IPlainRateLimiterFactory) repo.IPlainRateLimiter {
	return &PlainRateLimiterImpl{
		limiter: limiterFactory.NewPlainRateLimiter(),
	}
}

func (s *PlainRateLimiterImpl) AllowInvokeWithKeyLimit(ctx context.Context, key string, limit *commonentity.RateLimit) bool {
	if len(key) == 0 {
		logs.CtxError(ctx, "[AllowInvokeWithKeyLimit] key is empty")
		return false
	}
	if limit == nil {
		logs.CtxInfo(ctx, "[AllowInvokeWithKeyLimit] limit is not set, skip invoke limit")
		return true
	}
	res, err := s.limiter.AllowN(ctx, key, 1, limiter.WithLimit(&limiter.Limit{
		Rate:   int(gptr.Indirect(limit.Rate)),
		Burst:  int(gptr.Indirect(limit.Burst)),
		Period: gptr.Indirect(limit.Period),
	}))
	if err != nil {
		logs.CtxError(ctx, "[AllowInvokeWithKeyLimit] allow invoke failed, err=%v", err)
		return true
	}
	if res.Allowed {
		logs.CtxInfo(ctx, "[AllowInvokeWithKeyLimit] allow invoke")
		return true
	}
	logs.CtxInfo(ctx, "[AllowInvokeWithKeyLimit] not allow invoke")
	return false
}

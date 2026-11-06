// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package contexts

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type exptFilterPlanKey struct{}

type exptFilterSendGuardKey struct{}

// ExptFilterSendGuard bounds each application send, including both dual-write targets.
type ExptFilterSendGuard interface {
	BeforeSend(context.Context, entity.ExptFilterTarget, int64) (func(), error)
}

// ExptFilterEnvelopeGuard reserves serialization and retry-copy capacity before preparation.
type ExptFilterEnvelopeGuard interface {
	ExptFilterSendGuard
	ReserveEnvelope(context.Context, int64) (func(), error)
}

func WithExptFilterSendGuard(ctx context.Context, guard ExptFilterSendGuard) context.Context {
	return context.WithValue(ctx, exptFilterSendGuardKey{}, guard)
}

func ExptFilterSendGuardFromContext(ctx context.Context) ExptFilterSendGuard {
	guard, _ := ctx.Value(exptFilterSendGuardKey{}).(ExptFilterSendGuard)
	return guard
}

func WithExptFilterPlan(ctx context.Context, plan *entity.ExptFilterPlan) context.Context {
	return context.WithValue(ctx, exptFilterPlanKey{}, plan.Clone())
}

func ExptFilterPlanFromContext(ctx context.Context) *entity.ExptFilterPlan {
	plan, _ := ctx.Value(exptFilterPlanKey{}).(*entity.ExptFilterPlan)
	return plan.Clone()
}

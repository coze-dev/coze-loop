// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package contexts

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type exptFilterPlanKey struct{}

func WithExptFilterPlan(ctx context.Context, plan *entity.ExptFilterPlan) context.Context {
	return context.WithValue(ctx, exptFilterPlanKey{}, plan.Clone())
}

func ExptFilterPlanFromContext(ctx context.Context) *entity.ExptFilterPlan {
	plan, _ := ctx.Value(exptFilterPlanKey{}).(*entity.ExptFilterPlan)
	return plan.Clone()
}

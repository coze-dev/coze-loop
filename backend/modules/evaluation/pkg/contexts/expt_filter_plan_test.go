// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package contexts

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestExptFilterPlanSnapshotDoesNotFollowMutations(t *testing.T) {
	plan := &entity.ExptFilterPlan{WriteMode: "dual", ReadTarget: entity.ExptFilterTargetV2, RequiredTargets: []entity.ExptFilterTarget{entity.ExptFilterTargetLegacy, entity.ExptFilterTargetV2}, ExptIDs: []int64{1, 2}, ConfigHash: "initial"}
	ctx := WithExptFilterPlan(context.Background(), plan)
	plan.RequiredTargets[0] = entity.ExptFilterTargetV2
	plan.ExptIDs[0] = 99
	plan.ConfigHash = "changed"
	got := ExptFilterPlanFromContext(ctx)
	if got.ConfigHash != "initial" || got.ExptIDs[0] != 1 || got.RequiredTargets[0] != entity.ExptFilterTargetLegacy {
		t.Fatalf("request plan changed with configuration: %+v", got)
	}
	got.ExptIDs[0] = 98
	if ExptFilterPlanFromContext(ctx).ExptIDs[0] != 1 {
		t.Fatal("a consumer mutated the request's frozen experiment set")
	}
	if ExptFilterPlanFromContext(context.Background()) != nil {
		t.Fatal("missing plan must stay absent")
	}
}

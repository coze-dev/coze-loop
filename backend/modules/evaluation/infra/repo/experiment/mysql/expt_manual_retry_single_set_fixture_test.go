// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// The external manager integration test uses the existing real MySQL fixture.
// Keeping this bridge in test code avoids the production Wire import cycle.
func NewManualRetryFixtureForTest(t *testing.T) (db.Provider, *gorm.DB, repo.IExptManualRetryRepo, int64, int64, int64, int64, func() map[string]string) {
	f := newManualRetryFixture(t)
	return f.provider, f.conn, f.port, f.space, f.expt, f.run, f.base, f.snapshot
}

func TestManualRetryLockedSingleSetSource(t *testing.T) {
	for _, kind := range []string{"unchanged", "source_type", "set_id", "set_version", "set_space", "target_id", "target_version", "target_space", "target_type", "draft_zero", "draft_set_id", "item_log_version"} {
		t.Run(kind, func(t *testing.T) {
			expt := &model.Experiment{EvalSetSourceType: 1, EvalSetID: 31, EvalSetVersionID: 32, EvalSetSpaceID: 91, TargetID: 41, TargetVersionID: 42, TargetSpaceID: 92, TargetType: 1}
			item := &lockedManualRetryItem{item: &model.ExptItemResult{ItemVersionID: 0}, log: &model.ExptItemResultRunLog{ItemVersionID: 0}, prepared: &entity.PreparedRetryItem{ItemVersionID: 0, SingleSetSource: &entity.PreparedRetrySingleSetSource{SourceType: 1, EvalSetID: 31, EvalSetVersionID: 32, EvalSetSpaceID: 91, TargetID: 41, TargetVersionID: 42, TargetSpaceID: 92, TargetType: 1}}}
			switch kind {
			case "source_type":
				expt.EvalSetSourceType = 2
			case "set_id":
				expt.EvalSetID++
			case "set_version":
				expt.EvalSetVersionID++
			case "set_space":
				expt.EvalSetSpaceID++
			case "target_id":
				expt.TargetID++
			case "target_version":
				expt.TargetVersionID++
			case "target_space":
				expt.TargetSpaceID++
			case "target_type":
				expt.TargetType++
			case "draft_zero":
				expt.EvalSetVersionID = 0
				item.prepared.SingleSetSource.EvalSetVersionID = 0
			case "draft_set_id":
				expt.EvalSetVersionID = 31
				item.prepared.SingleSetSource.EvalSetVersionID = 31
			case "item_log_version":
				item.log.ItemVersionID = 1
			}
			reason, err := (&ExptManualRetryDAOImpl{}).lockManualRetrySource(context.Background(), &entity.RequeueTerminalItemsRequest{}, item, expt, nil)
			require.NoError(t, err)
			if kind == "unchanged" {
				require.Empty(t, reason)
			} else {
				require.Equal(t, "source_changed", reason)
			}
		})
	}
}

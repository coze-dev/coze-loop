// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type staleTimeRangeExperimentRepo struct {
	repo.IExperimentRepo
	stale *entity.Experiment
}

func (r *staleTimeRangeExperimentRepo) MGetBasicByID(context.Context, []int64) ([]*entity.Experiment, error) {
	frozen := *r.stale
	return []*entity.Experiment{&frozen}, nil
}

func TestTimeRangeInjectionDoesNotRestoreStaleRunState(t *testing.T) {
	f := newLateWriteFixture(t, "")
	ctx := context.Background()
	baseRepo := f.experimentRepo()
	got, err := baseRepo.MGetBasicByID(ctx, []int64{f.expt})
	require.NoError(t, err)
	require.Len(t, got, 1)

	replacement := f.run + 1
	require.NoError(t, f.conn.Model(&model.Experiment{}).
		Where("space_id=? AND id=?", f.space, f.expt).
		Updates(map[string]any{"status": int64(entity.ExptStatus_Success), "latest_run_id": replacement}).Error)

	manager := &ExptMangerImpl{exptRepo: &staleTimeRangeExperimentRepo{IExperimentRepo: baseRepo, stale: got[0]}}
	manager.InjectExptConfTimeRange(ctx, f.expt, &entity.TaskTimeRangeDO{StartTime: gptr.Of(int64(1000)), EndTime: gptr.Of(int64(2000))})

	current, err := baseRepo.GetByID(ctx, f.expt, f.space)
	require.NoError(t, err)
	t.Logf("TIME_RANGE_FINAL_STATE status=%d latest_run_id=%d", current.Status, current.LatestRunID)
	require.Equal(t, entity.ExptStatus_Success, current.Status)
	require.Equal(t, replacement, current.LatestRunID)
	require.NotNil(t, current.EvalConf)
	require.NotNil(t, current.EvalConf.TimeRange)
	require.Equal(t, int64(1000), gptr.Indirect(current.EvalConf.TimeRange.StartTime))
	require.Equal(t, int64(2000), gptr.Indirect(current.EvalConf.TimeRange.EndTime))
}

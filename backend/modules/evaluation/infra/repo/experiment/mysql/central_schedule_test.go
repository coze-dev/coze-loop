// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func newTestRunScheduleDAO(t *testing.T) RunScheduleDAO {
	t.Helper()
	return NewRunScheduleDAO(db.NewTestDB(t, &model.ExptRunSchedule{}))
}

// 证明调度模式/优先级/owner scope 真实落库，且重复 Upsert 是更新而不是写入第二行。
func TestCentralScheduleUpsertPersistsAndIsIdempotent(t *testing.T) {
	dao := newTestRunScheduleDAO(t)
	ctx := context.Background()

	s := &model.ExptRunSchedule{
		SpaceID:           100,
		ExptID:            201,
		RunID:             301,
		ScheduleMode:      1,
		Priority:          7,
		OwnerScope:        "ppe:lane_a:cn:stone.cozeloop.evaluation",
		TargetConcurrency: 20,
		Status:            entity.ExptRunScheduleStatusPending,
		AdmissionToken:    "tok-1",
		CreatedAt:         time.Unix(1000, 0),
	}
	require.NoError(t, dao.Upsert(ctx, s))

	got, err := dao.Get(ctx, 100, 201, 301)
	require.NoError(t, err)
	assert.Equal(t, int32(1), got.ScheduleMode)
	assert.Equal(t, int32(7), got.Priority)
	assert.Equal(t, s.OwnerScope, got.OwnerScope)
	assert.Equal(t, int64(20), got.TargetConcurrency)
	assert.Equal(t, entity.ExptRunScheduleStatusPending, got.Status)

	// 同 unique key 再写一次：模式和优先级被更新，行数保持 1。
	updated := *s
	updated.ScheduleMode = 0
	updated.Priority = 9
	updated.Status = entity.ExptRunScheduleStatusActive
	require.NoError(t, dao.Upsert(ctx, &updated))

	got, err = dao.Get(ctx, 100, 201, 301)
	require.NoError(t, err)
	assert.Equal(t, int32(0), got.ScheduleMode)
	assert.Equal(t, int32(9), got.Priority)
	assert.Equal(t, entity.ExptRunScheduleStatusActive, got.Status)

	all, err := dao.ScanActive(ctx, s.OwnerScope, 1000)
	require.NoError(t, err)
	assert.Len(t, all, 1)
}

// 证明全局扫描：只看本 owner scope 的活跃 run，按 priority desc + created_at asc 排序，
// 且继承的旧 run（schedule_mode=0）同样能被扫到（由调度侧决定是否跳过）。
func TestCentralScheduleScanActiveGlobal(t *testing.T) {
	dao := newTestRunScheduleDAO(t)
	ctx := context.Background()
	scope := "ppe:lane_a:cn:stone.cozeloop.evaluation"
	otherScope := "ppe:lane_b:cn:stone.cozeloop.evaluation"

	rows := []*model.ExptRunSchedule{
		{SpaceID: 100, ExptID: 1, RunID: 1, ScheduleMode: 1, Priority: 0, OwnerScope: scope,
			TargetConcurrency: 10, Status: entity.ExptRunScheduleStatusActive, CreatedAt: time.Unix(100, 0)},
		{SpaceID: 100, ExptID: 2, RunID: 2, ScheduleMode: 1, Priority: 5, OwnerScope: scope,
			TargetConcurrency: 10, Status: entity.ExptRunScheduleStatusActive, CreatedAt: time.Unix(200, 0)},
		{SpaceID: 100, ExptID: 3, RunID: 3, ScheduleMode: 1, Priority: 5, OwnerScope: scope,
			TargetConcurrency: 10, Status: entity.ExptRunScheduleStatusPending, CreatedAt: time.Unix(150, 0)},
		{SpaceID: 100, ExptID: 4, RunID: 4, ScheduleMode: 1, Priority: 9, OwnerScope: scope,
			TargetConcurrency: 10, Status: entity.ExptRunScheduleStatusTerminal, CreatedAt: time.Unix(50, 0)},
		{SpaceID: 100, ExptID: 5, RunID: 5, ScheduleMode: 1, Priority: 8, OwnerScope: otherScope,
			TargetConcurrency: 10, Status: entity.ExptRunScheduleStatusActive, CreatedAt: time.Unix(10, 0)},
	}
	for _, r := range rows {
		require.NoError(t, dao.Upsert(ctx, r))
	}

	got, err := dao.ScanActive(ctx, scope, 1000)
	require.NoError(t, err)
	require.Len(t, got, 3)
	// priority 5 的两条：created_at 150 在 200 之前；priority 0 最后。
	assert.Equal(t, int64(3), got[0].RunID)
	assert.Equal(t, int64(2), got[1].RunID)
	assert.Equal(t, int64(1), got[2].RunID)

	// limit 生效，且仍是全局序的前 limit 条。
	got, err = dao.ScanActive(ctx, scope, 2)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, int64(3), got[0].RunID)
	assert.Equal(t, int64(2), got[1].RunID)

	// 隔离缺失直接拒绝，不退化成公共池。
	_, err = dao.ScanActive(ctx, "", 1000)
	require.Error(t, err)

	// 另一个 scope 只看自己的。
	got, err = dao.ScanActive(ctx, otherScope, 1000)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int64(5), got[0].RunID)
}

func TestCentralScheduleUpdateStatus(t *testing.T) {
	dao := newTestRunScheduleDAO(t)
	ctx := context.Background()
	s := &model.ExptRunSchedule{
		SpaceID: 100, ExptID: 1, RunID: 1, ScheduleMode: 1, Priority: 0,
		OwnerScope: "ppe:lane_a:cn:stone.cozeloop.evaluation",
		Status:     entity.ExptRunScheduleStatusActive, CreatedAt: time.Unix(100, 0),
	}
	require.NoError(t, dao.Upsert(ctx, s))
	require.NoError(t, dao.UpdateStatus(ctx, 100, 1, 1, entity.ExptRunScheduleStatusTerminal, 3))

	got, err := dao.Get(ctx, 100, 1, 1)
	require.NoError(t, err)
	assert.Equal(t, entity.ExptRunScheduleStatusTerminal, got.Status)
	assert.Equal(t, int64(3), got.LedgerGeneration)
}

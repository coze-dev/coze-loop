// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
)

func TestHookFinalizationArchivalMustCompleteBeforeBegin(t *testing.T) {
	for _, resultState := range []*int32{nil, gptr.Of(int32(entity.ExptItemResultStateDefault)), gptr.Of(int32(entity.ExptItemResultStateLogged))} {
		for _, failed := range []bool{false, true} {
			name := fmt.Sprintf("state_%v/failed_%t", gptr.Indirect(resultState), failed)
			if resultState == nil {
				name = "nil/" + name
			}
			t.Run(name, func(t *testing.T) {
				f := newFinalizationManagerFixture(t)
				ctx := context.Background()
				itemStatus, turnStatus, want := int32(2), int32(1), entity.ExptStatus_Success
				if failed {
					itemStatus, turnStatus, want = 3, 2, entity.ExptStatus_Failed
				}
				scope := func(table any) *gorm.DB {
					return f.sql.Model(table).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.key.RunID)
				}
				require.NoError(t, scope(&model.ExptItemResultRunLog{}).UpdateColumns(map[string]any{"status": itemStatus, "result_state": resultState}).Error)
				require.NoError(t, scope(&model.ExptTurnResultRunLog{}).UpdateColumn("status", turnStatus).Error)
				require.NoError(t, scope(&model.ExptItemResult{}).UpdateColumn("status", int32(entity.ItemRunState_Processing)).Error)
				require.NoError(t, scope(&model.ExptTurnResult{}).UpdateColumn("status", int32(entity.TurnRunState_Processing)).Error)
				for _, complete := range []func() error{
					func() error { return f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, nil) },
					func() error { return f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil) },
					func() error { return f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}) },
				} {
					require.ErrorIs(t, complete(), entity.ErrHookFinalizationUnsettled)
					state := finalizationRead(t, f)
					require.Equal(t, entity.HookFinalizeNone, state.State.Finalize)
					require.False(t, state.State.After.Activated)
					quota, err := dao.NewQuotaDAO(f.redis).GetQuotaSpaceExpt(ctx, f.space)
					require.NoError(t, err)
					require.Equal(t, int64(123), quota.ExptID2RunTime[f.expt])
					require.Equal(t, int64(1), f.redis.Exists(ctx, fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)).Val())
					require.Zero(t, f.notifications)
				}
				require.NoError(t, scope(&model.ExptTurnResult{}).UpdateColumn("status", turnStatus).Error)
				require.NoError(t, scope(&model.ExptItemResult{}).UpdateColumn("status", itemStatus).Error)
				require.NoError(t, scope(&model.ExptItemResultRunLog{}).UpdateColumn("result_state", int32(entity.ExptItemResultStateResulted)).Error)
				require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil))
				state := finalizationRead(t, f)
				require.Equal(t, want, state.State.Status)
				require.True(t, state.State.After.Activated)
				require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil))
				require.Equal(t, state, finalizationRead(t, f))
				require.Equal(t, 1, f.notifications)
			})
		}
	}
}

// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// 与 b0b19095 同根因的迁移点：完成路径按 from-1 递减统计计数，旧链路漂移
// （计数已为 0）时会把 processing_cnt 写成负数，并外播到 progress/webhook。
// 状态 CAS 仍是唯一的并发闸门，计数只做 clamp + 告警。
func TestCompleteCurrentItemRunClampsLegacyStatsDrift(t *testing.T) {
	for _, path := range []string{"success", "yield"} {
		t.Run(path, func(t *testing.T) {
			f := newCurrentResultFixture(t, "")
			// 模拟旧链路漂移：条目仍是 Processing，但计数已被扣到 0。
			require.NoError(t, f.conn.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Updates(map[string]any{"processing_cnt": 0, "pending_cnt": 0, "success_cnt": 0}).Error)
			source := f.source()
			write := &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Success, RetryTimes: source.Current.RetryTimes}
			if path == "yield" {
				write.Status = entity.ItemRunState_Queueing
				write.Yield = true
			}
			applied, err := f.port.CompleteCurrentItemRun(context.Background(), source, write)
			require.NoError(t, err)
			require.True(t, applied)

			var stats model.ExptStats
			require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Take(&stats).Error)
			require.GreaterOrEqual(t, stats.ProcessingCnt, int32(0), "processing_cnt must never go negative")
			require.GreaterOrEqual(t, stats.PendingCnt, int32(0))
			require.GreaterOrEqual(t, stats.SuccessCnt, int32(0))
			if path == "success" {
				require.Zero(t, stats.ProcessingCnt)
				require.Equal(t, int32(1), stats.SuccessCnt)
			} else {
				require.Zero(t, stats.ProcessingCnt)
				require.Equal(t, int32(1), stats.PendingCnt)
			}

			// 状态 CAS 保留：item 主表仍按真实状态推进。
			var item model.ExptItemResult
			require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.item).Take(&item).Error)
			if path == "success" {
				require.Equal(t, int32(entity.ItemRunState_Success), item.Status)
			} else {
				require.Equal(t, int32(entity.ItemRunState_Queueing), item.Status)
			}
			var log model.ExptItemResultRunLog
			require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.item).Take(&log).Error)
			if path == "success" {
				require.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(log.ResultState))
			} else {
				require.Equal(t, int32(entity.ExptItemResultStateDefault), gptr.Indirect(log.ResultState))
			}
		})
	}
}

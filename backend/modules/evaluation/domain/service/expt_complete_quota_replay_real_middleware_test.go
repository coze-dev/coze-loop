// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	mysqlmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

// redisReleaseLedger 用一个最小账本替身把关键性质放在真实 Redis 上：
// Release 失败时 key 不动、成功时按 (run, item) 删除。
// 生产实现在商业版仓（OSS 只定义 port），这里只验证"释放失败 -> key 仍在；
// 重放 -> key 消失"这条生命周期，不冒充账本的完整语义。
type redisReleaseLedger struct {
	component.ICentralReservationGuard
	raw      *redis.Client
	fail     bool
	releases []releaseCall
}

func quotaLedgerKey(runID, itemID int64) string {
	return fmt.Sprintf("res:%d:%d", runID, itemID)
}

func (g *redisReleaseLedger) Release(ctx context.Context, schedulerScope string, exptRunID, itemID int64, expectedRequestID, reason string) error {
	g.releases = append(g.releases, releaseCall{Scope: schedulerScope, RunID: exptRunID, ItemID: itemID, RequestID: expectedRequestID, Reason: reason})
	if g.fail {
		return errors.New("ledger unavailable")
	}
	return g.raw.Del(ctx, quotaLedgerKey(exptRunID, itemID)).Err()
}

// 真实 MySQL + Redis 的最终状态证明：
//  1. 第一次完成时账本不可用 -> 实验仍是 Processing、run log 仍是 Processing（重放源保留）；
//  2. 账本恢复后重放同一个完成 -> 实验落终态、run log 落 Terminal、reservation key 被删除。
//
// 旧实现在第 1 步就返回 nil：实验落终态、run log 落 Terminal，而 key 还在 ——
// 这正是"瞬时释放失败 -> 永久预占"的最终状态。
func TestCompleteExpt_QuotaReleaseReplayRealMiddleware(t *testing.T) {
	f, manager := newCancelFilterFixture(t)
	require.NoError(t, f.conn.Exec("UPDATE experiment SET scheduler_mode=?, scheduler_scope=? WHERE space_id=? AND id=?",
		string(entity.ExptDispatchModeEnforce), testScope, f.space, f.expt).Error)

	addr := os.Getenv("EXPT_RETRY_REAL_REDIS_ADDR")
	if addr == "" {
		t.Skip("EXPT_RETRY_REAL_REDIS_ADDR missing; REAL REDIS NOT EXECUTED")
	}
	raw := redis.NewClient(&redis.Options{Addr: addr})
	require.NoError(t, raw.Ping(context.Background()).Err())
	t.Log("REAL_REDIS_EXECUTED reservation ledger key lifecycle")

	ledgerKey := quotaLedgerKey(f.run, f.item)
	t.Cleanup(func() {
		require.NoError(t, raw.Del(context.Background(), ledgerKey).Err())
		require.NoError(t, raw.Close())
	})
	ctx := context.Background()
	require.NoError(t, raw.Set(ctx, ledgerKey, "rq1", 0).Err())

	ledger := &redisReleaseLedger{raw: raw, fail: true}
	manager.centralGuard = ledger
	ctrl := gomock.NewController(t)
	manager.publisher = eventsMocks.NewMockExptEventPublisher(ctrl)
	manager.publisher.(*eventsMocks.MockExptEventPublisher).EXPECT().
		PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	manager.quotaRepo = repoMocks.NewMockQuotaRepo(ctrl)
	manager.quotaRepo.(*repoMocks.MockQuotaRepo).EXPECT().
		CreateOrUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	manager.exptAggrResultService = svcMocks.NewMockExptAggrResultService(ctrl)
	manager.exptAggrResultService.(*svcMocks.MockExptAggrResultService).EXPECT().
		PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	manager.statsRepo.(*repoMocks.MockIExptStatsRepo).EXPECT().
		UpdateByExptID(gomock.Any(), f.expt, f.space, gomock.Any()).Return(nil).AnyTimes()
	manager.statsRepo.(*repoMocks.MockIExptStatsRepo).EXPECT().
		Get(gomock.Any(), f.expt, f.space).Return(&entity.ExptStats{ProcessingItemCnt: 1}, nil).AnyTimes()

	// 第一次：账本不可用 -> 完成必须保持可重放。
	err := manager.CompleteExpt(ctx, f.expt, &f.run, f.space, &entity.Session{UserID: "operator"})
	t.Logf("ATTEMPT1 err=%v experiment_status=%v item_run_log_status=%v reservation_present=%v",
		err, experimentStatus(t, f), itemRunLogStatus(t, f), reservationPresent(t, raw, ledgerKey))
	require.Error(t, err)
	require.True(t, errno.NeedMQRetry(err), "释放失败必须转 MQRetry，而不是写终态")
	require.Equal(t, int32(entity.ExptStatus_Processing), experimentStatus(t, f), "实验不得在额度未归还时落终态")
	require.Equal(t, int32(entity.ItemRunState_Processing), itemRunLogStatus(t, f), "run log 必须保持未终态，作为重放源")
	require.True(t, reservationPresent(t, raw, ledgerKey), "释放失败后 reservation 仍在账本上")
	require.Len(t, ledger.releases, 1)
	require.Equal(t, "rq1", ledger.releases[0].RequestID, "重放必须使用原 RequestID")
	require.Equal(t, f.run, ledger.releases[0].RunID)
	require.Equal(t, testScope, ledger.releases[0].Scope)

	// 第二次：账本恢复，同一个完成重放 -> 释放成功、终态与 run log 才落地。
	ledger.fail = false
	err = manager.CompleteExpt(ctx, f.expt, &f.run, f.space, &entity.Session{UserID: "operator"})
	t.Logf("ATTEMPT2 err=%v experiment_status=%v item_run_log_status=%v reservation_present=%v",
		err, experimentStatus(t, f), itemRunLogStatus(t, f), reservationPresent(t, raw, ledgerKey))
	require.NoError(t, err)
	require.Equal(t, int32(entity.ExptStatus_Failed), experimentStatus(t, f), "重放成功后实验才落终态")
	require.Equal(t, int32(entity.ItemRunState_Terminal), itemRunLogStatus(t, f))
	require.False(t, reservationPresent(t, raw, ledgerKey), "重放成功后 reservation 必须从账本删除")
	require.Len(t, ledger.releases, 2, "重放复用原 item 与 RequestID，不产生新的预占")
	require.Equal(t, ledger.releases[0].RequestID, ledger.releases[1].RequestID)
	require.Equal(t, ledger.releases[0].ItemID, ledger.releases[1].ItemID)
}

func experimentStatus(t *testing.T, f *filterSyncFixture) int32 {
	t.Helper()
	var status int32
	require.NoError(t, f.conn.Raw("SELECT status FROM experiment WHERE space_id=? AND id=?", f.space, f.expt).Scan(&status).Error)
	return status
}

func itemRunLogStatus(t *testing.T, f *filterSyncFixture) int32 {
	t.Helper()
	var status int32
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemResultRunLog{}).
		Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, f.run, f.item).
		Limit(1).Pluck("status", &status).Error)
	return status
}

func reservationPresent(t *testing.T, raw *redis.Client, key string) bool {
	t.Helper()
	n, err := raw.Exists(context.Background(), key).Result()
	require.NoError(t, err)
	return n == 1
}

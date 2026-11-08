// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

func TestMixedConsumerCompleteRunReplacementHolderRealRedis(t *testing.T) {
	addr := os.Getenv("EXPT_RETRY_REAL_REDIS_ADDR")
	if addr == "" {
		t.Skip("EXPT_RETRY_REAL_REDIS_ADDR missing; REAL REDIS NOT EXECUTED")
	}
	client, err := infraredis.NewClient(&goredis.Options{Addr: addr})
	require.NoError(t, err)
	raw, ok := infraredis.Unwrap(client)
	require.True(t, ok)
	t.Cleanup(func() { require.NoError(t, raw.Close()) })

	ctrl := gomock.NewController(t)
	m := newTestExptManager(ctrl)
	m.runFinishRepo = &originalCompletionRunFinishRepo{}
	exptID := time.Now().UnixNano()
	r1, r2, spaceID := exptID+1, exptID+2, exptID+3
	fake := &latestRunRunLogRepo{latestRunID: r1}
	mock := repomocks.NewMockIExptRunLogRepo(ctrl)
	fake.IExptRunLogRepo = mock
	m.runLogRepo = fake
	m.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), exptID, spaceID).DoAndReturn(
		func(context.Context, int64, int64) (*entity.Experiment, error) {
			return &entity.Experiment{ID: exptID, SpaceID: spaceID, LatestRunID: fake.latestRunID, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline}, nil
		}).AnyTimes()
	mock.EXPECT().Get(gomock.Any(), exptID, r1).DoAndReturn(
		func(context.Context, int64, int64) (*entity.ExptRunLog, error) {
			fake.latestRunID = r2
			return &entity.ExptRunLog{ExptID: exptID, ExptRunID: r1, Status: int64(entity.ExptStatus_Processing)}, nil
		}).AnyTimes()
	m.turnResultRepo.(*repomocks.MockIExptTurnResultRepo).EXPECT().
		ListTurnResult(gomock.Any(), spaceID, exptID, gomock.Any(), gomock.Any(), false).
		Return([]*entity.ExptTurnResult{{ExptRunID: r2, Status: int32(entity.TurnRunState_Queueing)}}, int64(1), nil).AnyTimes()
	m.mutex = lock.NewRedisLocker(client)

	key := m.makeExptMutexLockKey(exptID)
	require.NoError(t, raw.Set(context.Background(), key, fmt.Sprint(r2), time.Minute).Err())
	t.Cleanup(func() { _, _ = raw.Del(context.Background(), key).Result() })
	require.NoError(t, m.CompleteRun(context.Background(), exptID, r1, spaceID, &entity.Session{}))
	holder, err := raw.Get(context.Background(), key).Result()
	require.NoError(t, err, "R1 completion deleted the replacement R2 holder")
	require.Equal(t, fmt.Sprint(r2), holder, "R1 completion changed the replacement R2 holder")
}

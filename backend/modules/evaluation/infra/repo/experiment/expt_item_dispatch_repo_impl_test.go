// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
)

// 本文件覆盖派发投影 repo。它是纯透传层，所以用例只钉两件事：
//
//  1. **入参按位置原样转发** —— 一个透传层唯一真实的失效模式就是把
//     spaceID / exptID / exptRunID 这几个同类型的 int64 传错位置。传错既不报错也不
//     编译失败，表现是查了另一个实验的数据。所以每个方法都用精确实参匹配，
//     不用 gomock.Any()：用 Any() 就恰好放过了唯一要防的那类 bug。
//  2. **返回值与错误原样上抛** —— 不吞、不改写。

const (
	testSpaceID   int64 = 100
	testExptID    int64 = 200
	testExptRunID int64 = 300
	testItemID    int64 = 400
)

func newDispatchRepoFixture(t *testing.T) (*ExptItemDispatchRepoImpl, *mocks.MockIExptItemDispatchDAO) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	dao := mocks.NewMockIExptItemDispatchDAO(ctrl)
	return &ExptItemDispatchRepoImpl{dispatchDAO: dao}, dao
}

func TestNewExptItemDispatchRepo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	r := NewExptItemDispatchRepo(mocks.NewMockIExptItemDispatchDAO(ctrl))
	require.NotNil(t, r)
	// 构造函数必须真的把 DAO 装进去：装了 nil 的话所有方法都在第一次调用时 panic
	impl, ok := r.(*ExptItemDispatchRepoImpl)
	require.True(t, ok)
	assert.NotNil(t, impl.dispatchDAO)
}

func TestExptItemDispatchRepo_ClaimQuotaReserved(t *testing.T) {
	itemIDs := []int64{1, 2, 3}

	t.Run("成功：实参按位置转发，返回值原样上抛", func(t *testing.T) {
		r, dao := newDispatchRepoFixture(t)
		dao.EXPECT().
			ClaimQuotaReserved(gomock.Any(), testSpaceID, testExptID, testExptRunID, itemIDs).
			Return([]int64{1, 3}, nil)

		got, err := r.ClaimQuotaReserved(context.Background(), testSpaceID, testExptID, testExptRunID, itemIDs)
		require.NoError(t, err)
		assert.Equal(t, []int64{1, 3}, got)
	})

	t.Run("失败：错误不被吞", func(t *testing.T) {
		r, dao := newDispatchRepoFixture(t)
		wantErr := errors.New("dao down")
		dao.EXPECT().ClaimQuotaReserved(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, wantErr)

		got, err := r.ClaimQuotaReserved(context.Background(), testSpaceID, testExptID, testExptRunID, itemIDs)
		require.ErrorIs(t, err, wantErr)
		assert.Nil(t, got)
	})
}

func TestExptItemDispatchRepo_ResetQuotaReserved(t *testing.T) {
	itemIDs := []int64{7, 8}

	t.Run("成功", func(t *testing.T) {
		r, dao := newDispatchRepoFixture(t)
		dao.EXPECT().
			ResetQuotaReserved(gomock.Any(), testSpaceID, testExptID, testExptRunID, itemIDs).
			Return(itemIDs, nil)

		got, err := r.ResetQuotaReserved(context.Background(), testSpaceID, testExptID, testExptRunID, itemIDs)
		require.NoError(t, err)
		assert.Equal(t, itemIDs, got)
	})

	t.Run("失败", func(t *testing.T) {
		r, dao := newDispatchRepoFixture(t)
		wantErr := errors.New("reset fail")
		dao.EXPECT().ResetQuotaReserved(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, wantErr)

		_, err := r.ResetQuotaReserved(context.Background(), testSpaceID, testExptID, testExptRunID, itemIDs)
		require.ErrorIs(t, err, wantErr)
	})
}

func TestExptItemDispatchRepo_LoadDispatchRuntime(t *testing.T) {
	t.Run("成功：candidateLimit 一并转发", func(t *testing.T) {
		r, dao := newDispatchRepoFixture(t)
		want := &repo.ExptDispatchRuntime{
			OccupiedItemIDs:  []int64{1, 2},
			CandidateItemIDs: []int64{3, 4, 5},
		}
		// candidateLimit 传错（比如恒传 0）会让调度器每拍都拿不到候选，
		// 表现是"实验在跑但一个 item 都不派"，所以它也要精确匹配。
		dao.EXPECT().
			LoadDispatchRuntime(gomock.Any(), testSpaceID, testExptID, testExptRunID, 2000).
			Return(want, nil)

		got, err := r.LoadDispatchRuntime(context.Background(), testSpaceID, testExptID, testExptRunID, 2000)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("失败", func(t *testing.T) {
		r, dao := newDispatchRepoFixture(t)
		wantErr := errors.New("load fail")
		dao.EXPECT().LoadDispatchRuntime(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, wantErr)

		got, err := r.LoadDispatchRuntime(context.Background(), testSpaceID, testExptID, testExptRunID, 10)
		require.ErrorIs(t, err, wantErr)
		assert.Nil(t, got)
	})
}

func TestExptItemDispatchRepo_StartReservedItem(t *testing.T) {
	// CAS 未命中返回 (false, nil) 而不是错误 —— 调用方据此决定是否继续执行 item，
	// 若这里把 false 改写成 true，重复投递的 item 会被执行两次。
	t.Run("CAS 命中返回 true", func(t *testing.T) {
		r, dao := newDispatchRepoFixture(t)
		dao.EXPECT().
			StartReservedItem(gomock.Any(), testSpaceID, testExptID, testExptRunID, testItemID).
			Return(true, nil)

		started, err := r.StartReservedItem(context.Background(), testSpaceID, testExptID, testExptRunID, testItemID)
		require.NoError(t, err)
		assert.True(t, started)
	})

	t.Run("CAS 未命中返回 false 且不报错", func(t *testing.T) {
		r, dao := newDispatchRepoFixture(t)
		dao.EXPECT().StartReservedItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(false, nil)

		started, err := r.StartReservedItem(context.Background(), testSpaceID, testExptID, testExptRunID, testItemID)
		require.NoError(t, err)
		assert.False(t, started, "CAS 未命中必须回 false —— 改写成 true 会让重复投递的 item 执行两次")
	})

	t.Run("失败", func(t *testing.T) {
		r, dao := newDispatchRepoFixture(t)
		wantErr := errors.New("start fail")
		dao.EXPECT().StartReservedItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(false, wantErr)

		_, err := r.StartReservedItem(context.Background(), testSpaceID, testExptID, testExptRunID, testItemID)
		require.ErrorIs(t, err, wantErr)
	})
}

func TestExptItemDispatchRepo_RequeueProcessingItem(t *testing.T) {
	t.Run("退回成功", func(t *testing.T) {
		r, dao := newDispatchRepoFixture(t)
		dao.EXPECT().
			RequeueProcessingItem(gomock.Any(), testSpaceID, testExptID, testExptRunID, testItemID).
			Return(true, nil)

		requeued, err := r.RequeueProcessingItem(context.Background(), testSpaceID, testExptID, testExptRunID, testItemID)
		require.NoError(t, err)
		assert.True(t, requeued)
	})

	t.Run("CAS 未命中返回 false 且不报错", func(t *testing.T) {
		r, dao := newDispatchRepoFixture(t)
		dao.EXPECT().RequeueProcessingItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(false, nil)

		requeued, err := r.RequeueProcessingItem(context.Background(), testSpaceID, testExptID, testExptRunID, testItemID)
		require.NoError(t, err)
		assert.False(t, requeued)
	})

	t.Run("失败", func(t *testing.T) {
		r, dao := newDispatchRepoFixture(t)
		wantErr := errors.New("requeue fail")
		dao.EXPECT().RequeueProcessingItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(false, wantErr)

		_, err := r.RequeueProcessingItem(context.Background(), testSpaceID, testExptID, testExptRunID, testItemID)
		require.ErrorIs(t, err, wantErr)
	})
}

func TestExptItemDispatchRepo_MGetDispatchObservations(t *testing.T) {
	itemIDs := []int64{11, 12}

	t.Run("成功：观测结果原样上抛", func(t *testing.T) {
		r, dao := newDispatchRepoFixture(t)
		want := []*repo.ExptDispatchObservation{
			{ItemID: 11, Status: 1, QuotaReservationState: entity.QuotaReservationStateReserved},
			{ItemID: 12, Status: 0, QuotaReservationState: entity.QuotaReservationStateNone},
		}
		dao.EXPECT().
			MGetDispatchObservations(gomock.Any(), testSpaceID, testExptID, testExptRunID, itemIDs).
			Return(want, nil)

		got, err := r.MGetDispatchObservations(context.Background(), testSpaceID, testExptID, testExptRunID, itemIDs)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("失败", func(t *testing.T) {
		r, dao := newDispatchRepoFixture(t)
		wantErr := errors.New("mget fail")
		dao.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, wantErr)

		got, err := r.MGetDispatchObservations(context.Background(), testSpaceID, testExptID, testExptRunID, itemIDs)
		require.ErrorIs(t, err, wantErr)
		assert.Nil(t, got)
	})
}

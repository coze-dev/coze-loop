// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

type retryTestItemResultRepo struct {
	*repomocks.MockIExptItemResultRepo
	reset *repomocks.MockIExptRetryItemResetRepo
}

func (r *retryTestItemResultRepo) ResetItemForRetry(ctx context.Context, spaceID, exptID, runID int64, runLog *entity.ExptItemResultRunLog, turnIDs []int64) (bool, error) {
	return r.reset.ResetItemForRetry(ctx, spaceID, exptID, runID, runLog, turnIDs)
}

type retryTestManager struct {
	*svcmocks.MockIExptManager
	locks *svcmocks.MockIExptNewRunItemLocker
}

func (m *retryTestManager) WithNewRunItemLocks(ctx context.Context, spaceID, exptID, runID int64, itemIDs []int64, write func(context.Context, int64) error) error {
	return m.locks.WithNewRunItemLocks(ctx, spaceID, exptID, runID, itemIDs, write)
}

func newRetryTestManager(ctrl *gomock.Controller) *retryTestManager {
	manager := &retryTestManager{MockIExptManager: svcmocks.NewMockIExptManager(ctrl), locks: svcmocks.NewMockIExptNewRunItemLocker(ctrl)}
	manager.locks.EXPECT().WithNewRunItemLocks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, _, _, _ int64, itemIDs []int64, write func(context.Context, int64) error) error {
			for _, itemID := range itemIDs {
				if err := write(ctx, itemID); err != nil {
					return err
				}
			}
			return nil
		}).AnyTimes()
	return manager
}

func newRetryTestItemResultRepo(ctrl *gomock.Controller) *retryTestItemResultRepo {
	return &retryTestItemResultRepo{MockIExptItemResultRepo: repomocks.NewMockIExptItemResultRepo(ctrl), reset: repomocks.NewMockIExptRetryItemResetRepo(ctrl)}
}

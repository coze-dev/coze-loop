// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo"
	mock_repo "github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// UT-DATA-02: CountDatasets 只组装 SpaceID+Category(无 name/creator/分页字段)，
// repo 返回 total 时正确回传，repo 返回 DBErr 时透传 error(不返回 0)。
func TestDatasetServiceImpl_CountDatasets(t *testing.T) {
	t.Run("params 仅含 SpaceID+Category，正常返回 total", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
		svc := &DatasetServiceImpl{repo: mockRepo}

		var captured *repo.ListDatasetsParams
		mockRepo.EXPECT().
			CountDatasets(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, params *repo.ListDatasetsParams, _ ...repo.Option) (int64, error) {
				captured = params
				return int64(3), nil
			})

		total, err := svc.CountDatasets(context.Background(), &SearchDatasetsParam{
			SpaceID:  100,
			Category: entity.DatasetCategoryEvaluation,
		})
		assert.NoError(t, err)
		assert.Equal(t, int64(3), total)

		// 断言退化为列表过滤计数的字段一律不存在
		assert.NotNil(t, captured)
		assert.Equal(t, int64(100), captured.SpaceID)
		assert.Equal(t, entity.DatasetCategoryEvaluation, captured.Category)
		assert.Empty(t, captured.NameLike, "不应混入 name 模糊搜索")
		assert.Empty(t, captured.CreatedBys, "不应混入 creator 过滤")
		assert.Empty(t, captured.IDs, "不应混入 dataset_ids 过滤")
		assert.Empty(t, captured.BizCategorys, "不应混入 biz_category 过滤")
		assert.Nil(t, captured.Paginator, "不应携带分页器")
	})

	t.Run("repo 返回 DBErr 时透传 error，不返回 0 兜底", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
		svc := &DatasetServiceImpl{repo: mockRepo}

		dbErr := errorx.NewByCode(errno.CommonMySqlErrorCode)
		mockRepo.EXPECT().
			CountDatasets(gomock.Any(), gomock.Any()).
			Return(int64(0), dbErr)

		total, err := svc.CountDatasets(context.Background(), &SearchDatasetsParam{
			SpaceID:  100,
			Category: entity.DatasetCategoryEvaluation,
		})
		assert.Error(t, err)
		assert.Equal(t, dbErr, err)
		assert.Equal(t, int64(0), total)
	})
}

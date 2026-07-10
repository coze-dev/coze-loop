// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	mock_repo "github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
)

func TestDatasetServiceImpl_CountDatasetsAboveItemCount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	svc := &DatasetServiceImpl{repo: mockRepo}

	tests := []struct {
		name      string
		req       *CountDatasetsParam
		mockSetup func()
		want      int64
		wantErr   bool
	}{
		{
			name: "invalid workspace id",
			req:  &CountDatasetsParam{SpaceID: 0, ItemCountGt: 10},
			mockSetup: func() {
			},
			wantErr: true,
		},
		{
			name: "invalid item_count_gt",
			req:  &CountDatasetsParam{SpaceID: 1, ItemCountGt: -1},
			mockSetup: func() {
			},
			wantErr: true,
		},
		{
			name: "empty space returns 0",
			req:  &CountDatasetsParam{SpaceID: 1, ItemCountGt: 10},
			mockSetup: func() {
				mockRepo.EXPECT().ListDatasetIDs(gomock.Any(), gomock.Any()).
					Return([]int64{}, &pagination.PageResult{}, nil)
			},
			want: 0,
		},
		{
			name: "strict greater than threshold (=10 excluded, =11 included)",
			req:  &CountDatasetsParam{SpaceID: 1, ItemCountGt: 10},
			mockSetup: func() {
				// 单页：A item_count=10 不计, B=11 计入, C=9 不计。
				mockRepo.EXPECT().ListDatasetIDs(gomock.Any(), gomock.Any()).
					Return([]int64{1, 2, 3}, &pagination.PageResult{}, nil)
				mockRepo.EXPECT().MGetItemCount(gomock.Any(), int64(1), int64(2), int64(3)).
					Return(map[int64]int64{1: 10, 2: 11, 3: 9}, nil)
			},
			want: 1,
		},
		{
			name: "accumulate across multiple pages",
			req:  &CountDatasetsParam{SpaceID: 1, ItemCountGt: 10},
			mockSetup: func() {
				// 第一页有 next cursor，第二页无。跨页累加：第一页 2 个 + 第二页 1 个 = 3。
				gomock.InOrder(
					mockRepo.EXPECT().ListDatasetIDs(gomock.Any(), gomock.Any()).
						Return([]int64{1, 2}, &pagination.PageResult{Cursor: "next"}, nil),
					mockRepo.EXPECT().MGetItemCount(gomock.Any(), int64(1), int64(2)).
						Return(map[int64]int64{1: 50, 2: 200}, nil),
					mockRepo.EXPECT().ListDatasetIDs(gomock.Any(), gomock.Any()).
						Return([]int64{3, 4}, &pagination.PageResult{}, nil),
					mockRepo.EXPECT().MGetItemCount(gomock.Any(), int64(3), int64(4)).
						Return(map[int64]int64{3: 11, 4: 0}, nil),
				)
			},
			want: 3,
		},
		{
			name: "missing item_count in redis treated as 0 (not counted)",
			req:  &CountDatasetsParam{SpaceID: 1, ItemCountGt: 10},
			mockSetup: func() {
				mockRepo.EXPECT().ListDatasetIDs(gomock.Any(), gomock.Any()).
					Return([]int64{1, 2}, &pagination.PageResult{}, nil)
				// id=2 缺失 → map 中取零值 0，不计入。
				mockRepo.EXPECT().MGetItemCount(gomock.Any(), int64(1), int64(2)).
					Return(map[int64]int64{1: 100}, nil)
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := svc.CountDatasetsAboveItemCount(context.Background(), tt.req)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

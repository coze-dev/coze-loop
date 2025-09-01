// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/tag"
	mocks3 "github.com/coze-dev/coze-loop/backend/modules/data/domain/component/rpc/mocks"
	mocks4 "github.com/coze-dev/coze-loop/backend/modules/data/domain/component/userinfo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/entity"
	mocks2 "github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/service/mocks"
)

func TestTagApplicationImpl_ArchiveOptionTag(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tagSvc := mocks.NewMockITagService(ctrl)
	tagRepo := mocks2.NewMockITagAPI(ctrl)
	auth := mocks3.NewMockIAuthProvider(ctrl)
	usrSvc := mocks4.NewMockUserInfoService(ctrl)
	svc := NewTagApplicationImpl(tagSvc, tagRepo, auth, usrSvc)
	ctx := context.Background()

	tests := []struct {
		name        string
		req         *tag.ArchiveOptionTagRequest
		mockSetup   func()
		expectedErr error
	}{
		{
			name: "成功案例",
			req: &tag.ArchiveOptionTagRequest{
				WorkspaceID: 123,
				TagKeyID:    456,
				Name:        gptr.Of("test-tag"),
				Description: gptr.Of("test description"),
			},
			mockSetup: func() {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				tagSvc.EXPECT().GetLatestTag(gomock.Any(), int64(123), int64(456), gomock.Any()).Return(&entity.TagKey{
					TagType: entity.TagTypeOption,
				}, nil)
				tagSvc.EXPECT().ArchiveOptionTag(gomock.Any(), int64(123), int64(456), gomock.Any()).Return(nil)
			},
		},
		{
			name: "鉴权失败",
			req: &tag.ArchiveOptionTagRequest{
				WorkspaceID: 123,
				TagKeyID:    456,
			},
			mockSetup: func() {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(errors.New("auth failed"))
			},
			expectedErr: errors.New("auth failed"),
		},
		{
			name: "获取标签失败",
			req: &tag.ArchiveOptionTagRequest{
				WorkspaceID: 123,
				TagKeyID:    456,
			},
			mockSetup: func() {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				tagSvc.EXPECT().GetLatestTag(gomock.Any(), int64(123), int64(456), gomock.Any()).Return(nil, errors.New("get tag failed"))
			},
			expectedErr: errors.New("get tag failed"),
		},
		{
			name: "标签类型不是Option",
			req: &tag.ArchiveOptionTagRequest{
				WorkspaceID: 123,
				TagKeyID:    456,
			},
			mockSetup: func() {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				tagSvc.EXPECT().GetLatestTag(gomock.Any(), int64(123), int64(456), gomock.Any()).Return(&entity.TagKey{
					TagType: entity.TagTypeTag,
				}, nil)
			},
			expectedErr: errors.New("tag key is not option tag"),
		},
		{
			name: "归档失败",
			req: &tag.ArchiveOptionTagRequest{
				WorkspaceID: 123,
				TagKeyID:    456,
				Name:        gptr.Of("test-tag"),
				Description: gptr.Of("test description"),
			},
			mockSetup: func() {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				tagSvc.EXPECT().GetLatestTag(gomock.Any(), int64(123), int64(456), gomock.Any()).Return(&entity.TagKey{
					TagType: entity.TagTypeOption,
				}, nil)
				tagSvc.EXPECT().ArchiveOptionTag(gomock.Any(), int64(123), int64(456), gomock.Any()).Return(errors.New("archive failed"))
			},
			expectedErr: errors.New("archive failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.mockSetup()

			result, err := svc.ArchiveOptionTag(ctx, tt.req)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}
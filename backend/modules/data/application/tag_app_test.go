// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

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

func TestTagApplicationImpl_CreateTag(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tagSvc := mocks.NewMockITagService(ctrl)
	tagRepo := mocks2.NewMockITagAPI(ctrl)
	auth := mocks3.NewMockIAuthProvider(ctrl)
	usrSvc := mocks4.NewMockUserInfoService(ctrl)
	svc := NewTagApplicationImpl(tagSvc, tagRepo, auth, usrSvc)
	ctx := context.Background()
	tests := []struct {
		name      string
		req       *tag.CreateTagRequest
		mockSetup func()
		wantID    int64
		wantErr   bool
	}{
		{
			name: "authorized failed",
			req: &tag.CreateTagRequest{
				WorkspaceID: 123,
			},
			mockSetup: func() {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(errors.New("123"))
			},
			wantErr: true,
		},
		{
			name: "tag service create tag failed",
			req: &tag.CreateTagRequest{
				WorkspaceID: 123,
			},
			mockSetup: func() {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				tagSvc.EXPECT().CreateTag(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(0), errors.New("123"))
			},
			wantErr: true,
		},
		{
			name: "normal case",
			req: &tag.CreateTagRequest{
				WorkspaceID: 123,
			},
			mockSetup: func() {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				tagSvc.EXPECT().CreateTag(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(123), nil)
			},
			wantErr: false,
			wantID:  int64(123),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			resp, err := svc.CreateTag(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateTag() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				assert.Equal(t, tt.wantID, *resp.TagKeyID)
			}
		})
	}
}

func TestTagApplicationImpl_UpdateTag(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tagSvc := mocks.NewMockITagService(ctrl)
	tagRepo := mocks2.NewMockITagAPI(ctrl)
	auth := mocks3.NewMockIAuthProvider(ctrl)
	usrSvc := mocks4.NewMockUserInfoService(ctrl)
	svc := NewTagApplicationImpl(tagSvc, tagRepo, auth, usrSvc)
	ctx := context.Background()

	tests := []struct {
		name      string
		req       *tag.UpdateTagRequest
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "auth failed",
			mockSetup: func() {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(errors.New("123"))
			},
			req:     &tag.UpdateTagRequest{},
			wantErr: true,
		},
		{
			name: "update tag failed",
			mockSetup: func() {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				tagSvc.EXPECT().UpdateTag(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("123"))
			},
			req:     &tag.UpdateTagRequest{},
			wantErr: true,
		},
		{
			name: "normal case",
			mockSetup: func() {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				tagSvc.EXPECT().UpdateTag(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			req:     &tag.UpdateTagRequest{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			_, err := svc.UpdateTag(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateTag() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTagApplicationImpl_GetTagSpec(t *testing.T) {
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
		req         *tag.GetTagSpecRequest
		mockSetup   func()
		wantErr     bool
		h, w, total int64
	}{
		{
			name: "auth failed",
			req:  &tag.GetTagSpecRequest{},
			mockSetup: func() {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(errors.New("123"))
			},
			wantErr: true,
		},
		{
			name: "get tag spec failed",
			req:  &tag.GetTagSpecRequest{},
			mockSetup: func() {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				tagSvc.EXPECT().GetTagSpec(gomock.Any(), gomock.Any()).Return(int64(0), int64(0), int64(0), errors.New("123"))
			},
			wantErr: true,
		},
		{
			name: "normal case",
			req:  &tag.GetTagSpecRequest{},
			mockSetup: func() {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				tagSvc.EXPECT().GetTagSpec(gomock.Any(), gomock.Any()).Return(int64(1), int64(20), int64(20), nil)
			},
			wantErr: false,
			h:       int64(1),
			w:       int64(20),
			total:   int64(20),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			resp, err := svc.GetTagSpec(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTagSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				assert.Equal(t, tt.w, resp.GetMaxWidth())
				assert.Equal(t, tt.h, resp.GetMaxHeight())
				assert.Equal(t, tt.total, resp.GetMaxTotal())
			}
		})
	}
}

func TestTagApplicationImpl_BatchGetTags(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tagSvc := mocks.NewMockITagService(ctrl)
	tagRepo := mocks2.NewMockITagAPI(ctrl)
	auth := mocks3.NewMockIAuthProvider(ctrl)
	usrSvc := mocks4.NewMockUserInfoService(ctrl)
	svc := NewTagApplicationImpl(tagSvc, tagRepo, auth, usrSvc)
	ctx := context.Background()

	tests := []struct {
		name          string
		req           *tag.BatchGetTagsRequest
		mockSetup     func()
		wantErr       bool
		wantTagKeyIDs []int64
	}{
		{
			name: "auth failed",
			req:  &tag.BatchGetTagsRequest{},
			mockSetup: func() {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(errors.New("123"))
			},
			wantErr: true,
		},
		{
			name: "MGetTagKeys failed",
			req:  &tag.BatchGetTagsRequest{},
			mockSetup: func() {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				tagSvc.EXPECT().BatchGetTagsByTagKeyIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("123"))
			},
			wantErr: true,
		},
		{
			name: "normal case",
			req:  &tag.BatchGetTagsRequest{},
			mockSetup: func() {
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				tagSvc.EXPECT().BatchGetTagsByTagKeyIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{
						TagKeyID:   123,
						VersionNum: gptr.Of(int32(12)),
					},
					{
						TagKeyID:   124,
						VersionNum: gptr.Of(int32(1)),
					},
				}, nil)
				usrSvc.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).Return()
			},
			wantErr:       false,
			wantTagKeyIDs: []int64{123, 124},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			resp, err := svc.BatchGetTags(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("BatchGetTags() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				assert.Equal(t, len(tt.wantTagKeyIDs), len(resp.GetTagInfoList()))
				for i := 0; i < len(tt.wantTagKeyIDs); i++ {
					assert.Equal(t, tt.wantTagKeyIDs[i], resp.GetTagInfoList()[i].GetTagKeyID())
				}
			}
		})
	}
}

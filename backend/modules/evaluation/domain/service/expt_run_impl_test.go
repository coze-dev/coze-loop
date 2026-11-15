// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

func TestQuotaServiceImpl_ReleaseExptRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockQuotaRepo := repoMocks.NewMockQuotaRepo(ctrl)
	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	service := &QuotaServiceImpl{
		QuotaRepo: mockQuotaRepo,
		Configer:  mockConfiger,
	}

	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}

	tests := []struct {
		name      string
		cur       *entity.QuotaSpaceExpt
		exptID    int64
		spaceID   int64
		setupMock func()
		wantErr   bool
	}{
		{
			name:    "cur为nil，无需删除，返回nil",
			cur:     nil,
			exptID:  1,
			spaceID: 100,
			setupMock: func() {
				mockQuotaRepo.EXPECT().CreateOrUpdate(gomock.Any(), int64(100), gomock.Any(), session).DoAndReturn(
					func(_ context.Context, _ int64, updater func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
						newCur, changed, err := updater(nil)
						assert.Nil(t, newCur)
						assert.False(t, changed)
						assert.NoError(t, err)
						return nil
					},
				)
			},
			wantErr: false,
		},
		{
			name:    "cur.ExptID2RunTime为nil，无需删除，返回nil",
			cur:     &entity.QuotaSpaceExpt{ExptID2RunTime: nil},
			exptID:  2,
			spaceID: 101,
			setupMock: func() {
				mockQuotaRepo.EXPECT().CreateOrUpdate(gomock.Any(), int64(101), gomock.Any(), session).DoAndReturn(
					func(_ context.Context, _ int64, updater func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
						cur := &entity.QuotaSpaceExpt{ExptID2RunTime: nil}
						newCur, changed, err := updater(cur)
						assert.Equal(t, cur, newCur)
						assert.False(t, changed)
						assert.NoError(t, err)
						return nil
					},
				)
			},
			wantErr: false,
		},
		{
			name:    "删除成功，返回true",
			cur:     &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{3: 123, 4: 456}},
			exptID:  3,
			spaceID: 102,
			setupMock: func() {
				mockQuotaRepo.EXPECT().CreateOrUpdate(gomock.Any(), int64(102), gomock.Any(), session).DoAndReturn(
					func(_ context.Context, _ int64, updater func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
						cur := &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{3: 123, 4: 456}}
						newCur, changed, err := updater(cur)
						assert.Equal(t, &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{4: 456}}, newCur)
						assert.True(t, changed)
						assert.NoError(t, err)
						return nil
					},
				)
			},
			wantErr: false,
		},
		{
			name:    "CreateOrUpdate返回错误",
			cur:     &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{5: 789}},
			exptID:  6,
			spaceID: 103,
			setupMock: func() {
				mockQuotaRepo.EXPECT().CreateOrUpdate(gomock.Any(), int64(103), gomock.Any(), session).Return(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			err := service.ReleaseExptRun(ctx, tt.exptID, tt.spaceID, session)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestQuotaServiceImpl_AllowExptRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockQuotaRepo := repoMocks.NewMockQuotaRepo(ctrl)
	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	service := &QuotaServiceImpl{
		QuotaRepo: mockQuotaRepo,
		Configer:  mockConfiger,
	}

	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}

	fakeConf := &entity.ExptExecConf{
		ZombieIntervalSecond: 10,
		SpaceExptConcurLimit: 2,
	}

	tests := []struct {
		name      string
		cur       *entity.QuotaSpaceExpt
		exptID    int64
		spaceID   int64
		setupMock func()
		wantErr   bool
	}{
		{
			name:    "cur为nil，直接插入",
			cur:     nil,
			exptID:  1,
			spaceID: 100,
			setupMock: func() {
				mockConfiger.EXPECT().GetExptExecConf(gomock.Any(), int64(100)).Return(fakeConf).Times(2)
				mockQuotaRepo.EXPECT().CreateOrUpdate(gomock.Any(), int64(100), gomock.Any(), session).DoAndReturn(
					func(_ context.Context, _ int64, updater func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
						newCur, changed, err := updater(nil)
						assert.NotNil(t, newCur)
						assert.True(t, changed)
						assert.NoError(t, err)
						return nil
					},
				)
			},
			wantErr: false,
		},
		{
			name:    "并发数已满，返回错误",
			cur:     &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{2: 123, 3: 456}},
			exptID:  4,
			spaceID: 101,
			setupMock: func() {
				mockConfiger.EXPECT().GetExptExecConf(gomock.Any(), int64(101)).Return(fakeConf).Times(2)
				mockQuotaRepo.EXPECT().CreateOrUpdate(gomock.Any(), int64(101), gomock.Any(), session).DoAndReturn(
					func(_ context.Context, _ int64, updater func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
						cur := &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{2: 123, 3: 456}}
						newCur, changed, err := updater(cur)
						assert.Nil(t, newCur)
						assert.False(t, changed)
						assert.Error(t, err)
						return err
					},
				)
			},
			wantErr: true,
		},
		{
			name:    "已存在实验重试在200边界不被误拒",
			cur:     &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{2: 1, 3: 2}},
			exptID:  2,
			spaceID: 104,
			setupMock: func() {
				mockConfiger.EXPECT().GetExptExecConf(gomock.Any(), int64(104)).Return(fakeConf).Times(2)
				mockQuotaRepo.EXPECT().CreateOrUpdate(gomock.Any(), int64(104), gomock.Any(), session).DoAndReturn(
					func(_ context.Context, _ int64, updater func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
						cur := &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{2: 1, 3: 2}}
						newCur, changed, err := updater(cur)
						assert.NotNil(t, newCur)
						assert.True(t, changed)
						assert.NoError(t, err, "an already admitted experiment must be idempotent even at the limit")
						return nil
					},
				)
			},
			wantErr: false,
		},
		{
			name:    "正常插入并清理zombie",
			cur:     &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{5: 1}},
			exptID:  6,
			spaceID: 102,
			setupMock: func() {
				mockConfiger.EXPECT().GetExptExecConf(gomock.Any(), int64(102)).Return(fakeConf).Times(2)
				mockQuotaRepo.EXPECT().CreateOrUpdate(gomock.Any(), int64(102), gomock.Any(), session).DoAndReturn(
					func(_ context.Context, _ int64, updater func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
						cur := &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{5: 1}}
						newCur, changed, err := updater(cur)
						assert.NotNil(t, newCur)
						assert.True(t, changed)
						assert.NoError(t, err)
						return nil
					},
				)
			},
			wantErr: false,
		},
		{
			name:    "CreateOrUpdate返回错误",
			cur:     &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{7: 789}},
			exptID:  8,
			spaceID: 103,
			setupMock: func() {
				mockConfiger.EXPECT().GetExptExecConf(gomock.Any(), int64(103)).Return(fakeConf).Times(2)
				mockQuotaRepo.EXPECT().CreateOrUpdate(gomock.Any(), int64(103), gomock.Any(), session).Return(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			err := service.AllowExptRun(ctx, tt.exptID, tt.spaceID, session)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

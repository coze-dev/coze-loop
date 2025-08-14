// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	db2 "github.com/coze-dev/coze-loop/backend/infra/db"
	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	mocks2 "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/conf"
	mocks3 "github.com/coze-dev/coze-loop/backend/modules/data/domain/component/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
)

func TestTagServiceImpl_CreateTag(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tagRepo := mocks.NewMockITagAPI(ctrl)
	db := dbmock.NewMockProvider(ctrl)
	locker := mocks2.NewMockILocker(ctrl)
	cfg := mocks3.NewMockIConfig(ctrl)

	svc := NewTagServiceImpl(
		tagRepo,
		db,
		locker,
		cfg,
	)
	ctx := context.Background()

	tests := []struct {
		name      string
		spaceID   int64
		key       *entity.TagKey
		mockSetup func()
		wantErr   bool
		want      int64
	}{
		{
			name:    "tag key is illegal",
			spaceID: 123,
			key:     &entity.TagKey{},
			wantErr: true,
			want:    0,
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec:  nil,
					SpecsBySpace: nil,
				})
			},
		},
		{
			name:    "lock meet error",
			spaceID: 123,
			key: &entity.TagKey{
				TagType:        entity.TagTypeTag,
				TagContentType: entity.TagContentTypeFreeText,
				TagKeyName:     "123",
			},
			wantErr: true,
			want:    0,
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxWidth:  20,
						MaxHeight: 1,
					},
					SpecsBySpace: nil,
				})
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, errors.New("123"))
			},
		},
		{
			name:    "lock failed",
			spaceID: 123,
			key: &entity.TagKey{
				TagType:        entity.TagTypeTag,
				TagContentType: entity.TagContentTypeFreeText,
				TagKeyName:     "123",
			},
			wantErr: true,
			want:    0,
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxWidth:  20,
						MaxHeight: 1,
					},
					SpecsBySpace: nil,
				})
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
			},
		},
		{
			name:    "MGetTagKeys failed",
			spaceID: 123,
			key: &entity.TagKey{
				TagType:        entity.TagTypeTag,
				TagContentType: entity.TagContentTypeFreeText,
				TagKeyName:     "123",
			},
			wantErr: true,
			want:    0,
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxWidth:  20,
						MaxHeight: 1,
					},
					SpecsBySpace: nil,
				})
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("123"))
			},
		},
		{
			name:    "tag name is already existed",
			spaceID: 123,
			key: &entity.TagKey{
				TagType:        entity.TagTypeTag,
				TagContentType: entity.TagContentTypeFreeText,
				TagKeyName:     "123",
			},
			wantErr: true,
			want:    0,
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxWidth:  20,
						MaxHeight: 1,
					},
					SpecsBySpace: nil,
				})
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{{}, {}}, nil, nil)
			},
		},
		{
			name:    "version check failed",
			spaceID: 123,
			key: &entity.TagKey{
				TagType:        entity.TagTypeTag,
				TagContentType: entity.TagContentTypeFreeText,
				TagKeyName:     "123",
				Version:        gptr.Of("asdfasdf"),
			},
			wantErr: true,
			want:    0,
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxWidth:  20,
						MaxHeight: 1,
					},
					SpecsBySpace: nil,
				})
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{}, nil, nil)
			},
		},
		{
			name:    "create tag key failed",
			spaceID: 123,
			key: &entity.TagKey{
				TagType:        entity.TagTypeTag,
				TagContentType: entity.TagContentTypeFreeText,
				TagKeyName:     "123",
				Version:        gptr.Of("1.1.1"),
			},
			wantErr: true,
			want:    0,
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxWidth:  20,
						MaxHeight: 1,
					},
					SpecsBySpace: nil,
				})
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{}, nil, nil)
				db.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db2.Option) error {
					return fn(&gorm.DB{})
				})
				tagRepo.EXPECT().MCreateTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("123"))
			},
		},
		{
			name:    "create tag values failed",
			spaceID: 123,
			key: &entity.TagKey{
				TagType:        entity.TagTypeTag,
				TagContentType: entity.TagContentTypeCategorical,
				TagKeyName:     "123",
				Version:        gptr.Of("1.1.1"),
				TagValues: []*entity.TagValue{
					{
						TagValueName: "123",
					},
				},
			},
			wantErr: true,
			want:    0,
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxWidth:  20,
						MaxHeight: 1,
					},
					SpecsBySpace: nil,
				})
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{}, nil, nil)
				db.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db2.Option) error {
					return fn(&gorm.DB{})
				})
				tagRepo.EXPECT().MCreateTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tagRepo.EXPECT().MCreateTagValues(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("123"))
			},
		},
		{
			name:    "normal case",
			spaceID: 123,
			key: &entity.TagKey{
				TagType:        entity.TagTypeTag,
				TagContentType: entity.TagContentTypeCategorical,
				TagKeyName:     "123",
				Version:        gptr.Of("1.1.1"),
				TagValues: []*entity.TagValue{
					{
						TagValueName: "123",
						Children: []*entity.TagValue{
							{
								TagValueName: "234",
							},
						},
					},
				},
			},
			wantErr: false,
			want:    0,
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxWidth:  20,
						MaxHeight: 2,
					},
					SpecsBySpace: nil,
				})
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{}, nil, nil)
				db.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db2.Option) error {
					return fn(&gorm.DB{})
				})
				tagRepo.EXPECT().MCreateTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tagRepo.EXPECT().MCreateTagValues(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			id, err := svc.CreateTag(ctx, tt.spaceID, tt.key)
			assert.Equal(t, id, tt.want)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateTag() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTagServiceImpl_GetAllTagKeyVersionsByKeyID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tagRepo := mocks.NewMockITagAPI(ctrl)
	db := dbmock.NewMockProvider(ctrl)
	locker := mocks2.NewMockILocker(ctrl)
	cfg := mocks3.NewMockIConfig(ctrl)

	svc := NewTagServiceImpl(
		tagRepo,
		db,
		locker,
		cfg,
	)
	ctx := context.Background()

	tests := []struct {
		name      string
		spaceID   int64
		tagKeyID  int64
		mockSetup func()
		wantErr   bool
		wantLen   int
	}{
		{
			name:     "MGetTagKeys failed",
			spaceID:  123,
			tagKeyID: 234,
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("123"))
			},
			wantErr: true,
		},
		{
			name:     "normal case",
			spaceID:  123,
			tagKeyID: 234,
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{},
					{},
				}, &pagination.PageResult{Cursor: "123"}, nil)
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{},
				}, &pagination.PageResult{}, nil)
			},
			wantErr: false,
			wantLen: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			res, err := svc.GetAllTagKeyVersionsByKeyID(ctx, tt.spaceID, tt.tagKeyID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAllTagKeyVersionsByKeyID() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, len(res), tt.wantLen)
		})
	}
}

func flattenTagValues(values []*entity.TagValue) []*entity.TagValue {
	var res []*entity.TagValue
	now := values
	for len(now) > 0 {
		res = append(res, now...)
		var next []*entity.TagValue
		for _, v := range now {
			next = append(next, v.Children...)
		}
		now = next
	}
	return res
}

func TestTagServiceImpl_GetAndBuildTagValues(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tagRepo := mocks.NewMockITagAPI(ctrl)
	db := dbmock.NewMockProvider(ctrl)
	locker := mocks2.NewMockILocker(ctrl)
	cfg := mocks3.NewMockIConfig(ctrl)

	svc := NewTagServiceImpl(
		tagRepo,
		db,
		locker,
		cfg,
	)
	ctx := context.Background()

	tests := []struct {
		name              string
		spaceID, tagKeyID int64
		versionNum        int32
		mockSetup         func()
		wantErr           bool
		want              []*entity.TagValue
	}{
		{
			name:       "MGetTagValue failed",
			spaceID:    123,
			tagKeyID:   123,
			versionNum: 123,
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("123"))
			},
			wantErr: true,
		},
		{
			name:       "normal case",
			spaceID:    123,
			tagKeyID:   123,
			versionNum: 123,
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagValue{
					{
						TagValueID: 1,
					},
					{
						TagValueID:    2,
						ParentValueID: 1,
					},
					{
						TagValueID:    3,
						ParentValueID: 1,
					},
				}, &pagination.PageResult{
					Cursor: "123",
				}, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagValue{
					{
						TagValueID: 4,
					},
					{
						TagValueID:    5,
						ParentValueID: 3,
					},
				}, &pagination.PageResult{}, nil)
			},
			want: []*entity.TagValue{
				{
					TagValueID: 1,
					Children: []*entity.TagValue{
						{
							TagValueID: 2,
						},
						{
							TagValueID: 3,
							Children: []*entity.TagValue{
								{
									TagValueID: 5,
								},
							},
						},
					},
				},
				{
					TagValueID: 4,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			res, err := svc.GetAndBuildTagValues(ctx, tt.spaceID, tt.tagKeyID, tt.versionNum)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAndBuildTagValues() error = %v, wantErr %v", err, tt.wantErr)
			}
			resKeys := flattenTagValues(res)
			wantKeys := flattenTagValues(tt.want)
			assert.EqualValues(t, len(resKeys), len(wantKeys))
			for i := 0; i < len(resKeys); i++ {
				assert.Equal(t, resKeys[i].TagValueID, wantKeys[i].TagValueID)
			}
		})
	}
}

func TestTagServiceImpl_GetLatestTag(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tagRepo := mocks.NewMockITagAPI(ctrl)
	db := dbmock.NewMockProvider(ctrl)
	locker := mocks2.NewMockILocker(ctrl)
	cfg := mocks3.NewMockIConfig(ctrl)

	svc := NewTagServiceImpl(
		tagRepo,
		db,
		locker,
		cfg,
	)
	ctx := context.Background()

	tests := []struct {
		name      string
		spaceID   int64
		tagKeyID  int64
		mockSetup func()
		wantErr   bool
		want      *entity.TagKey
	}{
		{
			name:     "MGetTagKeys failed",
			spaceID:  123,
			tagKeyID: 123,
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("123"))
			},
			wantErr: true,
		},
		{
			name:     "tag key is not existed",
			spaceID:  123,
			tagKeyID: 123,
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil)
			},
			wantErr: true,
		},
		{
			name:     "get and build tag values failed",
			spaceID:  123,
			tagKeyID: 123,
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{
						VersionNum: gptr.Of(int32(123)),
					},
				}, nil, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("123"))
			},
			wantErr: true,
		},
		{
			name:     "normal case",
			spaceID:  123,
			tagKeyID: 123,
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{
						VersionNum: gptr.Of(int32(123)),
						TagKeyName: "321",
					},
				}, nil, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagValue{
					{},
				}, &pagination.PageResult{}, nil)
			},
			wantErr: false,
			want:    &entity.TagKey{TagKeyName: "321"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			res, err := svc.GetLatestTag(ctx, tt.spaceID, tt.tagKeyID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetLatestTag() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				assert.Equal(t, res.TagKeyName, tt.want.TagKeyName)
			}
		})
	}
}

func TestTagServiceImpl_UpdateTag(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tagRepo := mocks.NewMockITagAPI(ctrl)
	db := dbmock.NewMockProvider(ctrl)
	locker := mocks2.NewMockILocker(ctrl)
	cfg := mocks3.NewMockIConfig(ctrl)

	svc := NewTagServiceImpl(
		tagRepo,
		db,
		locker,
		cfg,
	)
	ctx := context.Background()

	tests := []struct {
		name      string
		mockSetup func()
		patch     *entity.TagKey
		wantErr   bool
	}{
		{
			name: "validate failed",
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxHeight: 2,
						MaxWidth:  30,
					},
				})
			},
			wantErr: true,
		},
		{
			name: "tag name is existed",
			patch: &entity.TagKey{
				TagKeyName:     "123",
				TagType:        entity.TagTypeTag,
				TagContentType: entity.TagContentTypeCategorical,
			},
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxHeight: 2,
						MaxWidth:  30,
					},
				})
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{}, {}, {},
				}, nil, nil)
			},
			wantErr: true,
		},
		{
			name: "lock failed",
			patch: &entity.TagKey{
				TagKeyName:     "123",
				TagType:        entity.TagTypeTag,
				TagContentType: entity.TagContentTypeCategorical,
			},
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxHeight: 2,
						MaxWidth:  30,
					},
				})
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{}, nil, nil)
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, errors.New("123"))
			},
			wantErr: true,
		},
		{
			name: "lock failed 2",
			patch: &entity.TagKey{
				TagKeyName:     "123",
				TagType:        entity.TagTypeTag,
				TagContentType: entity.TagContentTypeCategorical,
			},
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxHeight: 2,
						MaxWidth:  30,
					},
				})
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{}, nil, nil)
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
			},
			wantErr: true,
		},
		{
			name: "check version failed",
			patch: &entity.TagKey{
				TagKeyName:     "123",
				TagType:        entity.TagTypeTag,
				TagContentType: entity.TagContentTypeCategorical,
				Version:        gptr.Of("123"),
			},
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxHeight: 2,
						MaxWidth:  30,
					},
				})
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{}, nil, nil)
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{{
					Version:    gptr.Of("1.1.2"),
					VersionNum: gptr.Of(int32(122)),
				}}, nil, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &pagination.PageResult{}, nil)
			},
			wantErr: true,
		},
		{
			name: "update tag status failed",
			patch: &entity.TagKey{
				TagKeyName:     "123",
				TagType:        entity.TagTypeTag,
				TagContentType: entity.TagContentTypeCategorical,
				Version:        gptr.Of("1.1.3"),
			},
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxHeight: 2,
						MaxWidth:  30,
					},
				})
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{}, nil, nil)
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{{
					Version:    gptr.Of("1.1.2"),
					VersionNum: gptr.Of(int32(122)),
					CreatedBy:  gptr.Of("123"),
				}}, nil, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &pagination.PageResult{}, nil)
				db.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db2.Option) error {
					return fn(&gorm.DB{Config: &gorm.Config{}})
				}).Times(2)
				tagRepo.EXPECT().UpdateTagKeysStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("123"))
			},
			wantErr: true,
		},
		{
			name: "create tag keys failed",
			patch: &entity.TagKey{
				TagKeyName:     "123",
				TagType:        entity.TagTypeTag,
				TagContentType: entity.TagContentTypeCategorical,
				Version:        gptr.Of("1.1.3"),
			},
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxHeight: 2,
						MaxWidth:  30,
					},
				})
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{}, nil, nil)
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{{
					Version:    gptr.Of("1.1.2"),
					VersionNum: gptr.Of(int32(122)),
					CreatedBy:  gptr.Of("123"),
				}}, nil, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &pagination.PageResult{}, nil)
				db.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db2.Option) error {
					return fn(&gorm.DB{Config: &gorm.Config{}})
				}).Times(2)
				tagRepo.EXPECT().UpdateTagKeysStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tagRepo.EXPECT().UpdateTagValuesStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tagRepo.EXPECT().MCreateTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("123"))
			},
			wantErr: true,
		},
		{
			name: "create tag values failed",
			patch: &entity.TagKey{
				TagKeyName:     "123",
				TagType:        entity.TagTypeTag,
				TagContentType: entity.TagContentTypeCategorical,
				Version:        gptr.Of("1.1.3"),
				TagValues: []*entity.TagValue{
					{
						TagValueName: "123",
					},
				},
			},
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxHeight: 2,
						MaxWidth:  30,
					},
				})
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{}, nil, nil)
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{{
					Version:    gptr.Of("1.1.2"),
					VersionNum: gptr.Of(int32(122)),
					CreatedBy:  gptr.Of("123"),
				}}, nil, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &pagination.PageResult{}, nil)
				db.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db2.Option) error {
					return fn(&gorm.DB{Config: &gorm.Config{}})
				}).Times(2)
				tagRepo.EXPECT().UpdateTagKeysStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tagRepo.EXPECT().UpdateTagValuesStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tagRepo.EXPECT().MCreateTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tagRepo.EXPECT().MCreateTagValues(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("123"))
			},
			wantErr: true,
		},
		{
			name: "normal case",
			patch: &entity.TagKey{
				TagKeyName:     "123",
				TagType:        entity.TagTypeTag,
				TagContentType: entity.TagContentTypeCategorical,
				Version:        gptr.Of("1.1.3"),
				TagValues: []*entity.TagValue{
					{
						TagValueName: "123",
						Children: []*entity.TagValue{
							{
								TagValueName: "234",
							},
						},
					},
				},
			},
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxHeight: 2,
						MaxWidth:  30,
					},
				})
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{}, nil, nil)
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{{
					Version:    gptr.Of("1.1.2"),
					VersionNum: gptr.Of(int32(122)),
					CreatedBy:  gptr.Of("123"),
				}}, nil, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &pagination.PageResult{}, nil)
				db.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db2.Option) error {
					return fn(&gorm.DB{Config: &gorm.Config{}})
				}).Times(2)
				tagRepo.EXPECT().UpdateTagKeysStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tagRepo.EXPECT().UpdateTagValuesStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tagRepo.EXPECT().MCreateTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tagRepo.EXPECT().MCreateTagValues(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := svc.UpdateTag(ctx, 123, 123, tt.patch)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateTag() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTagServiceImpl_UpdateTagStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tagRepo := mocks.NewMockITagAPI(ctrl)
	db := dbmock.NewMockProvider(ctrl)
	locker := mocks2.NewMockILocker(ctrl)
	cfg := mocks3.NewMockIConfig(ctrl)

	svc := NewTagServiceImpl(
		tagRepo,
		db,
		locker,
		cfg,
	)
	ctx := context.Background()

	tests := []struct {
		name      string
		needLock  bool
		mockSetup func()
		wantErr   bool
	}{
		{
			name:     "lock failed",
			needLock: true,
			wantErr:  true,
			mockSetup: func() {
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, errors.New("123"))
			},
		},
		{
			name:     "lock failed 2",
			needLock: true,
			wantErr:  true,
			mockSetup: func() {
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
			},
		},
		{
			name:     "update tag keys status failed",
			needLock: true,
			wantErr:  true,
			mockSetup: func() {
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				db.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db2.Option) error {
					return fn(&gorm.DB{})
				})
				tagRepo.EXPECT().UpdateTagKeysStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("123"))
			},
		},
		{
			name:     "update tag values status failed",
			needLock: true,
			wantErr:  true,
			mockSetup: func() {
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				db.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db2.Option) error {
					return fn(&gorm.DB{})
				})
				tagRepo.EXPECT().UpdateTagKeysStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tagRepo.EXPECT().UpdateTagValuesStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("123"))
			},
		},
		{
			name:     "normal case",
			needLock: true,
			wantErr:  false,
			mockSetup: func() {
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				db.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db2.Option) error {
					return fn(&gorm.DB{})
				})
				tagRepo.EXPECT().UpdateTagKeysStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tagRepo.EXPECT().UpdateTagValuesStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := svc.UpdateTagStatus(ctx, 123, 123, 123, entity.TagStatusActive, tt.needLock, true)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateTagStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTagServiceImpl_GetTagSpec(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tagRepo := mocks.NewMockITagAPI(ctrl)
	db := dbmock.NewMockProvider(ctrl)
	locker := mocks2.NewMockILocker(ctrl)
	cfg := mocks3.NewMockIConfig(ctrl)

	svc := NewTagServiceImpl(
		tagRepo,
		db,
		locker,
		cfg,
	)
	ctx := context.Background()

	tests := []struct {
		name                          string
		spaceID                       int64
		mockSetup                     func()
		wantErr                       bool
		maxHeight, maxWidth, maxTotal int64
	}{
		{
			name:    "tag spec is nil",
			spaceID: 123,
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(nil)
			},
			wantErr:   false,
			maxWidth:  20,
			maxHeight: 1,
			maxTotal:  20,
		},
		{
			name:    "return nil",
			spaceID: 123,
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec:  nil,
					SpecsBySpace: nil,
				})
			},
			wantErr:   false,
			maxWidth:  20,
			maxHeight: 1,
			maxTotal:  20,
		},
		{
			name:    "default spec",
			spaceID: 123,
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxHeight: 2,
						MaxWidth:  30,
					},
					SpecsBySpace: nil,
				})
			},
			wantErr:   false,
			maxWidth:  30,
			maxHeight: 2,
			maxTotal:  60,
		},
		{
			name:    "space spec",
			spaceID: 123,
			mockSetup: func() {
				cfg.EXPECT().GetTagSpec().Return(&conf.TagSpec{
					DefaultSpec: &entity.TagSpec{
						MaxHeight: 2,
						MaxWidth:  30,
					},
					SpecsBySpace: map[int64]*entity.TagSpec{
						123: {
							MaxHeight: 3,
							MaxWidth:  40,
						},
					},
				})
			},
			wantErr:   false,
			maxWidth:  40,
			maxHeight: 3,
			maxTotal:  120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			h, w, total, err := svc.GetTagSpec(ctx, tt.spaceID)
			assert.Equal(t, h, tt.maxHeight)
			assert.Equal(t, w, tt.maxWidth)
			assert.Equal(t, total, tt.maxTotal)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTagSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTagServiceImpl_BatchUpdateTagStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tagRepo := mocks.NewMockITagAPI(ctrl)
	db := dbmock.NewMockProvider(ctrl)
	locker := mocks2.NewMockILocker(ctrl)
	cfg := mocks3.NewMockIConfig(ctrl)

	svc := NewTagServiceImpl(
		tagRepo,
		db,
		locker,
		cfg,
	)
	ctx := context.Background()

	tests := []struct {
		name      string
		spaceID   int64
		tagKeyIDs []int64
		toStatus  entity.TagStatus
		mockSetup func()
		wantErr   bool
		want      map[int64]string
	}{
		{
			name:      "toStatus is illegal",
			spaceID:   int64(123),
			tagKeyIDs: []int64{234, 345},
			toStatus:  entity.TagStatusUndefined,
			mockSetup: func() {},
			wantErr:   true,
		},
		{
			name:      "tag keys is empty",
			spaceID:   int64(123),
			tagKeyIDs: []int64{},
			toStatus:  entity.TagStatusActive,
			mockSetup: func() {},
			wantErr:   true,
		},
		{
			name:      "lock failed",
			spaceID:   int64(123),
			tagKeyIDs: []int64{123},
			toStatus:  entity.TagStatusActive,
			mockSetup: func() {
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, errors.New("123"))
			},
			wantErr: false,
			want: map[int64]string{
				123: "123",
			},
		},
		{
			name:      "other updating operation is processing",
			spaceID:   int64(123),
			tagKeyIDs: []int64{123},
			toStatus:  entity.TagStatusActive,
			mockSetup: func() {
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
			},
			wantErr: false,
			want: map[int64]string{
				123: "other udpating operation is processing",
			},
		},
		{
			name:      "get latest tag failed",
			spaceID:   int64(123),
			tagKeyIDs: []int64{123},
			toStatus:  entity.TagStatusActive,
			mockSetup: func() {
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("123"))
			},
			wantErr: false,
			want: map[int64]string{
				123: "123",
			},
		},
		{
			name:      "status is same with pre status",
			spaceID:   int64(123),
			tagKeyIDs: []int64{123},
			toStatus:  entity.TagStatusActive,
			mockSetup: func() {
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{
						Status:     entity.TagStatusActive,
						VersionNum: gptr.Of(int32(123)),
					},
				}, &pagination.PageResult{}, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &pagination.PageResult{}, nil)
			},
			wantErr: false,
			want: map[int64]string{
				int64(123): "no need to update status",
			},
		},
		{
			name:      "increase version failed",
			spaceID:   int64(123),
			tagKeyIDs: []int64{123},
			toStatus:  entity.TagStatusActive,
			mockSetup: func() {
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{
						Status:     entity.TagStatusInactive,
						VersionNum: gptr.Of(int32(123)),
						Version:    gptr.Of("123123"),
					},
				}, &pagination.PageResult{}, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &pagination.PageResult{}, nil)
			},
			wantErr: false,
			want: map[int64]string{
				int64(123): "is invalid",
			},
		},
		{
			name:      "normal case",
			spaceID:   int64(123),
			tagKeyIDs: []int64{123},
			toStatus:  entity.TagStatusActive,
			mockSetup: func() {
				locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{
						Status:     entity.TagStatusInactive,
						VersionNum: gptr.Of(int32(123)),
					},
				}, &pagination.PageResult{}, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &pagination.PageResult{}, nil)
				db.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
			want:    map[int64]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			resp, err := svc.BatchUpdateTagStatus(ctx, tt.spaceID, tt.tagKeyIDs, tt.toStatus)
			if (err != nil) != tt.wantErr {
				t.Errorf("BatchUpdateTagStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				assert.Equal(t, len(tt.want), len(resp))
				for k, v := range tt.want {
					assert.Contains(t, resp[k], v)
				}
			}
		})
	}
}

func TestTagServiceImpl_SearchTags(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tagRepo := mocks.NewMockITagAPI(ctrl)
	db := dbmock.NewMockProvider(ctrl)
	locker := mocks2.NewMockILocker(ctrl)
	cfg := mocks3.NewMockIConfig(ctrl)

	svc := NewTagServiceImpl(
		tagRepo,
		db,
		locker,
		cfg,
	)
	ctx := context.Background()
	tests := []struct {
		name          string
		spaceID       int64
		param         *entity.MGetTagKeyParam
		mockSetup     func()
		wantErr       bool
		wantTagKeyIDs []int64
		wantTotal     int64
	}{
		{
			name:      "param is nil",
			spaceID:   int64(123),
			param:     nil,
			mockSetup: func() {},
			wantErr:   true,
		},
		{
			name:    "MGetTagKeys failed",
			spaceID: int64(123),
			param:   &entity.MGetTagKeyParam{},
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("123"))
			},
			wantErr: true,
		},
		{
			name:  "build tag values failed",
			param: &entity.MGetTagKeyParam{},
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{
						TagKeyID:   123,
						VersionNum: gptr.Of(int32(123)),
					},
					{
						TagKeyID:   234,
						VersionNum: gptr.Of(int32(123)),
					},
				}, &pagination.PageResult{}, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &pagination.PageResult{}, errors.New("123"))
			},
			wantErr: true,
		},
		{
			name:  "count tag keys failed",
			param: &entity.MGetTagKeyParam{},
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{
						TagKeyID:   123,
						VersionNum: gptr.Of(int32(123)),
					},
					{
						TagKeyID:   234,
						VersionNum: gptr.Of(int32(123)),
					},
				}, &pagination.PageResult{}, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &pagination.PageResult{}, nil).Times(2)
				tagRepo.EXPECT().CountTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(123), errors.New("123"))
			},
			wantErr: true,
		},
		{
			name:  "normal case",
			param: &entity.MGetTagKeyParam{},
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{
						TagKeyID:   123,
						VersionNum: gptr.Of(int32(123)),
					},
					{
						TagKeyID:   234,
						VersionNum: gptr.Of(int32(123)),
					},
				}, &pagination.PageResult{}, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &pagination.PageResult{}, nil).Times(2)
				tagRepo.EXPECT().CountTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(5), nil)
			},
			wantErr:       false,
			wantTotal:     5,
			wantTagKeyIDs: []int64{123, 234},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			resp, pr, err := svc.SearchTags(ctx, tt.spaceID, tt.param)
			if (err != nil) != tt.wantErr {
				t.Errorf("SearchTags() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				assert.Equal(t, tt.wantTotal, pr.Total)
				assert.Equal(t, len(tt.wantTagKeyIDs), len(resp))
				for i := 0; i < len(tt.wantTagKeyIDs); i++ {
					assert.Equal(t, tt.wantTagKeyIDs[i], resp[i].TagKeyID)
				}
			}
		})
	}
}

func TestTagServiceImpl_GetTagDetail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tagRepo := mocks.NewMockITagAPI(ctrl)
	db := dbmock.NewMockProvider(ctrl)
	locker := mocks2.NewMockILocker(ctrl)
	cfg := mocks3.NewMockIConfig(ctrl)

	svc := NewTagServiceImpl(
		tagRepo,
		db,
		locker,
		cfg,
	)
	ctx := context.Background()

	tests := []struct {
		name          string
		spaceID       int64
		req           *entity.GetTagDetailReq
		mockSetup     func()
		wantErr       bool
		wantTagKeyIDs []int64
		wantTotal     int64
	}{
		{
			name:      "param is nil",
			spaceID:   int64(123),
			req:       nil,
			mockSetup: func() {},
			wantErr:   true,
		},
		{
			name:    "get all failed",
			spaceID: int64(123),
			req: &entity.GetTagDetailReq{
				PageSize: 0,
			},
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("123"))
			},
			wantErr: true,
		},
		{
			name:    "get all, build tag values failed",
			spaceID: int64(123),
			req:     &entity.GetTagDetailReq{},
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{
						VersionNum: gptr.Of(int32(12)),
					},
				}, &pagination.PageResult{}, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("123"))
			},
			wantErr: true,
		},
		{
			name:    "get all, nomal case",
			spaceID: int64(123),
			req:     &entity.GetTagDetailReq{},
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{
						TagKeyID:   123,
						VersionNum: gptr.Of(int32(123)),
					},
					{
						TagKeyID:   123,
						VersionNum: gptr.Of(int32(122)),
					},
				}, &pagination.PageResult{}, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &pagination.PageResult{}, nil).Times(2)
			},
			wantErr:       false,
			wantTotal:     int64(2),
			wantTagKeyIDs: []int64{123, 123},
		},
		{
			name: "pagination, MGetTagKeys failed",
			req: &entity.GetTagDetailReq{
				PageSize: int32(10),
			},
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("123"))
			},
			wantErr: true,
		},
		{
			name: "pagination, count tag keys failed",
			req: &entity.GetTagDetailReq{
				PageSize: int32(10),
			},
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{
						TagKeyID:   123,
						VersionNum: gptr.Of(int32(12)),
					},
					{
						TagKeyID:   123,
						VersionNum: gptr.Of(int32(11)),
					},
				}, &pagination.PageResult{}, nil)
				tagRepo.EXPECT().CountTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(0), errors.New("123"))
			},
			wantErr: true,
		},
		{
			name: "pagination, normal case",
			req: &entity.GetTagDetailReq{
				PageSize: int32(10),
			},
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{
						TagKeyID:   123,
						VersionNum: gptr.Of(int32(12)),
					},
					{
						TagKeyID:   123,
						VersionNum: gptr.Of(int32(11)),
					},
				}, &pagination.PageResult{}, nil)
				tagRepo.EXPECT().CountTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(20), nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &pagination.PageResult{}, nil).Times(2)
			},
			wantErr:       false,
			wantTotal:     20,
			wantTagKeyIDs: []int64{123, 123},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			resp, err := svc.GetTagDetail(ctx, tt.spaceID, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTagDetail() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				assert.Equal(t, tt.wantTotal, resp.Total)
				assert.Equal(t, len(tt.wantTagKeyIDs), len(resp.TagKeys))
			}
		})
	}
}

func TestTagServiceImpl_BatchGetTags(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tagRepo := mocks.NewMockITagAPI(ctrl)
	db := dbmock.NewMockProvider(ctrl)
	locker := mocks2.NewMockILocker(ctrl)
	cfg := mocks3.NewMockIConfig(ctrl)

	svc := NewTagServiceImpl(
		tagRepo,
		db,
		locker,
		cfg,
	)
	ctx := context.Background()

	tests := []struct {
		name          string
		spaceID       int64
		tagKeyIDs     []int64
		mockSetup     func()
		wantErr       bool
		wantTagKeyIDs []int64
	}{
		{
			name:      "tag key length is 0",
			spaceID:   int64(123),
			tagKeyIDs: nil,
			mockSetup: func() {},
			wantErr:   true,
		},
		{
			name:      "MGetTagKeys failed",
			spaceID:   int64(123),
			tagKeyIDs: []int64{123, 234},
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("123"))
			},
			wantErr: true,
		},
		{
			name:      "build tag values failed",
			spaceID:   int64(123),
			tagKeyIDs: []int64{123, 234},
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{
						TagKeyID:   123,
						VersionNum: gptr.Of(int32(12)),
					},
					{
						TagKeyID:   124,
						VersionNum: gptr.Of(int32(1)),
					},
				}, nil, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &pagination.PageResult{}, errors.New("123"))
			},
			wantErr: true,
		},
		{
			name:      "normal case",
			spaceID:   int64(123),
			tagKeyIDs: []int64{123, 234},
			mockSetup: func() {
				tagRepo.EXPECT().MGetTagKeys(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.TagKey{
					{
						TagKeyID:   123,
						VersionNum: gptr.Of(int32(12)),
					},
					{
						TagKeyID:   124,
						VersionNum: gptr.Of(int32(1)),
					},
				}, nil, nil)
				tagRepo.EXPECT().MGetTagValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &pagination.PageResult{}, nil).Times(2)
			},
			wantErr:       false,
			wantTagKeyIDs: []int64{123, 124},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			resp, err := svc.BatchGetTagsByTagKeyIDs(ctx, tt.spaceID, tt.tagKeyIDs)
			if (err != nil) != tt.wantErr {
				t.Errorf("BatchGetTags() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				assert.Equal(t, len(tt.wantTagKeyIDs), len(resp))
				for i := 0; i < len(tt.wantTagKeyIDs); i++ {
					assert.Equal(t, tt.wantTagKeyIDs[i], resp[i].TagKeyID)
				}
			}
		})
	}
}

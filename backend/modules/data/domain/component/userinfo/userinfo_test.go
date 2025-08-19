// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package userinfo

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/common"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/entity"
)

func TestNewUserInfoServiceImpl(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	userProvider := mocks.NewMockIUserProvider(ctrl)
	svc := NewUserInfoServiceImpl(userProvider)
	assert.NotNil(t, svc)
}

func TestUserInfoServiceImpl_GetUserInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	userProvider := mocks.NewMockIUserProvider(ctrl)
	svc := NewUserInfoServiceImpl(userProvider)

	userID := "u1"
	userInfo := &entity.UserInfo{UserID: &userID}

	tests := []struct {
		name    string
		mock    func()
		wantErr bool
		wantNil bool
	}{
		{
			name: "正常返回",
			mock: func() {
				userProvider.EXPECT().MGetUserInfo(gomock.Any(), []string{userID}).Return([]*entity.UserInfo{userInfo}, nil)
			},
			wantErr: false,
			wantNil: false,
		},
		{
			name: "rpc错误",
			mock: func() {
				userProvider.EXPECT().MGetUserInfo(gomock.Any(), []string{userID}).Return(nil, errors.New("rpc error"))
			},
			wantErr: true,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mock()
			info, err := svc.(*UserInfoServiceImpl).GetUserInfo(context.Background(), userID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, info)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, info)
			}
		})
	}
}

type testDTO struct {
	baseInfo *commondto.BaseInfo
}

func (d *testDTO) GetBaseInfo() *commondto.BaseInfo  { return d.baseInfo }
func (d *testDTO) SetBaseInfo(b *commondto.BaseInfo) { d.baseInfo = b }

type testDO struct {
	baseInfo *entity.BaseInfo
}

func (d *testDO) GetBaseInfo() *entity.BaseInfo  { return d.baseInfo }
func (d *testDO) SetBaseInfo(b *entity.BaseInfo) { d.baseInfo = b }

func TestUserInfoServiceImpl_PackUserInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	userProvider := mocks.NewMockIUserProvider(ctrl)
	svc := NewUserInfoServiceImpl(userProvider)

	userID := "u1"
	userInfo := &entity.UserInfo{UserID: &userID}
	userInfoDTO := &commondto.UserInfo{UserID: &userID}

	t.Run("DTO正常流程", func(t *testing.T) {
		dto := &testDTO{baseInfo: &commondto.BaseInfo{}}
		dto.baseInfo.SetCreatedBy(userInfoDTO)
		dto.baseInfo.SetUpdatedBy(userInfoDTO)
		userProvider.EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).Return([]*entity.UserInfo{userInfo}, nil)
		svc.PackUserInfo(context.Background(), []UserInfoCarrier{dto})
		assert.NotNil(t, dto.baseInfo.GetCreatedBy())
		assert.NotNil(t, dto.baseInfo.GetUpdatedBy())
	})

	t.Run("DO正常流程", func(t *testing.T) {
		do := &testDO{baseInfo: &entity.BaseInfo{}}
		do.baseInfo.SetCreatedBy(userInfo)
		do.baseInfo.SetUpdatedBy(userInfo)
		userProvider.EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).Return([]*entity.UserInfo{userInfo}, nil)
		svc.PackUserInfo(context.Background(), []UserInfoDomainCarrier{do})
		assert.NotNil(t, do.baseInfo.GetCreatedBy())
		assert.NotNil(t, do.baseInfo.GetUpdatedBy())
	})

	t.Run("DTO异常流程-获取用户信息失败", func(t *testing.T) {
		dto := &testDTO{baseInfo: &commondto.BaseInfo{}}
		dto.baseInfo.SetCreatedBy(userInfoDTO)
		dto.baseInfo.SetUpdatedBy(userInfoDTO)
		userProvider.EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).Return(nil, errors.New("rpc error"))
		svc.PackUserInfo(context.Background(), []UserInfoCarrier{dto})
		// 只要不panic即可
	})

	t.Run("DO异常流程-获取用户信息失败", func(t *testing.T) {
		do := &testDO{baseInfo: &entity.BaseInfo{}}
		do.baseInfo.SetCreatedBy(userInfo)
		do.baseInfo.SetUpdatedBy(userInfo)
		userProvider.EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).Return(nil, errors.New("rpc error"))
		svc.PackUserInfo(context.Background(), []UserInfoDomainCarrier{do})
		// 只要不panic即可
	})
}

func TestBatchConvertDTO2UserInfoCarrier(t *testing.T) {
	dto1 := &testDTO{baseInfo: &commondto.BaseInfo{}}
	dto2 := &testDTO{baseInfo: &commondto.BaseInfo{}}
	tests := []struct {
		name    string
		input   []*testDTO
		wantLen int
	}{
		{"空输入", nil, 0},
		{"空slice", []*testDTO{}, 0},
		{"单元素", []*testDTO{dto1}, 1},
		{"多元素", []*testDTO{dto1, dto2}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BatchConvertDTO2UserInfoCarrier(tt.input)
			assert.Equal(t, tt.wantLen, len(got))
			for i, v := range got {
				assert.Equal(t, tt.input[i], v)
			}
		})
	}
}

func TestBatchConvertDO2UserInfoDomainCarrier(t *testing.T) {
	do1 := &testDO{baseInfo: &entity.BaseInfo{}}
	do2 := &testDO{baseInfo: &entity.BaseInfo{}}
	tests := []struct {
		name    string
		input   []*testDO
		wantLen int
	}{
		{"空输入", nil, 0},
		{"空slice", []*testDO{}, 0},
		{"单元素", []*testDO{do1}, 1},
		{"多元素", []*testDO{do1, do2}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BatchConvertDO2UserInfoDomainCarrier(tt.input)
			assert.Equal(t, tt.wantLen, len(got))
			for i, v := range got {
				assert.Equal(t, tt.input[i], v)
			}
		})
	}
}

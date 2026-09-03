// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// resolveRunConfSchedulingParams 是 UpdateExptRunConf 两面共用的闸门 + 校验。
// 它与创建期同口径：未获授权**静默丢弃**（不报错），所以用例断言的是"返回值有没有被清掉"。

func quotaDTO(amount int64) *domain_expt.ExpectedQuotaConsumption {
	return &domain_expt.ExpectedQuotaConsumption{
		Resources: []*domain_expt.ExpectedResourceConsumption{
			{Category: "sandbox", ResourceKey: "default", Amount: amount},
		},
	}
}

func TestResolveRunConfSchedulingParams_DropsBothWhenNotAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().GetExptSchedulingPrivilegeWhiteList(gomock.Any()).
		Return(&entity.ExptSchedulingPrivilegeWhiteList{})

	priority, quota, err := resolveRunConfSchedulingParams(
		ctxWithEmail("stranger@bytedance.com"), mockConfiger, 456, 789, gptr.Of(int32(99)), quotaDTO(1))

	// 丢弃而不是报错：这两个字段已在 IDL 里，突然报错会打挂已经在传的调用方。
	require.NoError(t, err)
	assert.Nil(t, priority, "未授权的 priority 必须丢弃，否则任何人都能把自己的实验设成 99 插队")
	assert.Nil(t, quota, "未授权的向量必须丢弃，否则可虚报消耗")
}

func TestResolveRunConfSchedulingParams_KeepsBothWhenAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().GetExptSchedulingPrivilegeWhiteList(gomock.Any()).
		Return(&entity.ExptSchedulingPrivilegeWhiteList{UserEmails: []string{"admin@bytedance.com"}})

	priority, quota, err := resolveRunConfSchedulingParams(
		ctxWithEmail("admin@bytedance.com"), mockConfiger, 456, 789, gptr.Of(int32(80)), quotaDTO(25))

	require.NoError(t, err)
	require.NotNil(t, priority)
	assert.Equal(t, int32(80), *priority)
	require.NotNil(t, quota)
	require.Len(t, quota.Resources, 1)
	assert.Equal(t, int64(25), quota.Resources[0].Amount)
}

func TestResolveRunConfSchedulingParams_SkipsConfigReadWhenNothingDeclared(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	// 一次配置读取都不该发生：绝大多数调用只改并发度/重试，
	// 每次都读白名单既浪费又会给没用这两个字段的调用方刷无关日志。
	mockConfiger.EXPECT().GetExptSchedulingPrivilegeWhiteList(gomock.Any()).Times(0)

	priority, quota, err := resolveRunConfSchedulingParams(
		ctxWithEmail("stranger@bytedance.com"), mockConfiger, 456, 789, nil, nil)

	require.NoError(t, err)
	assert.Nil(t, priority)
	assert.Nil(t, quota)
}

func TestResolveRunConfSchedulingParams_RejectsOutOfRangePriority(t *testing.T) {
	// 0 也要拒绝：nil 才是"不修改"。0 落到下游 Normalize 会被收敛成缺省优先级，
	// 于是"我传了 0"和"我想设成缺省"无法区分，静默改掉一个本不该动的值。
	for _, bad := range []int32{0, -1, 100} {
		ctrl := gomock.NewController(t)
		mockConfiger := componentMocks.NewMockIConfiger(ctrl)
		mockConfiger.EXPECT().GetExptSchedulingPrivilegeWhiteList(gomock.Any()).
			Return(&entity.ExptSchedulingPrivilegeWhiteList{UserEmails: []string{"admin@bytedance.com"}})

		_, _, err := resolveRunConfSchedulingParams(
			ctxWithEmail("admin@bytedance.com"), mockConfiger, 456, 789, gptr.Of(bad), nil)
		assert.Error(t, err, "priority_level=%d 应被拒绝", bad)
		ctrl.Finish()
	}
}

func TestResolveRunConfSchedulingParams_RejectsInvalidVector(t *testing.T) {
	cases := map[string]*domain_expt.ExpectedQuotaConsumption{
		"空 resources": {Resources: nil},
		"amount 非正":   quotaDTO(0),
		"申报了通配":       {Resources: []*domain_expt.ExpectedResourceConsumption{{Category: "sandbox", ResourceKey: "*", Amount: 1}}},
	}
	for name, dto := range cases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockConfiger := componentMocks.NewMockIConfiger(ctrl)
			mockConfiger.EXPECT().GetExptSchedulingPrivilegeWhiteList(gomock.Any()).
				Return(&entity.ExptSchedulingPrivilegeWhiteList{UserEmails: []string{"admin@bytedance.com"}})

			// 报错而不是当"不修改"：调用方明明传了这个字段，静默忽略等于让一个
			// 不被支持的意图看起来成功了。
			_, _, err := resolveRunConfSchedulingParams(
				ctxWithEmail("admin@bytedance.com"), mockConfiger, 456, 789, nil, dto)
			assert.Error(t, err)
		})
	}
}

func TestResolveRunConfSchedulingParams_ValidationHappensAfterTheGate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().GetExptSchedulingPrivilegeWhiteList(gomock.Any()).
		Return(&entity.ExptSchedulingPrivilegeWhiteList{})

	// 未授权调用方传一个非法值：必须走"丢弃"而不是"报参数错" ——
	// 否则报不报错这件事本身就把"你不在白名单里"泄漏了出去。
	priority, quota, err := resolveRunConfSchedulingParams(
		ctxWithEmail("stranger@bytedance.com"), mockConfiger, 456, 789, gptr.Of(int32(500)), quotaDTO(-1))

	require.NoError(t, err)
	assert.Nil(t, priority)
	assert.Nil(t, quota)
}

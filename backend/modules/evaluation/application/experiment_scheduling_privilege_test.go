// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// enforceSchedulingPrivilege 的行为是**静默作废**而不是报错，所以这些用例断言的是
// "字段有没有被清掉/降级"，而不是返回值 —— 该函数无返回值正是这个设计的体现。

// ctxWithEmail 造一个带已验证邮箱的 ctx。生产链路里这份邮箱由商业版 CtxUser 中间件
// 从 ByteTIM ticket claim 写入（不是请求体字段），所以它可以当授权键用。
func ctxWithEmail(email string) context.Context {
	return session.WithCtxUser(context.Background(), &session.User{ID: "123", Email: email})
}

// privilegedReq 造一个申报了全部三样特权参数的请求。
func privilegedReq() *expt.CreateExperimentRequest {
	return &expt.CreateExperimentRequest{
		WorkspaceID:   456,
		PriorityLevel: gptr.Of(int32(99)),
		TriggerType:   gptr.Of(domain_expt.Evalx),
		ExpectedQuotaConsumption: &domain_expt.ExpectedQuotaConsumption{
			Resources: []*domain_expt.ExpectedResourceConsumption{
				{Category: "sandbox", ResourceKey: "default", Amount: 1},
			},
		},
	}
}

// TestEnforceSchedulingPrivilege_DropsAllThreeWhenNotAllowed 未获授权时三样全部作废。
//
// 三样必须同时挡住：只挡其中一两样等于没挡 —— 只挡 priority 却放开 trigger，
// 任何人仍能自称 evalx 把实验塞进中心调度；只挡 trigger 却放开 quota，
// 纳管范围内的实验仍能虚报消耗。
func TestEnforceSchedulingPrivilege_DropsAllThreeWhenNotAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	// 空白名单 = 谁都没有特权
	mockConfiger.EXPECT().GetExptSchedulingPrivilegeWhiteList(gomock.Any()).
		Return(&entity.ExptSchedulingPrivilegeWhiteList{})

	app := &experimentApplication{configer: mockConfiger}
	req := privilegedReq()

	app.enforceSchedulingPrivilege(ctxWithEmail("stranger@bytedance.com"), req)

	assert.Nil(t, req.PriorityLevel, "未授权的 priority 必须清空，否则任何人都能设 99 插队")
	assert.Nil(t, req.ExpectedQuotaConsumption, "未授权的资源向量必须清空，否则可虚报消耗")
	assert.Equal(t, domain_expt.Manual, req.GetTriggerType(),
		"未授权的 evalx 必须降级，否则任何人都能让实验进 enforce")
}

// TestEnforceSchedulingPrivilege_KeepsAllThreeWhenUserAllowed 白名单用户三样全保留。
func TestEnforceSchedulingPrivilege_KeepsAllThreeWhenUserAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().GetExptSchedulingPrivilegeWhiteList(gomock.Any()).
		Return(&entity.ExptSchedulingPrivilegeWhiteList{UserEmails: []string{"admin@bytedance.com"}})

	app := &experimentApplication{configer: mockConfiger}
	req := privilegedReq()

	app.enforceSchedulingPrivilege(ctxWithEmail("admin@bytedance.com"), req)

	assert.Equal(t, int32(99), req.GetPriorityLevel())
	assert.NotNil(t, req.ExpectedQuotaConsumption)
	assert.Equal(t, domain_expt.Evalx, req.GetTriggerType())
}

// TestEnforceSchedulingPrivilege_KeepsWhenSpaceAllowed 靠 space 维度放行
// （验证 OR 语义在真实调用链上也成立：user 不在名单里）。
func TestEnforceSchedulingPrivilege_KeepsWhenSpaceAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().GetExptSchedulingPrivilegeWhiteList(gomock.Any()).
		Return(&entity.ExptSchedulingPrivilegeWhiteList{SpaceIDs: []string{"456"}})

	app := &experimentApplication{configer: mockConfiger}
	req := privilegedReq()

	app.enforceSchedulingPrivilege(ctxWithEmail("nobody@bytedance.com"), req)

	assert.Equal(t, int32(99), req.GetPriorityLevel())
	assert.Equal(t, domain_expt.Evalx, req.GetTriggerType())
}

// TestEnforceSchedulingPrivilege_SkipsConfigReadWhenNothingDeclared 什么都没申报时不读配置。
//
// 断言方式是"不给 mock 设 EXPECT" —— gomock 在发生未预期调用时会失败，
// 因此这个用例同时钉住了"不发无谓的配置读取"和"不给未使用这些字段的调用方刷日志"。
func TestEnforceSchedulingPrivilege_SkipsConfigReadWhenNothingDeclared(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	app := &experimentApplication{configer: mockConfiger}

	// 只有 manual trigger、无 priority、无向量 —— 控制台创建实验的典型形态
	req := &expt.CreateExperimentRequest{WorkspaceID: 456, TriggerType: gptr.Of(domain_expt.Manual)}
	app.enforceSchedulingPrivilege(ctxWithEmail("nobody@bytedance.com"), req)

	assert.Nil(t, req.PriorityLevel)
	assert.Equal(t, domain_expt.Manual, req.GetTriggerType(), "非 evalx 的 trigger 不得被改动")
}

// TestEnforceSchedulingPrivilege_NonEvalxTriggerUntouched 未授权时也不动非 evalx 的 trigger。
//
// 只有 evalx 能带来特权（进 enforce），伪造 openapi/schedule 不产生任何好处 ——
// 若把它们也一并降级成 manual，会把"调用来源"这个排查依据抹掉。
func TestEnforceSchedulingPrivilege_NonEvalxTriggerUntouched(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	// 申报了 priority 所以会读配置，但 trigger 不该被动
	mockConfiger.EXPECT().GetExptSchedulingPrivilegeWhiteList(gomock.Any()).
		Return(&entity.ExptSchedulingPrivilegeWhiteList{})

	app := &experimentApplication{configer: mockConfiger}
	req := &expt.CreateExperimentRequest{
		WorkspaceID:   456,
		TriggerType:   gptr.Of(domain_expt.OpenAPI),
		PriorityLevel: gptr.Of(int32(50)),
	}

	app.enforceSchedulingPrivilege(ctxWithEmail("nobody@bytedance.com"), req)

	assert.Nil(t, req.PriorityLevel, "priority 仍要清")
	assert.Equal(t, domain_expt.OpenAPI, req.GetTriggerType(), "openapi trigger 必须原样保留")
}

// TestEnforceSchedulingPrivilege_NilSafe req 为 nil 时不得 panic（这道闸在创建实验热路径上）。
func TestEnforceSchedulingPrivilege_NilSafe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	app := &experimentApplication{configer: mockConfiger}

	app.enforceSchedulingPrivilege(ctxWithEmail("admin@bytedance.com"), nil)

	// ctx 里没有用户信息（拿不到邮箱）但有申报值：需要读配置，邮箱按空串处理
	mockConfiger.EXPECT().GetExptSchedulingPrivilegeWhiteList(gomock.Any()).
		Return(&entity.ExptSchedulingPrivilegeWhiteList{UserEmails: []string{"admin@bytedance.com"}})
	req := privilegedReq()
	app.enforceSchedulingPrivilege(context.Background(), req)
	assert.Nil(t, req.PriorityLevel, "拿不到用户身份时不得放行")
}

// TestEnforceSchedulingPrivilege_ConfigReadFailureDeniesByDefault 配置读取失败（返回 nil）时拒绝。
//
// configer 的实现在读取失败时回落到 DefaultExptSchedulingPrivilegeWhiteList()，但这里直接
// 喂 nil 做纵深防御：万一将来某个实现真的返回了 nil，行为也必须是拒绝而不是 panic 或放行。
func TestEnforceSchedulingPrivilege_ConfigReadFailureDeniesByDefault(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().GetExptSchedulingPrivilegeWhiteList(gomock.Any()).Return(nil)

	app := &experimentApplication{configer: mockConfiger}
	req := privilegedReq()

	app.enforceSchedulingPrivilege(ctxWithEmail("admin@bytedance.com"), req)

	assert.Nil(t, req.PriorityLevel,
		"配置不可判定时必须拒绝 —— 这些参数被滥用的后果都是悄悄多占资源/插队")
	assert.Nil(t, req.ExpectedQuotaConsumption)
	assert.Equal(t, domain_expt.Manual, req.GetTriggerType())
}

// TestEnforceSchedulingPrivilege_QuotaOnlyDeclarationStillGated 只申报向量也要过闸。
//
// 容易漏的一种形态：调用方不设 priority、trigger 也不是 evalx，只塞一个资源向量。
// 若判定条件只看 priority 与 trigger，这条会绕过闸门 —— 向量虽然在 legacy 下不生效，
// 但它会被冻结进 eval_conf，一旦该实验后续被纳管就直接按虚报值扣额度。
func TestEnforceSchedulingPrivilege_QuotaOnlyDeclarationStillGated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().GetExptSchedulingPrivilegeWhiteList(gomock.Any()).
		Return(&entity.ExptSchedulingPrivilegeWhiteList{})

	app := &experimentApplication{configer: mockConfiger}
	req := &expt.CreateExperimentRequest{
		WorkspaceID: 456,
		ExpectedQuotaConsumption: &domain_expt.ExpectedQuotaConsumption{
			Resources: []*domain_expt.ExpectedResourceConsumption{
				{Category: "sandbox", ResourceKey: "default", Amount: 1},
			},
		},
	}

	app.enforceSchedulingPrivilege(ctxWithEmail("nobody@bytedance.com"), req)

	assert.Nil(t, req.ExpectedQuotaConsumption, "只申报向量同样要被挡")
}

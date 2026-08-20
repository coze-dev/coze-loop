// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ctxWithEmail 造一个带已验证邮箱的 ctx。生产链路里这份邮箱由商业版 CtxUser 中间件
// 从 ByteTIM ticket claim 写入（不是请求体字段），所以它可以当授权键用。
func ctxWithEmail(email string) context.Context {
	return session.WithCtxUser(context.Background(), &session.User{ID: "123", Email: email})
}

// enforcePriorityWhiteList 的行为是**静默清空**而不是报错，所以这些用例断言的是
// "req.PriorityLevel 有没有被清掉"，而不是返回值 —— 该函数无返回值正是这个设计的体现。

func TestEnforcePriorityWhiteList_ClearsWhenNotAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	// 空白名单 = 谁都不许指定
	mockConfiger.EXPECT().GetExptPriorityWhiteList(gomock.Any()).Return(&entity.ExptPriorityWhiteList{})

	app := &experimentApplication{configer: mockConfiger}
	req := &expt.CreateExperimentRequest{
		WorkspaceID:   456,
		PriorityLevel: gptr.Of(int32(99)),
	}

	app.enforcePriorityWhiteList(ctxWithEmail("admin@bytedance.com"), req)

	assert.Nil(t, req.PriorityLevel,
		"未获授权的申报值必须被清空，否则任何人都能设 99 插队")
}

func TestEnforcePriorityWhiteList_KeepsWhenUserAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().GetExptPriorityWhiteList(gomock.Any()).
		Return(&entity.ExptPriorityWhiteList{UserEmails: []string{"admin@bytedance.com"}})

	app := &experimentApplication{configer: mockConfiger}
	req := &expt.CreateExperimentRequest{
		WorkspaceID:   456,
		PriorityLevel: gptr.Of(int32(99)),
	}

	app.enforcePriorityWhiteList(ctxWithEmail("admin@bytedance.com"), req)

	assert.NotNil(t, req.PriorityLevel, "白名单用户的申报值必须保留")
	assert.Equal(t, int32(99), req.GetPriorityLevel())
}

func TestEnforcePriorityWhiteList_KeepsWhenSpaceAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().GetExptPriorityWhiteList(gomock.Any()).
		Return(&entity.ExptPriorityWhiteList{SpaceIDs: []int64{456}})

	app := &experimentApplication{configer: mockConfiger}
	req := &expt.CreateExperimentRequest{
		WorkspaceID:   456,
		PriorityLevel: gptr.Of(int32(50)),
	}

	// user 不在名单里，靠 space 维度放行（验证 OR 语义在真实调用链上也成立）
	app.enforcePriorityWhiteList(ctxWithEmail("nobody@bytedance.com"), req)

	assert.Equal(t, int32(50), req.GetPriorityLevel())
}

// TestEnforcePriorityWhiteList_SkipsConfigReadWhenUnset 没申报 priority 时不读配置。
//
// 断言方式是"不给 mock 设 EXPECT" —— gomock 在发生未预期调用时会失败，
// 因此这个用例同时钉住了"不发无谓的配置读取"和"不给未使用该字段的调用方刷日志"。
func TestEnforcePriorityWhiteList_SkipsConfigReadWhenUnset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	app := &experimentApplication{configer: mockConfiger}

	req := &expt.CreateExperimentRequest{WorkspaceID: 456} // 未设 PriorityLevel
	app.enforcePriorityWhiteList(ctxWithEmail("admin@bytedance.com"), req)

	assert.Nil(t, req.PriorityLevel)
}

// TestEnforcePriorityWhiteList_NilSafe req/session 为 nil 时不得 panic。
// 这道闸在热路径上，一次 panic 会打挂整个创建实验接口。
func TestEnforcePriorityWhiteList_NilSafe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	app := &experimentApplication{configer: mockConfiger}

	// req 为 nil：直接返回，不读配置
	app.enforcePriorityWhiteList(ctxWithEmail("admin@bytedance.com"), nil)

	// ctx 里没有用户信息（拿不到邮箱）但有申报值：需要读配置，邮箱按空串处理
	mockConfiger.EXPECT().GetExptPriorityWhiteList(gomock.Any()).
		Return(&entity.ExptPriorityWhiteList{UserEmails: []string{"admin@bytedance.com"}})
	req := &expt.CreateExperimentRequest{
		WorkspaceID:   456,
		PriorityLevel: gptr.Of(int32(99)),
	}
	app.enforcePriorityWhiteList(context.Background(), req)
	assert.Nil(t, req.PriorityLevel, "拿不到用户身份时不得放行")
}

// TestEnforcePriorityWhiteList_ConfigReadFailureDeniesByDefault 配置读取失败（返回 nil）时拒绝。
//
// configer 的实现在读取失败时回落到 DefaultExptPriorityWhiteList()，但这里直接喂 nil
// 做纵深防御：万一将来某个实现真的返回了 nil，行为也必须是拒绝而不是 panic 或放行。
func TestEnforcePriorityWhiteList_ConfigReadFailureDeniesByDefault(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().GetExptPriorityWhiteList(gomock.Any()).Return(nil)

	app := &experimentApplication{configer: mockConfiger}
	req := &expt.CreateExperimentRequest{
		WorkspaceID:   456,
		PriorityLevel: gptr.Of(int32(99)),
	}

	app.enforcePriorityWhiteList(ctxWithEmail("admin@bytedance.com"), req)

	assert.Nil(t, req.PriorityLevel,
		"配置不可判定时必须拒绝 —— priority 是插队能力，读不到配置宁可不给")
}


// ---- enforceTriggerTrust ----

// TestEnforceTriggerTrust_DowngradesUntrustedEvalx 不可信调用方自称 evalx 时降级为 manual。
//
// 这道闸的意义：enforce 的第一道判据只看请求体里的 trigger_type 字符串，那是调用方自己填的
// —— 不设闸的话"谁被中心调度纳管"部分取决于调用方自报，而它本该完全由我们的名单决定。
func TestEnforceTriggerTrust_DowngradesUntrustedEvalx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	// 启用校验且名单里没有本次 caller（测试环境下 caller 为空）
	mockConfiger.EXPECT().GetExptTriggerTrustConf(gomock.Any()).
		Return(&entity.ExptTriggerTrustConf{Enabled: true, EvalxCallerPSMs: []string{"stone.cozeloop.evalx"}})

	app := &experimentApplication{configer: mockConfiger}
	req := &expt.CreateExperimentRequest{
		WorkspaceID: 456,
		TriggerType: gptr.Of(domain_expt.Evalx),
	}

	app.enforceTriggerTrust(context.Background(), req)

	assert.Equal(t, domain_expt.Manual, req.GetTriggerType(),
		"不可信调用方声明的 evalx 必须被降级，否则任何人都能让实验进 enforce")
}

// TestEnforceTriggerTrust_KeepsWhenDisabled 未启用校验时保持原样（缺省行为，不额外收紧）。
func TestEnforceTriggerTrust_KeepsWhenDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().GetExptTriggerTrustConf(gomock.Any()).
		Return(&entity.ExptTriggerTrustConf{Enabled: false})

	app := &experimentApplication{configer: mockConfiger}
	req := &expt.CreateExperimentRequest{
		WorkspaceID: 456,
		TriggerType: gptr.Of(domain_expt.Evalx),
	}

	app.enforceTriggerTrust(context.Background(), req)

	assert.Equal(t, domain_expt.Evalx, req.GetTriggerType(), "未启用校验时不得改动 trigger")
}

// TestEnforceTriggerTrust_IgnoresNonEvalxTriggers 只处理 evalx。
//
// 其它 trigger 不触发 enforce，伪造它们不产生特权 —— 为此增加校验只会带来误伤风险。
// 断言方式是"不给 mock 设 EXPECT"：一旦实现去读了配置，gomock 会因未预期调用而失败。
func TestEnforceTriggerTrust_IgnoresNonEvalxTriggers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	app := &experimentApplication{configer: mockConfiger}

	for _, trigger := range []string{domain_expt.Manual, domain_expt.OpenAPI, domain_expt.Schedule} {
		req := &expt.CreateExperimentRequest{WorkspaceID: 456, TriggerType: gptr.Of(trigger)}
		app.enforceTriggerTrust(context.Background(), req)
		assert.Equal(t, trigger, req.GetTriggerType(), "非 evalx trigger 不得被改动")
	}

	// 未设 trigger 同样不该读配置
	reqUnset := &expt.CreateExperimentRequest{WorkspaceID: 456}
	app.enforceTriggerTrust(context.Background(), reqUnset)
}

// TestEnforceTriggerTrust_NilSafe req 为 nil 时不得 panic（这道闸在创建实验热路径上）。
func TestEnforceTriggerTrust_NilSafe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	app := &experimentApplication{configer: componentMocks.NewMockIConfiger(ctrl)}
	app.enforceTriggerTrust(context.Background(), nil)
}

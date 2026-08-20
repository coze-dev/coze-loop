// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitMocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// TestCheckBenefit_NilSessionDoesNotPanic 回归：nil session 曾在此处直接 panic。
//
// 触发路径是"item 事件的 Session 字段没被填" —— 中心调度那条派发链路漏过一次
// （legacy 链路一直填着 Session: event.Session）。该 panic 发生在 item 执行链里、
// 被 HandleEventErr 的 recover 转成 error，现象是"每个派发出去的 item 都失败"
// 而不是进程崩溃，从现象极难反推到"某个字段没填"。
//
// 修法取"退化成匿名"而不是提前报错：权益校验拿到空 UserID 会返回明确的业务错误，
// 那是可读的失败；而在此自造 error 会掩盖真实原因（调用方漏填）。
func TestCheckBenefit_NilSessionDoesNotPanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBenefit := benefitMocks.NewMockIBenefitService(ctrl)
	var gotUID string
	mockBenefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *benefit.CheckAndDeductEvalBenefitParams) (*benefit.CheckAndDeductEvalBenefitResult, error) {
			gotUID = req.ConnectorUID
			return &benefit.CheckAndDeductEvalBenefitResult{}, nil
		})

	impl := &DefaultExptTurnEvaluationImpl{benefitService: mockBenefit}

	// 不 panic 即为通过；这里同时断言它按匿名继续、而不是自造一个错误。
	err := impl.CheckBenefit(context.Background(), 1001, 2002, false, nil)
	require.NoError(t, err, "nil session 应退化成匿名校验，而不是返回自造的错误")
	assert.Equal(t, "", gotUID, "拿不到身份时 ConnectorUID 应为空串")
}

// TestCheckBenefit_PassesSessionUserID 正常 session 的 UserID 必须原样透传给权益服务。
// 与上一条成对：既要容忍 nil，又不能因为容忍而丢掉真实身份。
func TestCheckBenefit_PassesSessionUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBenefit := benefitMocks.NewMockIBenefitService(ctrl)
	var gotUID string
	mockBenefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *benefit.CheckAndDeductEvalBenefitParams) (*benefit.CheckAndDeductEvalBenefitResult, error) {
			gotUID = req.ConnectorUID
			return &benefit.CheckAndDeductEvalBenefitResult{}, nil
		})

	impl := &DefaultExptTurnEvaluationImpl{benefitService: mockBenefit}

	err := impl.CheckBenefit(context.Background(), 1001, 2002, false, &entity.Session{UserID: "7123456789"})
	require.NoError(t, err)
	assert.Equal(t, "7123456789", gotUID)
}

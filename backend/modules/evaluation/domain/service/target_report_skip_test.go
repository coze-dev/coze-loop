// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

// newReportInvokeRecordsSvc 快速构造用于 ReportInvokeRecords 分支测试的 EvalTargetServiceImpl。
// 关键点: configer.GetErrCtrl 仅在未 skip 时才应被调用 (convEvalTargetRunErr 内部消费)。
func newReportInvokeRecordsSvc(t *testing.T, ctrl *gomock.Controller, expectGetErrCtrl bool) (*EvalTargetServiceImpl, *repomocks.MockIEvalTargetRepo, *componentmocks.MockIConfiger) {
	t.Helper()
	repo := repomocks.NewMockIEvalTargetRepo(ctrl)
	configer := componentmocks.NewMockIConfiger(ctrl)
	// 通用桩: 允许调用一次 GetTargetTrajectoryConf/BuildEvalExt。
	configer.EXPECT().GetTargetTrajectoryConf(gomock.Any()).AnyTimes().Return(&entity.TargetTrajectoryConf{})
	configer.EXPECT().BuildEvalExt(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	if expectGetErrCtrl {
		configer.EXPECT().GetErrCtrl(gomock.Any()).Return(entity.DefaultExptErrCtrl()).Times(1)
	}
	// 若 expectGetErrCtrl=false 则不注册, 若被调用会 fail。

	metric := metricsmocks.NewMockEvalTargetMetrics(ctrl)
	metric.EXPECT().EmitRun(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	svc := &EvalTargetServiceImpl{
		evalTargetRepo: repo,
		idgen:          idgenmocks.NewMockIIDGenerator(ctrl),
		metric:         metric,
		configer:       configer,
		typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{},
	}
	return svc, repo, configer
}

// SkipErrMsgConvert=true: 不应调用 GetErrCtrl.ConvertErrMsg, err_msg 原样保留。
func TestReportInvokeRecords_SkipErrMsgConvert_True(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, repo, _ := newReportInvokeRecordsSvc(t, ctrl, false)

	record := &entity.EvalTargetRecord{
		ID:      1,
		SpaceID: 1,
		Status:  gptr.Of(entity.EvalTargetRunStatusAsyncInvoking),
		EvalTargetOutputData: &entity.EvalTargetOutputData{
			EvalTargetRunError: &entity.EvalTargetRunError{
				Code:    12345,
				Message: "raw sandbox agent error",
			},
		},
	}
	repo.EXPECT().GetEvalTargetRecordByIDAndSpaceID(gomock.Any(), int64(1), int64(1)).Return(record, nil)
	var saved *entity.EvalTargetRecord
	repo.EXPECT().SaveEvalTargetRecord(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, rec *entity.EvalTargetRecord, _ *bool) error {
			saved = rec
			return nil
		})

	param := &entity.ReportTargetRecordParam{
		SpaceID:  1,
		RecordID: 1,
		Status:   entity.EvalTargetRunStatusSuccess,
		OutputData: &entity.EvalTargetOutputData{
			EvalTargetRunError: &entity.EvalTargetRunError{
				Code:    12345,
				Message: "raw sandbox agent error",
			},
		},
		SkipErrMsgConvert: true,
		Session:           &entity.Session{UserID: "user"},
	}
	require.NoError(t, svc.ReportInvokeRecords(context.Background(), param))
	require.NotNil(t, saved)
	require.NotNil(t, saved.EvalTargetOutputData)
	require.NotNil(t, saved.EvalTargetOutputData.EvalTargetRunError)
	assert.Equal(t, "raw sandbox agent error", saved.EvalTargetOutputData.EvalTargetRunError.Message)
}

// SkipErrMsgConvert=false + 非 CustomEvalTargetInvokeFailCode: 调用 GetErrCtrl 归一化。
func TestReportInvokeRecords_SkipErrMsgConvert_False(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, repo, _ := newReportInvokeRecordsSvc(t, ctrl, true /* GetErrCtrl expected */)

	record := &entity.EvalTargetRecord{
		ID:      1,
		SpaceID: 1,
		Status:  gptr.Of(entity.EvalTargetRunStatusAsyncInvoking),
		EvalTargetOutputData: &entity.EvalTargetOutputData{
			EvalTargetRunError: &entity.EvalTargetRunError{
				Code:    99999, // 非 CustomEvalTargetInvokeFailCode, 会走 ConvertErrMsg
				Message: "some non-whitelisted err",
			},
		},
	}
	repo.EXPECT().GetEvalTargetRecordByIDAndSpaceID(gomock.Any(), int64(1), int64(1)).Return(record, nil)
	repo.EXPECT().SaveEvalTargetRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	param := &entity.ReportTargetRecordParam{
		SpaceID:  1,
		RecordID: 1,
		Status:   entity.EvalTargetRunStatusSuccess,
		OutputData: &entity.EvalTargetOutputData{
			EvalTargetRunError: &entity.EvalTargetRunError{
				Code:    99999,
				Message: "some non-whitelisted err",
			},
		},
		SkipErrMsgConvert: false,
		Session:           &entity.Session{UserID: "user"},
	}
	require.NoError(t, svc.ReportInvokeRecords(context.Background(), param))
}

// SkipErrMsgConvert=true 且 OutputData 无 EvalTargetRunError 时也不 panic。
func TestReportInvokeRecords_SkipErrMsgConvert_NoErr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, repo, _ := newReportInvokeRecordsSvc(t, ctrl, false)

	record := &entity.EvalTargetRecord{
		ID:      1,
		SpaceID: 1,
		Status:  gptr.Of(entity.EvalTargetRunStatusAsyncInvoking),
	}
	repo.EXPECT().GetEvalTargetRecordByIDAndSpaceID(gomock.Any(), int64(1), int64(1)).Return(record, nil)
	repo.EXPECT().SaveEvalTargetRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	param := &entity.ReportTargetRecordParam{
		SpaceID:           1,
		RecordID:          1,
		Status:            entity.EvalTargetRunStatusSuccess,
		OutputData:        &entity.EvalTargetOutputData{},
		SkipErrMsgConvert: true,
		Session:           &entity.Session{UserID: "user"},
	}
	require.NoError(t, svc.ReportInvokeRecords(context.Background(), param))
}

// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package apis

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/kitex/client/callopt"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/experimentservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
)

// MockExperimentServiceClient 实现 experimentservice.Client 接口用于测试
type MockExperimentServiceClientFixed struct {
	ctrl     *gomock.Controller
	recorder *MockExperimentServiceClientFixedMockRecorder
}

type MockExperimentServiceClientFixedMockRecorder struct {
	mock *MockExperimentServiceClientFixed
}

func NewMockExperimentServiceClientFixed(ctrl *gomock.Controller) *MockExperimentServiceClientFixed {
	mock := &MockExperimentServiceClientFixed{ctrl: ctrl}
	mock.recorder = &MockExperimentServiceClientFixedMockRecorder{mock}
	return mock
}

func (m *MockExperimentServiceClientFixed) EXPECT() *MockExperimentServiceClientFixedMockRecorder {
	return m.recorder
}

// 确保实现了 experimentservice.Client 接口
var _ experimentservice.Client = (*MockExperimentServiceClientFixed)(nil)

// 实现所有必需的方法
func (m *MockExperimentServiceClientFixed) CheckExperimentName(ctx context.Context, req *expt.CheckExperimentNameRequest, callOptions ...callopt.Option) (*expt.CheckExperimentNameResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CheckExperimentName", ctx, req)
	ret0, _ := ret[0].(*expt.CheckExperimentNameResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) CheckExperimentName(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "CheckExperimentName", ctx, req)
}

func (m *MockExperimentServiceClientFixed) SubmitExperiment(ctx context.Context, req *expt.SubmitExperimentRequest, callOptions ...callopt.Option) (*expt.SubmitExperimentResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubmitExperiment", ctx, req)
	ret0, _ := ret[0].(*expt.SubmitExperimentResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) SubmitExperiment(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "SubmitExperiment", ctx, req)
}

func (m *MockExperimentServiceClientFixed) UpdateExperiment(ctx context.Context, req *expt.UpdateExperimentRequest, callOptions ...callopt.Option) (*expt.UpdateExperimentResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateExperiment", ctx, req)
	ret0, _ := ret[0].(*expt.UpdateExperimentResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) UpdateExperiment(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "UpdateExperiment", ctx, req)
}

func (m *MockExperimentServiceClientFixed) DeleteExperiment(ctx context.Context, req *expt.DeleteExperimentRequest, callOptions ...callopt.Option) (*expt.DeleteExperimentResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteExperiment", ctx, req)
	ret0, _ := ret[0].(*expt.DeleteExperimentResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) DeleteExperiment(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "DeleteExperiment", ctx, req)
}

// 为其他方法添加mock支持
func (m *MockExperimentServiceClientFixed) CloneExperiment(ctx context.Context, req *expt.CloneExperimentRequest, callOptions ...callopt.Option) (*expt.CloneExperimentResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloneExperiment", ctx, req)
	ret0, _ := ret[0].(*expt.CloneExperimentResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) CloneExperiment(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "CloneExperiment", ctx, req)
}

func (m *MockExperimentServiceClientFixed) RetryExperiment(ctx context.Context, req *expt.RetryExperimentRequest, callOptions ...callopt.Option) (*expt.RetryExperimentResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RetryExperiment", ctx, req)
	ret0, _ := ret[0].(*expt.RetryExperimentResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) RetryExperiment(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "RetryExperiment", ctx, req)
}

func (m *MockExperimentServiceClientFixed) KillExperiment(ctx context.Context, req *expt.KillExperimentRequest, callOptions ...callopt.Option) (*expt.KillExperimentResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "KillExperiment", ctx, req)
	ret0, _ := ret[0].(*expt.KillExperimentResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) KillExperiment(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "KillExperiment", ctx, req)
}

func (m *MockExperimentServiceClientFixed) BatchGetExperiments(ctx context.Context, req *expt.BatchGetExperimentsRequest, callOptions ...callopt.Option) (*expt.BatchGetExperimentsResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BatchGetExperiments", ctx, req)
	ret0, _ := ret[0].(*expt.BatchGetExperimentsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) BatchGetExperiments(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "BatchGetExperiments", ctx, req)
}

func (m *MockExperimentServiceClientFixed) ListExperiments(ctx context.Context, req *expt.ListExperimentsRequest, callOptions ...callopt.Option) (*expt.ListExperimentsResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListExperiments", ctx, req)
	ret0, _ := ret[0].(*expt.ListExperimentsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) ListExperiments(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "ListExperiments", ctx, req)
}

func (m *MockExperimentServiceClientFixed) BatchDeleteExperiments(ctx context.Context, req *expt.BatchDeleteExperimentsRequest, callOptions ...callopt.Option) (*expt.BatchDeleteExperimentsResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BatchDeleteExperiments", ctx, req)
	ret0, _ := ret[0].(*expt.BatchDeleteExperimentsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) BatchDeleteExperiments(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "BatchDeleteExperiments", ctx, req)
}

func (m *MockExperimentServiceClientFixed) BatchGetExperimentResult_(ctx context.Context, req *expt.BatchGetExperimentResultRequest, callOptions ...callopt.Option) (*expt.BatchGetExperimentResultResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BatchGetExperimentResult_", ctx, req)
	ret0, _ := ret[0].(*expt.BatchGetExperimentResultResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) BatchGetExperimentResult_(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "BatchGetExperimentResult_", ctx, req)
}

func (m *MockExperimentServiceClientFixed) BatchGetExperimentAggrResult_(ctx context.Context, req *expt.BatchGetExperimentAggrResultRequest, callOptions ...callopt.Option) (*expt.BatchGetExperimentAggrResultResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BatchGetExperimentAggrResult_", ctx, req)
	ret0, _ := ret[0].(*expt.BatchGetExperimentAggrResultResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) BatchGetExperimentAggrResult_(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "BatchGetExperimentAggrResult_", ctx, req)
}

func (m *MockExperimentServiceClientFixed) InvokeExperiment(ctx context.Context, req *expt.InvokeExperimentRequest, callOptions ...callopt.Option) (*expt.InvokeExperimentResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "InvokeExperiment", ctx, req)
	ret0, _ := ret[0].(*expt.InvokeExperimentResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) InvokeExperiment(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "InvokeExperiment", ctx, req)
}

func (m *MockExperimentServiceClientFixed) FinishExperiment(ctx context.Context, req *expt.FinishExperimentRequest, callOptions ...callopt.Option) (*expt.FinishExperimentResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FinishExperiment", ctx, req)
	ret0, _ := ret[0].(*expt.FinishExperimentResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) FinishExperiment(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "FinishExperiment", ctx, req)
}

func (m *MockExperimentServiceClientFixed) AssociateAnnotationTag(ctx context.Context, req *expt.AssociateAnnotationTagReq, callOptions ...callopt.Option) (*expt.AssociateAnnotationTagResp, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AssociateAnnotationTag", ctx, req)
	ret0, _ := ret[0].(*expt.AssociateAnnotationTagResp)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) AssociateAnnotationTag(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "AssociateAnnotationTag", ctx, req)
}

func (m *MockExperimentServiceClientFixed) DeleteAnnotationTag(ctx context.Context, req *expt.DeleteAnnotationTagReq, callOptions ...callopt.Option) (*expt.DeleteAnnotationTagResp, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteAnnotationTag", ctx, req)
	ret0, _ := ret[0].(*expt.DeleteAnnotationTagResp)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) DeleteAnnotationTag(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "DeleteAnnotationTag", ctx, req)
}

func (m *MockExperimentServiceClientFixed) CreateAnnotateRecord(ctx context.Context, req *expt.CreateAnnotateRecordReq, callOptions ...callopt.Option) (*expt.CreateAnnotateRecordResp, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateAnnotateRecord", ctx, req)
	ret0, _ := ret[0].(*expt.CreateAnnotateRecordResp)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) CreateAnnotateRecord(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "CreateAnnotateRecord", ctx, req)
}

func (m *MockExperimentServiceClientFixed) UpdateAnnotateRecord(ctx context.Context, req *expt.UpdateAnnotateRecordReq, callOptions ...callopt.Option) (*expt.UpdateAnnotateRecordResp, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateAnnotateRecord", ctx, req)
	ret0, _ := ret[0].(*expt.UpdateAnnotateRecordResp)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) UpdateAnnotateRecord(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "UpdateAnnotateRecord", ctx, req)
}

func (m *MockExperimentServiceClientFixed) ExportExptResult_(ctx context.Context, req *expt.ExportExptResultRequest, callOptions ...callopt.Option) (*expt.ExportExptResultResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExportExptResult_", ctx, req)
	ret0, _ := ret[0].(*expt.ExportExptResultResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) ExportExptResult_(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "ExportExptResult_", ctx, req)
}

func (m *MockExperimentServiceClientFixed) ListExptResultExportRecord(ctx context.Context, req *expt.ListExptResultExportRecordRequest, callOptions ...callopt.Option) (*expt.ListExptResultExportRecordResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListExptResultExportRecord", ctx, req)
	ret0, _ := ret[0].(*expt.ListExptResultExportRecordResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) ListExptResultExportRecord(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "ListExptResultExportRecord", ctx, req)
}

func (m *MockExperimentServiceClientFixed) GetExptResultExportRecord(ctx context.Context, req *expt.GetExptResultExportRecordRequest, callOptions ...callopt.Option) (*expt.GetExptResultExportRecordResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetExptResultExportRecord", ctx, req)
	ret0, _ := ret[0].(*expt.GetExptResultExportRecordResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockExperimentServiceClientFixedMockRecorder) GetExptResultExportRecord(ctx, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCall(mr.mock, "GetExptResultExportRecord", ctx, req)
}

// 添加其他必需的方法存根
func (m *MockExperimentServiceClientFixed) CreateExperiment(ctx context.Context, req *expt.CreateExperimentRequest, callOptions ...callopt.Option) (*expt.CreateExperimentResponse, error) {
	return &expt.CreateExperimentResponse{}, nil
}

func (m *MockExperimentServiceClientFixed) RunExperiment(ctx context.Context, req *expt.RunExperimentRequest, callOptions ...callopt.Option) (*expt.RunExperimentResponse, error) {
	return &expt.RunExperimentResponse{}, nil
}

func (m *MockExperimentServiceClientFixed) ListExperimentStats(ctx context.Context, req *expt.ListExperimentStatsRequest, callOptions ...callopt.Option) (*expt.ListExperimentStatsResponse, error) {
	return &expt.ListExperimentStatsResponse{}, nil
}

func (m *MockExperimentServiceClientFixed) UpsertExptTurnResultFilter(ctx context.Context, req *expt.UpsertExptTurnResultFilterRequest, callOptions ...callopt.Option) (*expt.UpsertExptTurnResultFilterResponse, error) {
	return &expt.UpsertExptTurnResultFilterResponse{}, nil
}

func (m *MockExperimentServiceClientFixed) InsightAnalysisExperiment(ctx context.Context, req *expt.InsightAnalysisExperimentRequest, callOptions ...callopt.Option) (*expt.InsightAnalysisExperimentResponse, error) {
	return &expt.InsightAnalysisExperimentResponse{}, nil
}

func (m *MockExperimentServiceClientFixed) ListExptInsightAnalysisRecord(ctx context.Context, req *expt.ListExptInsightAnalysisRecordRequest, callOptions ...callopt.Option) (*expt.ListExptInsightAnalysisRecordResponse, error) {
	return &expt.ListExptInsightAnalysisRecordResponse{}, nil
}

func (m *MockExperimentServiceClientFixed) DeleteExptInsightAnalysisRecord(ctx context.Context, req *expt.DeleteExptInsightAnalysisRecordRequest, callOptions ...callopt.Option) (*expt.DeleteExptInsightAnalysisRecordResponse, error) {
	return &expt.DeleteExptInsightAnalysisRecordResponse{}, nil
}

func (m *MockExperimentServiceClientFixed) GetExptInsightAnalysisRecord(ctx context.Context, req *expt.GetExptInsightAnalysisRecordRequest, callOptions ...callopt.Option) (*expt.GetExptInsightAnalysisRecordResponse, error) {
	return &expt.GetExptInsightAnalysisRecordResponse{}, nil
}

func (m *MockExperimentServiceClientFixed) FeedbackExptInsightAnalysisReport(ctx context.Context, req *expt.FeedbackExptInsightAnalysisReportRequest, callOptions ...callopt.Option) (*expt.FeedbackExptInsightAnalysisReportResponse, error) {
	return &expt.FeedbackExptInsightAnalysisReportResponse{}, nil
}

func (m *MockExperimentServiceClientFixed) ListExptInsightAnalysisComment(ctx context.Context, req *expt.ListExptInsightAnalysisCommentRequest, callOptions ...callopt.Option) (*expt.ListExptInsightAnalysisCommentResponse, error) {
	return &expt.ListExptInsightAnalysisCommentResponse{}, nil
}

func TestCheckExperimentName(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		mockSetup      func(*MockExperimentServiceClientFixed)
		expectedStatus int
		wantErr        bool
	}{
		{
			name:        "成功检查实验名称",
			requestBody: `{"space_id": 123, "name": "test-experiment"}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().CheckExperimentName(gomock.Any(), gomock.Any()).Return(
					&expt.CheckExperimentNameResponse{Pass: func(b bool) *bool { return &b }(true)}, nil,
				).Times(1)
			},
			expectedStatus: consts.StatusOK,
			wantErr:        false,
		},
		{
			name:        "服务返回错误",
			requestBody: `{"space_id": 123, "name": "test-experiment"}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().CheckExperimentName(gomock.Any(), gomock.Any()).Return(
					nil, errors.New("service error"),
				).Times(1)
			},
			expectedStatus: consts.StatusInternalServerError,
			wantErr:        true,
		},
		{
			name:           "无效的请求体",
			requestBody:    `invalid json`,
			mockSetup:      func(mock *MockExperimentServiceClientFixed) {},
			expectedStatus: consts.StatusBadRequest,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := NewMockExperimentServiceClientFixed(ctrl)
			tt.mockSetup(mockClient)

			originalClient := localExptSvc
			localExptSvc = mockClient
			defer func() {
				localExptSvc = originalClient
			}()

			ctx := context.Background()
			c := app.NewContext(0)
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			CheckExperimentName(ctx, c)

			statusCode := c.Response.StatusCode()
			if tt.wantErr {
				assert.True(t, statusCode >= 400)
			} else {
				assert.DeepEqual(t, tt.expectedStatus, statusCode)
			}
		})
	}
}

func TestSubmitExperiment(t *testing.T) {

	tests := []struct {
		name           string
		requestBody    string
		mockSetup      func(*MockExperimentServiceClientFixed)
		expectedStatus int
		wantErr        bool
	}{
		{
			name:        "成功提交实验",
			requestBody: `{"space_id": 123, "name": "test-experiment"}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().SubmitExperiment(gomock.Any(), gomock.Any()).Return(
					&expt.SubmitExperimentResponse{}, nil,
				).Times(1)
			},
			expectedStatus: consts.StatusOK,
			wantErr:        false,
		},
		{
			name:        "服务返回错误",
			requestBody: `{"space_id": 123, "name": "test-experiment"}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().SubmitExperiment(gomock.Any(), gomock.Any()).Return(
					nil, errors.New("submit failed"),
				).Times(1)
			},
			expectedStatus: consts.StatusInternalServerError,
			wantErr:        true,
		},
		{
			name:           "无效的请求体",
			requestBody:    `invalid json`,
			mockSetup:      func(mock *MockExperimentServiceClientFixed) {},
			expectedStatus: consts.StatusBadRequest,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := NewMockExperimentServiceClientFixed(ctrl)
			tt.mockSetup(mockClient)

			originalClient := localExptSvc
			localExptSvc = mockClient
			defer func() {
				localExptSvc = originalClient
			}()

			ctx := context.Background()
			c := app.NewContext(0)
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			SubmitExperiment(ctx, c)

			statusCode := c.Response.StatusCode()
			if tt.wantErr {
				assert.True(t, statusCode >= 400)
			} else {
				assert.DeepEqual(t, tt.expectedStatus, statusCode)
			}
		})
	}
}func TestUpdateExperiment(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		mockSetup      func(*MockExperimentServiceClientFixed)
		expectedStatus int
		wantErr        bool
	}{
		{
			name:        "成功更新实验",
			requestBody: `{"experiment_id": 456, "name": "updated-experiment"}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().UpdateExperiment(gomock.Any(), gomock.Any()).Return(
					&expt.UpdateExperimentResponse{}, nil,
				).Times(1)
			},
			expectedStatus: consts.StatusOK,
			wantErr:        false,
		},
		{
			name:        "更新失败",
			requestBody: `{"experiment_id": 456, "name": "updated-experiment"}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().UpdateExperiment(gomock.Any(), gomock.Any()).Return(
					nil, errors.New("update failed"),
				).Times(1)
			},
			expectedStatus: consts.StatusInternalServerError,
			wantErr:        true,
		},
		{
			name:           "无效的请求体",
			requestBody:    `invalid json`,
			mockSetup:      func(mock *MockExperimentServiceClientFixed) {},
			expectedStatus: consts.StatusBadRequest,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := NewMockExperimentServiceClientFixed(ctrl)
			tt.mockSetup(mockClient)

			originalClient := localExptSvc
			localExptSvc = mockClient
			defer func() {
				localExptSvc = originalClient
			}()

			ctx := context.Background()
			c := app.NewContext(0)
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			UpdateExperiment(ctx, c)

			statusCode := c.Response.StatusCode()
			if tt.wantErr {
				assert.True(t, statusCode >= 400)
			} else {
				assert.DeepEqual(t, tt.expectedStatus, statusCode)
			}
		})
	}
}

func TestDeleteExperiment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		mockSetup      func(*MockExperimentServiceClientFixed)
		expectedStatus int
		wantErr        bool
	}{
		{
			name:        "成功删除实验",
			requestBody: `{"experiment_id": 456}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().DeleteExperiment(gomock.Any(), gomock.Any()).Return(
					&expt.DeleteExperimentResponse{}, nil,
				).Times(1)
			},
			expectedStatus: consts.StatusOK,
			wantErr:        false,
		},
		{
			name:        "删除失败",
			requestBody: `{"experiment_id": 456}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().DeleteExperiment(gomock.Any(), gomock.Any()).Return(
					nil, errors.New("delete failed"),
				).Times(1)
			},
			expectedStatus: consts.StatusInternalServerError,
			wantErr:        true,
		},
		{
			name:           "无效的请求体",
			requestBody:    `invalid json`,
			mockSetup:      func(mock *MockExperimentServiceClientFixed) {},
			expectedStatus: consts.StatusBadRequest,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := NewMockExperimentServiceClientFixed(ctrl)
			tt.mockSetup(mockClient)

			originalClient := localExptSvc
			localExptSvc = mockClient
			defer func() {
				localExptSvc = originalClient
			}()

			ctx := context.Background()
			c := app.NewContext(0)
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			DeleteExperiment(ctx, c)

			statusCode := c.Response.StatusCode()
			if tt.wantErr {
				assert.True(t, statusCode >= 400)
			} else {
				assert.DeepEqual(t, tt.expectedStatus, statusCode)
			}
		})
	}
}

func TestCloneExperiment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		mockSetup      func(*MockExperimentServiceClientFixed)
		expectedStatus int
		wantErr        bool
	}{
		{
			name:        "成功克隆实验",
			requestBody: `{"experiment_id": 456}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().CloneExperiment(gomock.Any(), gomock.Any()).Return(
					&expt.CloneExperimentResponse{}, nil,
				).Times(1)
			},
			expectedStatus: consts.StatusOK,
			wantErr:        false,
		},
		{
			name:        "克隆失败",
			requestBody: `{"experiment_id": 456}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().CloneExperiment(gomock.Any(), gomock.Any()).Return(
					nil, errors.New("clone failed"),
				).Times(1)
			},
			expectedStatus: consts.StatusInternalServerError,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := NewMockExperimentServiceClientFixed(ctrl)
			tt.mockSetup(mockClient)

			originalClient := localExptSvc
			localExptSvc = mockClient
			defer func() {
				localExptSvc = originalClient
			}()

			ctx := context.Background()
			c := app.NewContext(0)
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			CloneExperiment(ctx, c)

			statusCode := c.Response.StatusCode()
			if tt.wantErr {
				assert.True(t, statusCode >= 400)
			} else {
				assert.DeepEqual(t, tt.expectedStatus, statusCode)
			}
		})
	}
}

func TestRetryExperiment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		mockSetup      func(*MockExperimentServiceClientFixed)
		expectedStatus int
		wantErr        bool
	}{
		{
			name:        "成功重试实验",
			requestBody: `{"experiment_id": 456}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().RetryExperiment(gomock.Any(), gomock.Any()).Return(
					&expt.RetryExperimentResponse{}, nil,
				).Times(1)
			},
			expectedStatus: consts.StatusOK,
			wantErr:        false,
		},
		{
			name:        "重试失败",
			requestBody: `{"experiment_id": 456}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().RetryExperiment(gomock.Any(), gomock.Any()).Return(
					nil, errors.New("retry failed"),
				).Times(1)
			},
			expectedStatus: consts.StatusInternalServerError,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := NewMockExperimentServiceClientFixed(ctrl)
			tt.mockSetup(mockClient)

			originalClient := localExptSvc
			localExptSvc = mockClient
			defer func() {
				localExptSvc = originalClient
			}()

			ctx := context.Background()
			c := app.NewContext(0)
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			RetryExperiment(ctx, c)

			statusCode := c.Response.StatusCode()
			if tt.wantErr {
				assert.True(t, statusCode >= 400)
			} else {
				assert.DeepEqual(t, tt.expectedStatus, statusCode)
			}
		})
	}
}

func TestKillExperiment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		mockSetup      func(*MockExperimentServiceClientFixed)
		expectedStatus int
		wantErr        bool
	}{
		{
			name:        "成功终止实验",
			requestBody: `{"experiment_id": 456}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().KillExperiment(gomock.Any(), gomock.Any()).Return(
					&expt.KillExperimentResponse{}, nil,
				).Times(1)
			},
			expectedStatus: consts.StatusOK,
			wantErr:        false,
		},
		{
			name:        "终止失败",
			requestBody: `{"experiment_id": 456}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().KillExperiment(gomock.Any(), gomock.Any()).Return(
					nil, errors.New("kill failed"),
				).Times(1)
			},
			expectedStatus: consts.StatusInternalServerError,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := NewMockExperimentServiceClientFixed(ctrl)
			tt.mockSetup(mockClient)

			originalClient := localExptSvc
			localExptSvc = mockClient
			defer func() {
				localExptSvc = originalClient
			}()

			ctx := context.Background()
			c := app.NewContext(0)
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			KillExperiment(ctx, c)

			statusCode := c.Response.StatusCode()
			if tt.wantErr {
				assert.True(t, statusCode >= 400)
			} else {
				assert.DeepEqual(t, tt.expectedStatus, statusCode)
			}
		})
	}
}

func TestBatchGetExperiments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		mockSetup      func(*MockExperimentServiceClientFixed)
		expectedStatus int
		wantErr        bool
	}{
		{
			name:        "成功批量获取实验",
			requestBody: `{"experiment_ids": [123, 456]}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().BatchGetExperiments(gomock.Any(), gomock.Any()).Return(
					&expt.BatchGetExperimentsResponse{}, nil,
				).Times(1)
			},
			expectedStatus: consts.StatusOK,
			wantErr:        false,
		},
		{
			name:        "批量获取失败",
			requestBody: `{"experiment_ids": [123, 456]}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().BatchGetExperiments(gomock.Any(), gomock.Any()).Return(
					nil, errors.New("batch get failed"),
				).Times(1)
			},
			expectedStatus: consts.StatusInternalServerError,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := NewMockExperimentServiceClientFixed(ctrl)
			tt.mockSetup(mockClient)

			originalClient := localExptSvc
			localExptSvc = mockClient
			defer func() {
				localExptSvc = originalClient
			}()

			ctx := context.Background()
			c := app.NewContext(0)
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			BatchGetExperiments(ctx, c)

			statusCode := c.Response.StatusCode()
			if tt.wantErr {
				assert.True(t, statusCode >= 400)
			} else {
				assert.DeepEqual(t, tt.expectedStatus, statusCode)
			}
		})
	}
}

func TestListExperiments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		mockSetup      func(*MockExperimentServiceClientFixed)
		expectedStatus int
		wantErr        bool
	}{
		{
			name:        "成功列出实验",
			requestBody: `{"space_id": 123}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().ListExperiments(gomock.Any(), gomock.Any()).Return(
					&expt.ListExperimentsResponse{}, nil,
				).Times(1)
			},
			expectedStatus: consts.StatusOK,
			wantErr:        false,
		},
		{
			name:        "列出失败",
			requestBody: `{"space_id": 123}`,
			mockSetup: func(mock *MockExperimentServiceClientFixed) {
				mock.EXPECT().ListExperiments(gomock.Any(), gomock.Any()).Return(
					nil, errors.New("list failed"),
				).Times(1)
			},
			expectedStatus: consts.StatusInternalServerError,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := NewMockExperimentServiceClientFixed(ctrl)
			tt.mockSetup(mockClient)

			originalClient := localExptSvc
			localExptSvc = mockClient
			defer func() {
				localExptSvc = originalClient
			}()

			ctx := context.Background()
			c := app.NewContext(0)
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			ListExperiments(ctx, c)

			statusCode := c.Response.StatusCode()
			if tt.wantErr {
				assert.True(t, statusCode >= 400)
			} else {
				assert.DeepEqual(t, tt.expectedStatus, statusCode)
			}
		})
	}
}

func TestListExperimentStats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		wantErr        bool
	}{
		{
			name:           "成功获取实验统计",
			requestBody:    `{}`,
			expectedStatus: consts.StatusOK,
			wantErr:        false,
		},
		{
			name:           "请求参数格式错误",
			requestBody:    `invalid json`,
			expectedStatus: consts.StatusBadRequest,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx := context.Background()
			c := app.NewContext(0)
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			ListExperimentStats(ctx, c)

			statusCode := c.Response.StatusCode()
			if tt.wantErr {
				assert.True(t, statusCode >= 400)
			} else {
				assert.DeepEqual(t, tt.expectedStatus, statusCode)
			}
		})
	}
}

func TestUpsertExptTurnResultFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		wantErr        bool
	}{
		{
			name:           "成功更新实验结果过滤器",
			requestBody:    `{}`,
			expectedStatus: consts.StatusOK,
			wantErr:        false,
		},
		{
			name:           "请求参数格式错误",
			requestBody:    `invalid json`,
			expectedStatus: consts.StatusBadRequest,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx := context.Background()
			c := app.NewContext(0)
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			UpsertExptTurnResultFilter(ctx, c)

			statusCode := c.Response.StatusCode()
			if tt.wantErr {
				assert.True(t, statusCode >= 400)
			} else {
				assert.DeepEqual(t, tt.expectedStatus, statusCode)
			}
		})
	}
}

func TestInsightAnalysisExperiment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		wantErr        bool
	}{
		{
			name:           "成功进行洞察分析",
			requestBody:    `{}`,
			expectedStatus: consts.StatusOK,
			wantErr:        false,
		},
		{
			name:           "请求参数格式错误",
			requestBody:    `invalid json`,
			expectedStatus: consts.StatusBadRequest,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx := context.Background()
			c := app.NewContext(0)
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			InsightAnalysisExperiment(ctx, c)

			statusCode := c.Response.StatusCode()
			if tt.wantErr {
				assert.True(t, statusCode >= 400)
			} else {
				assert.DeepEqual(t, tt.expectedStatus, statusCode)
			}
		})
	}
}
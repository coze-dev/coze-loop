// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type evalTargetServiceTestDeps struct {
	repo     *repomocks.MockIEvalTargetRepo
	idgen    *idgenmocks.MockIIDGenerator
	metric   *metricsmocks.MockEvalTargetMetrics
	operator *servicemocks.MockISourceEvalTargetOperateService
}

func TestEvalTargetServiceImpl_CreateEvalTarget(t *testing.T) {
	t.Parallel()

	type args struct {
		spaceID             int64
		sourceTargetID      string
		sourceTargetVersion string
		targetType          entity.EvalTargetType
	}

	tests := []struct {
		name        string
		args        args
		prepare     func(ctx context.Context, deps *evalTargetServiceTestDeps) map[entity.EvalTargetType]ISourceEvalTargetOperateService
		wantErr     bool
		wantErrCode int32
		wantID      int64
		wantVersion int64
	}{
		{
			name: "unsupported target type",
			args: args{
				spaceID:             1,
				sourceTargetID:      "src",
				sourceTargetVersion: "v1",
				targetType:          entity.EvalTargetTypeLoopPrompt,
			},
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps) map[entity.EvalTargetType]ISourceEvalTargetOperateService {
				return map[entity.EvalTargetType]ISourceEvalTargetOperateService{}
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "build by source error",
			args: args{
				spaceID:             1,
				sourceTargetID:      "src",
				sourceTargetVersion: "v1",
				targetType:          entity.EvalTargetTypeLoopPrompt,
			},
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps) map[entity.EvalTargetType]ISourceEvalTargetOperateService {
				deps.operator.EXPECT().BuildBySource(ctx, int64(1), "src", "v1").Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
				return map[entity.EvalTargetType]ISourceEvalTargetOperateService{
					entity.EvalTargetTypeLoopPrompt: deps.operator,
				}
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "build by source returns nil",
			args: args{
				spaceID:             1,
				sourceTargetID:      "src",
				sourceTargetVersion: "v1",
				targetType:          entity.EvalTargetTypeLoopPrompt,
			},
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps) map[entity.EvalTargetType]ISourceEvalTargetOperateService {
				deps.operator.EXPECT().BuildBySource(ctx, int64(1), "src", "v1").Return(nil, nil)
				return map[entity.EvalTargetType]ISourceEvalTargetOperateService{
					entity.EvalTargetTypeLoopPrompt: deps.operator,
				}
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "success",
			args: args{
				spaceID:             1,
				sourceTargetID:      "src",
				sourceTargetVersion: "v1",
				targetType:          entity.EvalTargetTypeLoopPrompt,
			},
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps) map[entity.EvalTargetType]ISourceEvalTargetOperateService {
				expectDO := &entity.EvalTarget{
					SpaceID:        1,
					SourceTargetID: "src",
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
				}
				deps.operator.EXPECT().BuildBySource(ctx, int64(1), "src", "v1").Return(expectDO, nil)
				deps.repo.EXPECT().CreateEvalTarget(ctx, expectDO).Return(int64(11), int64(22), nil)
				return map[entity.EvalTargetType]ISourceEvalTargetOperateService{
					entity.EvalTargetTypeLoopPrompt: deps.operator,
				}
			},
			wantID:      11,
			wantVersion: 22,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			deps := &evalTargetServiceTestDeps{
				repo:     repomocks.NewMockIEvalTargetRepo(ctrl),
				idgen:    idgenmocks.NewMockIIDGenerator(ctrl),
				metric:   metricsmocks.NewMockEvalTargetMetrics(ctrl),
				operator: servicemocks.NewMockISourceEvalTargetOperateService(ctrl),
			}

			typedOps := map[entity.EvalTargetType]ISourceEvalTargetOperateService{}
			if tt.prepare != nil {
				typedOps = tt.prepare(ctx, deps)
			}

			deps.metric.EXPECT().EmitCreate(tt.args.spaceID, gomock.Any()).Times(1)

			svc := &EvalTargetServiceImpl{
				evalTargetRepo: deps.repo,
				idgen:          deps.idgen,
				metric:         deps.metric,
				typedOperators: typedOps,
			}

			gotID, gotVersion, err := svc.CreateEvalTarget(ctx, tt.args.spaceID, tt.args.sourceTargetID, tt.args.sourceTargetVersion, tt.args.targetType)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					require.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantID, gotID)
			assert.Equal(t, tt.wantVersion, gotVersion)
		})
	}
}

func TestEvalTargetServiceImpl_GetEvalTargetVersion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	type args struct {
		spaceID        int64
		versionID      int64
		needSourceInfo bool
	}

	tests := []struct {
		name    string
		args    args
		prepare func(deps *evalTargetServiceTestDeps, expectDo *entity.EvalTarget)
		wantErr bool
	}{
		{
			name: "repo error",
			args: args{spaceID: 1, versionID: 2, needSourceInfo: false},
			prepare: func(deps *evalTargetServiceTestDeps, expectDo *entity.EvalTarget) {
				deps.repo.EXPECT().GetEvalTargetVersion(ctx, int64(1), int64(2)).Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr: true,
		},
		{
			name: "need source info",
			args: args{spaceID: 1, versionID: 2, needSourceInfo: true},
			prepare: func(deps *evalTargetServiceTestDeps, expectDo *entity.EvalTarget) {
				deps.repo.EXPECT().GetEvalTargetVersion(ctx, int64(1), int64(2)).Return(expectDo, nil)
				deps.operator.EXPECT().PackSourceVersionInfo(ctx, int64(1), []*entity.EvalTarget{expectDo}).Return(nil)
			},
		},
		{
			name: "without source info",
			args: args{spaceID: 1, versionID: 2, needSourceInfo: false},
			prepare: func(deps *evalTargetServiceTestDeps, expectDo *entity.EvalTarget) {
				deps.repo.EXPECT().GetEvalTargetVersion(ctx, int64(1), int64(2)).Return(expectDo, nil)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			deps := &evalTargetServiceTestDeps{
				repo:     repomocks.NewMockIEvalTargetRepo(ctrl),
				idgen:    idgenmocks.NewMockIIDGenerator(ctrl),
				metric:   metricsmocks.NewMockEvalTargetMetrics(ctrl),
				operator: servicemocks.NewMockISourceEvalTargetOperateService(ctrl),
			}

			expectDo := &entity.EvalTarget{
				ID:             3,
				SpaceID:        tt.args.spaceID,
				EvalTargetType: entity.EvalTargetTypeLoopPrompt,
				EvalTargetVersion: &entity.EvalTargetVersion{
					ID: tt.args.versionID,
				},
			}

			if tt.prepare != nil {
				tt.prepare(deps, expectDo)
			}

			typedOps := map[entity.EvalTargetType]ISourceEvalTargetOperateService{}
			if tt.args.needSourceInfo {
				typedOps[entity.EvalTargetTypeLoopPrompt] = deps.operator
			}

			svc := &EvalTargetServiceImpl{
				evalTargetRepo: deps.repo,
				idgen:          deps.idgen,
				metric:         deps.metric,
				typedOperators: typedOps,
			}

			do, err := svc.GetEvalTargetVersion(ctx, tt.args.spaceID, tt.args.versionID, tt.args.needSourceInfo)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, expectDo, do)
		})
	}
}

func TestEvalTargetServiceImpl_asyncExecuteTarget(t *testing.T) {
	t.Parallel()

	type prepareFunc func(ctx context.Context, deps *evalTargetServiceTestDeps, target *entity.EvalTarget, input *entity.EvalTargetInputData)

	tests := []struct {
		name         string
		prepare      prepareFunc
		wantErr      bool
		wantErrCode  int32
		expectCallee string
		expectID     int64
	}{
		{
			name: "validate input failed",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, target *entity.EvalTarget, input *entity.EvalTargetInputData) {
				deps.operator.EXPECT().ValidateInput(ctx, target.SpaceID, target.EvalTargetVersion.InputSchema, input).Return(errorx.NewByCode(errno.CommonInvalidParamCode))
				deps.metric.EXPECT().EmitRun(target.SpaceID, gomock.Any(), gomock.Any()).Times(1)
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "async execute failed",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, target *entity.EvalTarget, input *entity.EvalTargetInputData) {
				deps.operator.EXPECT().ValidateInput(ctx, target.SpaceID, target.EvalTargetVersion.InputSchema, input).Return(nil)
				deps.operator.EXPECT().AsyncExecute(ctx, target.SpaceID, gomock.Any()).Return(int64(0), "callee", errorx.NewByCode(errno.CommonInternalErrorCode))
				deps.metric.EXPECT().EmitRun(target.SpaceID, gomock.Any(), gomock.Any()).Times(1)
			},
			wantErr:      true,
			wantErrCode:  errno.CommonInternalErrorCode,
			expectCallee: "callee",
		},
		{
			name: "success",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, target *entity.EvalTarget, input *entity.EvalTargetInputData) {
				deps.operator.EXPECT().ValidateInput(ctx, target.SpaceID, target.EvalTargetVersion.InputSchema, input).Return(nil)
				deps.operator.EXPECT().AsyncExecute(ctx, target.SpaceID, gomock.Any()).Return(int64(999), "callee", nil)
				deps.repo.EXPECT().CreateEvalTargetRecord(ctx, gomock.Any()).Return(int64(999), nil)
				deps.metric.EXPECT().EmitRun(target.SpaceID, gomock.Any(), gomock.Any()).Times(1)
			},
			expectCallee: "callee",
			expectID:     999,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			deps := &evalTargetServiceTestDeps{
				repo:     repomocks.NewMockIEvalTargetRepo(ctrl),
				idgen:    idgenmocks.NewMockIIDGenerator(ctrl),
				metric:   metricsmocks.NewMockEvalTargetMetrics(ctrl),
				operator: servicemocks.NewMockISourceEvalTargetOperateService(ctrl),
			}

			target := &entity.EvalTarget{
				ID:             1,
				SpaceID:        1,
				SourceTargetID: "source",
				EvalTargetType: entity.EvalTargetTypeCustomRPCServer,
				EvalTargetVersion: &entity.EvalTargetVersion{
					ID:                  2,
					SourceTargetVersion: "v1",
					InputSchema: []*entity.ArgsSchema{
						{Key: gptr.Of("a")},
					},
				},
			}
			input := &entity.EvalTargetInputData{InputFields: map[string]*entity.Content{"a": {ContentType: gptr.Of(entity.ContentTypeText)}}}

			typedOps := map[entity.EvalTargetType]ISourceEvalTargetOperateService{
				entity.EvalTargetTypeCustomRPCServer: deps.operator,
			}

			svc := &EvalTargetServiceImpl{
				evalTargetRepo: deps.repo,
				idgen:          deps.idgen,
				metric:         deps.metric,
				typedOperators: typedOps,
			}

			if tt.prepare != nil {
				tt.prepare(ctx, deps, target, input)
			}

			record, callee, err := svc.asyncExecuteTarget(ctx, target.SpaceID, target, &entity.ExecuteTargetCtx{ItemID: 1, TurnID: 2}, input)

			if tt.wantErr {
				require.Error(t, err)
				statusErr, ok := errorx.FromStatusError(err)
				require.True(t, ok)
				assert.Equal(t, tt.wantErrCode, statusErr.Code())
				assert.Equal(t, tt.expectCallee, callee)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, record)
			assert.Equal(t, tt.expectCallee, callee)
			assert.Equal(t, tt.expectID, record.ID)
			assert.Equal(t, entity.EvalTargetRunStatusAsyncInvoking, gptr.Indirect(record.Status))
		})
	}
}

func TestEvalTargetServiceImpl_ReportInvokeRecords(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name    string
		prepare func(deps *evalTargetServiceTestDeps, param *entity.ReportTargetRecordParam, record *entity.EvalTargetRecord)
		wantErr bool
		errCode int32
	}{
		{
			name: "record query error",
			prepare: func(deps *evalTargetServiceTestDeps, param *entity.ReportTargetRecordParam, record *entity.EvalTargetRecord) {
				deps.repo.EXPECT().GetEvalTargetRecordByIDAndSpaceID(ctx, param.SpaceID, param.RecordID).Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr: true,
			errCode: errno.CommonInternalErrorCode,
		},
		{
			name: "record not found",
			prepare: func(deps *evalTargetServiceTestDeps, param *entity.ReportTargetRecordParam, record *entity.EvalTargetRecord) {
				deps.repo.EXPECT().GetEvalTargetRecordByIDAndSpaceID(ctx, param.SpaceID, param.RecordID).Return(nil, nil)
			},
			wantErr: true,
			errCode: errno.CommonBadRequestCode,
		},
		{
			name: "status not async",
			prepare: func(deps *evalTargetServiceTestDeps, param *entity.ReportTargetRecordParam, record *entity.EvalTargetRecord) {
				status := entity.EvalTargetRunStatusSuccess
				record.Status = &status
				deps.repo.EXPECT().GetEvalTargetRecordByIDAndSpaceID(ctx, param.SpaceID, param.RecordID).Return(record, nil)
			},
			wantErr: true,
			errCode: errno.CommonBadRequestCode,
		},
		{
			name: "success",
			prepare: func(deps *evalTargetServiceTestDeps, param *entity.ReportTargetRecordParam, record *entity.EvalTargetRecord) {
				status := entity.EvalTargetRunStatusAsyncInvoking
				record.Status = &status
				record.EvalTargetOutputData = &entity.EvalTargetOutputData{}
				deps.repo.EXPECT().GetEvalTargetRecordByIDAndSpaceID(ctx, param.SpaceID, param.RecordID).Return(record, nil)
				var saved *entity.EvalTargetRecord
				deps.repo.EXPECT().SaveEvalTargetRecord(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rec *entity.EvalTargetRecord) error {
					saved = rec
					return nil
				})
				deps.repo.EXPECT().GetEvalTargetVersion(gomock.Any(), record.SpaceID, record.TargetVersionID).Return(&entity.EvalTarget{EvalTargetType: entity.EvalTargetTypeCustomRPCServer}, nil)
				deps.repo.EXPECT().CreateEvalTargetRecord(gomock.Any(), gomock.Any()).AnyTimes()
				deps.metric.EXPECT().EmitRun(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

				param.Session = &entity.Session{UserID: "user"}
				param.OutputData = &entity.EvalTargetOutputData{
					OutputFields: map[string]*entity.Content{
						"key": {
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("value"),
						},
					},
					EvalTargetUsage:    &entity.EvalTargetUsage{InputTokens: 1, OutputTokens: 2},
					EvalTargetRunError: &entity.EvalTargetRunError{Code: 1, Message: "oops"},
					TimeConsumingMS:    gptr.Of(int64(10)),
				}

				deps.metric.EXPECT().EmitRun(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

				t.Cleanup(func() {
					require.NotNil(t, saved)
					assert.Equal(t, param.OutputData, saved.EvalTargetOutputData)
					assert.Equal(t, param.Status, gptr.Indirect(saved.Status))
				})
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			deps := &evalTargetServiceTestDeps{
				repo:     repomocks.NewMockIEvalTargetRepo(ctrl),
				idgen:    idgenmocks.NewMockIIDGenerator(ctrl),
				metric:   metricsmocks.NewMockEvalTargetMetrics(ctrl),
				operator: servicemocks.NewMockISourceEvalTargetOperateService(ctrl),
			}

			svc := &EvalTargetServiceImpl{
				evalTargetRepo: deps.repo,
				idgen:          deps.idgen,
				metric:         deps.metric,
				typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{},
			}

			record := &entity.EvalTargetRecord{
				ID:              1,
				SpaceID:         1,
				TargetID:        2,
				TargetVersionID: 3,
				Status:          gptr.Of(entity.EvalTargetRunStatusAsyncInvoking),
			}
			param := &entity.ReportTargetRecordParam{
				SpaceID:  1,
				RecordID: 1,
				Status:   entity.EvalTargetRunStatusSuccess,
				OutputData: &entity.EvalTargetOutputData{
					OutputFields: map[string]*entity.Content{},
				},
			}

			if tt.prepare != nil {
				tt.prepare(deps, param, record)
			}

			err := svc.ReportInvokeRecords(ctx, param)
			if tt.wantErr {
				require.Error(t, err)
				statusErr, ok := errorx.FromStatusError(err)
				require.True(t, ok)
				assert.Equal(t, tt.errCode, statusErr.Code())
				return
			}

			require.NoError(t, err)
		})
	}
}

type fakeRuntimeParam struct {
	parseErr error
}

func (f *fakeRuntimeParam) GetJSONDemo() string  { return "{}" }
func (f *fakeRuntimeParam) GetJSONValue() string { return "{}" }
func (f *fakeRuntimeParam) ParseFromJSON(string) (entity.IRuntimeParam, error) {
	if f.parseErr != nil {
		return nil, f.parseErr
	}
	return &fakeRuntimeParam{}, nil
}

func TestEvalTargetServiceImpl_ValidateRuntimeParam(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	operator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)
	operator.EXPECT().RuntimeParam().Return(&fakeRuntimeParam{parseErr: nil}).Times(1)

	svc := &EvalTargetServiceImpl{
		typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
			entity.EvalTargetTypeLoopPrompt: operator,
		},
	}

	require.NoError(t, svc.ValidateRuntimeParam(context.Background(), entity.EvalTargetTypeLoopPrompt, "{}"))

	err := svc.ValidateRuntimeParam(context.Background(), entity.EvalTargetTypeLoopPrompt, "")
	require.NoError(t, err)

	err = svc.ValidateRuntimeParam(context.Background(), entity.EvalTargetTypeCustomRPCServer, "{}")
	require.Error(t, err)
}

func TestSetSpanInputOutput(t *testing.T) {
	t.Parallel()

	textType := entity.ContentTypeText
	imageType := entity.ContentTypeImage

	tests := []struct {
		name        string
		input       *entity.EvalTargetInputData
		output      *entity.EvalTargetOutputData
		wantInputs  int
		wantOutputs int
	}{
		{
			name: "text content",
			input: &entity.EvalTargetInputData{InputFields: map[string]*entity.Content{
				"text": {ContentType: &textType, Text: gptr.Of("hello")},
			}},
			output: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{
				"text": {ContentType: &textType, Text: gptr.Of("world")},
			}, EvalTargetUsage: &entity.EvalTargetUsage{InputTokens: 1, OutputTokens: 2}},
			wantInputs:  1,
			wantOutputs: 1,
		},
		{
			name: "image content",
			input: &entity.EvalTargetInputData{InputFields: map[string]*entity.Content{
				"image": {ContentType: &imageType, Image: &entity.Image{Name: gptr.Of("img"), URL: gptr.Of("http://img")}},
			}},
			output:      &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{}},
			wantInputs:  1,
			wantOutputs: 0,
		},
		{
			name:        "nil",
			input:       nil,
			output:      nil,
			wantInputs:  0,
			wantOutputs: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spanParam := &targetSpanTagsParams{}
			setSpanInputOutput(context.Background(), spanParam, tt.input, tt.output)
			assert.Len(t, spanParam.Inputs, tt.wantInputs)
			assert.Len(t, spanParam.Outputs, tt.wantOutputs)
		})
	}
}

func TestToTraceParts(t *testing.T) {
	t.Parallel()

	textType := entity.ContentTypeText
	imageType := entity.ContentTypeImage
	multipartType := entity.ContentTypeMultipart

	tests := []struct {
		name    string
		content *entity.Content
		wantLen int
	}{
		{
			name: "text",
			content: &entity.Content{
				ContentType: &textType,
				Text:        gptr.Of("hello"),
			},
			wantLen: 1,
		},
		{
			name: "image",
			content: &entity.Content{
				ContentType: &imageType,
				Image: &entity.Image{
					Name: gptr.Of("img"),
					URL:  gptr.Of("http://img"),
				},
			},
			wantLen: 1,
		},
		{
			name: "multipart",
			content: &entity.Content{
				ContentType: &multipartType,
				MultiPart: []*entity.Content{
					{ContentType: &textType, Text: gptr.Of("part1")},
					{ContentType: &textType, Text: gptr.Of("part2")},
				},
			},
			wantLen: 2,
		},
		{
			name: "unknown",
			content: &entity.Content{
				ContentType: nil,
			},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parts := toTraceParts(context.Background(), tt.content)
			assert.Len(t, parts, tt.wantLen)
		})
	}
}

func TestConvert2TraceString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{
			name:  "nil",
			input: nil,
			want:  "",
		},
		{
			name:  "map",
			input: map[string]string{"a": "b"},
			want:  "{\"a\":\"b\"}",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Convert2TraceString(tt.input)
			if tt.input == nil {
				assert.Equal(t, tt.want, got)
				return
			}

			var expect interface{}
			require.NoError(t, json.Unmarshal([]byte(tt.want), &expect))

			var actual interface{}
			require.NoError(t, json.Unmarshal([]byte(got), &actual))
			assert.Equal(t, expect, actual)
		})
	}
}

func TestEvalTargetServiceImpl_GenerateMockOutputData(t *testing.T) {
	t.Parallel()

	svc := &EvalTargetServiceImpl{}

	validSchema := `{"type":"object","properties":{"name":{"type":"string"}}}`
	invalidSchema := "invalid"

	tests := []struct {
		name    string
		schemas []*entity.ArgsSchema
		wantLen int
	}{
		{
			name:    "empty schema",
			schemas: nil,
			wantLen: 0,
		},
		{
			name: "valid schema",
			schemas: []*entity.ArgsSchema{
				{Key: gptr.Of("name"), JsonSchema: &validSchema},
			},
			wantLen: 1,
		},
		{
			name: "invalid schema",
			schemas: []*entity.ArgsSchema{
				{Key: gptr.Of("invalid"), JsonSchema: &invalidSchema},
			},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := svc.GenerateMockOutputData(tt.schemas)
			require.NoError(t, err)
			assert.Len(t, result, tt.wantLen)
		})
	}
}

func TestBuildPageByCursor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cursor   *string
		wantPage int32
		wantErr  bool
	}{
		{
			name:     "nil cursor",
			cursor:   nil,
			wantPage: 1,
		},
		{
			name:     "valid cursor",
			cursor:   gptr.Of("5"),
			wantPage: 5,
		},
		{
			name:    "invalid cursor",
			cursor:  gptr.Of("abc"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			page, err := buildPageByCursor(tt.cursor)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantPage, page)
		})
	}
}

// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	trajectorymocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
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
	configer *componentmocks.MockIConfiger
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
				configer: componentmocks.NewMockIConfiger(ctrl),
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
				configer: componentmocks.NewMockIConfiger(ctrl),
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
				configer: componentmocks.NewMockIConfiger(ctrl),
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

func TestEvalTargetServiceImpl_ExecuteTarget(t *testing.T) {
	type prepareResult struct {
		typedOps       map[entity.EvalTargetType]ISourceEvalTargetOperateService
		expectedOutput *entity.EvalTargetOutputData
	}

	tests := []struct {
		name        string
		prepare     func(ctx context.Context, deps *evalTargetServiceTestDeps, evalTarget *entity.EvalTarget, input *entity.EvalTargetInputData) prepareResult
		wantStatus  entity.EvalTargetRunStatus
		wantErrCode int32
	}{
		{
			name: "success",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, evalTarget *entity.EvalTarget, input *entity.EvalTargetInputData) prepareResult {
				outputData := &entity.EvalTargetOutputData{
					OutputFields: map[string]*entity.Content{
						"answer": {
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("ok"),
						},
					},
					EvalTargetUsage: &entity.EvalTargetUsage{InputTokens: 1, OutputTokens: 2},
				}
				deps.operator.EXPECT().ValidateInput(ctx, evalTarget.SpaceID, evalTarget.EvalTargetVersion.InputSchema, input).Return(nil)
				deps.operator.EXPECT().Execute(ctx, evalTarget.SpaceID, gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, param *entity.ExecuteEvalTargetParam) (*entity.EvalTargetOutputData, entity.EvalTargetRunStatus, error) {
					assert.Equal(t, evalTarget.ID, param.TargetID)
					assert.Equal(t, evalTarget.EvalTargetVersion.ID, param.VersionID)
					assert.Equal(t, evalTarget.SourceTargetID, param.SourceTargetID)
					assert.Equal(t, evalTarget.EvalTargetType, param.TargetType)
					return outputData, entity.EvalTargetRunStatusSuccess, nil
				})
				return prepareResult{
					typedOps: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
						evalTarget.EvalTargetType: deps.operator,
					},
					expectedOutput: outputData,
				}
			},
			wantStatus: entity.EvalTargetRunStatusSuccess,
		},
		{
			name: "validate input failed",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, evalTarget *entity.EvalTarget, input *entity.EvalTargetInputData) prepareResult {
				deps.operator.EXPECT().ValidateInput(ctx, evalTarget.SpaceID, evalTarget.EvalTargetVersion.InputSchema, input).Return(errorx.NewByCode(errno.CommonInvalidParamCode))
				return prepareResult{
					typedOps: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
						evalTarget.EvalTargetType: deps.operator,
					},
				}
			},
			wantStatus:  entity.EvalTargetRunStatusFail,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "execute failed",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, evalTarget *entity.EvalTarget, input *entity.EvalTargetInputData) prepareResult {
				deps.operator.EXPECT().ValidateInput(ctx, evalTarget.SpaceID, evalTarget.EvalTargetVersion.InputSchema, input).Return(nil)
				deps.operator.EXPECT().Execute(ctx, evalTarget.SpaceID, gomock.Any()).Return(nil, entity.EvalTargetRunStatusFail, errorx.NewByCode(errno.CommonInternalErrorCode))
				return prepareResult{
					typedOps: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
						evalTarget.EvalTargetType: deps.operator,
					},
				}
			},
			wantStatus:  entity.EvalTargetRunStatusFail,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "execute returns nil output",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, evalTarget *entity.EvalTarget, input *entity.EvalTargetInputData) prepareResult {
				deps.operator.EXPECT().ValidateInput(ctx, evalTarget.SpaceID, evalTarget.EvalTargetVersion.InputSchema, input).Return(nil)
				deps.operator.EXPECT().Execute(ctx, evalTarget.SpaceID, gomock.Any()).Return(nil, entity.EvalTargetRunStatusSuccess, nil)
				return prepareResult{
					typedOps: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
						evalTarget.EvalTargetType: deps.operator,
					},
				}
			},
			wantStatus:  entity.EvalTargetRunStatusFail,
			wantErrCode: errno.CommonInternalErrorCode,
		},
	}

	for _, tc := range tests {
		tcase := tc
		t.Run(tcase.name, func(t *testing.T) {
			// avoid parallel here because ExecuteTarget contains time.Sleep for trajectory extraction

			ctx := context.Background()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			deps := &evalTargetServiceTestDeps{
				repo:     repomocks.NewMockIEvalTargetRepo(ctrl),
				idgen:    idgenmocks.NewMockIIDGenerator(ctrl),
				metric:   metricsmocks.NewMockEvalTargetMetrics(ctrl),
				operator: servicemocks.NewMockISourceEvalTargetOperateService(ctrl),
				configer: componentmocks.NewMockIConfiger(ctrl),
			}

			evalTarget := &entity.EvalTarget{
				ID:             200,
				SpaceID:        100,
				SourceTargetID: "src-id",
				EvalTargetType: entity.EvalTargetTypeLoopPrompt,
				EvalTargetVersion: &entity.EvalTargetVersion{
					ID:                  300,
					SourceTargetVersion: "v1",
					InputSchema: []*entity.ArgsSchema{
						{Key: gptr.Of("field")},
					},
				},
			}

			input := &entity.EvalTargetInputData{
				InputFields: map[string]*entity.Content{
					"field": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("hello"),
					},
				},
			}

			param := &entity.ExecuteTargetCtx{
				ExperimentRunID: gptr.Of(int64(555)),
				ItemID:          777,
				TurnID:          888,
			}

			deps.repo.EXPECT().GetEvalTargetVersion(ctx, evalTarget.SpaceID, evalTarget.EvalTargetVersion.ID).Return(evalTarget, nil)
			deps.metric.EXPECT().EmitRun(evalTarget.SpaceID, gomock.Any(), gomock.Any()).Times(1)
			// default trajectory conf, not used in these cases (target type does not support trajectory)
			deps.configer.EXPECT().GetTargetTrajectoryConf(gomock.Any()).AnyTimes().Return(&entity.TargetTrajectoryConf{})
			deps.idgen.EXPECT().GenID(ctx).Return(int64(9999), nil)

			var savedRecord *entity.EvalTargetRecord
			deps.repo.EXPECT().CreateEvalTargetRecord(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, rec *entity.EvalTargetRecord) (int64, error) {
				savedRecord = rec
				return rec.ID, nil
			})

			prepareRes := tcase.prepare(ctx, deps, evalTarget, input)

			svc := &EvalTargetServiceImpl{
				evalTargetRepo: deps.repo,
				idgen:          deps.idgen,
				metric:         deps.metric,
				typedOperators: prepareRes.typedOps,
				configer:       deps.configer,
			}

			record, err := svc.ExecuteTarget(ctx, evalTarget.SpaceID, evalTarget.ID, evalTarget.EvalTargetVersion.ID, param, input)
			require.NoError(t, err)
			require.NotNil(t, record)
			require.NotNil(t, savedRecord)
			assert.Equal(t, savedRecord, record)
			assert.Equal(t, int64(9999), record.ID)
			assert.Equal(t, tcase.wantStatus, gptr.Indirect(record.Status))

			if prepareRes.expectedOutput != nil {
				assert.Equal(t, prepareRes.expectedOutput, record.EvalTargetOutputData)
				assert.Nil(t, record.EvalTargetOutputData.EvalTargetRunError)
			} else {
				if assert.NotNil(t, record.EvalTargetOutputData) {
					if tcase.wantErrCode == 0 {
						assert.Nil(t, record.EvalTargetOutputData.EvalTargetRunError)
					} else {
						if assert.NotNil(t, record.EvalTargetOutputData.EvalTargetRunError) {
							assert.Equal(t, tcase.wantErrCode, record.EvalTargetOutputData.EvalTargetRunError.Code)
						}
					}
				}
			}
		})
	}
}

func TestEvalTargetServiceImpl_ExecuteTarget_TrajectoryExtraction(t *testing.T) {
	// do not run in parallel, this test involves time.Sleep

	ctx := context.Background()
	spaceID := int64(100)

	tests := []struct {
		name              string
		trajectories      []*entity.Trajectory
		err               error
		expectHasField    bool
		expectFieldNonNil bool
	}{
		{
			name: "trajectory extract error - no field added",
			err:  errors.New("extract fail"),
		},
		{
			name: "trajectory extracted successfully - field added",
			trajectories: []*entity.Trajectory{
				{
					ID: gptr.Of("traj-id"),
				},
			},
			expectHasField:    true,
			expectFieldNonNil: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			deps := &evalTargetServiceTestDeps{
				repo:     repomocks.NewMockIEvalTargetRepo(ctrl),
				idgen:    idgenmocks.NewMockIIDGenerator(ctrl),
				metric:   metricsmocks.NewMockEvalTargetMetrics(ctrl),
				operator: servicemocks.NewMockISourceEvalTargetOperateService(ctrl),
				configer: componentmocks.NewMockIConfiger(ctrl),
			}

			trajectoryAdapter := trajectorymocks.NewMockITrajectoryAdapter(ctrl)

			evalTarget := &entity.EvalTarget{
				ID:             200,
				SpaceID:        spaceID,
				SourceTargetID: "src-id",
				EvalTargetType: entity.EvalTargetTypeCustomRPCServer, // supports trajectory
				EvalTargetVersion: &entity.EvalTargetVersion{
					ID:                  300,
					SourceTargetVersion: "v1",
					InputSchema: []*entity.ArgsSchema{
						{Key: gptr.Of("field")},
					},
				},
			}

			input := &entity.EvalTargetInputData{
				InputFields: map[string]*entity.Content{
					"field": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("hello"),
					},
				},
			}

			param := &entity.ExecuteTargetCtx{
				ExperimentRunID: gptr.Of(int64(555)),
				ItemID:          777,
				TurnID:          888,
			}

			deps.repo.EXPECT().GetEvalTargetVersion(ctx, evalTarget.SpaceID, evalTarget.EvalTargetVersion.ID).Return(evalTarget, nil)
			deps.metric.EXPECT().EmitRun(evalTarget.SpaceID, gomock.Any(), gomock.Any()).Times(1)
			// configure trajectory interval to 1 second to avoid long sleeps
			deps.configer.EXPECT().GetTargetTrajectoryConf(gomock.Any()).AnyTimes().Return(&entity.TargetTrajectoryConf{
				SpaceExtractIntervalSecond: map[int64]int64{
					spaceID: 1,
				},
			})
			deps.idgen.EXPECT().GenID(ctx).Return(int64(9999), nil)

			trajectoryAdapter.EXPECT().
				ListTrajectory(gomock.Any(), spaceID, gomock.Any(), gomock.Nil()).
				Return(tt.trajectories, tt.err)

			outputData := &entity.EvalTargetOutputData{
				OutputFields: map[string]*entity.Content{},
				EvalTargetUsage: &entity.EvalTargetUsage{
					InputTokens:  1,
					OutputTokens: 2,
				},
			}
			deps.operator.EXPECT().ValidateInput(ctx, evalTarget.SpaceID, evalTarget.EvalTargetVersion.InputSchema, input).Return(nil)
			deps.operator.EXPECT().Execute(ctx, evalTarget.SpaceID, gomock.Any()).
				Return(outputData, entity.EvalTargetRunStatusSuccess, nil)

			var savedRecord *entity.EvalTargetRecord
			deps.repo.EXPECT().CreateEvalTargetRecord(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, rec *entity.EvalTargetRecord) (int64, error) {
				savedRecord = rec
				return rec.ID, nil
			})

			svc := &EvalTargetServiceImpl{
				evalTargetRepo:    deps.repo,
				idgen:             deps.idgen,
				metric:            deps.metric,
				typedOperators:    map[entity.EvalTargetType]ISourceEvalTargetOperateService{evalTarget.EvalTargetType: deps.operator},
				trajectoryAdapter: trajectoryAdapter,
				configer:          deps.configer,
			}

			record, err := svc.ExecuteTarget(ctx, evalTarget.SpaceID, evalTarget.ID, evalTarget.EvalTargetVersion.ID, param, input)
			require.NoError(t, err)
			require.NotNil(t, record)
			require.NotNil(t, savedRecord)

			// wait for the goroutine which extracts trajectory to complete
			time.Sleep(1100 * time.Millisecond)

			content, ok := record.EvalTargetOutputData.OutputFields[consts.EvalTargetOutputFieldKeyTrajectory]
			if !tt.expectHasField {
				assert.False(t, ok)
				return
			}
			assert.True(t, ok)
			if tt.expectFieldNonNil {
				require.NotNil(t, content)
				assert.Equal(t, entity.ContentTypeText, gptr.Indirect(content.ContentType))
				assert.Equal(t, entity.FieldDisplayFormat_JSON, gptr.Indirect(content.Format))
				assert.NotEmpty(t, gptr.Indirect(content.Text))
			}
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
				configer: componentmocks.NewMockIConfiger(ctrl),
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

func TestEvalTargetServiceImpl_ReportInvokeRecords_Trajectory(t *testing.T) {
	// do not run in parallel, this test involves time.Sleep

	ctx := context.Background()
	spaceID := int64(1)

	tests := []struct {
		name              string
		trajectories      []*entity.Trajectory
		err               error
		expectHasField    bool
		expectFieldNonNil bool
	}{
		{
			name: "extract trajectory error - no trajectory field",
			err:  errors.New("extract fail"),
		},
		{
			name: "extract trajectory success - trajectory field added",
			trajectories: []*entity.Trajectory{
				{
					ID: gptr.Of("traj-id"),
				},
			},
			expectHasField:    true,
			expectFieldNonNil: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			deps := &evalTargetServiceTestDeps{
				repo:     repomocks.NewMockIEvalTargetRepo(ctrl),
				idgen:    idgenmocks.NewMockIIDGenerator(ctrl),
				metric:   metricsmocks.NewMockEvalTargetMetrics(ctrl),
				operator: servicemocks.NewMockISourceEvalTargetOperateService(ctrl),
				configer: componentmocks.NewMockIConfiger(ctrl),
			}

			trajectoryAdapter := trajectorymocks.NewMockITrajectoryAdapter(ctrl)

			record := &entity.EvalTargetRecord{
				ID:                   10,
				SpaceID:              spaceID,
				TargetID:             20,
				TargetVersionID:      30,
				Status:               gptr.Of(entity.EvalTargetRunStatusAsyncInvoking),
				EvalTargetOutputData: &entity.EvalTargetOutputData{},
				TraceID:              "trace-id-1",
				LogID:                "log-id-1",
			}

			param := &entity.ReportTargetRecordParam{
				SpaceID:  spaceID,
				RecordID: record.ID,
				Status:   entity.EvalTargetRunStatusSuccess,
				OutputData: &entity.EvalTargetOutputData{
					OutputFields: map[string]*entity.Content{
						"key": {
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("value"),
						},
					},
					EvalTargetUsage:    &entity.EvalTargetUsage{InputTokens: 1, OutputTokens: 2},
					EvalTargetRunError: &entity.EvalTargetRunError{Code: 1, Message: "oops"},
					TimeConsumingMS:    gptr.Of(int64(10)),
				},
				Session: &entity.Session{UserID: "user"},
			}

			// main flow expectations (same as success case)
			deps.repo.EXPECT().GetEvalTargetRecordByIDAndSpaceID(ctx, param.SpaceID, param.RecordID).Return(record, nil)
			var saved *entity.EvalTargetRecord
			deps.repo.EXPECT().SaveEvalTargetRecord(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rec *entity.EvalTargetRecord) error {
				saved = rec
				return nil
			})
			deps.repo.EXPECT().GetEvalTargetVersion(gomock.Any(), record.SpaceID, record.TargetVersionID).
				Return(&entity.EvalTarget{EvalTargetType: entity.EvalTargetTypeCustomRPCServer}, nil)
			deps.repo.EXPECT().CreateEvalTargetRecord(gomock.Any(), gomock.Any()).AnyTimes()
			deps.metric.EXPECT().EmitRun(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			// trajectory extraction path
			deps.configer.EXPECT().GetTargetTrajectoryConf(gomock.Any()).AnyTimes().Return(&entity.TargetTrajectoryConf{
				SpaceExtractIntervalSecond: map[int64]int64{
					spaceID: 1,
				},
			})
			trajectoryAdapter.EXPECT().
				ListTrajectory(gomock.Any(), spaceID, gomock.Any(), gomock.Nil()).
				Return(tt.trajectories, tt.err)

			// use channel to safely observe async UpdateEvalTargetRecord calls from goroutine
			updatedCh := make(chan *entity.EvalTargetRecord, 1)
			deps.repo.EXPECT().UpdateEvalTargetRecord(gomock.Any(), gomock.Any()).
				AnyTimes().
				DoAndReturn(func(_ context.Context, rec *entity.EvalTargetRecord) error {
					// non-blocking send in case of multiple updates
					select {
					case updatedCh <- rec:
					default:
					}
					return nil
				})

			svc := &EvalTargetServiceImpl{
				evalTargetRepo:    deps.repo,
				idgen:             deps.idgen,
				metric:            deps.metric,
				typedOperators:    map[entity.EvalTargetType]ISourceEvalTargetOperateService{},
				trajectoryAdapter: trajectoryAdapter,
				configer:          deps.configer,
			}

			err := svc.ReportInvokeRecords(ctx, param)
			require.NoError(t, err)
			require.NotNil(t, saved)
			assert.Equal(t, param.OutputData, saved.EvalTargetOutputData)
			assert.Equal(t, param.Status, gptr.Indirect(saved.Status))

			// wait for trajectory goroutine to complete
			time.Sleep(1100 * time.Millisecond)
			var updated *entity.EvalTargetRecord
			select {
			case updated = <-updatedCh:
			default:
			}

			if !tt.expectHasField {
				assert.Nil(t, updated)
				return
			}

			require.NotNil(t, updated)
			content, ok := updated.EvalTargetOutputData.OutputFields[consts.EvalTargetOutputFieldKeyTrajectory]
			assert.True(t, ok)
			if tt.expectFieldNonNil {
				require.NotNil(t, content)
				assert.Equal(t, entity.ContentTypeText, gptr.Indirect(content.ContentType))
				assert.Equal(t, entity.FieldDisplayFormat_JSON, gptr.Indirect(content.Format))
				assert.NotEmpty(t, gptr.Indirect(content.Text))
			}
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

func TestEvalTargetServiceImpl_DebugTarget(t *testing.T) {
	t.Parallel()

	type prepareFunc func(ctx context.Context, deps *evalTargetServiceTestDeps, param *entity.DebugTargetParam)

	tests := []struct {
		name        string
		prepare     prepareFunc
		wantErr     bool
		wantErrCode int32
		wantStatus  entity.EvalTargetRunStatus
	}{
		{
			name: "unsupported target type",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, param *entity.DebugTargetParam) {
				// 不设置任何 operator，模拟不支持的类型
				deps.metric.EXPECT().EmitRun(param.SpaceID, gomock.Any(), gomock.Any()).Times(1)
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "validate input failed",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, param *entity.DebugTargetParam) {
				deps.operator.EXPECT().ValidateInput(ctx, param.SpaceID, param.PatchyTarget.EvalTargetVersion.InputSchema, param.InputData).Return(errorx.NewByCode(errno.CommonInvalidParamCode))
				deps.metric.EXPECT().EmitRun(param.SpaceID, gomock.Any(), gomock.Any()).Times(1)
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "execute failed with status error",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, param *entity.DebugTargetParam) {
				deps.operator.EXPECT().ValidateInput(ctx, param.SpaceID, param.PatchyTarget.EvalTargetVersion.InputSchema, param.InputData).Return(nil)
				deps.operator.EXPECT().Execute(ctx, param.SpaceID, gomock.Any()).Return(nil, entity.EvalTargetRunStatusFail, errorx.NewByCode(errno.CommonInternalErrorCode))
				deps.idgen.EXPECT().GenID(ctx).Return(int64(999), nil)
				deps.repo.EXPECT().CreateEvalTargetRecord(ctx, gomock.Any()).Return(int64(999), nil)
				deps.metric.EXPECT().EmitRun(param.SpaceID, gomock.Any(), gomock.Any()).Times(1)
			},
			wantStatus: entity.EvalTargetRunStatusFail,
		},
		{
			name: "execute failed with common error",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, param *entity.DebugTargetParam) {
				deps.operator.EXPECT().ValidateInput(ctx, param.SpaceID, param.PatchyTarget.EvalTargetVersion.InputSchema, param.InputData).Return(nil)
				deps.operator.EXPECT().Execute(ctx, param.SpaceID, gomock.Any()).Return(nil, entity.EvalTargetRunStatusFail, errorx.New("common error"))
				deps.idgen.EXPECT().GenID(ctx).Return(int64(999), nil)
				deps.repo.EXPECT().CreateEvalTargetRecord(ctx, gomock.Any()).Return(int64(999), nil)
				deps.metric.EXPECT().EmitRun(param.SpaceID, gomock.Any(), gomock.Any()).Times(1)
			},
			wantStatus: entity.EvalTargetRunStatusFail,
		},
		{
			name: "execute success",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, param *entity.DebugTargetParam) {
				outputData := &entity.EvalTargetOutputData{
					OutputFields: map[string]*entity.Content{
						"answer": {
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("test answer"),
						},
					},
					EvalTargetUsage: &entity.EvalTargetUsage{
						InputTokens:  10,
						OutputTokens: 20,
					},
				}
				deps.operator.EXPECT().ValidateInput(ctx, param.SpaceID, param.PatchyTarget.EvalTargetVersion.InputSchema, param.InputData).Return(nil)
				deps.operator.EXPECT().Execute(ctx, param.SpaceID, gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, execParam *entity.ExecuteEvalTargetParam) (*entity.EvalTargetOutputData, entity.EvalTargetRunStatus, error) {
					assert.Equal(t, param.InputData, execParam.Input)
					assert.Equal(t, param.PatchyTarget.EvalTargetType, execParam.TargetType)
					assert.Equal(t, param.PatchyTarget, execParam.EvalTarget)
					return outputData, entity.EvalTargetRunStatusSuccess, nil
				})
				deps.idgen.EXPECT().GenID(ctx).Return(int64(999), nil)
				deps.repo.EXPECT().CreateEvalTargetRecord(ctx, gomock.Any()).Return(int64(999), nil)
				deps.metric.EXPECT().EmitRun(param.SpaceID, gomock.Any(), gomock.Any()).Times(1)
			},
			wantStatus: entity.EvalTargetRunStatusSuccess,
		},
		{
			name: "idgen error",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, param *entity.DebugTargetParam) {
				deps.operator.EXPECT().ValidateInput(ctx, param.SpaceID, param.PatchyTarget.EvalTargetVersion.InputSchema, param.InputData).Return(nil)
				deps.operator.EXPECT().Execute(ctx, param.SpaceID, gomock.Any()).Return(&entity.EvalTargetOutputData{}, entity.EvalTargetRunStatusSuccess, nil)
				deps.idgen.EXPECT().GenID(ctx).Return(int64(0), errorx.NewByCode(errno.CommonInternalErrorCode))
				deps.metric.EXPECT().EmitRun(param.SpaceID, gomock.Any(), gomock.Any()).Times(1)
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "create record error",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, param *entity.DebugTargetParam) {
				deps.operator.EXPECT().ValidateInput(ctx, param.SpaceID, param.PatchyTarget.EvalTargetVersion.InputSchema, param.InputData).Return(nil)
				deps.operator.EXPECT().Execute(ctx, param.SpaceID, gomock.Any()).Return(&entity.EvalTargetOutputData{}, entity.EvalTargetRunStatusSuccess, nil)
				deps.idgen.EXPECT().GenID(ctx).Return(int64(999), nil)
				deps.repo.EXPECT().CreateEvalTargetRecord(ctx, gomock.Any()).Return(int64(0), errorx.NewByCode(errno.CommonInternalErrorCode))
				deps.metric.EXPECT().EmitRun(param.SpaceID, gomock.Any(), gomock.Any()).Times(1)
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
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
				configer: componentmocks.NewMockIConfiger(ctrl),
			}

			param := &entity.DebugTargetParam{
				SpaceID: 100,
				PatchyTarget: &entity.EvalTarget{
					ID:             200,
					SpaceID:        100,
					SourceTargetID: "test-source",
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						ID:                  300,
						SourceTargetVersion: "v1",
						InputSchema: []*entity.ArgsSchema{
							{
								Key:                 gptr.Of("input_field"),
								SupportContentTypes: []entity.ContentType{entity.ContentTypeText},
								JsonSchema:          gptr.Of(`{"type":"string"}`),
							},
						},
					},
				},
				InputData: &entity.EvalTargetInputData{
					InputFields: map[string]*entity.Content{
						"input_field": {
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("test input"),
						},
					},
				},
			}

			typedOps := map[entity.EvalTargetType]ISourceEvalTargetOperateService{}
			if tt.prepare != nil {
				tt.prepare(ctx, deps, param)
				// 只有在 prepare 函数中设置了 operator 时才添加到 typedOps
				if tt.name != "unsupported target type" {
					typedOps[entity.EvalTargetTypeLoopPrompt] = deps.operator
				}
			}

			svc := &EvalTargetServiceImpl{
				evalTargetRepo: deps.repo,
				idgen:          deps.idgen,
				metric:         deps.metric,
				typedOperators: typedOps,
			}

			record, err := svc.DebugTarget(ctx, param)

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
			require.NotNil(t, record)
			assert.Equal(t, int64(999), record.ID)
			assert.Equal(t, param.SpaceID, record.SpaceID)
			assert.Equal(t, param.InputData, record.EvalTargetInputData)
			assert.Equal(t, tt.wantStatus, gptr.Indirect(record.Status))

			// 验证 BaseInfo 字段
			require.NotNil(t, record.BaseInfo)
			require.NotNil(t, record.BaseInfo.CreatedBy)
			require.NotNil(t, record.BaseInfo.UpdatedBy)
			require.NotNil(t, record.BaseInfo.CreatedAt)
			require.NotNil(t, record.BaseInfo.UpdatedAt)
		})
	}
}

func TestEvalTargetServiceImpl_AsyncExecuteTarget(t *testing.T) {
	t.Parallel()

	type prepareFunc func(ctx context.Context, deps *evalTargetServiceTestDeps, spaceID int64, targetID int64, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData)

	tests := []struct {
		name         string
		prepare      prepareFunc
		wantErr      bool
		wantErrCode  int32
		expectCallee string
		expectID     int64
	}{
		{
			name: "nil input data",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, spaceID int64, targetID int64, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) {
				// 不设置任何 mock，因为会在参数验证阶段失败
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "nil param",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, spaceID int64, targetID int64, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) {
				// 不设置任何 mock，因为会在参数验证阶段失败
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "get eval target version failed",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, spaceID int64, targetID int64, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) {
				deps.repo.EXPECT().GetEvalTargetVersion(ctx, spaceID, targetVersionID).Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "unsupported target type",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, spaceID int64, targetID int64, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) {
				evalTarget := &entity.EvalTarget{
					ID:             targetID,
					SpaceID:        spaceID,
					SourceTargetID: "test-source",
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						ID:                  targetVersionID,
						SourceTargetVersion: "v1",
						InputSchema: []*entity.ArgsSchema{
							{Key: gptr.Of("input_field")},
						},
					},
				}
				deps.repo.EXPECT().GetEvalTargetVersion(ctx, spaceID, targetVersionID).Return(evalTarget, nil)
				deps.metric.EXPECT().EmitRun(spaceID, gomock.Any(), gomock.Any()).Times(1)
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "validate input failed",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, spaceID int64, targetID int64, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) {
				evalTarget := &entity.EvalTarget{
					ID:             targetID,
					SpaceID:        spaceID,
					SourceTargetID: "test-source",
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						ID:                  targetVersionID,
						SourceTargetVersion: "v1",
						InputSchema: []*entity.ArgsSchema{
							{Key: gptr.Of("input_field")},
						},
					},
				}
				deps.repo.EXPECT().GetEvalTargetVersion(ctx, spaceID, targetVersionID).Return(evalTarget, nil)
				deps.operator.EXPECT().ValidateInput(ctx, spaceID, evalTarget.EvalTargetVersion.InputSchema, inputData).Return(errorx.NewByCode(errno.CommonInvalidParamCode))
				deps.metric.EXPECT().EmitRun(spaceID, gomock.Any(), gomock.Any()).Times(1)
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name: "async execute failed",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, spaceID int64, targetID int64, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) {
				evalTarget := &entity.EvalTarget{
					ID:             targetID,
					SpaceID:        spaceID,
					SourceTargetID: "test-source",
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						ID:                  targetVersionID,
						SourceTargetVersion: "v1",
						InputSchema: []*entity.ArgsSchema{
							{Key: gptr.Of("input_field")},
						},
					},
				}
				deps.repo.EXPECT().GetEvalTargetVersion(ctx, spaceID, targetVersionID).Return(evalTarget, nil)
				deps.operator.EXPECT().ValidateInput(ctx, spaceID, evalTarget.EvalTargetVersion.InputSchema, inputData).Return(nil)
				deps.operator.EXPECT().AsyncExecute(ctx, spaceID, gomock.Any()).Return(int64(0), "callee", errorx.NewByCode(errno.CommonInternalErrorCode))
				deps.metric.EXPECT().EmitRun(spaceID, gomock.Any(), gomock.Any()).Times(1)
			},
			wantErr:      true,
			wantErrCode:  errno.CommonInternalErrorCode,
			expectCallee: "callee",
		},
		{
			name: "create record failed",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, spaceID int64, targetID int64, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) {
				evalTarget := &entity.EvalTarget{
					ID:             targetID,
					SpaceID:        spaceID,
					SourceTargetID: "test-source",
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						ID:                  targetVersionID,
						SourceTargetVersion: "v1",
						InputSchema: []*entity.ArgsSchema{
							{Key: gptr.Of("input_field")},
						},
					},
				}
				deps.repo.EXPECT().GetEvalTargetVersion(ctx, spaceID, targetVersionID).Return(evalTarget, nil)
				deps.operator.EXPECT().ValidateInput(ctx, spaceID, evalTarget.EvalTargetVersion.InputSchema, inputData).Return(nil)
				deps.operator.EXPECT().AsyncExecute(ctx, spaceID, gomock.Any()).Return(int64(999), "callee", nil)
				deps.repo.EXPECT().CreateEvalTargetRecord(ctx, gomock.Any()).Return(int64(0), errorx.NewByCode(errno.CommonInternalErrorCode))
				deps.metric.EXPECT().EmitRun(spaceID, gomock.Any(), gomock.Any()).Times(1)
			},
			wantErr:      true,
			wantErrCode:  errno.CommonInternalErrorCode,
			expectCallee: "callee",
		},
		{
			name: "success",
			prepare: func(ctx context.Context, deps *evalTargetServiceTestDeps, spaceID int64, targetID int64, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) {
				evalTarget := &entity.EvalTarget{
					ID:             targetID,
					SpaceID:        spaceID,
					SourceTargetID: "test-source",
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						ID:                  targetVersionID,
						SourceTargetVersion: "v1",
						InputSchema: []*entity.ArgsSchema{
							{Key: gptr.Of("input_field")},
						},
					},
				}
				deps.repo.EXPECT().GetEvalTargetVersion(ctx, spaceID, targetVersionID).Return(evalTarget, nil)
				deps.operator.EXPECT().ValidateInput(ctx, spaceID, evalTarget.EvalTargetVersion.InputSchema, inputData).Return(nil)
				deps.operator.EXPECT().AsyncExecute(ctx, spaceID, gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, execParam *entity.ExecuteEvalTargetParam) (int64, string, error) {
					assert.Equal(t, targetID, execParam.TargetID)
					assert.Equal(t, targetVersionID, execParam.VersionID)
					assert.Equal(t, "test-source", execParam.SourceTargetID)
					assert.Equal(t, "v1", execParam.SourceTargetVersion)
					assert.Equal(t, inputData, execParam.Input)
					assert.Equal(t, entity.EvalTargetTypeLoopPrompt, execParam.TargetType)
					assert.Equal(t, evalTarget, execParam.EvalTarget)
					return int64(999), "callee", nil
				})
				deps.repo.EXPECT().CreateEvalTargetRecord(ctx, gomock.Any()).Return(int64(999), nil)
				deps.metric.EXPECT().EmitRun(spaceID, gomock.Any(), gomock.Any()).Times(1)
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
				configer: componentmocks.NewMockIConfiger(ctrl),
			}

			spaceID := int64(100)
			targetID := int64(200)
			targetVersionID := int64(300)
			param := &entity.ExecuteTargetCtx{
				ExperimentRunID: gptr.Of(int64(555)),
				ItemID:          777,
				TurnID:          888,
			}
			inputData := &entity.EvalTargetInputData{
				InputFields: map[string]*entity.Content{
					"input_field": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test input"),
					},
				},
			}

			// 根据测试用例设置不同的参数
			var testParam *entity.ExecuteTargetCtx
			var testInputData *entity.EvalTargetInputData
			switch tt.name {
			case "nil input data":
				testParam = param
				testInputData = nil
			case "nil param":
				testParam = nil
				testInputData = inputData
			default:
				testParam = param
				testInputData = inputData
			}

			typedOps := map[entity.EvalTargetType]ISourceEvalTargetOperateService{}
			if tt.prepare != nil {
				tt.prepare(ctx, deps, spaceID, targetID, targetVersionID, testParam, testInputData)
				// 只有在 prepare 函数中设置了 operator 时才添加到 typedOps
				if tt.name != "unsupported target type" {
					typedOps[entity.EvalTargetTypeLoopPrompt] = deps.operator
				}
			}

			svc := &EvalTargetServiceImpl{
				evalTargetRepo: deps.repo,
				idgen:          deps.idgen,
				metric:         deps.metric,
				typedOperators: typedOps,
			}

			record, callee, err := svc.AsyncExecuteTarget(ctx, spaceID, targetID, targetVersionID, testParam, testInputData)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					require.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
				assert.Equal(t, tt.expectCallee, callee)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, record)
			assert.Equal(t, tt.expectCallee, callee)
			assert.Equal(t, tt.expectID, record.ID)
			assert.Equal(t, spaceID, record.SpaceID)
			assert.Equal(t, targetID, record.TargetID)
			assert.Equal(t, targetVersionID, record.TargetVersionID)
			assert.Equal(t, param.ItemID, record.ItemID)
			assert.Equal(t, param.TurnID, record.TurnID)
			assert.Equal(t, inputData, record.EvalTargetInputData)
			assert.Equal(t, entity.EvalTargetRunStatusAsyncInvoking, gptr.Indirect(record.Status))

			// 验证 BaseInfo 字段
			require.NotNil(t, record.BaseInfo)
			require.NotNil(t, record.BaseInfo.CreatedBy)
			require.NotNil(t, record.BaseInfo.UpdatedBy)
			require.NotNil(t, record.BaseInfo.CreatedAt)
			require.NotNil(t, record.BaseInfo.UpdatedAt)
		})
	}
}

// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	metrics_mocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repo_mocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func Test_NewEvalTargetServiceImpl(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建mock对象
	mockRepo := repo_mocks.NewMockIEvalTargetRepo(ctrl)
	mockMetric := metrics_mocks.NewMockEvalTargetMetrics(ctrl)
	mockIdgen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockOperator := mocks.NewMockISourceEvalTargetOperateService(ctrl)

	// 定义测试用例
	tests := []struct {
		mockSetup      func()
		name           string
		evalTargetRepo repo.IEvalTargetRepo
		idgen          idgen.IIDGenerator
		metric         metrics.EvalTargetMetrics
		typedOperators map[entity.EvalTargetType]ISourceEvalTargetOperateService
		wantInstance   *EvalTargetServiceImpl
	}{
		{
			name:           "正常场景 - 所有参数有效",
			evalTargetRepo: mockRepo,
			idgen:          mockIdgen,
			metric:         mockMetric,
			typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
				entity.EvalTargetType(1): mockOperator,
			},
			wantInstance: &EvalTargetServiceImpl{
				evalTargetRepo: mockRepo,
				idgen:          mockIdgen,
				metric:         mockMetric,
				typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
					entity.EvalTargetType(1): mockOperator,
				},
			},
		},
		{
			name:           "边界场景 - typedOperators为空map",
			evalTargetRepo: mockRepo,
			idgen:          mockIdgen,
			metric:         mockMetric,
			typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{},
			wantInstance: &EvalTargetServiceImpl{
				evalTargetRepo: mockRepo,
				idgen:          mockIdgen,
				metric:         mockMetric,
				typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{},
			},
		},
		{
			name:           "边界场景 - typedOperators为nil",
			evalTargetRepo: mockRepo,
			idgen:          mockIdgen,
			metric:         mockMetric,
			typedOperators: nil,
			wantInstance: &EvalTargetServiceImpl{
				evalTargetRepo: mockRepo,
				idgen:          mockIdgen,
				metric:         mockMetric,
				typedOperators: nil,
			},
		},
		{
			name:           "边界场景 - evalTargetRepo为nil",
			evalTargetRepo: nil,
			idgen:          mockIdgen,
			metric:         mockMetric,
			typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
				entity.EvalTargetType(1): mockOperator,
			},
			wantInstance: &EvalTargetServiceImpl{
				evalTargetRepo: nil,
				idgen:          mockIdgen,
				metric:         mockMetric,
				typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
					entity.EvalTargetType(1): mockOperator,
				},
			},
		},
		{
			name:           "边界场景 - idgen为nil",
			evalTargetRepo: mockRepo,
			idgen:          nil,
			metric:         mockMetric,
			typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
				entity.EvalTargetType(1): mockOperator,
			},
			wantInstance: &EvalTargetServiceImpl{
				evalTargetRepo: mockRepo,
				idgen:          nil,
				metric:         mockMetric,
				typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
					entity.EvalTargetType(1): mockOperator,
				},
			},
		},
		{
			name:           "边界场景 - metric为nil",
			evalTargetRepo: mockRepo,
			idgen:          mockIdgen,
			metric:         nil,
			typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
				entity.EvalTargetType(1): mockOperator,
			},
			wantInstance: &EvalTargetServiceImpl{
				evalTargetRepo: mockRepo,
				idgen:          mockIdgen,
				metric:         nil,
				typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
					entity.EvalTargetType(1): mockOperator,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup()
			}

			serviceInstance := NewEvalTargetServiceImpl(tt.evalTargetRepo, tt.idgen, tt.metric, tt.typedOperators)

			actualInstance, ok := serviceInstance.(*EvalTargetServiceImpl)
			assert.True(t, ok)
			assert.Equal(t, tt.wantInstance, actualInstance)
		})
	}
}

func TestEvalTargetServiceImpl_CreateEvalTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIEvalTargetRepo(ctrl)
	mockMetrics := metrics_mocks.NewMockEvalTargetMetrics(ctrl)
	mockOperator := mocks.NewMockISourceEvalTargetOperateService(ctrl)
	service := &EvalTargetServiceImpl{
		evalTargetRepo: mockRepo,
		metric:         mockMetrics,
		typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
			entity.EvalTargetTypeLoopPrompt: mockOperator,
		},
	}

	ctx := context.Background()
	spaceID := int64(123)
	sourceTargetID := "456"
	sourceTargetVersion := "v1.0"
	supportedType := entity.EvalTargetTypeLoopPrompt
	unsupportedType := entity.EvalTargetType(999)

	tests := []struct {
		name           string
		targetType     entity.EvalTargetType
		mockSetup      func()
		wantID         int64
		wantVersionID  int64
		wantErr        bool
		wantErrCode    int32
		wantErrContain string
	}{
		{
			name:       "unsupported target type",
			targetType: unsupportedType,
			mockSetup: func() {
				mockMetrics.EXPECT().EmitCreate(gomock.Any(), gomock.Any()).Return()
			},
			wantErr:        true,
			wantErrCode:    errno.CommonInvalidParamCode,
			wantErrContain: "target type not support",
		},
		{
			name:       "BuildBySource returns error",
			targetType: supportedType,
			mockSetup: func() {
				mockMetrics.EXPECT().EmitCreate(gomock.Any(), gomock.Any()).Return()
				mockOperator.EXPECT().
					BuildBySource(ctx, spaceID, sourceTargetID, sourceTargetVersion, gomock.Any()).
					Return(nil, errors.New("build error"))
			},
			wantErr:        true,
			wantErrContain: "build error",
		},
		{
			name:       "BuildBySource returns nil",
			targetType: supportedType,
			mockSetup: func() {
				mockOperator.EXPECT().
					BuildBySource(ctx, spaceID, sourceTargetID, sourceTargetVersion, gomock.Any()).
					Return(nil, nil)
				mockMetrics.EXPECT().
					EmitCreate(spaceID, gomock.Any())
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name:       "CreateEvalTarget success",
			targetType: supportedType,
			mockSetup: func() {
				evalTarget := &entity.EvalTarget{
					SpaceID: spaceID,
					EvalTargetVersion: &entity.EvalTargetVersion{
						SpaceID: spaceID,
					},
				}
				mockOperator.EXPECT().
					BuildBySource(ctx, spaceID, sourceTargetID, sourceTargetVersion, gomock.Any()).
					Return(evalTarget, nil)
				mockRepo.EXPECT().
					CreateEvalTarget(ctx, evalTarget).
					Return(int64(1), int64(2), nil)
				mockMetrics.EXPECT().
					EmitCreate(spaceID, nil)
			},
			wantID:        1,
			wantVersionID: 2,
			wantErr:       false,
		},
		{
			name:       "CreateEvalTarget repo error",
			targetType: supportedType,
			mockSetup: func() {
				evalTarget := &entity.EvalTarget{
					SpaceID: spaceID,
					EvalTargetVersion: &entity.EvalTargetVersion{
						SpaceID: spaceID,
					},
				}
				mockOperator.EXPECT().
					BuildBySource(ctx, spaceID, sourceTargetID, sourceTargetVersion, gomock.Any()).
					Return(evalTarget, nil)
				mockRepo.EXPECT().
					CreateEvalTarget(ctx, evalTarget).
					Return(int64(0), int64(0), errors.New("repo error"))
				mockMetrics.EXPECT().
					EmitCreate(spaceID, gomock.Any())
			},
			wantErr:        true,
			wantErrContain: "repo error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup()
			}

			id, versionID, err := service.CreateEvalTarget(ctx, spaceID, sourceTargetID, sourceTargetVersion, tt.targetType)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantID, id)
				assert.Equal(t, tt.wantVersionID, versionID)
			}
		})
	}
}

func TestEvalTargetServiceImpl_GetEvalTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIEvalTargetRepo(ctrl)
	service := &EvalTargetServiceImpl{
		evalTargetRepo: mockRepo,
	}

	ctx := context.Background()
	targetID := int64(1)
	expectedTarget := &entity.EvalTarget{ID: targetID}
	mockError := errors.New("repo error")

	tests := []struct {
		name      string
		targetID  int64
		mockSetup func()
		wantDo    *entity.EvalTarget
		wantErr   bool
		wantErrIs error
	}{
		{
			name:     "成功获取EvalTarget",
			targetID: targetID,
			mockSetup: func() {
				mockRepo.EXPECT().GetEvalTarget(ctx, targetID).Return(expectedTarget, nil)
			},
			wantDo:  expectedTarget,
			wantErr: false,
		},
		{
			name:     "仓库返回错误",
			targetID: targetID,
			mockSetup: func() {
				mockRepo.EXPECT().GetEvalTarget(ctx, targetID).Return(nil, mockError)
			},
			wantDo:    nil,
			wantErr:   true,
			wantErrIs: mockError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup()
			}

			do, err := service.GetEvalTarget(ctx, tt.targetID)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrIs != nil {
					assert.True(t, errors.Is(err, tt.wantErrIs) || err.Error() == tt.wantErrIs.Error(), "Expected error %v, got %v", tt.wantErrIs, err)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantDo, do)
		})
	}
}

func TestEvalTargetServiceImpl_GetEvalTargetVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIEvalTargetRepo(ctrl)
	mockOperator := mocks.NewMockISourceEvalTargetOperateService(ctrl)
	service := &EvalTargetServiceImpl{
		evalTargetRepo: mockRepo,
		typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
			entity.EvalTargetTypeLoopPrompt: mockOperator, // 假设有一个类型
		},
	}

	ctx := context.Background()
	spaceID := int64(123)
	versionID := int64(456)
	expectedTarget := &entity.EvalTarget{ID: versionID, EvalTargetType: entity.EvalTargetTypeLoopPrompt}
	repoError := errors.New("repo error")
	packError := errors.New("pack error")

	tests := []struct {
		name           string
		spaceID        int64
		versionID      int64
		needSourceInfo bool
		mockSetup      func()
		wantDo         *entity.EvalTarget
		wantErr        bool
		wantErrIs      error
	}{
		{
			name:           "仓库返回错误",
			spaceID:        spaceID,
			versionID:      versionID,
			needSourceInfo: false,
			mockSetup: func() {
				mockRepo.EXPECT().GetEvalTargetVersion(ctx, spaceID, versionID).Return(nil, repoError)
			},
			wantDo:    nil,
			wantErr:   true,
			wantErrIs: repoError,
		},
		{
			name:           "成功获取且不需要源信息",
			spaceID:        spaceID,
			versionID:      versionID,
			needSourceInfo: false,
			mockSetup: func() {
				mockRepo.EXPECT().GetEvalTargetVersion(ctx, spaceID, versionID).Return(expectedTarget, nil)
			},
			wantDo:  expectedTarget,
			wantErr: false,
		},
		{
			name:           "成功获取且需要源信息 - Pack成功",
			spaceID:        spaceID,
			versionID:      versionID,
			needSourceInfo: true,
			mockSetup: func() {
				mockRepo.EXPECT().GetEvalTargetVersion(ctx, spaceID, versionID).Return(expectedTarget, nil)
				// 假设只有一个 operator，并且类型匹配
				mockOperator.EXPECT().PackSourceVersionInfo(ctx, spaceID, []*entity.EvalTarget{expectedTarget}).Return(nil)
			},
			wantDo:  expectedTarget,
			wantErr: false,
		},
		{
			name:           "成功获取但PackSourceVersionInfo返回错误",
			spaceID:        spaceID,
			versionID:      versionID,
			needSourceInfo: true,
			mockSetup: func() {
				mockRepo.EXPECT().GetEvalTargetVersion(ctx, spaceID, versionID).Return(expectedTarget, nil)
				mockOperator.EXPECT().PackSourceVersionInfo(ctx, spaceID, []*entity.EvalTarget{expectedTarget}).Return(packError)
			},
			wantDo:    nil,
			wantErr:   true,
			wantErrIs: packError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 为特定测试用例设置不同的service实例
			currentService := service
			if tt.name == "成功获取但typedOperators为空（或不匹配）且需要源信息" {
				currentService = &EvalTargetServiceImpl{
					evalTargetRepo: mockRepo,
					typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{},
				}
			}

			if tt.mockSetup != nil {
				tt.mockSetup()
			}

			do, err := currentService.GetEvalTargetVersion(ctx, tt.spaceID, tt.versionID, tt.needSourceInfo)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrIs != nil {
					assert.True(t, errors.Is(err, tt.wantErrIs) || err.Error() == tt.wantErrIs.Error(), "Expected error %v, got %v", tt.wantErrIs, err)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantDo, do)
		})
	}
}

func TestEvalTargetServiceImpl_BatchGetEvalTargetBySource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIEvalTargetRepo(ctrl)
	service := &EvalTargetServiceImpl{
		evalTargetRepo: mockRepo,
	}

	ctx := context.Background()
	param := &entity.BatchGetEvalTargetBySourceParam{
		SpaceID:        1,
		SourceTargetID: []string{"s1", "s2"},
		TargetType:     entity.EvalTargetTypeLoopPrompt,
	}
	expectedDos := []*entity.EvalTarget{{ID: 1}, {ID: 2}}
	mockError := errors.New("repo error")

	tests := []struct {
		name      string
		param     *entity.BatchGetEvalTargetBySourceParam
		mockSetup func()
		wantDos   []*entity.EvalTarget
		wantErr   bool
		wantErrIs error
	}{
		{
			name:  "成功批量获取",
			param: param,
			mockSetup: func() {
				repoParam := &repo.BatchGetEvalTargetBySourceParam{
					SpaceID:        param.SpaceID,
					SourceTargetID: param.SourceTargetID,
					TargetType:     param.TargetType,
				}
				mockRepo.EXPECT().BatchGetEvalTargetBySource(ctx, repoParam).Return(expectedDos, nil)
			},
			wantDos: expectedDos,
			wantErr: false,
		},
		{
			name:  "仓库返回错误",
			param: param,
			mockSetup: func() {
				repoParam := &repo.BatchGetEvalTargetBySourceParam{
					SpaceID:        param.SpaceID,
					SourceTargetID: param.SourceTargetID,
					TargetType:     param.TargetType,
				}
				mockRepo.EXPECT().BatchGetEvalTargetBySource(ctx, repoParam).Return(nil, mockError)
			},
			wantDos:   nil,
			wantErr:   true,
			wantErrIs: mockError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup()
			}

			// 确保在param为nil的测试用例中，我们传递的是nil
			// 否则，传递tt.param
			var currentParam *entity.BatchGetEvalTargetBySourceParam
			if tt.name == "输入参数为nil（虽然函数不直接处理，但调用者可能传入）" {
				// 实际上，如果param为nil，函数会panic。
				// 这个测试用例的意图可能是测试当repo调用失败时的情况，即使输入是某种形式的“无效”
				// 但对于这个函数，它不检查param是否为nil。
				// 为了避免panic，我们应该传递一个有效的param给函数，然后mock repo的失败。
				// 或者，如果确实想测试nil param，那么应该期望panic。
				// 这里我们调整为测试repo失败，使用tt.param（假设它在测试用例中定义为有效）
				currentParam = &entity.BatchGetEvalTargetBySourceParam{ // 使用一个有效的param来触发repo调用
					SpaceID:        1,
					SourceTargetID: []string{"s1"},
					TargetType:     entity.EvalTargetTypeLoopPrompt,
				}
			} else {
				currentParam = tt.param
			}

			dos, err := service.BatchGetEvalTargetBySource(ctx, currentParam)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrIs != nil {
					assert.True(t, errors.Is(err, tt.wantErrIs) || err.Error() == tt.wantErrIs.Error(), "Expected error %v, got %v", tt.wantErrIs, err)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantDos, dos)
		})
	}
}

func TestEvalTargetServiceImpl_BatchGetEvalTargetVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIEvalTargetRepo(ctrl)
	mockOperator := mocks.NewMockISourceEvalTargetOperateService(ctrl)
	service := &EvalTargetServiceImpl{
		evalTargetRepo: mockRepo,
		typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
			entity.EvalTargetTypeLoopPrompt: mockOperator,
		},
	}

	ctx := context.Background()
	spaceID := int64(123)
	versionIDs := []int64{1, 2}
	expectedVersions := []*entity.EvalTarget{
		{ID: 1, EvalTargetType: entity.EvalTargetTypeLoopPrompt},
		{ID: 2, EvalTargetType: entity.EvalTargetTypeLoopPrompt},
	}
	repoError := errors.New("repo error")
	packError := errors.New("pack error")

	tests := []struct {
		name           string
		spaceID        int64
		versionIDs     []int64
		needSourceInfo bool
		mockSetup      func()
		wantVersions   []*entity.EvalTarget
		wantErr        bool
		wantErrIs      error
	}{
		{
			name:           "成功获取版本列表且不需要源信息",
			spaceID:        spaceID,
			versionIDs:     versionIDs,
			needSourceInfo: false,
			mockSetup: func() {
				mockRepo.EXPECT().
					BatchGetEvalTargetVersion(ctx, spaceID, versionIDs).
					Return(expectedVersions, nil)
			},
			wantVersions: expectedVersions,
			wantErr:      false,
		},
		{
			name:           "仓库返回错误",
			spaceID:        spaceID,
			versionIDs:     versionIDs,
			needSourceInfo: false,
			mockSetup: func() {
				mockRepo.EXPECT().
					BatchGetEvalTargetVersion(ctx, spaceID, versionIDs).
					Return(nil, repoError)
			},
			wantVersions: nil,
			wantErr:      true,
			wantErrIs:    repoError,
		},
		{
			name:           "成功获取且需要源信息 - Pack成功",
			spaceID:        spaceID,
			versionIDs:     versionIDs,
			needSourceInfo: true,
			mockSetup: func() {
				mockRepo.EXPECT().
					BatchGetEvalTargetVersion(ctx, spaceID, versionIDs).
					Return(expectedVersions, nil)
				mockOperator.EXPECT().
					PackSourceVersionInfo(ctx, spaceID, expectedVersions).
					Return(nil)
			},
			wantVersions: expectedVersions,
			wantErr:      false,
		},
		{
			name:           "成功获取但PackSourceVersionInfo返回错误",
			spaceID:        spaceID,
			versionIDs:     versionIDs,
			needSourceInfo: true,
			mockSetup: func() {
				mockRepo.EXPECT().
					BatchGetEvalTargetVersion(ctx, spaceID, versionIDs).
					Return(expectedVersions, nil)
				mockOperator.EXPECT().
					PackSourceVersionInfo(ctx, spaceID, expectedVersions).
					Return(packError)
			},
			wantVersions: nil,
			wantErr:      true,
			wantErrIs:    packError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup()
			}

			versions, err := service.BatchGetEvalTargetVersion(ctx, tt.spaceID, tt.versionIDs, tt.needSourceInfo)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrIs != nil {
					assert.True(t, errors.Is(err, tt.wantErrIs) || err.Error() == tt.wantErrIs.Error())
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantVersions, versions)
		})
	}
}

func TestEvalTargetServiceImpl_GetRecordByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIEvalTargetRepo(ctrl)
	service := &EvalTargetServiceImpl{
		evalTargetRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(123)
	recordID := int64(456)
	expectedRecord := &entity.EvalTargetRecord{ID: recordID}
	repoError := errors.New("repo error")

	tests := []struct {
		name       string
		spaceID    int64
		recordID   int64
		mockSetup  func()
		wantRecord *entity.EvalTargetRecord
		wantErr    bool
		wantErrIs  error
	}{
		{
			name:     "成功获取记录",
			spaceID:  spaceID,
			recordID: recordID,
			mockSetup: func() {
				mockRepo.EXPECT().
					GetEvalTargetRecordByIDAndSpaceID(ctx, spaceID, recordID).
					Return(expectedRecord, nil)
			},
			wantRecord: expectedRecord,
			wantErr:    false,
		},
		{
			name:     "仓库返回错误",
			spaceID:  spaceID,
			recordID: recordID,
			mockSetup: func() {
				mockRepo.EXPECT().
					GetEvalTargetRecordByIDAndSpaceID(ctx, spaceID, recordID).
					Return(nil, repoError)
			},
			wantRecord: nil,
			wantErr:    true,
			wantErrIs:  repoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup()
			}

			record, err := service.GetRecordByID(ctx, tt.spaceID, tt.recordID)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrIs != nil {
					assert.True(t, errors.Is(err, tt.wantErrIs) || err.Error() == tt.wantErrIs.Error())
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantRecord, record)
		})
	}
}

func TestEvalTargetServiceImpl_BatchGetRecordByIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIEvalTargetRepo(ctrl)
	service := &EvalTargetServiceImpl{
		evalTargetRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(123)
	recordIDs := []int64{1, 2}
	expectedRecords := []*entity.EvalTargetRecord{
		{ID: 1},
		{ID: 2},
	}
	repoError := errors.New("repo error")

	tests := []struct {
		name        string
		spaceID     int64
		recordIDs   []int64
		mockSetup   func()
		wantRecords []*entity.EvalTargetRecord
		wantErr     bool
		wantErrCode int32
		wantErrIs   error
	}{
		{
			name:        "参数校验失败 - spaceID为0",
			spaceID:     0,
			recordIDs:   recordIDs,
			mockSetup:   func() {},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name:        "参数校验失败 - recordIDs为空",
			spaceID:     spaceID,
			recordIDs:   []int64{},
			mockSetup:   func() {},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name:      "成功批量获取记录",
			spaceID:   spaceID,
			recordIDs: recordIDs,
			mockSetup: func() {
				mockRepo.EXPECT().
					ListEvalTargetRecordByIDsAndSpaceID(ctx, spaceID, recordIDs).
					Return(expectedRecords, nil)
			},
			wantRecords: expectedRecords,
			wantErr:     false,
		},
		{
			name:      "仓库返回错误",
			spaceID:   spaceID,
			recordIDs: recordIDs,
			mockSetup: func() {
				mockRepo.EXPECT().
					ListEvalTargetRecordByIDsAndSpaceID(ctx, spaceID, recordIDs).
					Return(nil, repoError)
			},
			wantRecords: nil,
			wantErr:     true,
			wantErrIs:   repoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup()
			}

			records, err := service.BatchGetRecordByIDs(ctx, tt.spaceID, tt.recordIDs)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
				if tt.wantErrIs != nil {
					assert.True(t, errors.Is(err, tt.wantErrIs) || err.Error() == tt.wantErrIs.Error())
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantRecords, records)
		})
	}
}

func TestEvalTargetServiceImpl_ExecuteTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIEvalTargetRepo(ctrl)
	mockOperator := mocks.NewMockISourceEvalTargetOperateService(ctrl)
	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockMetrics := metrics_mocks.NewMockEvalTargetMetrics(ctrl)

	service := &EvalTargetServiceImpl{
		evalTargetRepo: mockRepo,
		idgen:          mockIDGen,
		metric:         mockMetrics,
		typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
			entity.EvalTargetTypeLoopPrompt: mockOperator,
		},
	}

	ctx := context.Background()
	spaceID := int64(123)
	targetID := int64(456)
	targetVersionID := int64(789)
	recordID := int64(999)

	tests := []struct {
		name            string
		spaceID         int64
		targetID        int64
		targetVersionID int64
		param           *entity.ExecuteTargetCtx
		inputData       *entity.EvalTargetInputData
		mockSetup       func()
		wantErr         bool
		wantErrCode     int32
	}{
		{
			name:            "参数校验失败 - spaceID为0",
			spaceID:         0,
			targetID:        targetID,
			targetVersionID: targetVersionID,
			param:           &entity.ExecuteTargetCtx{},
			inputData:       &entity.EvalTargetInputData{},
			mockSetup: func() {
				mockMetrics.EXPECT().EmitRun(int64(0), gomock.Any(), gomock.Any())
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(recordID, nil)
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name:            "参数校验失败 - inputData为nil",
			spaceID:         spaceID,
			targetID:        targetID,
			targetVersionID: targetVersionID,
			param:           &entity.ExecuteTargetCtx{},
			inputData:       nil,
			mockSetup: func() {
				mockMetrics.EXPECT().EmitRun(spaceID, gomock.Any(), gomock.Any())
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name:            "参数校验失败 - param为nil",
			spaceID:         spaceID,
			targetID:        targetID,
			targetVersionID: targetVersionID,
			param:           nil,
			inputData:       &entity.EvalTargetInputData{},
			mockSetup: func() {
				mockMetrics.EXPECT().EmitRun(spaceID, gomock.Any(), gomock.Any())
			},
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name:            "执行成功",
			spaceID:         spaceID,
			targetID:        targetID,
			targetVersionID: targetVersionID,
			param:           &entity.ExecuteTargetCtx{},
			inputData:       &entity.EvalTargetInputData{},
			mockSetup: func() {
				mockMetrics.EXPECT().EmitRun(spaceID, gomock.Any(), gomock.Any())
				mockRepo.EXPECT().GetEvalTargetVersion(ctx, spaceID, targetVersionID).Return(
					&entity.EvalTarget{
						EvalTargetType: entity.EvalTargetTypeLoopPrompt,
						EvalTargetVersion: &entity.EvalTargetVersion{
							InputSchema: []*entity.ArgsSchema{},
						},
					}, nil)
				mockOperator.EXPECT().ValidateInput(gomock.Any(), spaceID, gomock.Any(), gomock.Any()).Return(nil)
				mockOperator.EXPECT().Execute(gomock.Any(), spaceID, gomock.Any()).Return(
					&entity.EvalTargetOutputData{
						OutputFields:    map[string]*entity.Content{},
						EvalTargetUsage: &entity.EvalTargetUsage{},
					},
					entity.EvalTargetRunStatusSuccess,
					nil,
				)
				mockRepo.EXPECT().CreateEvalTargetRecord(gomock.Any(), gomock.Any()).Return(recordID, nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup()
			}

			record, err := service.ExecuteTarget(ctx, tt.spaceID, tt.targetID, tt.targetVersionID, tt.param, tt.inputData)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, record)
				assert.Equal(t, recordID, record.ID)
			}
		})
	}
}

func TestEvalTargetServiceImpl_ValidateRuntimeParam(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOperator := mocks.NewMockISourceEvalTargetOperateService(ctrl)
	mockRuntimeParam := entity.NewPromptRuntimeParam(nil)

	service := &EvalTargetServiceImpl{
		typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
			entity.EvalTargetTypeLoopPrompt: mockOperator,
		},
	}

	ctx := context.Background()

	tests := []struct {
		name           string
		targetType     entity.EvalTargetType
		runtimeParam   string
		mockSetup      func()
		wantErr        bool
		wantErrContain string
	}{
		{
			name:         "valid_runtime_param_success",
			targetType:   entity.EvalTargetTypeLoopPrompt,
			runtimeParam: `{"model_config":{"model_id":"123","model_name":"test"}}`,
			mockSetup: func() {
				mockOperator.EXPECT().RuntimeParam().Return(mockRuntimeParam).Times(1)
			},
			wantErr: false,
		},
		{
			name:         "invalid_json_format",
			targetType:   entity.EvalTargetTypeLoopPrompt,
			runtimeParam: `{"model_config":invalid_json}`,
			mockSetup: func() {
				mockOperator.EXPECT().RuntimeParam().Return(mockRuntimeParam).Times(1)
			},
			wantErr:        true,
			wantErrContain: "PromptRuntimeParam json unmarshal fail",
		},
		{
			name:           "unsupported_target_type",
			targetType:     entity.EvalTargetType(999),
			runtimeParam:   `{"model_config":{"model_id":"123"}}`,
			mockSetup:      func() {},
			wantErr:        true,
			wantErrContain: "operator not found",
		},
		{
			name:         "empty_runtime_param",
			targetType:   entity.EvalTargetTypeLoopPrompt,
			runtimeParam: "",
			mockSetup: func() {
				// 空字符串直接返回 nil，不会调用 RuntimeParam()
			},
			wantErr: false, // 空字符串应该返回 nil，不是错误
		},
		{
			name:         "malformed_runtime_param_structure",
			targetType:   entity.EvalTargetTypeLoopPrompt,
			runtimeParam: `{"wrong_field":"value"}`,
			mockSetup: func() {
				mockOperator.EXPECT().RuntimeParam().Return(mockRuntimeParam).Times(1)
			},
			wantErr: false, // This should pass as the JSON is valid, just different structure
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup()
			}

			err := service.ValidateRuntimeParam(ctx, tt.targetType, tt.runtimeParam)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEvalTargetServiceImpl_sourceTargetOperator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOperator := mocks.NewMockISourceEvalTargetOperateService(ctrl)

	service := &EvalTargetServiceImpl{
		typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
			entity.EvalTargetTypeLoopPrompt: mockOperator,
		},
	}

	tests := []struct {
		name           string
		targetType     entity.EvalTargetType
		service        *EvalTargetServiceImpl
		wantOperator   ISourceEvalTargetOperateService
		wantErr        bool
		wantErrContain string
	}{
		{
			name:         "existing_operator_found",
			targetType:   entity.EvalTargetTypeLoopPrompt,
			service:      service,
			wantOperator: mockOperator,
			wantErr:      false,
		},
		{
			name:           "operator_not_found",
			targetType:     entity.EvalTargetType(999),
			service:        service,
			wantOperator:   nil,
			wantErr:        true,
			wantErrContain: "operator not found",
		},
		{
			name:       "operator_exists_but_is_nil",
			targetType: entity.EvalTargetTypeLoopPrompt,
			service: &EvalTargetServiceImpl{
				typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{
					entity.EvalTargetTypeLoopPrompt: nil,
				},
			},
			wantOperator:   nil,
			wantErr:        true,
			wantErrContain: "operator not found",
		},
		{
			name:       "typedOperators_is_nil",
			targetType: entity.EvalTargetTypeLoopPrompt,
			service: &EvalTargetServiceImpl{
				typedOperators: nil,
			},
			wantOperator:   nil,
			wantErr:        true,
			wantErrContain: "operator not found",
		},
		{
			name:       "typedOperators_is_empty",
			targetType: entity.EvalTargetTypeLoopPrompt,
			service: &EvalTargetServiceImpl{
				typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{},
			},
			wantOperator:   nil,
			wantErr:        true,
			wantErrContain: "operator not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operator, err := tt.service.sourceTargetOperator(tt.targetType)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
				assert.Nil(t, operator)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantOperator, operator)
			}
		})
	}
}

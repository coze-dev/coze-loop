// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	dbMocks "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func newTestExptAnnotateService(ctrl *gomock.Controller) *ExptAnnotateServiceImpl {
	return &ExptAnnotateServiceImpl{
		txDB:                     dbMocks.NewMockProvider(ctrl),
		repo:                     repoMocks.NewMockIExptAnnotateRepo(ctrl),
		exptRepo:                 repoMocks.NewMockIExperimentRepo(ctrl),
		exptTurnResultRepo:       repoMocks.NewMockIExptTurnResultRepo(ctrl),
		exptPublisher:            eventsMocks.NewMockExptEventPublisher(ctrl),
		evaluationSetItemService: svcMocks.NewMockEvaluationSetItemService(ctrl),
		exptResultService:        svcMocks.NewMockExptResultService(ctrl),
		exptAggrResultRepo:       repoMocks.NewMockIExptAggrResultRepo(ctrl),
		exptTurnResultFilterRepo: repoMocks.NewMockIExptTurnResultFilterRepo(ctrl),
	}
}

func TestExptAnnotateServiceImpl_CreateExptTurnResultTagRefs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := newTestExptAnnotateService(ctrl)
	ctx := context.Background()

	tests := []struct {
		name    string
		refs    []*entity.ExptTurnResultTagRef
		setup   func()
		wantErr bool
	}{
		{
			name: "成功创建标签引用",
			refs: []*entity.ExptTurnResultTagRef{
				{
					ExptID:   1,
					SpaceID:  1,
					TagKeyID: 1,
				},
			},
			setup: func() {
				expt := &entity.Experiment{
					ID:        1,
					EvalSetID: 1,
				}
				svc.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
					GetByID(ctx, int64(1), int64(1)).
					Return(expt, nil).Times(1)

				svc.evaluationSetItemService.(*svcMocks.MockEvaluationSetItemService).EXPECT().
					ListEvaluationSetItems(ctx, gomock.Any()).
					Return(nil, ptr.Of(int64((1))), nil, nil).Times(1)

				svc.repo.(*repoMocks.MockIExptAnnotateRepo).EXPECT().
					CreateExptTurnResultTagRefs(ctx, gomock.Any()).
					Return(nil).Times(1)

				svc.exptResultService.(*svcMocks.MockExptResultService).EXPECT().InsertExptTurnResultFilterKeyMappings(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name: "获取实验失败",
			refs: []*entity.ExptTurnResultTagRef{
				{
					ExptID:   1,
					SpaceID:  1,
					TagKeyID: 1,
				},
			},
			setup: func() {
				svc.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
					GetByID(ctx, int64(1), int64(1)).
					Return(nil, errors.New("db error")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := svc.CreateExptTurnResultTagRefs(ctx, tt.refs)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateExptTurnResultTagRefs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptAnnotateServiceImpl_GetExptTurnResultTagRefs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := newTestExptAnnotateService(ctrl)
	ctx := context.Background()

	tests := []struct {
		name    string
		exptID  int64
		spaceID int64
		setup   func()
		want    []*entity.ExptTurnResultTagRef
		wantErr bool
	}{
		{
			name:    "成功获取标签引用",
			exptID:  1,
			spaceID: 1,
			setup: func() {
				refs := []*entity.ExptTurnResultTagRef{
					{
						ID:          1,
						ExptID:      1,
						SpaceID:     1,
						TagKeyID:    1,
						TotalCnt:    10,
						CompleteCnt: 5,
					},
				}
				svc.repo.(*repoMocks.MockIExptAnnotateRepo).EXPECT().
					GetExptTurnResultTagRefs(ctx, int64(1), int64(1)).
					Return(refs, nil).Times(1)
			},
			want: []*entity.ExptTurnResultTagRef{
				{
					ID:          1,
					ExptID:      1,
					SpaceID:     1,
					TagKeyID:    1,
					TotalCnt:    10,
					CompleteCnt: 5,
				},
			},
			wantErr: false,
		},
		{
			name:    "获取标签引用失败",
			exptID:  1,
			spaceID: 1,
			setup: func() {
				svc.repo.(*repoMocks.MockIExptAnnotateRepo).EXPECT().
					GetExptTurnResultTagRefs(ctx, int64(1), int64(1)).
					Return(nil, errors.New("db error")).Times(1)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := svc.GetExptTurnResultTagRefs(ctx, tt.exptID, tt.spaceID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetExptTurnResultTagRefs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != len(tt.want) {
				t.Errorf("GetExptTurnResultTagRefs() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExptAnnotateServiceImpl_SaveAnnotateRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := newTestExptAnnotateService(ctrl)
	ctx := context.Background()

	tests := []struct {
		name    string
		exptID  int64
		itemID  int64
		turnID  int64
		record  *entity.AnnotateRecord
		setup   func()
		wantErr bool
	}{
		{
			name:   "成功保存标注记录",
			exptID: 1,
			itemID: 1,
			turnID: 1,
			record: &entity.AnnotateRecord{
				SpaceID:      1,
				TagKeyID:     1,
				ExperimentID: 1,
			},
			setup: func() {
				turnResult := &entity.ExptTurnResult{
					ID: 1,
				}
				svc.exptTurnResultRepo.(*repoMocks.MockIExptTurnResultRepo).EXPECT().
					Get(ctx, int64(1), int64(1), int64(1), int64(1)).
					Return(turnResult, nil).Times(1)

				// svc.txDB.(*dbMocks.MockProvider).EXPECT().
				//	Transaction(ctx, gomock.Any()).
				//	Return(nil).Times(1)
				svc.txDB.(*dbMocks.MockProvider).EXPECT().
					Transaction(ctx, gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db.Option) error {
						return fn(nil)
					}).Times(1)
				svc.repo.(*repoMocks.MockIExptAnnotateRepo).EXPECT().SaveAnnotateRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				svc.repo.(*repoMocks.MockIExptAnnotateRepo).EXPECT().UpdateCompleteCount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(int32(1), int32(1), nil).Times(1)

				tagRef := &entity.ExptTurnResultTagRef{
					TotalCnt:    10,
					CompleteCnt: 10,
				}
				svc.repo.(*repoMocks.MockIExptAnnotateRepo).EXPECT().
					GetTagRefByTagKeyID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(tagRef, nil).Times(1)
				svc.exptResultService.(*svcMocks.MockExptResultService).EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				svc.exptPublisher.(*eventsMocks.MockExptEventPublisher).EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				svc.exptPublisher.(*eventsMocks.MockExptEventPublisher).EXPECT().PublishExptAggrCalculateEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name:   "获取轮次结果失败",
			exptID: 1,
			itemID: 1,
			turnID: 1,
			record: &entity.AnnotateRecord{
				SpaceID:      1,
				TagKeyID:     1,
				ExperimentID: 1,
			},
			setup: func() {
				svc.exptTurnResultRepo.(*repoMocks.MockIExptTurnResultRepo).EXPECT().
					Get(ctx, int64(1), int64(1), int64(1), int64(1)).
					Return(nil, errors.New("db error")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := svc.SaveAnnotateRecord(ctx, tt.exptID, tt.itemID, tt.turnID, tt.record)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveAnnotateRecord() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptAnnotateServiceImpl_UpdateAnnotateRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := newTestExptAnnotateService(ctrl)
	ctx := context.Background()

	tests := []struct {
		name    string
		record  *entity.AnnotateRecord
		setup   func()
		wantErr bool
	}{
		{
			name: "成功更新标注记录",
			record: &entity.AnnotateRecord{
				ID:           1,
				SpaceID:      1,
				TagKeyID:     1,
				ExperimentID: 1,
			},
			setup: func() {
				tagRef := &entity.ExptTurnResultTagRef{
					TotalCnt:    10,
					CompleteCnt: 10,
				}
				svc.repo.(*repoMocks.MockIExptAnnotateRepo).EXPECT().
					GetTagRefByTagKeyID(ctx, int64(1), int64(1), int64(1)).
					Return(tagRef, nil).Times(1)

				svc.repo.(*repoMocks.MockIExptAnnotateRepo).EXPECT().
					UpdateAnnotateRecord(ctx, gomock.Any()).
					Return(nil).Times(1)

				svc.exptPublisher.(*eventsMocks.MockExptEventPublisher).EXPECT().
					PublishExptAggrCalculateEvent(ctx, gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
				svc.exptResultService.(*svcMocks.MockExptResultService).EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				svc.exptPublisher.(*eventsMocks.MockExptEventPublisher).EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name: "获取标签引用失败",
			record: &entity.AnnotateRecord{
				ID:           1,
				SpaceID:      1,
				TagKeyID:     1,
				ExperimentID: 1,
			},
			setup: func() {
				svc.repo.(*repoMocks.MockIExptAnnotateRepo).EXPECT().
					GetTagRefByTagKeyID(ctx, int64(1), int64(1), int64(1)).
					Return(nil, errors.New("db error")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := svc.UpdateAnnotateRecord(ctx, 1, 1, tt.record)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateAnnotateRecord() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptAnnotateServiceImpl_GetAnnotateRecordsByIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := newTestExptAnnotateService(ctrl)
	ctx := context.Background()

	tests := []struct {
		name      string
		spaceID   int64
		recordIDs []int64
		setup     func()
		want      []*entity.AnnotateRecord
		wantErr   bool
	}{
		{
			name:      "成功获取标注记录",
			spaceID:   1,
			recordIDs: []int64{1, 2},
			setup: func() {
				records := []*entity.AnnotateRecord{
					{
						ID:           1,
						SpaceID:      1,
						TagKeyID:     1,
						ExperimentID: 1,
					},
					{
						ID:           2,
						SpaceID:      1,
						TagKeyID:     2,
						ExperimentID: 1,
					},
				}
				svc.repo.(*repoMocks.MockIExptAnnotateRepo).EXPECT().
					GetAnnotateRecordsByIDs(ctx, int64(1), []int64{1, 2}).
					Return(records, nil).Times(1)
			},
			want: []*entity.AnnotateRecord{
				{
					ID:           1,
					SpaceID:      1,
					TagKeyID:     1,
					ExperimentID: 1,
				},
				{
					ID:           2,
					SpaceID:      1,
					TagKeyID:     2,
					ExperimentID: 1,
				},
			},
			wantErr: false,
		},
		{
			name:      "获取标注记录失败",
			spaceID:   1,
			recordIDs: []int64{1, 2},
			setup: func() {
				svc.repo.(*repoMocks.MockIExptAnnotateRepo).EXPECT().
					GetAnnotateRecordsByIDs(ctx, int64(1), []int64{1, 2}).
					Return(nil, errors.New("db error")).Times(1)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := svc.GetAnnotateRecordsByIDs(ctx, tt.spaceID, tt.recordIDs)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAnnotateRecordsByIDs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != len(tt.want) {
				t.Errorf("GetAnnotateRecordsByIDs() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExptAnnotateServiceImpl_DeleteExptTurnResultTagRef(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := newTestExptAnnotateService(ctrl)
	ctx := context.Background()

	tests := []struct {
		name     string
		exptID   int64
		spaceID  int64
		tagKeyID int64
		setup    func()
		wantErr  bool
	}{{
		name:     "正常删除标签引用",
		exptID:   1,
		spaceID:  1,
		tagKeyID: 1,
		setup: func() {
			// 预期事务调用成功
			svc.txDB.(*dbMocks.MockProvider).EXPECT().
				Transaction(ctx, gomock.Any()).
				Return(nil).Times(1)
		},
		wantErr: false,
	}, {
		name:     "事务执行失败",
		exptID:   1,
		spaceID:  1,
		tagKeyID: 1,
		setup: func() {
			// 模拟事务返回错误
			svc.txDB.(*dbMocks.MockProvider).EXPECT().
				Transaction(ctx, gomock.Any()).
				Return(errors.New("事务执行失败")).Times(1)
		},
		wantErr: true,
	}, {
		name:     "删除标签引用失败",
		exptID:   1,
		spaceID:  1,
		tagKeyID: 1,
		setup: func() {
			// 模拟标签引用删除失败
			svc.txDB.(*dbMocks.MockProvider).EXPECT().
				Transaction(ctx, gomock.Any()).
				DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db.Option) error {
					return fn(nil)
				}).Times(1)
			svc.repo.(*repoMocks.MockIExptAnnotateRepo).EXPECT().
				DeleteExptTurnResultTagRef(ctx, int64(1), int64(1), int64(1), gomock.Any()).
				Return(errors.New("删除标签引用失败")).Times(1)
		},
		wantErr: true,
	}, {
		name:     "删除聚合结果失败",
		exptID:   1,
		spaceID:  1,
		tagKeyID: 1,
		setup: func() {
			// 模拟聚合结果删除失败
			svc.txDB.(*dbMocks.MockProvider).EXPECT().
				Transaction(ctx, gomock.Any()).
				DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db.Option) error {
					return fn(nil)
				}).Times(1)
			svc.repo.(*repoMocks.MockIExptAnnotateRepo).EXPECT().
				DeleteExptTurnResultTagRef(ctx, int64(1), int64(1), int64(1), gomock.Any()).
				Return(nil).Times(1)
			svc.repo.(*repoMocks.MockIExptAnnotateRepo).EXPECT().
				DeleteTurnAnnotateRecordRef(ctx, int64(1), int64(1), int64(1), gomock.Any()).
				Return(nil).Times(1)
			svc.exptAggrResultRepo.(*repoMocks.MockIExptAggrResultRepo).EXPECT().
				DeleteExptAggrResult(ctx, gomock.Any()).
				Return(errors.New("删除聚合结果失败")).Times(1)
		},
		wantErr: true,
	}, {
		name:     "删除过滤映射失败",
		exptID:   1,
		spaceID:  1,
		tagKeyID: 1,
		setup: func() {
			// 模拟过滤映射删除失败
			svc.txDB.(*dbMocks.MockProvider).EXPECT().
				Transaction(ctx, gomock.Any()).
				DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db.Option) error {
					return fn(nil)
				}).Times(1)
			svc.repo.(*repoMocks.MockIExptAnnotateRepo).EXPECT().
				DeleteExptTurnResultTagRef(ctx, int64(1), int64(1), int64(1), gomock.Any()).
				Return(nil).Times(1)
			svc.repo.(*repoMocks.MockIExptAnnotateRepo).EXPECT().
				DeleteTurnAnnotateRecordRef(ctx, int64(1), int64(1), int64(1), gomock.Any()).
				Return(nil).Times(1)
			svc.exptAggrResultRepo.(*repoMocks.MockIExptAggrResultRepo).EXPECT().
				DeleteExptAggrResult(ctx, gomock.Any()).
				Return(nil).Times(1)
			svc.exptTurnResultFilterRepo.(*repoMocks.MockIExptTurnResultFilterRepo).EXPECT().
				DeleteExptTurnResultFilterKeyMapping(ctx, gomock.Any(), gomock.Any()).
				Return(errors.New("删除过滤映射失败")).Times(1)
		},
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := svc.DeleteExptTurnResultTagRef(ctx, tt.exptID, tt.spaceID, tt.tagKeyID)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteExptTurnResultTagRef() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptAnnotateServiceImpl_NewExptAnnotateService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建所有依赖的mock实例
	txDB := dbMocks.NewMockProvider(ctrl)
	repo := repoMocks.NewMockIExptAnnotateRepo(ctrl)
	exptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	exptPublisher := eventsMocks.NewMockExptEventPublisher(ctrl)
	evaluationSetItemService := svcMocks.NewMockEvaluationSetItemService(ctrl)
	exptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	exptResultService := svcMocks.NewMockExptResultService(ctrl)
	exptTurnResultFilterRepo := repoMocks.NewMockIExptTurnResultFilterRepo(ctrl)
	exptAggrResultRepo := repoMocks.NewMockIExptAggrResultRepo(ctrl)

	// 调用构造函数
	svc := NewExptAnnotateService(
		txDB,
		repo,
		exptTurnResultRepo,
		exptPublisher,
		evaluationSetItemService,
		exptRepo,
		exptResultService,
		exptTurnResultFilterRepo,
		exptAggrResultRepo,
	)

	// 验证服务实例不为nil且所有依赖已正确注入
	assert.NotNil(t, svc)
}

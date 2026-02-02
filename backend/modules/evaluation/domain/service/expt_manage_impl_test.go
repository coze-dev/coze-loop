// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	auditMocks "github.com/coze-dev/coze-loop/backend/infra/external/audit/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitMocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	idgenMocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	lockMocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	lwtMocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	idemMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/assert"
)

func newTestExptManager(ctrl *gomock.Controller) *ExptMangerImpl {
	return &ExptMangerImpl{
		exptResultService:           svcMocks.NewMockExptResultService(ctrl),
		exptAggrResultService:       svcMocks.NewMockExptAggrResultService(ctrl),
		exptRepo:                    repoMocks.NewMockIExperimentRepo(ctrl),
		runLogRepo:                  repoMocks.NewMockIExptRunLogRepo(ctrl),
		statsRepo:                   repoMocks.NewMockIExptStatsRepo(ctrl),
		itemResultRepo:              repoMocks.NewMockIExptItemResultRepo(ctrl),
		turnResultRepo:              repoMocks.NewMockIExptTurnResultRepo(ctrl),
		configer:                    componentMocks.NewMockIConfiger(ctrl),
		quotaRepo:                   repoMocks.NewMockQuotaRepo(ctrl),
		mutex:                       lockMocks.NewMockILocker(ctrl),
		idem:                        idemMocks.NewMockIdempotentService(ctrl),
		publisher:                   eventsMocks.NewMockExptEventPublisher(ctrl),
		audit:                       auditMocks.NewMockIAuditService(ctrl),
		mtr:                         metricsMocks.NewMockExptMetric(ctrl),
		idgenerator:                 idgenMocks.NewMockIIDGenerator(ctrl),
		lwt:                         lwtMocks.NewMockILatestWriteTracker(ctrl),
		evaluationSetVersionService: svcMocks.NewMockEvaluationSetVersionService(ctrl),
		evaluationSetService:        svcMocks.NewMockIEvaluationSetService(ctrl),
		evalTargetService:           svcMocks.NewMockIEvalTargetService(ctrl),
		evaluatorService:            svcMocks.NewMockEvaluatorService(ctrl),
		benefitService:              benefitMocks.NewMockIBenefitService(ctrl),
		templateRepo:                repoMocks.NewMockIExptTemplateRepo(ctrl),
		templateManager:             svcMocks.NewMockIExptTemplateManager(ctrl),
	}
}

func TestExptMangerImpl_MGetDetail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}
	exptID := int64(123)
	expt := &entity.Experiment{ID: exptID}

	mgr.lwt.(*lwtMocks.MockILatestWriteTracker).
		EXPECT().
		CheckWriteFlagByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(false).AnyTimes()
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().MGetByID(ctx, []int64{exptID}, int64(1)).Return([]*entity.Experiment{expt}, nil).AnyTimes()
	mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().MGetStats(ctx, []int64{exptID}, int64(1), session).Return([]*entity.ExptStats{{ExptID: exptID}}, nil).AnyTimes()
	mgr.exptAggrResultService.(*svcMocks.MockExptAggrResultService).EXPECT().BatchGetExptAggrResultByExperimentIDs(ctx, int64(1), []int64{exptID}).Return([]*entity.ExptAggregateResult{}, nil).AnyTimes()
	mgr.evaluationSetService.(*svcMocks.MockIEvaluationSetService).EXPECT().BatchGetEvaluationSets(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSet{{}}, nil).AnyTimes()
	mgr.evalTargetService.(*svcMocks.MockIEvalTargetService).EXPECT().BatchGetEvalTargetVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvalTarget{{}}, nil).AnyTimes()
	mgr.evaluatorService.(*svcMocks.MockEvaluatorService).EXPECT().BatchGetEvaluatorVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.Evaluator{}, nil).AnyTimes()

	tests := []struct {
		name    string
		exptIDs []int64
		spaceID int64
		session *entity.Session
		wantErr bool
	}{
		{"normal", []int64{exptID}, 1, session, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mgr.MGetDetail(ctx, tt.exptIDs, tt.spaceID, tt.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("MGetDetail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_CheckName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}

	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().GetByName(ctx, "foo", int64(1)).Return(nil, false, nil).AnyTimes()
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().GetByName(ctx, "bar", int64(1)).Return(nil, true, nil).AnyTimes()
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().GetByName(ctx, "err", int64(1)).Return(nil, false, errors.New("db error")).AnyTimes()

	tests := []struct {
		name    string
		input   string
		want    bool
		wantErr bool
	}{
		{"not exist", "foo", true, false},
		{"exist", "bar", false, false},
		{"db error", "err", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mgr.CheckName(ctx, tt.input, 1, session)
			if got != tt.want || (err != nil) != tt.wantErr {
				t.Errorf("CheckName() = %v, err = %v, want %v, wantErr %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_CreateExpt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}
	param := &entity.CreateExptParam{
		WorkspaceID:      1,
		Name:             "expt",
		EvalSetID:        2,
		EvalSetVersionID: 3,
		CreateEvalTargetParam: &entity.CreateEvalTargetParam{
			EvalTargetType:      gptr.Of(entity.EvalTargetTypeLoopPrompt),
			SourceTargetID:      gptr.Of("100"),
			SourceTargetVersion: gptr.Of("v1"),
		},
		EvaluatorVersionIds: []int64{10},
	}

	mgr.evalTargetService.(*svcMocks.MockIEvalTargetService).
		EXPECT().
		CreateEvalTarget(ctx, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(int64(100), int64(101), nil).AnyTimes()
	mgr.evalTargetService.(*svcMocks.MockIEvalTargetService).
		EXPECT().
		GetEvalTargetVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.EvalTarget{
			ID:             100,
			EvalTargetType: entity.EvalTargetTypeLoopPrompt,
			EvalTargetVersion: &entity.EvalTargetVersion{
				OutputSchema: []*entity.ArgsSchema{},
			},
		}, nil).AnyTimes()
	mgr.evaluationSetVersionService.(*svcMocks.MockEvaluationSetVersionService).
		EXPECT().
		GetEvaluationSetVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, &entity.EvaluationSet{
			EvaluationSetVersion: &entity.EvaluationSetVersion{
				EvaluationSetSchema: &entity.EvaluationSetSchema{
					FieldSchemas: []*entity.FieldSchema{},
				},
			},
		}, nil).AnyTimes()
	mgr.evaluatorService.(*svcMocks.MockEvaluatorService).
		EXPECT().
		BatchGetEvaluatorVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.Evaluator{{
			ID:                     10,
			EvaluatorType:          entity.EvaluatorTypePrompt,
			PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{EvaluatorID: 10},
		}}, nil).AnyTimes()
	mgr.idgenerator.(*idgenMocks.MockIIDGenerator).EXPECT().GenMultiIDs(ctx, 2).Return([]int64{1, 2}, nil).AnyTimes()
	mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().CreateStats(ctx, gomock.Any(), session).Return(nil).AnyTimes()
	// 模拟 InsertExptTurnResultFilterKeyMappings 方法
	mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().InsertExptTurnResultFilterKeyMappings(ctx, gomock.Any()).Return(nil).AnyTimes()
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().Create(ctx, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mgr.lwt.(*lwtMocks.MockILatestWriteTracker).EXPECT().SetWriteFlag(ctx, gomock.Any(), gomock.Any()).Return().AnyTimes()
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().GetByName(ctx, gomock.Any(), gomock.Any()).Return(nil, true, nil).AnyTimes()
	mgr.audit.(*auditMocks.MockIAuditService).
		EXPECT().
		Audit(gomock.Any(), gomock.Any()).
		Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil).AnyTimes()

	// Mock CheckRun dependencies
	mgr.benefitService.(*benefitMocks.MockIBenefitService).
		EXPECT().
		CheckAndDeductEvalBenefit(ctx, gomock.Any()).
		Return(&benefit.CheckAndDeductEvalBenefitResult{
			IsFreeEvaluate: gptr.Of(true),
		}, nil).AnyTimes()
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).
		EXPECT().
		Update(ctx, gomock.Any()).
		Return(nil).AnyTimes()

	t.Run("normal", func(t *testing.T) {
		_, err := mgr.CreateExpt(ctx, param, session)
		if err == nil {
			t.Logf("CreateExpt() 依赖mock通过，未覆盖getExptTupleByID/CheckRun逻辑")
		}
	})

	t.Run("设置ExptTemplateMeta", func(t *testing.T) {
		paramWithTemplate := &entity.CreateExptParam{
			WorkspaceID:         1,
			Name:                "expt_with_template",
			EvalSetID:           2,
			EvalSetVersionID:    3,
			ExptTemplateID:      100,
			EvaluatorVersionIds: []int64{10},
		}
		expt, err := mgr.CreateExpt(ctx, paramWithTemplate, session)
		if err == nil && expt != nil {
			if expt.ExptTemplateMeta == nil {
				t.Errorf("CreateExpt() ExptTemplateMeta should be set when ExptTemplateID > 0")
			} else if expt.ExptTemplateMeta.ID != 100 {
				t.Errorf("CreateExpt() ExptTemplateMeta.ID = %v, want 100", expt.ExptTemplateMeta.ID)
			}
		}
	})

	t.Run("根据ScoreWeight设置EnableScoreWeight", func(t *testing.T) {
		scoreWeight := 0.5
		paramWithScoreWeight := &entity.CreateExptParam{
			WorkspaceID:         1,
			Name:                "expt_with_score_weight",
			EvalSetID:           2,
			EvalSetVersionID:    3,
			EvaluatorVersionIds: []int64{10},
			ExptConf: &entity.EvaluationConfiguration{
				ConnectorConf: entity.Connector{
					EvaluatorsConf: &entity.EvaluatorsConf{
						EvaluatorConf: []*entity.EvaluatorConf{
							{
								EvaluatorVersionID: 10,
								ScoreWeight:        &scoreWeight,
							},
						},
					},
				},
			},
		}
		expt, err := mgr.CreateExpt(ctx, paramWithScoreWeight, session)
		if err == nil && expt != nil && expt.EvalConf != nil &&
			expt.EvalConf.ConnectorConf.EvaluatorsConf != nil {
			if !expt.EvalConf.ConnectorConf.EvaluatorsConf.EnableScoreWeight {
				t.Errorf("CreateExpt() EnableScoreWeight should be true when ScoreWeight > 0")
			}
		}
	})

	t.Run("ScoreWeight为0或nil时不启用EnableScoreWeight", func(t *testing.T) {
		zeroWeight := 0.0
		paramWithoutScoreWeight := &entity.CreateExptParam{
			WorkspaceID:         1,
			Name:                "expt_without_score_weight",
			EvalSetID:           2,
			EvalSetVersionID:    3,
			EvaluatorVersionIds: []int64{10},
			ExptConf: &entity.EvaluationConfiguration{
				ConnectorConf: entity.Connector{
					EvaluatorsConf: &entity.EvaluatorsConf{
						EvaluatorConf: []*entity.EvaluatorConf{
							{
								EvaluatorVersionID: 10,
								ScoreWeight:        &zeroWeight,
							},
						},
					},
				},
			},
		}
		expt, err := mgr.CreateExpt(ctx, paramWithoutScoreWeight, session)
		if err == nil && expt != nil && expt.EvalConf != nil &&
			expt.EvalConf.ConnectorConf.EvaluatorsConf != nil {
			if expt.EvalConf.ConnectorConf.EvaluatorsConf.EnableScoreWeight {
				t.Errorf("CreateExpt() EnableScoreWeight should be false when ScoreWeight <= 0")
			}
		}
	})
}

func TestExptMangerImpl_Update(t *testing.T) {
	tests := []struct {
		name    string
		expt    *entity.Experiment
		session *entity.Session
		setup   func(mockAudit *auditMocks.MockIAuditService, mockExptRepo *repoMocks.MockIExperimentRepo)
		wantErr bool
	}{
		{
			name: "audit rejected",
			expt: &entity.Experiment{
				ID:          1,
				Name:        "test",
				Description: "test",
			},
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockAudit *auditMocks.MockIAuditService, mockExptRepo *repoMocks.MockIExperimentRepo) {
				mockAudit.EXPECT().
					Audit(
						gomock.Any(),
						gomock.Any(),
					).
					Return(audit.AuditRecord{
						AuditStatus: audit.AuditStatus_Rejected,
					}, nil).
					Times(1)
			},
			wantErr: true,
		},
		{
			name: "audit passed",
			expt: &entity.Experiment{
				ID:          1,
				Name:        "test",
				Description: "test",
			},
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockAudit *auditMocks.MockIAuditService, mockExptRepo *repoMocks.MockIExperimentRepo) {
				mockAudit.EXPECT().
					Audit(
						gomock.Any(),
						gomock.Any(),
					).
					Return(audit.AuditRecord{
						AuditStatus: audit.AuditStatus_Approved,
					}, nil).
					Times(1)

				mockExptRepo.EXPECT().
					Update(
						gomock.Any(),
						gomock.Any(),
					).
					Return(nil).
					Times(1)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockAudit := auditMocks.NewMockIAuditService(ctrl)
			mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)

			mgr := &ExptMangerImpl{
				audit:    mockAudit,
				exptRepo: mockExptRepo,
			}

			tt.setup(mockAudit, mockExptRepo)

			err := mgr.Update(context.Background(), tt.expt, tt.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("Update() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptMangerImpl_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}
	spaceID := int64(1)
	exptID := int64(1)

	t.Run("正常删除", func(t *testing.T) {
		mockExpt := &entity.Experiment{
			ID:      exptID,
			SpaceID: spaceID,
		}
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().GetByID(ctx, exptID, spaceID).Return(mockExpt, nil)
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().Delete(ctx, exptID, spaceID).Return(nil)

		err := mgr.Delete(ctx, exptID, spaceID, session)
		if err != nil {
			t.Errorf("Delete() error = %v", err)
		}
	})

	t.Run("删除时关联模板，更新模板ExptInfo", func(t *testing.T) {
		templateID := int64(100)
		mockExpt := &entity.Experiment{
			ID:               exptID,
			SpaceID:          spaceID,
			Status:           entity.ExptStatus_Success,
			ExptTemplateMeta: &entity.ExptTemplateMeta{ID: templateID},
		}
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().GetByID(ctx, exptID, spaceID).Return(mockExpt, nil)
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().Delete(ctx, exptID, spaceID).Return(nil)
		mgr.templateManager.(*svcMocks.MockIExptTemplateManager).EXPECT().
			UpdateExptInfo(ctx, templateID, spaceID, exptID, entity.ExptStatus_Success, int64(-1)).Return(nil)

		err := mgr.Delete(ctx, exptID, spaceID, session)
		if err != nil {
			t.Errorf("Delete() error = %v", err)
		}
	})

	t.Run("UpdateExptInfo失败不影响主流程", func(t *testing.T) {
		templateID := int64(100)
		mockExpt := &entity.Experiment{
			ID:               exptID,
			SpaceID:          spaceID,
			Status:           entity.ExptStatus_Failed,
			ExptTemplateMeta: &entity.ExptTemplateMeta{ID: templateID},
		}
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().GetByID(ctx, exptID, spaceID).Return(mockExpt, nil)
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().Delete(ctx, exptID, spaceID).Return(nil)
		mgr.templateManager.(*svcMocks.MockIExptTemplateManager).EXPECT().
			UpdateExptInfo(ctx, templateID, spaceID, exptID, entity.ExptStatus_Failed, int64(-1)).
			Return(errors.New("update error"))

		// UpdateExptInfo失败不应该影响主流程，应该返回nil
		err := mgr.Delete(ctx, exptID, spaceID, session)
		if err != nil {
			t.Errorf("Delete() should not return error when UpdateExptInfo fails, got %v", err)
		}
	})

	t.Run("实验没有关联模板，跳过UpdateExptInfo", func(t *testing.T) {
		mockExpt := &entity.Experiment{
			ID:               exptID,
			SpaceID:          spaceID,
			ExptTemplateMeta: nil,
		}
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().GetByID(ctx, exptID, spaceID).Return(mockExpt, nil)
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().Delete(ctx, exptID, spaceID).Return(nil)

		err := mgr.Delete(ctx, exptID, spaceID, session)
		if err != nil {
			t.Errorf("Delete() error = %v", err)
		}
	})

	t.Run("templateManager为nil，跳过UpdateExptInfo", func(t *testing.T) {
		mgrNoTemplateManager := newTestExptManager(ctrl)
		mgrNoTemplateManager.templateManager = nil
		templateID := int64(100)
		mockExpt := &entity.Experiment{
			ID:               exptID,
			SpaceID:          spaceID,
			Status:           entity.ExptStatus_Success,
			ExptTemplateMeta: &entity.ExptTemplateMeta{ID: templateID},
		}
		mgrNoTemplateManager.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().GetByID(ctx, exptID, spaceID).Return(mockExpt, nil)
		mgrNoTemplateManager.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().Delete(ctx, exptID, spaceID).Return(nil)

		err := mgrNoTemplateManager.Delete(ctx, exptID, spaceID, session)
		if err != nil {
			t.Errorf("Delete() error = %v", err)
		}
	})
}

func TestExptMangerImpl_Clone(t *testing.T) {
	tests := []struct {
		name    string
		exptID  int64
		spaceID int64
		session *entity.Session
		setup   func(mockExptRepo *repoMocks.MockIExperimentRepo, mockIDGen *idgenMocks.MockIIDGenerator, mockLWT *lwtMocks.MockILatestWriteTracker)
		want    *entity.Experiment
		wantErr bool
	}{
		{
			name:    "normal",
			exptID:  1,
			spaceID: 100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptRepo *repoMocks.MockIExperimentRepo, mockIDGen *idgenMocks.MockIIDGenerator, mockLWT *lwtMocks.MockILatestWriteTracker) {
				// 设置 GetByID 的 mock
				mockExptRepo.EXPECT().
					GetByID(gomock.Any(), int64(1), int64(100)).
					Return(&entity.Experiment{
						ID:          1,
						SpaceID:     100,
						Name:        "test",
						Description: "test",
					}, nil).
					Times(1)

				// 设置 GenID 的 mock
				mockIDGen.EXPECT().
					GenID(gomock.Any()).
					Return(int64(2), nil).
					Times(1)

				// 设置 GetByName 的 mock
				mockExptRepo.EXPECT().
					GetByName(gomock.Any(), "test", int64(100)).
					Return(nil, false, nil).
					Times(1)

				// 设置 Create 的 mock
				mockExptRepo.EXPECT().
					Create(
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
					).
					Return(nil).
					Times(1)

				// 设置 SetWriteFlag 的 mock - 不需要 Return
				mockLWT.EXPECT().
					SetWriteFlag(
						gomock.Any(),
						platestwrite.ResourceTypeExperiment,
						int64(2),
					).
					Times(1)
			},
			want: &entity.Experiment{
				ID:          2,
				SpaceID:     100,
				Name:        "test",
				Description: "test",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
			mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)
			mockLWT := lwtMocks.NewMockILatestWriteTracker(ctrl)

			mgr := &ExptMangerImpl{
				exptRepo:    mockExptRepo,
				idgenerator: mockIDGen,
				lwt:         mockLWT,
			}

			tt.setup(mockExptRepo, mockIDGen, mockLWT)

			got, err := mgr.Clone(context.Background(), tt.exptID, tt.spaceID, tt.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("Clone() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.ID != tt.want.ID {
					t.Errorf("Clone() got ID = %v, want %v", got.ID, tt.want.ID)
				}
				if got.SpaceID != tt.want.SpaceID {
					t.Errorf("Clone() got SpaceID = %v, want %v", got.SpaceID, tt.want.SpaceID)
				}
				if got.Name != tt.want.Name {
					t.Errorf("Clone() got Name = %v, want %v", got.Name, tt.want.Name)
				}
				if got.Description != tt.want.Description {
					t.Errorf("Clone() got Description = %v, want %v", got.Description, tt.want.Description)
				}
			}
		})
	}
}

func TestExptMangerImpl_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockLWT := lwtMocks.NewMockILatestWriteTracker(ctrl)

	mgr := &ExptMangerImpl{
		exptRepo: mockExptRepo,
		lwt:      mockLWT,
	}

	ctx := context.Background()
	session := &entity.Session{UserID: "test"}
	exptID := int64(123)
	spaceID := int64(1)
	expt := &entity.Experiment{ID: exptID}

	tests := []struct {
		name      string
		setup     func()
		want      *entity.Experiment
		wantErr   bool
		errorCode int
	}{
		{
			name: "正常获取",
			setup: func() {
				mockLWT.EXPECT().
					CheckWriteFlagByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(false).AnyTimes()
				mockExptRepo.EXPECT().
					MGetByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*entity.Experiment{expt}, nil).Times(1)
			},
			want:    expt,
			wantErr: false,
		},
		{
			name: "repo返回错误",
			setup: func() {
				mockExptRepo.EXPECT().
					MGetByID(ctx, []int64{exptID}, spaceID).
					Return(nil, fmt.Errorf("db error")).Times(1)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "返回空列表",
			setup: func() {
				mockExptRepo.EXPECT().
					MGetByID(ctx, []int64{exptID}, spaceID).
					Return([]*entity.Experiment{}, nil).Times(1)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "返回nil列表",
			setup: func() {
				mockExptRepo.EXPECT().
					MGetByID(ctx, []int64{exptID}, spaceID).
					Return(nil, nil).Times(1)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "返回列表第一个为nil",
			setup: func() {
				mockExptRepo.EXPECT().
					MGetByID(ctx, []int64{exptID}, spaceID).
					Return([]*entity.Experiment{nil}, nil).Times(1)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := mgr.Get(ctx, exptID, spaceID, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Get() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExptMangerImpl_List(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockLWT := lwtMocks.NewMockILatestWriteTracker(ctrl)
	mockEvaluationSetService := svcMocks.NewMockIEvaluationSetService(ctrl)
	mockEvaluationSetVersionService := svcMocks.NewMockEvaluationSetVersionService(ctrl)
	mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
	mockEvaluatorService := svcMocks.NewMockEvaluatorService(ctrl)
	mockExptResultService := svcMocks.NewMockExptResultService(ctrl)
	mockExptAggrResultService := svcMocks.NewMockExptAggrResultService(ctrl)

	mgr := &ExptMangerImpl{
		exptRepo:                    mockExptRepo,
		lwt:                         mockLWT,
		evaluationSetService:        mockEvaluationSetService,
		evaluationSetVersionService: mockEvaluationSetVersionService,
		evalTargetService:           mockEvalTargetService,
		evaluatorService:            mockEvaluatorService,
		exptResultService:           mockExptResultService,
		exptAggrResultService:       mockExptAggrResultService,
	}

	ctx := context.Background()
	session := &entity.Session{UserID: "test"}
	spaceID := int64(1)
	page := int32(1)
	pageSize := int32(10)
	filter := &entity.ExptListFilter{}
	orderBys := []*entity.OrderBy{}

	expt := &entity.Experiment{
		ID:                  123,
		EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{{EvaluatorVersionID: 111}},
	}
	exptTuple := &entity.ExptTuple{
		EvalSet:    &entity.EvaluationSet{},
		Target:     &entity.EvalTarget{},
		Evaluators: []*entity.Evaluator{},
	}

	t.Run("正常获取", func(t *testing.T) {
		mockExptRepo.EXPECT().
			List(ctx, page, pageSize, filter, orderBys, spaceID).
			Return([]*entity.Experiment{expt}, int64(1), nil).Times(1)
		mockEvaluationSetService.EXPECT().
			BatchGetEvaluationSets(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*entity.EvaluationSet{exptTuple.EvalSet}, nil).AnyTimes()
		mockEvalTargetService.EXPECT().
			BatchGetEvalTargetVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*entity.EvalTarget{exptTuple.Target}, nil).AnyTimes()
		mockEvaluatorService.EXPECT().
			BatchGetEvaluatorVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*entity.Evaluator{}, nil).AnyTimes()
		mockExptResultService.EXPECT().
			MGetStats(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*entity.ExptStats{}, nil).AnyTimes()
		mockExptAggrResultService.EXPECT().
			BatchGetExptAggrResultByExperimentIDs(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*entity.ExptAggregateResult{}, nil).AnyTimes()

		got, count, err := mgr.List(ctx, page, pageSize, spaceID, filter, orderBys, session)
		if err != nil {
			t.Errorf("List() error = %v, wantErr %v", err, false)
		}
		if count != 1 {
			t.Errorf("List() count = %v, want %v", count, 1)
		}
		if len(got) != 1 || got[0].ID != expt.ID {
			t.Errorf("List() got = %v, want %v", got, []*entity.Experiment{expt})
		}
	})

	t.Run("repo返回错误", func(t *testing.T) {
		mockExptRepo.EXPECT().
			List(ctx, page, pageSize, filter, orderBys, spaceID).
			Return(nil, int64(0), fmt.Errorf("db error")).Times(1)
		got, count, err := mgr.List(ctx, page, pageSize, spaceID, filter, orderBys, session)
		if err == nil {
			t.Errorf("List() error = nil, wantErr true")
		}
		if got != nil || count != 0 {
			t.Errorf("List() got = %v, count = %v, want nil, 0", got, count)
		}
	})

	t.Run("mgetExptTupleByID返回错误", func(t *testing.T) {
		mockExptRepo.EXPECT().
			List(ctx, page, pageSize, filter, orderBys, spaceID).
			Return([]*entity.Experiment{expt}, int64(1), nil).Times(1)
		// 所有相关依赖都返回错误
		mockEvaluationSetService.EXPECT().
			BatchGetEvaluationSets(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("tuple error")).AnyTimes()
		mockEvalTargetService.EXPECT().
			BatchGetEvalTargetVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("tuple error")).AnyTimes()
		mockEvaluatorService.EXPECT().
			BatchGetEvaluatorVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("tuple error")).AnyTimes()

		got, count, err := mgr.List(ctx, page, pageSize, spaceID, filter, orderBys, session)
		if got == nil || count != 1 {
			t.Errorf("List() got = %v, count = %v, want not nil, 1", got, count)
		}
		if err != nil {
			t.Errorf("List() error = %v, wantErr nil", err)
		}
	})

	t.Run("packExperimentResult返回错误", func(t *testing.T) {
		mockExptRepo.EXPECT().
			List(ctx, page, pageSize, filter, orderBys, spaceID).
			Return([]*entity.Experiment{expt}, int64(1), nil).Times(1)
		mockEvaluationSetService.EXPECT().
			BatchGetEvaluationSets(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*entity.EvaluationSet{exptTuple.EvalSet}, nil).AnyTimes()
		mockEvalTargetService.EXPECT().
			BatchGetEvalTargetVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*entity.EvalTarget{exptTuple.Target}, nil).AnyTimes()
		mockEvaluatorService.EXPECT().
			BatchGetEvaluatorVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*entity.Evaluator{}, nil).AnyTimes()
		mockExptResultService.EXPECT().
			MGetStats(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("stats error")).AnyTimes()

		got, count, err := mgr.List(ctx, page, pageSize, spaceID, filter, orderBys, session)
		if got == nil || count != 1 {
			t.Errorf("List() got = %v, count = %v, want not nil, 1", got, count)
		}
		if err != nil {
			t.Errorf("List() error = %v, wantErr nil", err)
		}
	})
}

func TestExptMangerImpl_ListExptRaw(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mgr := &ExptMangerImpl{
		exptRepo: mockExptRepo,
	}

	ctx := context.Background()
	spaceID := int64(1)
	page := int32(1)
	pageSize := int32(10)
	filter := &entity.ExptListFilter{}

	expt := &entity.Experiment{ID: 123}

	t.Run("正常获取", func(t *testing.T) {
		mockExptRepo.EXPECT().
			List(ctx, page, pageSize, filter, nil, spaceID).
			Return([]*entity.Experiment{expt}, int64(1), nil).Times(1)

		got, count, err := mgr.ListExptRaw(ctx, page, pageSize, spaceID, filter)
		if err != nil {
			t.Errorf("ListExptRaw() error = %v, wantErr %v", err, false)
		}
		if count != 1 {
			t.Errorf("ListExptRaw() count = %v, want %v", count, 1)
		}
		if len(got) != 1 || got[0].ID != expt.ID {
			t.Errorf("ListExptRaw() got = %v, want %v", got, []*entity.Experiment{expt})
		}
	})

	t.Run("repo返回错误", func(t *testing.T) {
		mockExptRepo.EXPECT().
			List(ctx, page, pageSize, filter, nil, spaceID).
			Return(nil, int64(0), fmt.Errorf("db error")).Times(1)

		got, count, err := mgr.ListExptRaw(ctx, page, pageSize, spaceID, filter)
		if err == nil {
			t.Errorf("ListExptRaw() error = nil, wantErr true")
		}
		if got != nil || count != 0 {
			t.Errorf("ListExptRaw() got = %v, count = %v, want nil, 0", got, count)
		}
	})
}

func TestExptMangerImpl_GetDetail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockLWT := lwtMocks.NewMockILatestWriteTracker(ctrl)
	mockEvaluationSetService := svcMocks.NewMockIEvaluationSetService(ctrl)
	mockEvaluationSetVersionService := svcMocks.NewMockEvaluationSetVersionService(ctrl)
	mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
	mockEvaluatorService := svcMocks.NewMockEvaluatorService(ctrl)
	mockExptResultService := svcMocks.NewMockExptResultService(ctrl)
	mockExptAggrResultService := svcMocks.NewMockExptAggrResultService(ctrl)

	mgr := &ExptMangerImpl{
		exptRepo:                    mockExptRepo,
		lwt:                         mockLWT,
		evaluationSetService:        mockEvaluationSetService,
		evaluationSetVersionService: mockEvaluationSetVersionService,
		evalTargetService:           mockEvalTargetService,
		evaluatorService:            mockEvaluatorService,
		exptResultService:           mockExptResultService,
		exptAggrResultService:       mockExptAggrResultService,
	}

	ctx := context.Background()
	session := &entity.Session{UserID: "test"}
	exptID := int64(123)
	spaceID := int64(1)
	expt := &entity.Experiment{ID: exptID}
	tuple := &entity.ExptTuple{
		EvalSet:    &entity.EvaluationSet{},
		Target:     &entity.EvalTarget{},
		Evaluators: []*entity.Evaluator{},
	}

	t.Run("正常获取", func(t *testing.T) {
		mockExptRepo.EXPECT().
			MGetByID(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*entity.Experiment{expt}, nil).Times(1)
		mockEvalTargetService.EXPECT().
			GetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), gomock.Any()).
			Return(tuple.Target, nil).AnyTimes()
		mockEvaluationSetService.EXPECT().
			GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(tuple.EvalSet, nil).AnyTimes()
		mockEvaluatorService.EXPECT().
			BatchGetEvaluatorVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(tuple.Evaluators, nil).AnyTimes()
		mockExptResultService.EXPECT().
			MGetStats(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*entity.ExptStats{}, nil).AnyTimes()
		mockExptAggrResultService.EXPECT().
			BatchGetExptAggrResultByExperimentIDs(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*entity.ExptAggregateResult{}, nil).AnyTimes()
		mockLWT.EXPECT().
			CheckWriteFlagByID(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(false).AnyTimes()

		got, err := mgr.GetDetail(ctx, exptID, spaceID, session)
		if err != nil {
			t.Errorf("GetDetail() error = %v, wantErr %v", err, false)
		}
		if got == nil || got.ID != exptID {
			t.Errorf("GetDetail() got = %v, want exptID %v", got, exptID)
		}
	})

	t.Run("MGet返回错误", func(t *testing.T) {
		mockExptRepo.EXPECT().
			MGetByID(gomock.Any(), []int64{exptID}, spaceID).
			Return(nil, fmt.Errorf("db error")).Times(1)
		got, err := mgr.GetDetail(ctx, exptID, spaceID, session)
		if err == nil {
			t.Errorf("GetDetail() error = nil, wantErr true")
		}
		if got != nil {
			t.Errorf("GetDetail() got = %v, want nil", got)
		}
	})
}

func TestNewExptManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptResultService := svcMocks.NewMockExptResultService(ctrl)
	mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockExptRunLogRepo := repoMocks.NewMockIExptRunLogRepo(ctrl)
	mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockQuotaRepo := repoMocks.NewMockQuotaRepo(ctrl)
	mockMutex := lockMocks.NewMockILocker(ctrl)
	mockIdem := idemMocks.NewMockIdempotentService(ctrl)
	mockPublisher := eventsMocks.NewMockExptEventPublisher(ctrl)
	mockAudit := auditMocks.NewMockIAuditService(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)
	mockMetric := metricsMocks.NewMockExptMetric(ctrl)
	mockLWT := lwtMocks.NewMockILatestWriteTracker(ctrl)
	mockEvaluationSetVersionService := svcMocks.NewMockEvaluationSetVersionService(ctrl)
	mockEvaluationSetService := svcMocks.NewMockIEvaluationSetService(ctrl)
	mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
	mockEvaluatorService := svcMocks.NewMockEvaluatorService(ctrl)
	mockBenefitService := benefitMocks.NewMockIBenefitService(ctrl)
	mockExptAggrResultService := svcMocks.NewMockExptAggrResultService(ctrl)
	mockTemplateRepo := repoMocks.NewMockIExptTemplateRepo(ctrl)
	mockTemplateManager := svcMocks.NewMockIExptTemplateManager(ctrl)

	mgr := NewExptManager(
		mockExptResultService,
		mockExptRepo,
		mockExptRunLogRepo,
		mockExptStatsRepo,
		mockExptItemResultRepo,
		mockExptTurnResultRepo,
		mockConfiger,
		mockQuotaRepo,
		mockMutex,
		mockIdem,
		mockPublisher,
		mockAudit,
		mockIDGen,
		mockMetric,
		mockLWT,
		mockEvaluationSetVersionService,
		mockEvaluationSetService,
		mockEvalTargetService,
		mockEvaluatorService,
		mockBenefitService,
		mockExptAggrResultService,
		mockTemplateRepo,
		mockTemplateManager,
	)

	impl, ok := mgr.(*ExptMangerImpl)
	if !ok {
		t.Fatalf("NewExptManager should return *ExptMangerImpl")
	}

	// 断言部分关键依赖
	if impl.exptResultService != mockExptResultService {
		t.Errorf("exptResultService not set correctly")
	}
	if impl.exptRepo != mockExptRepo {
		t.Errorf("exptRepo not set correctly")
	}
	if impl.lwt != mockLWT {
		t.Errorf("lwt not set correctly")
	}
	if impl.evaluationSetService != mockEvaluationSetService {
		t.Errorf("evaluationSetService not set correctly")
	}
	if impl.benefitService != mockBenefitService {
		t.Errorf("benefitService not set correctly")
	}
}

func TestExptMangerImpl_MDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}
	spaceID := int64(1)

	t.Run("正常批量删除", func(t *testing.T) {
		exptIDs := []int64{1, 2}
		expts := []*entity.Experiment{
			{ID: 1, SpaceID: spaceID},
			{ID: 2, SpaceID: spaceID},
		}
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().MGetByID(ctx, exptIDs, spaceID).Return(expts, nil)
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().MDelete(ctx, exptIDs, spaceID).Return(nil)

		err := mgr.MDelete(ctx, exptIDs, spaceID, session)
		if err != nil {
			t.Errorf("MDelete() error = %v", err)
		}
	})

	t.Run("批量删除时关联模板，更新模板ExptInfo", func(t *testing.T) {
		exptIDs := []int64{1, 2}
		expts := []*entity.Experiment{
			{
				ID:               1,
				SpaceID:          spaceID,
				Status:           entity.ExptStatus_Success,
				ExptTemplateMeta: &entity.ExptTemplateMeta{ID: 100},
			},
			{
				ID:               2,
				SpaceID:          spaceID,
				Status:           entity.ExptStatus_Failed,
				ExptTemplateMeta: &entity.ExptTemplateMeta{ID: 200},
			},
		}
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().MGetByID(ctx, exptIDs, spaceID).Return(expts, nil)
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().MDelete(ctx, exptIDs, spaceID).Return(nil)
		mgr.templateManager.(*svcMocks.MockIExptTemplateManager).EXPECT().
			UpdateExptInfo(ctx, int64(100), spaceID, int64(1), entity.ExptStatus_Success, int64(-1)).Return(nil)
		mgr.templateManager.(*svcMocks.MockIExptTemplateManager).EXPECT().
			UpdateExptInfo(ctx, int64(200), spaceID, int64(2), entity.ExptStatus_Failed, int64(-1)).Return(nil)

		err := mgr.MDelete(ctx, exptIDs, spaceID, session)
		if err != nil {
			t.Errorf("MDelete() error = %v", err)
		}
	})

	t.Run("MGetByID失败", func(t *testing.T) {
		exptIDs := []int64{1, 2}
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().MGetByID(ctx, exptIDs, spaceID).Return(nil, errors.New("db error"))

		err := mgr.MDelete(ctx, exptIDs, spaceID, session)
		if err == nil {
			t.Errorf("MDelete() expected error, got nil")
		}
	})

	t.Run("MDelete失败", func(t *testing.T) {
		exptIDs := []int64{1, 2}
		expts := []*entity.Experiment{
			{ID: 1, SpaceID: spaceID},
			{ID: 2, SpaceID: spaceID},
		}
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().MGetByID(ctx, exptIDs, spaceID).Return(expts, nil)
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().MDelete(ctx, exptIDs, spaceID).Return(errors.New("delete error"))

		err := mgr.MDelete(ctx, exptIDs, spaceID, session)
		if err == nil {
			t.Errorf("MDelete() expected error, got nil")
		}
	})

	t.Run("UpdateExptInfo失败不影响主流程", func(t *testing.T) {
		exptIDs := []int64{1}
		expts := []*entity.Experiment{
			{
				ID:               1,
				SpaceID:          spaceID,
				Status:           entity.ExptStatus_Success,
				ExptTemplateMeta: &entity.ExptTemplateMeta{ID: 100},
			},
		}
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().MGetByID(ctx, exptIDs, spaceID).Return(expts, nil)
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().MDelete(ctx, exptIDs, spaceID).Return(nil)
		mgr.templateManager.(*svcMocks.MockIExptTemplateManager).EXPECT().
			UpdateExptInfo(ctx, int64(100), spaceID, int64(1), entity.ExptStatus_Success, int64(-1)).
			Return(errors.New("update error"))

		// UpdateExptInfo失败不应该影响主流程，应该返回nil
		err := mgr.MDelete(ctx, exptIDs, spaceID, session)
		if err != nil {
			t.Errorf("MDelete() should not return error when UpdateExptInfo fails, got %v", err)
		}
	})

	t.Run("实验没有关联模板，跳过UpdateExptInfo", func(t *testing.T) {
		exptIDs := []int64{1, 2}
		expts := []*entity.Experiment{
			{ID: 1, SpaceID: spaceID, ExptTemplateMeta: nil},
			{ID: 2, SpaceID: spaceID, ExptTemplateMeta: &entity.ExptTemplateMeta{ID: 0}},
		}
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().MGetByID(ctx, exptIDs, spaceID).Return(expts, nil)
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().MDelete(ctx, exptIDs, spaceID).Return(nil)

		err := mgr.MDelete(ctx, exptIDs, spaceID, session)
		if err != nil {
			t.Errorf("MDelete() error = %v", err)
		}
	})
}

func TestExptMangerImpl_fillExptTemplates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	spaceID := int64(1)

	t.Run("expts为空，直接返回", func(t *testing.T) {
		err := mgr.fillExptTemplates(ctx, nil, spaceID)
		assert.NoError(t, err)

		err = mgr.fillExptTemplates(ctx, []*entity.Experiment{}, spaceID)
		assert.NoError(t, err)
	})

	t.Run("templateRepo为nil，直接返回", func(t *testing.T) {
		mgrNoRepo := newTestExptManager(ctrl)
		mgrNoRepo.templateRepo = nil
		expts := []*entity.Experiment{
			{
				ID:               1,
				ExptTemplateMeta: &entity.ExptTemplateMeta{ID: 100},
			},
		}
		err := mgrNoRepo.fillExptTemplates(ctx, expts, spaceID)
		assert.NoError(t, err)
	})

	t.Run("没有有效的ExptTemplateMeta，直接返回", func(t *testing.T) {
		expts := []*entity.Experiment{
			{ID: 1, ExptTemplateMeta: nil},
			{ID: 2, ExptTemplateMeta: &entity.ExptTemplateMeta{ID: 0}},
		}
		err := mgr.fillExptTemplates(ctx, expts, spaceID)
		assert.NoError(t, err)
	})

	t.Run("成功填充模板信息", func(t *testing.T) {
		templateID1 := int64(100)
		templateID2 := int64(200)
		expts := []*entity.Experiment{
			{
				ID:               1,
				ExptTemplateMeta: &entity.ExptTemplateMeta{ID: templateID1},
			},
			{
				ID:               2,
				ExptTemplateMeta: &entity.ExptTemplateMeta{ID: templateID2},
			},
		}

		templates := []*entity.ExptTemplate{
			{
				Meta: &entity.ExptTemplateMeta{
					ID:          templateID1,
					WorkspaceID: spaceID,
					Name:        "template1",
					Desc:        "desc1",
				},
			},
			{
				Meta: &entity.ExptTemplateMeta{
					ID:          templateID2,
					WorkspaceID: spaceID,
					Name:        "template2",
					Desc:        "desc2",
				},
			},
		}

		mgr.templateRepo.(*repoMocks.MockIExptTemplateRepo).EXPECT().
			MGetByID(ctx, gomock.Any(), spaceID).
			DoAndReturn(func(_ context.Context, ids []int64, _ int64) ([]*entity.ExptTemplate, error) {
				// 验证ids包含templateID1和templateID2，但不关心顺序
				assert.Len(t, ids, 2)
				assert.Contains(t, ids, templateID1)
				assert.Contains(t, ids, templateID2)
				// 返回对应的模板
				result := make([]*entity.ExptTemplate, 0, 2)
				for _, id := range ids {
					switch id {
					case templateID1:
						result = append(result, templates[0])
					case templateID2:
						result = append(result, templates[1])
					}
				}
				return result, nil
			})

		err := mgr.fillExptTemplates(ctx, expts, spaceID)
		assert.NoError(t, err)
		// 验证模板信息被正确填充
		assert.Equal(t, "template1", expts[0].ExptTemplateMeta.Name)
		assert.Equal(t, "template2", expts[1].ExptTemplateMeta.Name)
	})

	t.Run("模板查询返回空列表，将ExptTemplateMeta置为nil", func(t *testing.T) {
		templateID := int64(100)
		expts := []*entity.Experiment{
			{
				ID:               1,
				ExptTemplateMeta: &entity.ExptTemplateMeta{ID: templateID},
			},
		}

		mgr.templateRepo.(*repoMocks.MockIExptTemplateRepo).EXPECT().
			MGetByID(ctx, gomock.Any(), spaceID).
			DoAndReturn(func(_ context.Context, ids []int64, _ int64) ([]*entity.ExptTemplate, error) {
				// 验证ids包含templateID
				assert.Len(t, ids, 1)
				assert.Contains(t, ids, templateID)
				return []*entity.ExptTemplate{}, nil
			})

		err := mgr.fillExptTemplates(ctx, expts, spaceID)
		assert.NoError(t, err)
		// 模板查询为空，ExptTemplateMeta应该被置为nil
		assert.Nil(t, expts[0].ExptTemplateMeta)
	})

	t.Run("模板在数据库中查不到，将ExptTemplateMeta置为nil", func(t *testing.T) {
		templateID1 := int64(100)
		templateID2 := int64(200)
		expts := []*entity.Experiment{
			{
				ID:               1,
				ExptTemplateMeta: &entity.ExptTemplateMeta{ID: templateID1},
			},
			{
				ID:               2,
				ExptTemplateMeta: &entity.ExptTemplateMeta{ID: templateID2},
			},
		}

		// 只返回一个模板，另一个查不到
		templates := []*entity.ExptTemplate{
			{
				Meta: &entity.ExptTemplateMeta{
					ID:          templateID1,
					WorkspaceID: spaceID,
					Name:        "template1",
				},
			},
		}

		mgr.templateRepo.(*repoMocks.MockIExptTemplateRepo).EXPECT().
			MGetByID(ctx, gomock.Any(), spaceID).
			DoAndReturn(func(_ context.Context, ids []int64, _ int64) ([]*entity.ExptTemplate, error) {
				// 验证ids包含templateID1和templateID2，但不关心顺序
				assert.Len(t, ids, 2)
				assert.Contains(t, ids, templateID1)
				assert.Contains(t, ids, templateID2)
				// 只返回templateID1对应的模板（templateID2查不到）
				result := make([]*entity.ExptTemplate, 0, 1)
				for _, id := range ids {
					if id == templateID1 {
						result = append(result, templates[0])
						break
					}
				}
				return result, nil
			})

		err := mgr.fillExptTemplates(ctx, expts, spaceID)
		assert.NoError(t, err)
		// 第一个模板应该被填充
		assert.NotNil(t, expts[0].ExptTemplateMeta)
		assert.Equal(t, "template1", expts[0].ExptTemplateMeta.Name)
		// 第二个模板查不到，应该被置为nil
		assert.Nil(t, expts[1].ExptTemplateMeta)
	})

	t.Run("模板查询失败，返回错误", func(t *testing.T) {
		templateID := int64(100)
		expts := []*entity.Experiment{
			{
				ID:               1,
				ExptTemplateMeta: &entity.ExptTemplateMeta{ID: templateID},
			},
		}

		mgr.templateRepo.(*repoMocks.MockIExptTemplateRepo).EXPECT().
			MGetByID(ctx, gomock.Any(), spaceID).
			DoAndReturn(func(_ context.Context, ids []int64, _ int64) ([]*entity.ExptTemplate, error) {
				// 验证ids包含templateID
				assert.Len(t, ids, 1)
				assert.Contains(t, ids, templateID)
				return nil, errors.New("db error")
			})

		err := mgr.fillExptTemplates(ctx, expts, spaceID)
		assert.Error(t, err)
	})

	t.Run("模板为nil，跳过", func(t *testing.T) {
		templateID1 := int64(100)
		templateID2 := int64(200)
		expts := []*entity.Experiment{
			{
				ID:               1,
				ExptTemplateMeta: &entity.ExptTemplateMeta{ID: templateID1},
			},
			{
				ID:               2,
				ExptTemplateMeta: &entity.ExptTemplateMeta{ID: templateID2},
			},
		}

		// 返回的模板中包含nil
		templates := []*entity.ExptTemplate{
			{
				Meta: &entity.ExptTemplateMeta{
					ID:          templateID1,
					WorkspaceID: spaceID,
					Name:        "template1",
				},
			},
			nil, // nil模板
		}

		mgr.templateRepo.(*repoMocks.MockIExptTemplateRepo).EXPECT().
			MGetByID(ctx, gomock.Any(), spaceID).
			DoAndReturn(func(_ context.Context, ids []int64, _ int64) ([]*entity.ExptTemplate, error) {
				// 验证ids包含templateID1和templateID2，但不关心顺序
				assert.Len(t, ids, 2)
				assert.Contains(t, ids, templateID1)
				assert.Contains(t, ids, templateID2)
				// 返回对应的模板
				result := make([]*entity.ExptTemplate, 0, 2)
				for _, id := range ids {
					switch id {
					case templateID1:
						result = append(result, templates[0])
					case templateID2:
						result = append(result, templates[1])
					}
				}
				return result, nil
			})

		err := mgr.fillExptTemplates(ctx, expts, spaceID)
		assert.NoError(t, err)
		// 第一个模板应该被填充
		assert.NotNil(t, expts[0].ExptTemplateMeta)
		// 第二个模板为nil，对应的实验的ExptTemplateMeta应该被置为nil
		assert.Nil(t, expts[1].ExptTemplateMeta)
	})
}

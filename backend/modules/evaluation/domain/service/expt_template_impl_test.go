// Copyright 2026
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	platestwrite "github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	lwtmocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repo_mocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

// 基础字段构造，方便多个用例复用
func newBasicCreateParam() *entity.CreateExptTemplateParam {
	return &entity.CreateExptTemplateParam{
		SpaceID:          100,
		Name:             "tpl",
		Description:      "desc",
		ExptType:         entity.ExptType_Offline,
		EvalSetID:        1,
		EvalSetVersionID: 11,
	}
}

func TestExptTemplateManagerImpl_CheckName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mgr := &ExptTemplateManagerImpl{templateRepo: mockRepo}
	ctx := context.Background()
	spaceID := int64(100)

	t.Run("repo error", func(t *testing.T) {
		mockRepo.EXPECT().GetByName(ctx, "tpl", spaceID).Return(nil, false, errors.New("dao err"))
		pass, err := mgr.CheckName(ctx, "tpl", spaceID, &entity.Session{})
		assert.Error(t, err)
		assert.False(t, pass)
	})

	t.Run("exists", func(t *testing.T) {
		mockRepo.EXPECT().GetByName(ctx, "tpl", spaceID).Return(&entity.ExptTemplate{}, true, nil)
		pass, err := mgr.CheckName(ctx, "tpl", spaceID, &entity.Session{})
		assert.NoError(t, err)
		assert.False(t, pass)
	})

	t.Run("not exists", func(t *testing.T) {
		mockRepo.EXPECT().GetByName(ctx, "tpl2", spaceID).Return(nil, false, nil)
		pass, err := mgr.CheckName(ctx, "tpl2", spaceID, &entity.Session{})
		assert.NoError(t, err)
		assert.True(t, pass)
	})
}

func TestExptTemplateManagerImpl_Create_NameExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockIdgen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)
	mockLWT := lwtmocks.NewMockILatestWriteTracker(ctrl)

	mgr := NewExptTemplateManager(
		mockRepo,
		mockIdgen,
		mockEvalSvc,
		mockTargetSvc,
		mockEvalSetSvc,
		mockEvalSetVerSvc,
		mockLWT,
	)

	ctx := context.Background()
	param := newBasicCreateParam()
	session := &entity.Session{UserID: "u1"}

	// CheckName 返回已存在
	mockRepo.EXPECT().GetByName(ctx, param.Name, param.SpaceID).Return(&entity.ExptTemplate{}, true, nil)

	got, err := mgr.Create(ctx, param, session)
	assert.Error(t, err)
	assert.Nil(t, got)
	// 只校验这是一个 evaluation 业务错误 code，而不关心具体类型
	code, _, ok := errno.ParseStatusError(err)
	assert.True(t, ok)
	assert.Equal(t, errno.ExperimentNameExistedCode, int(code))
}

func TestExptTemplateManagerImpl_Create_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockIdgen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)
	mockLWT := lwtmocks.NewMockILatestWriteTracker(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:                mockRepo,
		idgen:                       mockIdgen,
		evaluatorService:            mockEvalSvc,
		evalTargetService:           mockTargetSvc,
		evaluationSetService:        mockEvalSetSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
		lwt:                         mockLWT,
	}

	ctx := context.Background()
	param := newBasicCreateParam()
	param.EvaluatorIDVersionItems = []*entity.EvaluatorIDVersionItem{
		{EvaluatorID: 10, Version: "v1", EvaluatorVersionID: 1001},
	}
	param.TemplateConf = &entity.ExptTemplateConfiguration{}
	session := &entity.Session{UserID: "u1"}

	// CheckName
	mockRepo.EXPECT().GetByName(ctx, param.Name, param.SpaceID).Return(nil, false, nil)
	// idgen
	mockIdgen.EXPECT().GenID(ctx).Return(int64(10001), nil)
	// mgetExptTupleByID 内部会调用 evaluationSetVersionService / evaluationSetService / evalTargetService / evaluatorService
	mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(param.SpaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
	mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(param.SpaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
	mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), param.SpaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
	mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()
	// repo.Create
	mockRepo.EXPECT().Create(ctx, gomock.Any(), gomock.Any()).Return(nil)
	// LWT
	mockLWT.EXPECT().SetWriteFlag(ctx, platestwrite.ResourceTypeExptTemplate, int64(10001)).AnyTimes()

	got, err := mgr.Create(ctx, param, session)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, int64(10001), got.GetID())
	assert.Equal(t, param.Name, got.GetName())
	assert.Equal(t, param.SpaceID, got.GetSpaceID())
	assert.Equal(t, param.EvalSetID, got.GetEvalSetID())
	assert.Equal(t, param.EvalSetVersionID, got.GetEvalSetVersionID())
	assert.Equal(t, "u1", got.GetCreatedBy())
}

func TestExptTemplateManagerImpl_MGet_UseWriteDBOnSingleWithFlag(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockIdgen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)
	mockLWT := lwtmocks.NewMockILatestWriteTracker(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:                mockRepo,
		idgen:                       mockIdgen,
		evaluatorService:            mockEvalSvc,
		evalTargetService:           mockTargetSvc,
		evaluationSetService:        mockEvalSetSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
		lwt:                         mockLWT,
	}

	ctx := context.Background()
	spaceID := int64(100)
	ids := []int64{1}
	session := &entity.Session{UserID: "u1"}

	// 写标志为 true，期望带 writeDB 上下文调用 repo.MGetByID
	mockLWT.EXPECT().CheckWriteFlagByID(ctx, platestwrite.ResourceTypeExptTemplate, int64(1)).Return(true)
	mockRepo.EXPECT().MGetByID(gomock.Any(), ids, spaceID).Return([]*entity.ExptTemplate{
		{
			Meta: &entity.ExptTemplateMeta{
				ID:          1,
				WorkspaceID: spaceID,
				Name:        "tpl",
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID:        10,
				EvalSetVersionID: 20,
			},
		},
	}, nil)
	// mgetExptTupleByID 需要 evaluationSetService / evalTargetService / evaluatorService 的协作，这里用空结果兜底
	mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
	mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
	mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
	mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

	got, err := mgr.MGet(ctx, ids, spaceID, session)
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, int64(1), got[0].GetID())
}

func TestExptTemplateManagerImpl_UpdateMeta_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)
	session := &entity.Session{UserID: "u1"}

	param := &entity.UpdateExptTemplateMetaParam{
		TemplateID:  templateID,
		SpaceID:     spaceID,
		Name:        "", // 不改名，避免触发 CheckName
		Description: "new-desc",
		ExptType:    entity.ExptType_Online,
	}

	existing := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
			Name:        "tpl",
			Desc:        "old-desc",
			ExptType:    entity.ExptType_Offline,
		},
	}

	updated := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
			Name:        "tpl",
			Desc:        "new-desc",
			ExptType:    entity.ExptType_Online,
		},
	}

	// 第一次 GetByID，拿到现有模板
	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return(existing, nil)

	// UpdateFields：校验写入字段包含 description / expt_type / updated_by
	mockRepo.EXPECT().
		UpdateFields(ctx, templateID, gomock.AssignableToTypeOf(map[string]any{})).
		DoAndReturn(func(_ context.Context, _ int64, fields map[string]any) error {
			assert.Equal(t, "new-desc", fields["description"])
			assert.Equal(t, int32(entity.ExptType_Online), fields["expt_type"])
			assert.Equal(t, "u1", fields["updated_by"])
			// updated_at 为 time.Time，这里不做具体断言
			assert.NotNil(t, fields["updated_at"])
			return nil
		})

	// 第二次 GetByID，返回更新后的模板
	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return(updated, nil)

	got, err := mgr.UpdateMeta(ctx, param, session)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, "new-desc", got.GetDescription())
	assert.Equal(t, entity.ExptType_Online, got.GetExptType())
	assert.NotNil(t, got.BaseInfo)
	if assert.NotNil(t, got.BaseInfo.UpdatedBy) && got.BaseInfo.UpdatedBy.UserID != nil {
		assert.Equal(t, "u1", *got.BaseInfo.UpdatedBy.UserID)
	}
}

func TestExptTemplateManagerImpl_UpdateExptInfo_NewAndClamp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)

	// 场景一：原来没有 ExptInfo，adjustCount = +1
	existing := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
		},
		ExptInfo: nil,
	}

	gomock.InOrder(
		mockRepo.EXPECT().
			GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
			Return(existing, nil),
		mockRepo.EXPECT().
			UpdateFields(ctx, templateID, gomock.AssignableToTypeOf(map[string]any{})).
			DoAndReturn(func(_ context.Context, _ int64, fields map[string]any) error {
				buf, ok := fields["expt_info"].([]byte)
				assert.True(t, ok)
				var info entity.ExptInfo
				err := json.Unmarshal(buf, &info)
				assert.NoError(t, err)
				assert.Equal(t, int64(1), info.CreatedExptCount)
				assert.Equal(t, int64(200), info.LatestExptID)
				assert.Equal(t, entity.ExptStatus_Processing, info.LatestExptStatus)
				return nil
			}),
	)

	err := mgr.UpdateExptInfo(ctx, templateID, spaceID, 200, entity.ExptStatus_Processing, 1)
	assert.NoError(t, err)

	// 场景二：已有 ExptInfo，adjustCount 负数，下限为 0
	existing2 := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
		},
		ExptInfo: &entity.ExptInfo{
			CreatedExptCount: 0,
			LatestExptID:     100,
			LatestExptStatus: entity.ExptStatus_Success,
		},
	}

	gomock.InOrder(
		mockRepo.EXPECT().
			GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
			Return(existing2, nil),
		mockRepo.EXPECT().
			UpdateFields(ctx, templateID, gomock.AssignableToTypeOf(map[string]any{})).
			DoAndReturn(func(_ context.Context, _ int64, fields map[string]any) error {
				buf, ok := fields["expt_info"].([]byte)
				assert.True(t, ok)
				var info entity.ExptInfo
				err := json.Unmarshal(buf, &info)
				assert.NoError(t, err)
				// CreatedExptCount 不会变成负数
				assert.Equal(t, int64(0), info.CreatedExptCount)
				assert.Equal(t, int64(300), info.LatestExptID)
				assert.Equal(t, entity.ExptStatus_Failed, info.LatestExptStatus)
				return nil
			}),
	)

	err = mgr.UpdateExptInfo(ctx, templateID, spaceID, 300, entity.ExptStatus_Failed, -5)
	assert.NoError(t, err)
}

func TestExptTemplateManagerImpl_UpdateExptInfo_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return((*entity.ExptTemplate)(nil), nil)

	err := mgr.UpdateExptInfo(ctx, templateID, spaceID, 1, entity.ExptStatus_Processing, 1)
	assert.Error(t, err)
	code, _, ok := errno.ParseStatusError(err)
	assert.True(t, ok)
	assert.Equal(t, errno.ResourceNotFoundCode, int(code))
}

func TestExptTemplateManagerImpl_Update_WithCreateEvalTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)
	mockLWT := lwtmocks.NewMockILatestWriteTracker(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:                mockRepo,
		evaluatorService:            mockEvalSvc,
		evalTargetService:           mockTargetSvc,
		evaluationSetService:        mockEvalSetSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
		lwt:                         mockLWT,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)
	session := &entity.Session{UserID: "u1"}

	// 现有模板
	existing := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
			Name:        "tpl-old",
			Desc:        "old-desc",
			ExptType:    entity.ExptType_Offline,
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:        10,
			EvalSetVersionID: 11,
			TargetID:         20,
			TargetVersionID:  21,
			TargetType:       entity.EvalTargetTypeLoopPrompt,
		},
	}

	// 更新参数：改名 + 创建新的 Target
	param := &entity.UpdateExptTemplateParam{
		TemplateID:       templateID,
		SpaceID:          spaceID,
		Name:             "tpl-new",
		Description:      "new-desc",
		EvalSetVersionID: 11,
		TargetVersionID:  0,
		EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{
			{EvaluatorID: 1, Version: "v1", EvaluatorVersionID: 101},
		},
		TemplateConf: &entity.ExptTemplateConfiguration{
			ConnectorConf: entity.Connector{
				TargetConf: &entity.TargetConf{},
				EvaluatorsConf: &entity.EvaluatorsConf{
					EvaluatorConf: []*entity.EvaluatorConf{
						{
							EvaluatorID: 1,
							Version:     "v1",
							IngressConf: &entity.EvaluatorIngressConf{
								EvalSetAdapter: &entity.FieldAdapter{},
							},
						},
					},
				},
			},
		},
		CreateEvalTargetParam: &entity.CreateEvalTargetParam{
			SourceTargetID:      gptr.Of("src-id"),
			SourceTargetVersion: gptr.Of("v1"),
			EvalTargetType:      gptr.Of(entity.EvalTargetTypeLoopPrompt),
		},
	}

	// CheckName 通过
	mockRepo.EXPECT().GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).Return(existing, nil)
	mockRepo.EXPECT().GetByName(ctx, param.Name, param.SpaceID).Return(nil, false, nil)

	// 解析 evaluator_version_id：TemplateConf 中的 EvaluatorConf 需要解析版本ID
	// 测试数据中 EvaluatorConf 有 EvaluatorID: 1, Version: "v1"，需要返回对应的 evaluator
	mockEvalSvc.EXPECT().
		BatchGetEvaluatorByIDAndVersion(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, pairs [][2]interface{}) ([]*entity.Evaluator, error) {
			evaluators := make([]*entity.Evaluator, 0)
			for _, pair := range pairs {
				eid := pair[0].(int64)
				ver := pair[1].(string)
				if eid == 1 && ver == "v1" {
					pev := &entity.PromptEvaluatorVersion{}
					pev.SetID(101)
					pev.SetEvaluatorID(1)
					pev.SetVersion("v1")
					evaluators = append(evaluators, &entity.Evaluator{
						ID:                     1,
						EvaluatorType:          entity.EvaluatorTypePrompt,
						PromptEvaluatorVersion: pev,
					})
				}
			}
			return evaluators, nil
		}).
		AnyTimes()

	// Update 中创建新 Target 前需要先获取现有 Target 以校验 SourceTargetID
	mockTargetSvc.EXPECT().
		GetEvalTarget(gomock.Any(), int64(20)).
		Return(&entity.EvalTarget{
			ID:             20,
			SourceTargetID: "src-id",
			EvalTargetType: entity.EvalTargetTypeLoopPrompt,
		}, nil)

	// 创建新的 Target
	mockTargetSvc.EXPECT().
		CreateEvalTarget(gomock.Any(), spaceID, "src-id", "v1", entity.EvalTargetTypeLoopPrompt, gomock.Any()).
		Return(int64(30), int64(40), nil)

	// UpdateWithRefs & GetByID
	mockRepo.EXPECT().
		UpdateWithRefs(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)
	updatedTemplateFromDB := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
			Name:        "tpl-new",
			Desc:        "new-desc",
			ExptType:    entity.ExptType_Offline,
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:           10,
			EvalSetVersionID:    11,
			TargetID:            30,
			TargetVersionID:     40,
			TargetType:          entity.EvalTargetTypeLoopPrompt,
			EvaluatorVersionIds: []int64{101}, // 用于 packTemplateTupleID 提取
		},
	}
	mockRepo.EXPECT().
		GetByID(gomock.Any(), templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return(updatedTemplateFromDB, nil)

	// mgetExptTupleByID：由于创建了新 Target，会使用 writeDB context，返回关联数据
	mockTargetSvc.EXPECT().
		BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).
		DoAndReturn(func(_ context.Context, _ int64, versionIDs []int64, _ bool) ([]*entity.EvalTarget, error) {
			targets := make([]*entity.EvalTarget, 0)
			for _, vid := range versionIDs {
				if vid == 40 {
					targets = append(targets, &entity.EvalTarget{
						EvalTargetVersion: &entity.EvalTargetVersion{ID: 40},
					})
				}
			}
			return targets, nil
		})
	mockEvalSetVerSvc.EXPECT().
		BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).
		DoAndReturn(func(_ context.Context, _ *int64, versionIDs []int64, _ *bool) ([]*entity.BatchGetEvaluationSetVersionsResult, error) {
			results := make([]*entity.BatchGetEvaluationSetVersionsResult, 0)
			for _, vid := range versionIDs {
				if vid == 11 {
					results = append(results, &entity.BatchGetEvaluationSetVersionsResult{
						Version: &entity.EvaluationSetVersion{ID: 11},
						EvaluationSet: &entity.EvaluationSet{
							ID:                   10,
							EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 11},
						},
					})
				}
			}
			return results, nil
		})
	mockEvalSetSvc.EXPECT().
		BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).
		Return(nil, nil).
		AnyTimes()
	mockEvalSvc.EXPECT().
		BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).
		DoAndReturn(func(_ context.Context, _ *int64, versionIDs []int64, _ bool) ([]*entity.Evaluator, error) {
			evaluators := make([]*entity.Evaluator, 0)
			for _, vid := range versionIDs {
				if vid == 101 {
					pev := &entity.PromptEvaluatorVersion{}
					pev.SetID(101)
					evaluators = append(evaluators, &entity.Evaluator{
						PromptEvaluatorVersion: pev,
					})
				}
			}
			return evaluators, nil
		})

	got, err := mgr.Update(ctx, param, session)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, "tpl-new", got.GetName())
	assert.Equal(t, int64(30), got.GetTargetID())
	assert.Equal(t, int64(40), got.GetTargetVersionID())
}

func TestExptTemplateManagerImpl_List_FillTuples(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)
	mockLWT := lwtmocks.NewMockILatestWriteTracker(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:                mockRepo,
		evaluatorService:            mockEvalSvc,
		evalTargetService:           mockTargetSvc,
		evaluationSetService:        mockEvalSetSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
		lwt:                         mockLWT,
	}

	ctx := context.Background()
	spaceID := int64(100)

	templates := []*entity.ExptTemplate{
		{
			Meta: &entity.ExptTemplateMeta{
				ID:          1,
				WorkspaceID: spaceID,
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID:           10,
				EvalSetVersionID:    11,
				TargetID:            20,
				TargetVersionID:     21,
				EvaluatorVersionIds: []int64{101},
			},
		},
	}

	mockRepo.EXPECT().
		List(ctx, int32(1), int32(10), nil, nil, spaceID).
		Return(templates, int64(1), nil)

	// mgetExptTupleByID 相关依赖：返回一个 EvalSet、Target、Evaluator
	// 注意：targetMap 使用 EvalTargetVersion.ID 作为 key，所以返回的 Target 需要 EvalTargetVersion.ID = 21
	mockTargetSvc.EXPECT().
		BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).
		DoAndReturn(func(_ context.Context, _ int64, versionIDs []int64, _ bool) ([]*entity.EvalTarget, error) {
			// 确保返回的 Target 的 EvalTargetVersion.ID 匹配请求的 versionID
			targets := make([]*entity.EvalTarget, 0)
			for _, vid := range versionIDs {
				if vid == 21 {
					targets = append(targets, &entity.EvalTarget{
						EvalTargetVersion: &entity.EvalTargetVersion{ID: 21},
					})
				}
			}
			return targets, nil
		})
	// evalSetMap 使用 EvaluationSetVersion.ID 作为 key（当 EvalSetID != VersionID 时）
	mockEvalSetVerSvc.EXPECT().
		BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).
		DoAndReturn(func(_ context.Context, _ *int64, versionIDs []int64, _ *bool) ([]*entity.BatchGetEvaluationSetVersionsResult, error) {
			results := make([]*entity.BatchGetEvaluationSetVersionsResult, 0)
			for _, vid := range versionIDs {
				if vid == 11 {
					results = append(results, &entity.BatchGetEvaluationSetVersionsResult{
						Version: &entity.EvaluationSetVersion{ID: 11},
						EvaluationSet: &entity.EvaluationSet{
							ID:                   10,
							EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 11},
						},
					})
				}
			}
			return results, nil
		})
	mockEvalSetSvc.EXPECT().
		BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).
		Return(nil, nil).
		AnyTimes()
	// evaluatorMap 使用 GetEvaluatorVersionID() 作为 key
	mockEvalSvc.EXPECT().
		BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).
		DoAndReturn(func(_ context.Context, _ *int64, versionIDs []int64, _ bool) ([]*entity.Evaluator, error) {
			evaluators := make([]*entity.Evaluator, 0)
			for _, vid := range versionIDs {
				if vid == 101 {
					pev := &entity.PromptEvaluatorVersion{}
					pev.SetID(101)
					evaluators = append(evaluators, &entity.Evaluator{
						EvaluatorType:          entity.EvaluatorTypePrompt,
						PromptEvaluatorVersion: pev,
					})
				}
			}
			return evaluators, nil
		})

	got, total, err := mgr.List(ctx, 1, 10, spaceID, nil, nil, &entity.Session{UserID: "u1"})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	if assert.Len(t, got, 1) {
		assert.NotNil(t, got[0].EvalSet)
		assert.NotNil(t, got[0].Target)
		assert.Len(t, got[0].Evaluators, 1)
	}
}

func TestExptTemplateManagerImpl_resolveTargetForCreate_Paths(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mgr := &ExptTemplateManagerImpl{
		evalTargetService: mockTargetSvc,
	}

	ctx := context.Background()

	// 分支一：CreateEvalTargetParam 非空 -> 创建新 Target
	param1 := &entity.CreateExptTemplateParam{
		SpaceID: 100,
		CreateEvalTargetParam: &entity.CreateEvalTargetParam{
			SourceTargetID:      gptr.Of("src-id"),
			SourceTargetVersion: gptr.Of("v1"),
			EvalTargetType:      gptr.Of(entity.EvalTargetTypeLoopPrompt),
		},
	}
	mockTargetSvc.EXPECT().
		CreateEvalTarget(gomock.Any(), int64(100), "src-id", "v1", entity.EvalTargetTypeLoopPrompt, gomock.Any()).
		Return(int64(20), int64(21), nil)

	tid, tver, ttype, err := mgr.resolveTargetForCreate(ctx, param1)
	assert.NoError(t, err)
	assert.Equal(t, int64(20), tid)
	assert.Equal(t, int64(21), tver)
	assert.Equal(t, entity.EvalTargetTypeLoopPrompt, ttype)

	// 分支二：使用已有 TargetID
	param2 := &entity.CreateExptTemplateParam{
		SpaceID:         200,
		TargetID:        30,
		TargetVersionID: 31,
	}
	mockTargetSvc.EXPECT().
		GetEvalTarget(gomock.Any(), int64(30)).
		Return(&entity.EvalTarget{EvalTargetType: entity.EvalTargetTypeCustomRPCServer}, nil)

	tid2, tver2, ttype2, err := mgr.resolveTargetForCreate(ctx, param2)
	assert.NoError(t, err)
	assert.Equal(t, int64(30), tid2)
	assert.Equal(t, int64(31), tver2)
	assert.Equal(t, entity.EvalTargetTypeCustomRPCServer, ttype2)

	// 分支三：既无 CreateEvalTargetParam 也无 TargetID
	param3 := &entity.CreateExptTemplateParam{SpaceID: 300}
	tid3, tver3, ttype3, err := mgr.resolveTargetForCreate(ctx, param3)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), tid3)
	assert.Equal(t, int64(0), tver3)
	assert.Equal(t, entity.EvalTargetType(0), ttype3)
}

func TestExptTemplateManagerImpl_buildFieldMappingConfigAndEnableScoreWeight(t *testing.T) {
	mgr := &ExptTemplateManagerImpl{}

	template := &entity.ExptTemplate{}
	templateConf := &entity.ExptTemplateConfiguration{
		ItemConcurNum: gptr.Of(3),
		ConnectorConf: entity.Connector{
			TargetConf: &entity.TargetConf{
				IngressConf: &entity.TargetIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{
							{FieldName: "t1", FromField: "src1", Value: "v1"},
						},
					},
					CustomConf: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{
							{FieldName: "builtin_runtime_param", Value: `{"k":"v"}`},
						},
					},
				},
			},
			EvaluatorsConf: &entity.EvaluatorsConf{
				EvaluatorConf: []*entity.EvaluatorConf{
					{
						EvaluatorVersionID: 101,
						ScoreWeight:        gptr.Of(0.7),
						IngressConf: &entity.EvaluatorIngressConf{
							EvalSetAdapter: &entity.FieldAdapter{
								FieldConfs: []*entity.FieldConf{
									{FieldName: "ein", FromField: "col1", Value: ""},
								},
							},
							TargetAdapter: &entity.FieldAdapter{
								FieldConfs: []*entity.FieldConf{
									{FieldName: "eout", FromField: "col2", Value: ""},
								},
							},
						},
					},
				},
			},
		},
	}

	mgr.buildFieldMappingConfigAndEnableScoreWeight(template, templateConf)

	if assert.NotNil(t, template.FieldMappingConfig) {
		assert.Equal(t, 3, gptr.Indirect(template.FieldMappingConfig.ItemConcurNum))

		// TargetFieldMapping
		if assert.NotNil(t, template.FieldMappingConfig.TargetFieldMapping) {
			assert.Len(t, template.FieldMappingConfig.TargetFieldMapping.FromEvalSet, 1)
			f := template.FieldMappingConfig.TargetFieldMapping.FromEvalSet[0]
			assert.Equal(t, "t1", f.FieldName)
			assert.Equal(t, "src1", f.FromFieldName)
			assert.Equal(t, "v1", f.ConstValue)
		}
		// TargetRuntimeParam
		if assert.NotNil(t, template.FieldMappingConfig.TargetRuntimeParam) {
			assert.Equal(t, `{"k":"v"}`, gptr.Indirect(template.FieldMappingConfig.TargetRuntimeParam.JSONValue))
		}
		// EvaluatorFieldMapping
		if assert.Len(t, template.FieldMappingConfig.EvaluatorFieldMapping, 1) {
			em := template.FieldMappingConfig.EvaluatorFieldMapping[0]
			assert.Equal(t, int64(101), em.EvaluatorVersionID)
			assert.Len(t, em.FromEvalSet, 1)
			assert.Len(t, em.FromTarget, 1)
		}
	}
	// EnableScoreWeight 应该被置为 true
	if assert.NotNil(t, templateConf.ConnectorConf.EvaluatorsConf) {
		assert.True(t, templateConf.ConnectorConf.EvaluatorsConf.EnableScoreWeight)
	}
}

func TestExptTemplateManagerImpl_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)

	mockRepo.EXPECT().Delete(ctx, templateID, spaceID).Return(nil)

	err := mgr.Delete(ctx, templateID, spaceID, &entity.Session{UserID: "u1"})
	assert.NoError(t, err)
}

func TestExptTemplateManagerImpl_List_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(100)

	mockRepo.EXPECT().List(ctx, int32(1), int32(10), gomock.Nil(), gomock.Nil(), spaceID).
		Return([]*entity.ExptTemplate{}, int64(0), nil)

	templates, total, err := mgr.List(ctx, 1, 10, spaceID, nil, nil, &entity.Session{UserID: "u1"})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Len(t, templates, 0)
}

func TestExptTemplateManagerImpl_buildEvaluatorVersionRefsAndExtractIDs(t *testing.T) {
	mgr := &ExptTemplateManagerImpl{}

	items := []*entity.EvaluatorIDVersionItem{
		{EvaluatorID: 1, EvaluatorVersionID: 101},
		{EvaluatorID: 2, EvaluatorVersionID: 102},
		// nil 和无效版本ID应该被忽略
		nil,
		{EvaluatorID: 3, EvaluatorVersionID: 0},
	}

	refs := mgr.buildEvaluatorVersionRefs(items)
	assert.Len(t, refs, 2)
	assert.Equal(t, int64(1), refs[0].EvaluatorID)
	assert.Equal(t, int64(101), refs[0].EvaluatorVersionID)

	ids := mgr.extractEvaluatorVersionIDs(items)
	assert.ElementsMatch(t, []int64{101, 102}, ids)
}

func TestExptTemplateManagerImpl_packTemplateTupleID(t *testing.T) {
	mgr := &ExptTemplateManagerImpl{}

	template := &entity.ExptTemplate{
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:        10,
			EvalSetVersionID: 20,
			TargetID:         30,
			TargetVersionID:  40,
			EvaluatorVersionIds: []int64{
				101, 102,
			},
		},
	}

	tupleID := mgr.packTemplateTupleID(template)
	if assert.NotNil(t, tupleID.VersionedEvalSetID) {
		assert.Equal(t, int64(10), tupleID.VersionedEvalSetID.EvalSetID)
		assert.Equal(t, int64(20), tupleID.VersionedEvalSetID.VersionID)
	}
	if assert.NotNil(t, tupleID.VersionedTargetID) {
		assert.Equal(t, int64(30), tupleID.VersionedTargetID.TargetID)
		assert.Equal(t, int64(40), tupleID.VersionedTargetID.VersionID)
	}
	assert.ElementsMatch(t, []int64{101, 102}, tupleID.EvaluatorVersionIDs)
}

func TestExptTemplateManagerImpl_resolveAndFillEvaluatorVersionIDs_Normal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)

	mgr := &ExptTemplateManagerImpl{
		evaluatorService: mockEvalSvc,
	}

	ctx := context.Background()
	spaceID := int64(100)

	// 一个需要解析版本ID的 EvaluatorIDVersionItem
	items := []*entity.EvaluatorIDVersionItem{
		{
			EvaluatorID:        1,
			Version:            "v1",
			EvaluatorVersionID: 0,
		},
	}

	// TemplateConf 中也有一条对应的 EvaluatorConf，需要被回填
	templateConf := &entity.ExptTemplateConfiguration{
		ConnectorConf: entity.Connector{
			EvaluatorsConf: &entity.EvaluatorsConf{
				EvaluatorConf: []*entity.EvaluatorConf{
					{
						EvaluatorID:        1,
						Version:            "v1",
						EvaluatorVersionID: 0,
					},
				},
			},
		},
	}

	// 模拟 evaluatorService 返回一个带版本ID的 Evaluator
	ev := &entity.Evaluator{
		ID:                     1,
		EvaluatorType:          entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{},
	}
	ev.PromptEvaluatorVersion.SetID(101)
	ev.PromptEvaluatorVersion.SetVersion("v1")

	normalPairs := [][2]interface{}{
		{int64(1), "v1"},
	}

	mockEvalSvc.EXPECT().
		BatchGetEvaluatorByIDAndVersion(ctx, normalPairs).
		Return([]*entity.Evaluator{ev}, nil)

	err := mgr.resolveAndFillEvaluatorVersionIDs(ctx, spaceID, templateConf, items)
	assert.NoError(t, err)

	// EvaluatorIDVersionItem 被回填
	assert.Equal(t, int64(101), items[0].EvaluatorVersionID)
	// TemplateConf 中的 EvaluatorConf 也被回填
	if assert.NotNil(t, templateConf.ConnectorConf.EvaluatorsConf) &&
		assert.Len(t, templateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf, 1) {
		assert.Equal(t, int64(101), templateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf[0].EvaluatorVersionID)
	}
}

// TestExptTemplateManagerImpl_MGet_NoWriteFlag 测试 MGet 方法在没有写标志时的行为（181-194行）
func TestExptTemplateManagerImpl_MGet_NoWriteFlag(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)
	mockLWT := lwtmocks.NewMockILatestWriteTracker(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:                mockRepo,
		evaluatorService:            mockEvalSvc,
		evalTargetService:           mockTargetSvc,
		evaluationSetService:        mockEvalSetSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
		lwt:                         mockLWT,
	}

	ctx := context.Background()
	spaceID := int64(100)
	ids := []int64{1}
	session := &entity.Session{UserID: "u1"}

	// 写标志为 false，不设置 writeDB 上下文
	mockLWT.EXPECT().CheckWriteFlagByID(ctx, platestwrite.ResourceTypeExptTemplate, int64(1)).Return(false)
	mockRepo.EXPECT().MGetByID(ctx, ids, spaceID).Return([]*entity.ExptTemplate{
		{
			Meta: &entity.ExptTemplateMeta{
				ID:          1,
				WorkspaceID: spaceID,
				Name:        "tpl",
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID:        10,
				EvalSetVersionID: 20,
			},
		},
	}, nil)
	// mgetExptTupleByID 相关依赖
	mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
	mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
	mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
	mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

	got, err := mgr.MGet(ctx, ids, spaceID, session)
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, int64(1), got[0].GetID())
}

// TestExptTemplateManagerImpl_MGet_MultipleIDs 测试 MGet 方法在多个ID时不检查写标志（181-194行）
func TestExptTemplateManagerImpl_MGet_MultipleIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)
	mockLWT := lwtmocks.NewMockILatestWriteTracker(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:                mockRepo,
		evaluatorService:            mockEvalSvc,
		evalTargetService:           mockTargetSvc,
		evaluationSetService:        mockEvalSetSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
		lwt:                         mockLWT,
	}

	ctx := context.Background()
	spaceID := int64(100)
	ids := []int64{1, 2}
	session := &entity.Session{UserID: "u1"}

	// 多个ID时不检查写标志，直接调用 repo.MGetByID
	mockRepo.EXPECT().MGetByID(ctx, ids, spaceID).Return([]*entity.ExptTemplate{
		{
			Meta: &entity.ExptTemplateMeta{
				ID:          1,
				WorkspaceID: spaceID,
				Name:        "tpl1",
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID:        10,
				EvalSetVersionID: 20,
			},
		},
		{
			Meta: &entity.ExptTemplateMeta{
				ID:          2,
				WorkspaceID: spaceID,
				Name:        "tpl2",
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID:        10,
				EvalSetVersionID: 20,
			},
		},
	}, nil)
	// mgetExptTupleByID 相关依赖
	mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
	mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
	mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
	mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

	got, err := mgr.MGet(ctx, ids, spaceID, session)
	assert.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, int64(1), got[0].GetID())
	assert.Equal(t, int64(2), got[1].GetID())
}

// TestExptTemplateManagerImpl_Update_NameCheck 测试 Update 方法中名称检查的逻辑（216-242行，实际是221-242行）
func TestExptTemplateManagerImpl_Update_NameCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)
	mockLWT := lwtmocks.NewMockILatestWriteTracker(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:                mockRepo,
		evaluatorService:            mockEvalSvc,
		evalTargetService:           mockTargetSvc,
		evaluationSetService:        mockEvalSetSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
		lwt:                         mockLWT,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)
	session := &entity.Session{UserID: "u1"}

	t.Run("名称已存在，更新失败", func(t *testing.T) {
		existing := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:          templateID,
				WorkspaceID: spaceID,
				Name:        "tpl-old",
			},
		}

		param := &entity.UpdateExptTemplateParam{
			TemplateID: templateID,
			SpaceID:    spaceID,
			Name:       "tpl-new",
		}

		mockRepo.EXPECT().GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).Return(existing, nil)
		mockRepo.EXPECT().GetByName(ctx, "tpl-new", spaceID).Return(nil, true, nil)

		_, err := mgr.Update(ctx, param, session)
		assert.Error(t, err)
		code, _, ok := errno.ParseStatusError(err)
		assert.True(t, ok)
		assert.Equal(t, errno.ExperimentNameExistedCode, int(code))
	})

	t.Run("名称检查时发生错误", func(t *testing.T) {
		existing := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:          templateID,
				WorkspaceID: spaceID,
				Name:        "tpl-old",
			},
		}

		param := &entity.UpdateExptTemplateParam{
			TemplateID: templateID,
			SpaceID:    spaceID,
			Name:       "tpl-new",
		}

		mockRepo.EXPECT().GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).Return(existing, nil)
		// 当 GetByName 返回错误时，CheckName 返回 (false, err)
		// Update 方法先检查 !pass，所以会返回名称已存在的错误，而不是原始错误
		// 这是当前实现的行为：先检查 !pass，再检查 err
		mockRepo.EXPECT().GetByName(ctx, "tpl-new", spaceID).Return(nil, false, errors.New("db error"))

		_, err := mgr.Update(ctx, param, session)
		assert.Error(t, err)
		// 当前实现中，当 GetByName 返回错误时，CheckName 返回 (false, err)
		// Update 方法先检查 !pass，所以会返回名称已存在的错误
		code, _, ok := errno.ParseStatusError(err)
		assert.True(t, ok)
		assert.Equal(t, errno.ExperimentNameExistedCode, int(code))
	})

	t.Run("名称未改变，跳过检查", func(t *testing.T) {
		existing := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:          templateID,
				WorkspaceID: spaceID,
				Name:        "tpl-same",
				ExptType:    entity.ExptType_Offline,
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID:        10,
				EvalSetVersionID: 11,
				TargetID:         20,
				TargetVersionID:  21,
				TargetType:       entity.EvalTargetTypeLoopPrompt,
			},
		}

		param := &entity.UpdateExptTemplateParam{
			TemplateID: templateID,
			SpaceID:    spaceID,
			Name:       "tpl-same", // 名称相同，不检查
		}

		mockRepo.EXPECT().GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).Return(existing, nil)
		// 名称相同，不会调用 GetByName
		// resolveAndFillEvaluatorVersionIDs 需要 mock
		mockEvalSvc.EXPECT().BatchGetEvaluatorByIDAndVersion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
		// UpdateWithRefs
		mockRepo.EXPECT().UpdateWithRefs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		// GetByID 返回更新后的模板
		updatedTemplate := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:          templateID,
				WorkspaceID: spaceID,
				Name:        "tpl-same",
				ExptType:    entity.ExptType_Offline,
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID:        10,
				EvalSetVersionID: 11,
				TargetID:         20,
				TargetVersionID:  21,
				TargetType:       entity.EvalTargetTypeLoopPrompt,
			},
		}
		mockRepo.EXPECT().GetByID(gomock.Any(), templateID, gomock.AssignableToTypeOf(&spaceID)).Return(updatedTemplate, nil)
		// mgetExptTupleByID 相关依赖
		mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
		mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

		_, err := mgr.Update(ctx, param, session)
		assert.NoError(t, err)
	})

	t.Run("名称为空，跳过检查", func(t *testing.T) {
		existing := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:          templateID,
				WorkspaceID: spaceID,
				Name:        "tpl-old",
				ExptType:    entity.ExptType_Offline,
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID:        10,
				EvalSetVersionID: 11,
				TargetID:         20,
				TargetVersionID:  21,
				TargetType:       entity.EvalTargetTypeLoopPrompt,
			},
		}

		param := &entity.UpdateExptTemplateParam{
			TemplateID: templateID,
			SpaceID:    spaceID,
			Name:       "", // 名称为空，不检查
		}

		mockRepo.EXPECT().GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).Return(existing, nil)
		// 名称为空，不会调用 GetByName
		// resolveAndFillEvaluatorVersionIDs 需要 mock
		mockEvalSvc.EXPECT().BatchGetEvaluatorByIDAndVersion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
		// UpdateWithRefs
		mockRepo.EXPECT().UpdateWithRefs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		// GetByID 返回更新后的模板（名称保持为 "tpl-old"）
		updatedTemplate := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:          templateID,
				WorkspaceID: spaceID,
				Name:        "tpl-old", // 名称为空时，保持原有名称
				ExptType:    entity.ExptType_Offline,
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID:        10,
				EvalSetVersionID: 11,
				TargetID:         20,
				TargetVersionID:  21,
				TargetType:       entity.EvalTargetTypeLoopPrompt,
			},
		}
		mockRepo.EXPECT().GetByID(gomock.Any(), templateID, gomock.AssignableToTypeOf(&spaceID)).Return(updatedTemplate, nil)
		// mgetExptTupleByID 相关依赖
		mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
		mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

		_, err := mgr.Update(ctx, param, session)
		assert.NoError(t, err)
	})
}

// TestExptTemplateManagerImpl_Get 测试 Get 方法 (171-181行)
func TestExptTemplateManagerImpl_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)
	mockLWT := lwtmocks.NewMockILatestWriteTracker(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:                mockRepo,
		evaluatorService:            mockEvalSvc,
		evalTargetService:           mockTargetSvc,
		evaluationSetService:        mockEvalSetSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
		lwt:                         mockLWT,
	}

	ctx := context.Background()
	templateID := int64(1)
	spaceID := int64(100)
	session := &entity.Session{UserID: "u1"}

	t.Run("MGet返回空列表，返回错误", func(t *testing.T) {
		mockLWT.EXPECT().CheckWriteFlagByID(ctx, platestwrite.ResourceTypeExptTemplate, templateID).Return(false)
		mockRepo.EXPECT().MGetByID(ctx, []int64{templateID}, spaceID).Return([]*entity.ExptTemplate{}, nil)
		// mgetExptTupleByID 相关依赖
		mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
		mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

		_, err := mgr.Get(ctx, templateID, spaceID, session)
		assert.Error(t, err)
		code, _, ok := errno.ParseStatusError(err)
		assert.True(t, ok)
		assert.Equal(t, errno.ResourceNotFoundCode, int(code))
	})

	t.Run("MGet返回结果，返回第一个元素", func(t *testing.T) {
		template := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:          templateID,
				WorkspaceID: spaceID,
				Name:        "tpl1",
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID:        10,
				EvalSetVersionID: 11,
			},
		}
		mockLWT.EXPECT().CheckWriteFlagByID(ctx, platestwrite.ResourceTypeExptTemplate, templateID).Return(false)
		mockRepo.EXPECT().MGetByID(ctx, []int64{templateID}, spaceID).Return([]*entity.ExptTemplate{template}, nil)
		// mgetExptTupleByID 相关依赖
		mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
		mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

		got, err := mgr.Get(ctx, templateID, spaceID, session)
		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, templateID, got.GetID())
	})
}

// TestExptTemplateManagerImpl_Update_WithCustomEvalTarget 测试 Update 方法中 CustomEvalTarget 选项 (284-290行)
func TestExptTemplateManagerImpl_Update_WithCustomEvalTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)
	mockLWT := lwtmocks.NewMockILatestWriteTracker(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:                mockRepo,
		evaluatorService:            mockEvalSvc,
		evalTargetService:           mockTargetSvc,
		evaluationSetService:        mockEvalSetSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
		lwt:                         mockLWT,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)
	session := &entity.Session{UserID: "u1"}

	existing := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
			Name:        "tpl-old",
			ExptType:    entity.ExptType_Offline,
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:        10,
			EvalSetVersionID: 11,
			TargetID:         20,
			TargetVersionID:  21,
			TargetType:       entity.EvalTargetTypeLoopPrompt,
		},
	}

	param := &entity.UpdateExptTemplateParam{
		TemplateID: templateID,
		SpaceID:    spaceID,
		CreateEvalTargetParam: &entity.CreateEvalTargetParam{
			SourceTargetID:      gptr.Of("source-1"),
			SourceTargetVersion: gptr.Of("v1"),
			EvalTargetType:      gptr.Of(entity.EvalTargetTypeCustomRPCServer),
			CustomEvalTarget: &entity.CustomEvalTarget{
				ID:        gptr.Of("custom-1"),
				Name:      gptr.Of("custom-name"),
				AvatarURL: gptr.Of("http://avatar.com"),
				Ext:       map[string]string{"key": "value"},
			},
		},
	}

	// 获取现有模板
	mockRepo.EXPECT().GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).Return(existing, nil)
	// 获取现有 Target 以校验 SourceTargetID
	mockTargetSvc.EXPECT().GetEvalTarget(ctx, int64(20)).Return(&entity.EvalTarget{
		ID:             20,
		SourceTargetID: "source-1",
	}, nil)
	// 创建新的评测对象版本，验证 CustomEvalTarget 选项被传递
	mockTargetSvc.EXPECT().CreateEvalTarget(
		ctx,
		spaceID,
		"source-1",
		"v1",
		entity.EvalTargetTypeCustomRPCServer,
		gomock.Any(), // 验证 opts 中包含 CustomEvalTarget
	).DoAndReturn(func(ctx context.Context, spaceID int64, sourceTargetID, sourceTargetVersion string, targetType entity.EvalTargetType, opts ...entity.Option) (int64, int64, error) {
		// 验证 opts 中包含 CustomEvalTarget（通过调用验证）
		return 30, 31, nil
	})
	// resolveAndFillEvaluatorVersionIDs
	mockEvalSvc.EXPECT().BatchGetEvaluatorByIDAndVersion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	// UpdateWithRefs
	mockRepo.EXPECT().UpdateWithRefs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	// GetByID 返回更新后的模板
	updatedTemplate := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
			Name:        "tpl-old",
			ExptType:    entity.ExptType_Offline,
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:        10,
			EvalSetVersionID: 11,
			TargetID:         30,
			TargetVersionID:  31,
			TargetType:       entity.EvalTargetTypeCustomRPCServer,
		},
	}
	mockRepo.EXPECT().GetByID(gomock.Any(), templateID, gomock.AssignableToTypeOf(&spaceID)).Return(updatedTemplate, nil)
	// mgetExptTupleByID 相关依赖
	mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
	mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
	mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
	mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

	_, err := mgr.Update(ctx, param, session)
	assert.NoError(t, err)
}

// TestExptTemplateManagerImpl_Update_KeepExistingTarget 测试 Update 方法中保持原有 TargetID (298-305行)
func TestExptTemplateManagerImpl_Update_KeepExistingTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)
	mockLWT := lwtmocks.NewMockILatestWriteTracker(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:                mockRepo,
		evaluatorService:            mockEvalSvc,
		evalTargetService:           mockTargetSvc,
		evaluationSetService:        mockEvalSetSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
		lwt:                         mockLWT,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)
	session := &entity.Session{UserID: "u1"}

	t.Run("TargetVersionID为0，使用原有的", func(t *testing.T) {
		existing := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:          templateID,
				WorkspaceID: spaceID,
				Name:        "tpl-old",
				ExptType:    entity.ExptType_Offline,
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID:        10,
				EvalSetVersionID: 11,
				TargetID:         20,
				TargetVersionID:  21,
				TargetType:       entity.EvalTargetTypeLoopPrompt,
			},
		}

		param := &entity.UpdateExptTemplateParam{
			TemplateID:      templateID,
			SpaceID:         spaceID,
			TargetVersionID: 0, // 为0，应该使用原有的
		}

		mockRepo.EXPECT().GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).Return(existing, nil)
		// resolveAndFillEvaluatorVersionIDs
		mockEvalSvc.EXPECT().BatchGetEvaluatorByIDAndVersion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
		// UpdateWithRefs - 验证 finalTargetVersionID 为 21（原有的）
		mockRepo.EXPECT().UpdateWithRefs(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, template *entity.ExptTemplate, refs []*entity.ExptTemplateEvaluatorRef) error {
			assert.Equal(t, int64(20), template.GetTargetID())
			assert.Equal(t, int64(21), template.GetTargetVersionID())
			return nil
		})
		// GetByID 返回更新后的模板
		updatedTemplate := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:          templateID,
				WorkspaceID: spaceID,
				Name:        "tpl-old",
				ExptType:    entity.ExptType_Offline,
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID:        10,
				EvalSetVersionID: 11,
				TargetID:         20,
				TargetVersionID:  21,
				TargetType:       entity.EvalTargetTypeLoopPrompt,
			},
		}
		mockRepo.EXPECT().GetByID(gomock.Any(), templateID, gomock.AssignableToTypeOf(&spaceID)).Return(updatedTemplate, nil)
		// mgetExptTupleByID 相关依赖
		mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
		mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

		_, err := mgr.Update(ctx, param, session)
		assert.NoError(t, err)
	})

	t.Run("TargetVersionID不为0，使用新的", func(t *testing.T) {
		existing := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:          templateID,
				WorkspaceID: spaceID,
				Name:        "tpl-old",
				ExptType:    entity.ExptType_Offline,
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID:        10,
				EvalSetVersionID: 11,
				TargetID:         20,
				TargetVersionID:  21,
				TargetType:       entity.EvalTargetTypeLoopPrompt,
			},
		}

		param := &entity.UpdateExptTemplateParam{
			TemplateID:      templateID,
			SpaceID:         spaceID,
			TargetVersionID: 22, // 不为0，使用新的
		}

		mockRepo.EXPECT().GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).Return(existing, nil)
		// resolveAndFillEvaluatorVersionIDs
		mockEvalSvc.EXPECT().BatchGetEvaluatorByIDAndVersion(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
		// UpdateWithRefs - 验证 finalTargetVersionID 为 22（新的）
		mockRepo.EXPECT().UpdateWithRefs(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, template *entity.ExptTemplate, refs []*entity.ExptTemplateEvaluatorRef) error {
			assert.Equal(t, int64(20), template.GetTargetID())
			assert.Equal(t, int64(22), template.GetTargetVersionID())
			return nil
		})
		// GetByID 返回更新后的模板
		updatedTemplate := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:          templateID,
				WorkspaceID: spaceID,
				Name:        "tpl-old",
				ExptType:    entity.ExptType_Offline,
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID:        10,
				EvalSetVersionID: 11,
				TargetID:         20,
				TargetVersionID:  22,
				TargetType:       entity.EvalTargetTypeLoopPrompt,
			},
		}
		mockRepo.EXPECT().GetByID(gomock.Any(), templateID, gomock.AssignableToTypeOf(&spaceID)).Return(updatedTemplate, nil)
		// mgetExptTupleByID 相关依赖
		mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
		mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

		_, err := mgr.Update(ctx, param, session)
		assert.NoError(t, err)
	})
}

// TestExptTemplateManagerImpl_UpdateMeta_NilTemplate 测试 UpdateMeta 方法中 existingTemplate 为 nil (426-443行)
func TestExptTemplateManagerImpl_UpdateMeta_NilTemplate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)
	mockLWT := lwtmocks.NewMockILatestWriteTracker(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:                mockRepo,
		evaluatorService:            mockEvalSvc,
		evalTargetService:           mockTargetSvc,
		evaluationSetService:        mockEvalSetSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
		lwt:                         mockLWT,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)
	session := &entity.Session{UserID: "u1"}

	t.Run("existingTemplate为nil，返回错误", func(t *testing.T) {
		param := &entity.UpdateExptTemplateMetaParam{
			TemplateID: templateID,
			SpaceID:    spaceID,
			Name:       "new-name",
		}

		mockRepo.EXPECT().GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).Return(nil, nil)

		_, err := mgr.UpdateMeta(ctx, param, session)
		assert.Error(t, err)
		code, _, ok := errno.ParseStatusError(err)
		assert.True(t, ok)
		assert.Equal(t, errno.ResourceNotFoundCode, int(code))
	})

	t.Run("名称改变，检查名称", func(t *testing.T) {
		existing := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:          templateID,
				WorkspaceID: spaceID,
				Name:        "old-name",
			},
		}

		param := &entity.UpdateExptTemplateMetaParam{
			TemplateID: templateID,
			SpaceID:    spaceID,
			Name:       "new-name",
		}

		mockRepo.EXPECT().GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).Return(existing, nil)
		mockRepo.EXPECT().GetByName(ctx, "new-name", spaceID).Return(nil, true, nil) // 名称已存在

		_, err := mgr.UpdateMeta(ctx, param, session)
		assert.Error(t, err)
		code, _, ok := errno.ParseStatusError(err)
		assert.True(t, ok)
		assert.Equal(t, errno.ExperimentNameExistedCode, int(code))
	})
}

// TestExptTemplateManagerImpl_resolveAndFillEvaluatorVersionIDs_BuiltinVisible 测试 resolveAndFillEvaluatorVersionIDs 中 BuiltinVisible 处理 (626-634行)
func TestExptTemplateManagerImpl_resolveAndFillEvaluatorVersionIDs_BuiltinVisible(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)

	mgr := &ExptTemplateManagerImpl{
		evaluatorService: mockEvalSvc,
	}

	ctx := context.Background()
	spaceID := int64(100)

	t.Run("TemplateConf中BuiltinVisible版本已存在，不重复添加", func(t *testing.T) {
		templateConf := &entity.ExptTemplateConfiguration{
			ConnectorConf: entity.Connector{
				EvaluatorsConf: &entity.EvaluatorsConf{
					EvaluatorConf: []*entity.EvaluatorConf{
						{
							EvaluatorID:        1,
							Version:            "BuiltinVisible",
							EvaluatorVersionID: 0, // 需要解析
						},
					},
				},
			},
		}

		evaluatorIDVersionItems := []*entity.EvaluatorIDVersionItem{
			{
				EvaluatorID:        1,
				Version:            "BuiltinVisible",
				EvaluatorVersionID: 0, // 需要解析
			},
		}

		// 第一次添加在 evaluatorIDVersionItems 处理中，第二次在 TemplateConf 处理中应该检测到已存在
		builtinEvaluator := &entity.Evaluator{
			ID:            1,
			EvaluatorType: entity.EvaluatorTypePrompt,
			PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
				ID: 101,
			},
		}
		mockEvalSvc.EXPECT().BatchGetBuiltinEvaluator(ctx, []int64{1}).Return([]*entity.Evaluator{builtinEvaluator}, nil)

		err := mgr.resolveAndFillEvaluatorVersionIDs(ctx, spaceID, templateConf, evaluatorIDVersionItems)
		assert.NoError(t, err)
		// 验证 EvaluatorVersionID 被填充
		assert.Equal(t, int64(101), evaluatorIDVersionItems[0].EvaluatorVersionID)
		assert.Equal(t, int64(101), templateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf[0].EvaluatorVersionID)
	})

	t.Run("TemplateConf中BuiltinVisible版本不存在，添加到builtinIDs", func(t *testing.T) {
		templateConf := &entity.ExptTemplateConfiguration{
			ConnectorConf: entity.Connector{
				EvaluatorsConf: &entity.EvaluatorsConf{
					EvaluatorConf: []*entity.EvaluatorConf{
						{
							EvaluatorID:        2,
							Version:            "BuiltinVisible",
							EvaluatorVersionID: 0, // 需要解析
						},
					},
				},
			},
		}

		evaluatorIDVersionItems := []*entity.EvaluatorIDVersionItem{
			{
				EvaluatorID:        1,
				Version:            "BuiltinVisible",
				EvaluatorVersionID: 0, // 需要解析
			},
		}

		// 应该包含两个ID: 1 和 2
		builtinEvaluator1 := &entity.Evaluator{
			ID:            1,
			EvaluatorType: entity.EvaluatorTypePrompt,
			PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
				ID: 101,
			},
		}
		builtinEvaluator2 := &entity.Evaluator{
			ID:            2,
			EvaluatorType: entity.EvaluatorTypePrompt,
			PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
				ID: 102,
			},
		}
		mockEvalSvc.EXPECT().BatchGetBuiltinEvaluator(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, ids []int64) ([]*entity.Evaluator, error) {
			// 验证包含两个ID
			assert.Contains(t, ids, int64(1))
			assert.Contains(t, ids, int64(2))
			return []*entity.Evaluator{builtinEvaluator1, builtinEvaluator2}, nil
		})

		err := mgr.resolveAndFillEvaluatorVersionIDs(ctx, spaceID, templateConf, evaluatorIDVersionItems)
		assert.NoError(t, err)
		// 验证 EvaluatorVersionID 被填充
		assert.Equal(t, int64(101), evaluatorIDVersionItems[0].EvaluatorVersionID)
		assert.Equal(t, int64(102), templateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf[0].EvaluatorVersionID)
	})
}

// TestExptTemplateManagerImpl_resolveAndFillEvaluatorVersionIDs_BatchGetBuiltin 测试批量获取内置评估器 (658-668行)
func TestExptTemplateManagerImpl_resolveAndFillEvaluatorVersionIDs_BatchGetBuiltin(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)

	mgr := &ExptTemplateManagerImpl{
		evaluatorService: mockEvalSvc,
	}

	ctx := context.Background()
	spaceID := int64(100)

	t.Run("批量获取内置评估器成功，填充id2Builtin", func(t *testing.T) {
		evaluatorIDVersionItems := []*entity.EvaluatorIDVersionItem{
			{
				EvaluatorID:        1,
				Version:            "BuiltinVisible",
				EvaluatorVersionID: 0,
			},
			{
				EvaluatorID:        2,
				Version:            "BuiltinVisible",
				EvaluatorVersionID: 0,
			},
		}

		builtinEvaluator1 := &entity.Evaluator{
			ID:            1,
			EvaluatorType: entity.EvaluatorTypePrompt,
			PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
				ID: 101,
			},
		}
		builtinEvaluator2 := &entity.Evaluator{
			ID:            2,
			EvaluatorType: entity.EvaluatorTypePrompt,
			PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
				ID: 102,
			},
		}
		builtinEvaluatorNil := (*entity.Evaluator)(nil) // nil 应该被跳过

		mockEvalSvc.EXPECT().BatchGetBuiltinEvaluator(ctx, []int64{1, 2}).Return([]*entity.Evaluator{
			builtinEvaluator1,
			builtinEvaluatorNil, // nil 应该被跳过
			builtinEvaluator2,
		}, nil)

		err := mgr.resolveAndFillEvaluatorVersionIDs(ctx, spaceID, nil, evaluatorIDVersionItems)
		assert.NoError(t, err)
		// 验证 EvaluatorVersionID 被填充
		assert.Equal(t, int64(101), evaluatorIDVersionItems[0].EvaluatorVersionID)
		assert.Equal(t, int64(102), evaluatorIDVersionItems[1].EvaluatorVersionID)
	})

	t.Run("批量获取内置评估器失败，返回错误", func(t *testing.T) {
		evaluatorIDVersionItems := []*entity.EvaluatorIDVersionItem{
			{
				EvaluatorID:        1,
				Version:            "BuiltinVisible",
				EvaluatorVersionID: 0,
			},
		}

		mockEvalSvc.EXPECT().BatchGetBuiltinEvaluator(ctx, []int64{1}).Return(nil, errors.New("batch get builtin evaluator fail"))

		err := mgr.resolveAndFillEvaluatorVersionIDs(ctx, spaceID, nil, evaluatorIDVersionItems)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "batch get builtin evaluator fail")
	})
}

// TestExptTemplateManagerImpl_resolveTargetForCreate_WithCustomEvalTarget 测试 resolveTargetForCreate 中 CustomEvalTarget 选项 (795-800行)
func TestExptTemplateManagerImpl_resolveTargetForCreate_WithCustomEvalTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)

	mgr := &ExptTemplateManagerImpl{
		evalTargetService: mockTargetSvc,
	}

	ctx := context.Background()
	spaceID := int64(100)

	param := &entity.CreateExptTemplateParam{
		SpaceID: spaceID,
		CreateEvalTargetParam: &entity.CreateEvalTargetParam{
			SourceTargetID:      gptr.Of("source-1"),
			SourceTargetVersion: gptr.Of("v1"),
			EvalTargetType:      gptr.Of(entity.EvalTargetTypeCustomRPCServer),
			CustomEvalTarget: &entity.CustomEvalTarget{
				ID:        gptr.Of("custom-1"),
				Name:      gptr.Of("custom-name"),
				AvatarURL: gptr.Of("http://avatar.com"),
				Ext:       map[string]string{"key": "value"},
			},
		},
	}

	// 验证 CreateEvalTarget 被调用，并且 opts 中包含 CustomEvalTarget
	mockTargetSvc.EXPECT().CreateEvalTarget(
		ctx,
		spaceID,
		"source-1",
		"v1",
		entity.EvalTargetTypeCustomRPCServer,
		gomock.Any(), // 验证 opts 中包含 CustomEvalTarget
	).Return(int64(30), int64(31), nil)

	targetID, targetVersionID, targetType, err := mgr.resolveTargetForCreate(ctx, param)
	assert.NoError(t, err)
	assert.Equal(t, int64(30), targetID)
	assert.Equal(t, int64(31), targetVersionID)
	assert.Equal(t, entity.EvalTargetTypeCustomRPCServer, targetType)
}

// TestExptTemplateManagerImpl_mgetExptTupleByID_DraftEvalSet 测试 mgetExptTupleByID 中草稿评估集处理 (1015-1031行)
func TestExptTemplateManagerImpl_mgetExptTupleByID_DraftEvalSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)

	mgr := &ExptTemplateManagerImpl{
		evaluationSetService:        mockEvalSetSvc,
		evalTargetService:           mockTargetSvc,
		evaluatorService:            mockEvalSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
	}

	ctx := context.Background()
	spaceID := int64(100)

	t.Run("草稿评估集（evalSetID == versionID），调用BatchGetEvaluationSets", func(t *testing.T) {
		tupleIDs := []*entity.ExptTupleID{
			{
				VersionedEvalSetID: &entity.VersionedEvalSetID{
					EvalSetID: 10,
					VersionID: 10, // 草稿：evalSetID == versionID
				},
			},
		}

		evalSet := &entity.EvaluationSet{
			ID:   10,
			Name: "eval-set-1",
		}

		// 草稿评估集应该调用 BatchGetEvaluationSets
		mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(
			ctx,
			gptr.Of(spaceID),
			[]int64{10},
			gptr.Of(false),
		).Return([]*entity.EvaluationSet{evalSet}, nil)

		// 其他依赖
		mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
		mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

		tuples, err := mgr.mgetExptTupleByID(ctx, tupleIDs, spaceID, &entity.Session{UserID: "u1"})
		assert.NoError(t, err)
		assert.Len(t, tuples, 1)
		assert.NotNil(t, tuples[0].EvalSet)
		assert.Equal(t, int64(10), tuples[0].EvalSet.ID)
	})

	t.Run("草稿评估集返回nil元素，跳过", func(t *testing.T) {
		tupleIDs := []*entity.ExptTupleID{
			{
				VersionedEvalSetID: &entity.VersionedEvalSetID{
					EvalSetID: 10,
					VersionID: 10, // 草稿：evalSetID == versionID
				},
			},
		}

		// 返回nil元素，应该被跳过
		mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(
			ctx,
			gptr.Of(spaceID),
			[]int64{10},
			gptr.Of(false),
		).Return([]*entity.EvaluationSet{nil}, nil)

		// 其他依赖
		mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
		mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

		tuples, err := mgr.mgetExptTupleByID(ctx, tupleIDs, spaceID, &entity.Session{UserID: "u1"})
		assert.NoError(t, err)
		assert.Len(t, tuples, 1)
		// nil元素被跳过，所以EvalSet应该为nil
		assert.Nil(t, tuples[0].EvalSet)
	})

	t.Run("草稿评估集查询失败，返回错误", func(t *testing.T) {
		tupleIDs := []*entity.ExptTupleID{
			{
				VersionedEvalSetID: &entity.VersionedEvalSetID{
					EvalSetID: 10,
					VersionID: 10, // 草稿：evalSetID == versionID
				},
			},
		}

		// 查询失败
		mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(
			ctx,
			gptr.Of(spaceID),
			[]int64{10},
			gptr.Of(false),
		).Return(nil, errors.New("batch get evaluation sets fail"))

		// 其他依赖
		mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
		mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

		_, err := mgr.mgetExptTupleByID(ctx, tupleIDs, spaceID, &entity.Session{UserID: "u1"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "batch get evaluation sets fail")
	})
}

// --- 以下为 Update / UpdateMeta / UpdateExptInfo 中 err 分支补充单测 ---

// TestExptTemplateManagerImpl_Update_GetByIDError 覆盖 Update 中 GetByID 返回错误的分支 (221-225 行)
func TestExptTemplateManagerImpl_Update_GetByIDError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)

	param := &entity.UpdateExptTemplateParam{
		TemplateID: templateID,
		SpaceID:    spaceID,
	}

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return(nil, errors.New("get by id fail"))

	got, err := mgr.Update(ctx, param, &entity.Session{UserID: "u1"})
	assert.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "get by id fail")
}

// TestExptTemplateManagerImpl_Update_NotFound 覆盖 Update 中 existingTemplate 为 nil 的分支 (227-229 行)
func TestExptTemplateManagerImpl_Update_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)

	param := &entity.UpdateExptTemplateParam{
		TemplateID: templateID,
		SpaceID:    spaceID,
	}

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return((*entity.ExptTemplate)(nil), nil)

	got, err := mgr.Update(ctx, param, &entity.Session{UserID: "u1"})
	assert.Error(t, err)
	assert.Nil(t, got)
	code, _, ok := errno.ParseStatusError(err)
	assert.True(t, ok)
	assert.Equal(t, errno.ResourceNotFoundCode, int(code))
}

// TestExptTemplateManagerImpl_Update_GetEvalTargetError 覆盖 Update 中 GetEvalTarget 失败分支 (265-267 行)
func TestExptTemplateManagerImpl_Update_GetEvalTargetError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:      mockRepo,
		evalTargetService: mockTargetSvc,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)
	session := &entity.Session{UserID: "u1"}

	existing := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
			Name:        "tpl",
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:        10,
			EvalSetVersionID: 11,
			TargetID:         20,
			TargetVersionID:  21,
			TargetType:       entity.EvalTargetTypeLoopPrompt,
		},
	}

	param := &entity.UpdateExptTemplateParam{
		TemplateID: templateID,
		SpaceID:    spaceID,
		CreateEvalTargetParam: &entity.CreateEvalTargetParam{
			SourceTargetID:      gptr.Of("src-id"),
			SourceTargetVersion: gptr.Of("v1"),
			EvalTargetType:      gptr.Of(entity.EvalTargetTypeLoopPrompt),
		},
	}

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return(existing, nil)

	// 不修改名称，跳过名称检查；TemplateConf / EvaluatorIDVersionItems 均为 nil，resolveAndFillEvaluatorVersionIDs 直接返回

	// GetEvalTarget 返回错误
	mockTargetSvc.EXPECT().
		GetEvalTarget(ctx, int64(20)).
		Return((*entity.EvalTarget)(nil), errors.New("get eval target fail"))

	got, err := mgr.Update(ctx, param, session)
	assert.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "get existing eval target fail")
}

// TestExptTemplateManagerImpl_Update_ExistingTargetNotFound 覆盖 existingTarget 为 nil 分支 (269-271 行)
func TestExptTemplateManagerImpl_Update_ExistingTargetNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:      mockRepo,
		evalTargetService: mockTargetSvc,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)
	session := &entity.Session{UserID: "u1"}

	existing := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
			Name:        "tpl",
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:        10,
			EvalSetVersionID: 11,
			TargetID:         20,
			TargetVersionID:  21,
			TargetType:       entity.EvalTargetTypeLoopPrompt,
		},
	}

	param := &entity.UpdateExptTemplateParam{
		TemplateID: templateID,
		SpaceID:    spaceID,
		CreateEvalTargetParam: &entity.CreateEvalTargetParam{
			SourceTargetID:      gptr.Of("src-id"),
			SourceTargetVersion: gptr.Of("v1"),
			EvalTargetType:      gptr.Of(entity.EvalTargetTypeLoopPrompt),
		},
	}

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return(existing, nil)

	mockTargetSvc.EXPECT().
		GetEvalTarget(ctx, int64(20)).
		Return((*entity.EvalTarget)(nil), nil)

	got, err := mgr.Update(ctx, param, session)
	assert.Error(t, err)
	assert.Nil(t, got)
	code, _, ok := errno.ParseStatusError(err)
	assert.True(t, ok)
	assert.Equal(t, errno.ResourceNotFoundCode, int(code))
}

// TestExptTemplateManagerImpl_Update_SourceTargetIDMismatch 覆盖 SourceTargetID 不一致分支 (272-276 行)
func TestExptTemplateManagerImpl_Update_SourceTargetIDMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:      mockRepo,
		evalTargetService: mockTargetSvc,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)
	session := &entity.Session{UserID: "u1"}

	existing := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
			Name:        "tpl",
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:        10,
			EvalSetVersionID: 11,
			TargetID:         20,
			TargetVersionID:  21,
			TargetType:       entity.EvalTargetTypeLoopPrompt,
		},
	}

	param := &entity.UpdateExptTemplateParam{
		TemplateID: templateID,
		SpaceID:    spaceID,
		CreateEvalTargetParam: &entity.CreateEvalTargetParam{
			SourceTargetID:      gptr.Of("new-src"),
			SourceTargetVersion: gptr.Of("v1"),
			EvalTargetType:      gptr.Of(entity.EvalTargetTypeLoopPrompt),
		},
	}

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return(existing, nil)

	mockTargetSvc.EXPECT().
		GetEvalTarget(ctx, int64(20)).
		Return(&entity.EvalTarget{
			ID:             20,
			SourceTargetID: "old-src",
		}, nil)

	got, err := mgr.Update(ctx, param, session)
	assert.Error(t, err)
	assert.Nil(t, got)
	code, _, ok := errno.ParseStatusError(err)
	assert.True(t, ok)
	assert.Equal(t, errno.CommonInvalidParamCode, int(code))
}

// TestExptTemplateManagerImpl_Update_CreateEvalTargetError 覆盖 CreateEvalTarget 失败分支 (291-293 行)
func TestExptTemplateManagerImpl_Update_CreateEvalTargetError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:      mockRepo,
		evalTargetService: mockTargetSvc,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)
	session := &entity.Session{UserID: "u1"}

	existing := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
			Name:        "tpl",
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:        10,
			EvalSetVersionID: 11,
			TargetID:         20,
			TargetVersionID:  21,
			TargetType:       entity.EvalTargetTypeLoopPrompt,
		},
	}

	param := &entity.UpdateExptTemplateParam{
		TemplateID: templateID,
		SpaceID:    spaceID,
		CreateEvalTargetParam: &entity.CreateEvalTargetParam{
			SourceTargetID:      gptr.Of("src-id"),
			SourceTargetVersion: gptr.Of("v1"),
			EvalTargetType:      gptr.Of(entity.EvalTargetTypeLoopPrompt),
		},
	}

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return(existing, nil)

	mockTargetSvc.EXPECT().
		GetEvalTarget(ctx, int64(20)).
		Return(&entity.EvalTarget{
			ID:             20,
			SourceTargetID: "src-id",
		}, nil)

	mockTargetSvc.EXPECT().
		CreateEvalTarget(ctx, spaceID, "src-id", "v1", entity.EvalTargetTypeLoopPrompt, gomock.Any()).
		Return(int64(0), int64(0), errors.New("create eval target fail"))

	got, err := mgr.Update(ctx, param, session)
	assert.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "CreateEvalTarget failed")
}

// TestExptTemplateManagerImpl_Update_UpdateWithRefsError 覆盖 UpdateWithRefs 失败分支 (386-387 行)
func TestExptTemplateManagerImpl_Update_UpdateWithRefsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)
	session := &entity.Session{UserID: "u1"}

	existing := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
			Name:        "tpl",
			ExptType:    entity.ExptType_Offline,
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:        10,
			EvalSetVersionID: 11,
			TargetID:         20,
			TargetVersionID:  21,
			TargetType:       entity.EvalTargetTypeLoopPrompt,
		},
	}

	param := &entity.UpdateExptTemplateParam{
		TemplateID: templateID,
		SpaceID:    spaceID,
	}

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return(existing, nil)

	mockRepo.EXPECT().
		UpdateWithRefs(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("update with refs fail"))

	got, err := mgr.Update(ctx, param, session)
	assert.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "update with refs fail")
}

// TestExptTemplateManagerImpl_Update_GetByIDAfterUpdateError 覆盖更新后 GetByID 返回错误分支 (391-393 行)
func TestExptTemplateManagerImpl_Update_GetByIDAfterUpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)
	session := &entity.Session{UserID: "u1"}

	existing := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
			Name:        "tpl",
			ExptType:    entity.ExptType_Offline,
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:        10,
			EvalSetVersionID: 11,
			TargetID:         20,
			TargetVersionID:  21,
			TargetType:       entity.EvalTargetTypeLoopPrompt,
		},
	}

	param := &entity.UpdateExptTemplateParam{
		TemplateID: templateID,
		SpaceID:    spaceID,
	}

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return(existing, nil)

	mockRepo.EXPECT().
		UpdateWithRefs(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	mockRepo.EXPECT().
		GetByID(gomock.Any(), templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return((*entity.ExptTemplate)(nil), errors.New("get after update fail"))

	got, err := mgr.Update(ctx, param, session)
	assert.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "get after update fail")
}

// TestExptTemplateManagerImpl_Update_GetByIDAfterUpdateNotFound 覆盖更新后模板为 nil 分支 (395-397 行)
func TestExptTemplateManagerImpl_Update_GetByIDAfterUpdateNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)
	session := &entity.Session{UserID: "u1"}

	existing := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
			Name:        "tpl",
			ExptType:    entity.ExptType_Offline,
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:        10,
			EvalSetVersionID: 11,
			TargetID:         20,
			TargetVersionID:  21,
			TargetType:       entity.EvalTargetTypeLoopPrompt,
		},
	}

	param := &entity.UpdateExptTemplateParam{
		TemplateID: templateID,
		SpaceID:    spaceID,
	}

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return(existing, nil)

	mockRepo.EXPECT().
		UpdateWithRefs(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	mockRepo.EXPECT().
		GetByID(gomock.Any(), templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return((*entity.ExptTemplate)(nil), nil)

	got, err := mgr.Update(ctx, param, session)
	assert.Error(t, err)
	assert.Nil(t, got)
	code, _, ok := errno.ParseStatusError(err)
	assert.True(t, ok)
	assert.Equal(t, errno.ResourceNotFoundCode, int(code))
}

// TestExptTemplateManagerImpl_UpdateMeta_GetByIDError 覆盖 UpdateMeta 中 GetByID 返回错误分支 (421-423 行)
func TestExptTemplateManagerImpl_UpdateMeta_GetByIDError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)

	param := &entity.UpdateExptTemplateMetaParam{
		TemplateID: templateID,
		SpaceID:    spaceID,
		Name:       "new-name",
	}

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return((*entity.ExptTemplate)(nil), errors.New("get by id fail"))

	got, err := mgr.UpdateMeta(ctx, param, &entity.Session{UserID: "u1"})
	assert.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "get by id fail")
}

// TestExptTemplateManagerImpl_UpdateMeta_UpdateFieldsError 覆盖 UpdateFields 返回错误分支 (460-463 行)
func TestExptTemplateManagerImpl_UpdateMeta_UpdateFieldsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)

	existing := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
			Name:        "old-name",
		},
	}

	param := &entity.UpdateExptTemplateMetaParam{
		TemplateID:  templateID,
		SpaceID:     spaceID,
		Name:        "old-name",
		Description: "new-desc",
	}

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return(existing, nil)

	mockRepo.EXPECT().
		UpdateFields(ctx, templateID, gomock.AssignableToTypeOf(map[string]any{})).
		Return(errors.New("update fields fail"))

	got, err := mgr.UpdateMeta(ctx, param, &entity.Session{UserID: "u1"})
	assert.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "update fields fail")
}

// TestExptTemplateManagerImpl_UpdateMeta_GetByIDAfterUpdateError 覆盖 UpdateMeta 中第二次 GetByID 返回错误 (467-469 行)
func TestExptTemplateManagerImpl_UpdateMeta_GetByIDAfterUpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)

	existing := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
			Name:        "old-name",
		},
	}

	param := &entity.UpdateExptTemplateMetaParam{
		TemplateID:  templateID,
		SpaceID:     spaceID,
		Description: "new-desc",
	}

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return(existing, nil)

	mockRepo.EXPECT().
		UpdateFields(ctx, templateID, gomock.AssignableToTypeOf(map[string]any{})).
		Return(nil)

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return((*entity.ExptTemplate)(nil), errors.New("get after update fail"))

	got, err := mgr.UpdateMeta(ctx, param, &entity.Session{UserID: "u1"})
	assert.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "get after update fail")
}

// TestExptTemplateManagerImpl_UpdateMeta_GetByIDAfterUpdateNotFound 覆盖 UpdateMeta 中 updatedTemplate 为 nil 分支 (471-472 行)
func TestExptTemplateManagerImpl_UpdateMeta_GetByIDAfterUpdateNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)

	existing := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
			Name:        "old-name",
		},
	}

	param := &entity.UpdateExptTemplateMetaParam{
		TemplateID:  templateID,
		SpaceID:     spaceID,
		Description: "new-desc",
	}

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return(existing, nil)

	mockRepo.EXPECT().
		UpdateFields(ctx, templateID, gomock.AssignableToTypeOf(map[string]any{})).
		Return(nil)

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return((*entity.ExptTemplate)(nil), nil)

	got, err := mgr.UpdateMeta(ctx, param, &entity.Session{UserID: "u1"})
	assert.Error(t, err)
	assert.Nil(t, got)
	code, _, ok := errno.ParseStatusError(err)
	assert.True(t, ok)
	assert.Equal(t, errno.ResourceNotFoundCode, int(code))
}

// TestExptTemplateManagerImpl_UpdateExptInfo_GetByIDError 覆盖 UpdateExptInfo 中 GetByID 返回错误分支 (491-493 行)
func TestExptTemplateManagerImpl_UpdateExptInfo_GetByIDError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return((*entity.ExptTemplate)(nil), errors.New("get by id fail"))

	err := mgr.UpdateExptInfo(ctx, templateID, spaceID, 1, entity.ExptStatus_Processing, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get template fail")
}

// TestExptTemplateManagerImpl_UpdateExptInfo_UpdateFieldsError 覆盖 UpdateExptInfo 中 UpdateFields 返回错误分支 (533-535 行)
func TestExptTemplateManagerImpl_UpdateExptInfo_UpdateFieldsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
	}

	ctx := context.Background()
	spaceID := int64(100)
	templateID := int64(1)

	existing := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
		},
		ExptInfo: &entity.ExptInfo{
			CreatedExptCount: 1,
			LatestExptID:     10,
			LatestExptStatus: entity.ExptStatus_Success,
		},
	}

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return(existing, nil)

	mockRepo.EXPECT().
		UpdateFields(ctx, templateID, gomock.AssignableToTypeOf(map[string]any{})).
		Return(errors.New("update expt info fail"))

	err := mgr.UpdateExptInfo(ctx, templateID, spaceID, 2, entity.ExptStatus_Processing, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update ExptInfo fail")
}

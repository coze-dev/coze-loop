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
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	platestwrite "github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	lwtmocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	observability_common "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	observability_dataset "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	taskfilter "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	taskdomain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
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
	mockTaskRPCAdapter := mocks.NewMockITaskRPCAdapter(ctrl)
	mockPipelineRPCAdapter := mocks.NewMockIPipelineListAdapter(ctrl)
	mockExptRepo := repo_mocks.NewMockIExperimentRepo(ctrl)

	mgr := NewExptTemplateManager(
		mockRepo,
		mockIdgen,
		mockEvalSvc,
		mockTargetSvc,
		mockEvalSetSvc,
		mockEvalSetVerSvc,
		mockLWT,
		mockTaskRPCAdapter,
		mockPipelineRPCAdapter,
		mockExptRepo,
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

	err := mgr.UpdateExptInfo(ctx, templateID, spaceID, 200, entity.ExptStatus_Processing, 1, nil)
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

	err = mgr.UpdateExptInfo(ctx, templateID, spaceID, 300, entity.ExptStatus_Failed, -5, nil)
	assert.NoError(t, err)

	// 场景三：传入 latestExptStartTime，验证字段被正确更新
	latestStartTime := int64(1700000000000) // 毫秒时间戳
	existing3 := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: spaceID,
		},
		ExptInfo: &entity.ExptInfo{
			CreatedExptCount:    1,
			LatestExptID:        100,
			LatestExptStatus:    entity.ExptStatus_Pending,
			LatestExptStartTime: 0,
		},
	}

	gomock.InOrder(
		mockRepo.EXPECT().
			GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
			Return(existing3, nil),
		mockRepo.EXPECT().
			UpdateFields(ctx, templateID, gomock.AssignableToTypeOf(map[string]any{})).
			DoAndReturn(func(_ context.Context, _ int64, fields map[string]any) error {
				buf, ok := fields["expt_info"].([]byte)
				assert.True(t, ok)
				var info entity.ExptInfo
				err := json.Unmarshal(buf, &info)
				assert.NoError(t, err)
				assert.Equal(t, latestStartTime, info.LatestExptStartTime)
				return nil
			}),
	)

	err = mgr.UpdateExptInfo(ctx, templateID, spaceID, 400, entity.ExptStatus_Pending, 1, &latestStartTime)
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

	err := mgr.UpdateExptInfo(ctx, templateID, spaceID, 1, entity.ExptStatus_Processing, 1, nil)
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
					ev := &entity.Evaluator{
						ID:                     1,
						EvaluatorType:          entity.EvaluatorTypePrompt,
						PromptEvaluatorVersion: pev,
					}
					ev.SetSpaceID(spaceID) // 设置 SpaceID 以通过 workspace 校验
					evaluators = append(evaluators, ev)
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

// TestExptTemplateManagerImpl_Update_PreservesExptSourceInTemplateConf 增量更新只改 template_conf 部分字段时须保留原有 expt_source
func TestExptTemplateManagerImpl_Update_PreservesExptSourceInTemplateConf(t *testing.T) {
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
			Name:        "tpl",
			Desc:        "d",
			ExptType:    entity.ExptType_Offline,
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:               10,
			EvalSetVersionID:        11,
			TargetID:                20,
			TargetVersionID:         21,
			TargetType:              entity.EvalTargetTypeLoopPrompt,
			EvaluatorVersionIds:     []int64{101},
			EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{{EvaluatorID: 1, Version: "v1", EvaluatorVersionID: 101}},
		},
		TemplateConf: &entity.ExptTemplateConfiguration{
			ItemConcurNum: gptr.Of(3),
			ExptSource: &entity.ExptSource{
				SourceType: entity.SourceType_Workflow,
				SourceID:   "42",
			},
		},
	}

	newItemConcur := 4
	param := &entity.UpdateExptTemplateParam{
		TemplateID: templateID,
		SpaceID:    spaceID,
		EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{
			{EvaluatorID: 1, Version: "v1", EvaluatorVersionID: 101},
		},
		TemplateConf: &entity.ExptTemplateConfiguration{
			ItemConcurNum: &newItemConcur,
		},
	}

	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return(existing, nil)

	mockRepo.EXPECT().
		UpdateWithRefs(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, tpl *entity.ExptTemplate, _ []*entity.ExptTemplateEvaluatorRef) error {
			assert.NotNil(t, tpl.TemplateConf)
			assert.NotNil(t, tpl.TemplateConf.ExptSource)
			assert.Equal(t, entity.SourceType_Workflow, tpl.TemplateConf.ExptSource.SourceType)
			assert.Equal(t, "42", tpl.TemplateConf.ExptSource.SourceID)
			assert.Equal(t, newItemConcur, gptr.Indirect(tpl.TemplateConf.ItemConcurNum))
			return nil
		})

	updatedFromDB := &entity.ExptTemplate{
		Meta: existing.Meta,
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:               10,
			EvalSetVersionID:        11,
			TargetID:                20,
			TargetVersionID:         21,
			TargetType:              entity.EvalTargetTypeLoopPrompt,
			EvaluatorVersionIds:     []int64{101},
			EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{{EvaluatorID: 1, Version: "v1", EvaluatorVersionID: 101}},
		},
		EvaluatorVersionRef: []*entity.ExptTemplateEvaluatorVersionRef{{EvaluatorID: 1, EvaluatorVersionID: 101}},
		TemplateConf: &entity.ExptTemplateConfiguration{
			ItemConcurNum: &newItemConcur,
			ExptSource: &entity.ExptSource{
				SourceType: entity.SourceType_Workflow,
				SourceID:   "42",
			},
		},
	}
	mockRepo.EXPECT().
		GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).
		Return(updatedFromDB, nil)

	mockTargetSvc.EXPECT().
		BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).
		DoAndReturn(func(_ context.Context, _ int64, versionIDs []int64, _ bool) ([]*entity.EvalTarget, error) {
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
	assert.Equal(t, "42", got.TemplateConf.ExptSource.SourceID)
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
	ev.SetSpaceID(spaceID) // 设置 SpaceID 以通过 workspace 校验

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

	err := mgr.UpdateExptInfo(ctx, templateID, spaceID, 1, entity.ExptStatus_Processing, 1, nil)
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

	err := mgr.UpdateExptInfo(ctx, templateID, spaceID, 2, entity.ExptStatus_Processing, 1, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update ExptInfo fail")
}

func TestExptTemplateManagerImpl_matchesTemplateFilter(t *testing.T) {
	mgr := &ExptTemplateManagerImpl{}

	t.Run("filters为nil，返回true", func(t *testing.T) {
		result := mgr.matchesTemplateFilter(&entity.ExptTemplate{}, nil)
		assert.True(t, result)
	})

	t.Run("CreatedBy匹配", func(t *testing.T) {
		template := &entity.ExptTemplate{
			BaseInfo: &entity.BaseInfo{
				CreatedBy: &entity.UserInfo{UserID: gptr.Of("user1")},
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				CreatedBy: []string{"user1", "user2"},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.True(t, result)
	})

	t.Run("CreatedBy不匹配", func(t *testing.T) {
		template := &entity.ExptTemplate{
			BaseInfo: &entity.BaseInfo{
				CreatedBy: &entity.UserInfo{UserID: gptr.Of("user3")},
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				CreatedBy: []string{"user1", "user2"},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.False(t, result)
	})

	t.Run("UpdatedBy匹配", func(t *testing.T) {
		template := &entity.ExptTemplate{
			BaseInfo: &entity.BaseInfo{
				UpdatedBy: &entity.UserInfo{UserID: gptr.Of("user1")},
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				UpdatedBy: []string{"user1", "user2"},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.True(t, result)
	})

	t.Run("UpdatedBy不匹配", func(t *testing.T) {
		template := &entity.ExptTemplate{
			BaseInfo: &entity.BaseInfo{
				UpdatedBy: &entity.UserInfo{UserID: gptr.Of("user3")},
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				UpdatedBy: []string{"user1", "user2"},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.False(t, result)
	})

	t.Run("EvalSetIDs匹配", func(t *testing.T) {
		template := &entity.ExptTemplate{
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID: 100,
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				EvalSetIDs: []int64{100, 200},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.True(t, result)
	})

	t.Run("EvalSetIDs不匹配", func(t *testing.T) {
		template := &entity.ExptTemplate{
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID: 300,
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				EvalSetIDs: []int64{100, 200},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.False(t, result)
	})

	t.Run("TargetIDs匹配", func(t *testing.T) {
		template := &entity.ExptTemplate{
			TripleConfig: &entity.ExptTemplateTuple{
				TargetID: 100,
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				TargetIDs: []int64{100, 200},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.True(t, result)
	})

	t.Run("TargetIDs不匹配", func(t *testing.T) {
		template := &entity.ExptTemplate{
			TripleConfig: &entity.ExptTemplateTuple{
				TargetID: 300,
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				TargetIDs: []int64{100, 200},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.False(t, result)
	})

	t.Run("TargetType匹配", func(t *testing.T) {
		template := &entity.ExptTemplate{
			TripleConfig: &entity.ExptTemplateTuple{
				TargetType: entity.EvalTargetTypeLoopPrompt,
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				TargetType: []int64{int64(entity.EvalTargetTypeLoopPrompt), int64(entity.EvalTargetTypeCustomRPCServer)},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.True(t, result)
	})

	t.Run("TargetType不匹配", func(t *testing.T) {
		template := &entity.ExptTemplate{
			TripleConfig: &entity.ExptTemplateTuple{
				TargetType: entity.EvalTargetTypeCozeBot,
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				TargetType: []int64{int64(entity.EvalTargetTypeLoopPrompt), int64(entity.EvalTargetTypeCustomRPCServer)},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.False(t, result)
	})

	t.Run("ExptType匹配", func(t *testing.T) {
		template := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ExptType: entity.ExptType_Online,
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				ExptType: []int64{int64(entity.ExptType_Online), int64(entity.ExptType_Offline)},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.True(t, result)
	})

	t.Run("ExptType不匹配", func(t *testing.T) {
		template := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ExptType: entity.ExptType_Offline,
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				ExptType: []int64{int64(entity.ExptType_Online)},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.False(t, result)
	})

	t.Run("FuzzyName匹配", func(t *testing.T) {
		template := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				Name: "TestTemplate123",
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Includes:  &entity.ExptTemplateFilterFields{},
			FuzzyName: "template",
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.True(t, result)
	})

	t.Run("FuzzyName不匹配", func(t *testing.T) {
		template := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				Name: "TestTemplate123",
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Includes:  &entity.ExptTemplateFilterFields{},
			FuzzyName: "invalid",
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.False(t, result)
	})

	t.Run("Excludes-CreatedBy匹配被排除", func(t *testing.T) {
		template := &entity.ExptTemplate{
			BaseInfo: &entity.BaseInfo{
				CreatedBy: &entity.UserInfo{UserID: gptr.Of("user1")},
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Excludes: &entity.ExptTemplateFilterFields{
				CreatedBy: []string{"user1", "user2"},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.False(t, result)
	})

	t.Run("Excludes-CreatedBy不匹配不被排除", func(t *testing.T) {
		template := &entity.ExptTemplate{
			BaseInfo: &entity.BaseInfo{
				CreatedBy: &entity.UserInfo{UserID: gptr.Of("user3")},
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Excludes: &entity.ExptTemplateFilterFields{
				CreatedBy: []string{"user1", "user2"},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.True(t, result)
	})

	t.Run("Excludes-EvalSetIDs匹配被排除", func(t *testing.T) {
		template := &entity.ExptTemplate{
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID: 100,
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Excludes: &entity.ExptTemplateFilterFields{
				EvalSetIDs: []int64{100, 200},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.False(t, result)
	})

	t.Run("Excludes-TargetIDs匹配被排除", func(t *testing.T) {
		template := &entity.ExptTemplate{
			TripleConfig: &entity.ExptTemplateTuple{
				TargetID: 100,
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Excludes: &entity.ExptTemplateFilterFields{
				TargetIDs: []int64{100, 200},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.False(t, result)
	})

	t.Run("Excludes-TargetType匹配被排除", func(t *testing.T) {
		template := &entity.ExptTemplate{
			TripleConfig: &entity.ExptTemplateTuple{
				TargetType: entity.EvalTargetTypeLoopPrompt,
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Excludes: &entity.ExptTemplateFilterFields{
				TargetType: []int64{int64(entity.EvalTargetTypeLoopPrompt)},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.False(t, result)
	})

	t.Run("Excludes-ExptType匹配被排除", func(t *testing.T) {
		template := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ExptType: entity.ExptType_Online,
			},
		}
		filters := &entity.ExptTemplateListFilter{
			Excludes: &entity.ExptTemplateFilterFields{
				ExptType: []int64{int64(entity.ExptType_Online)},
			},
		}
		result := mgr.matchesTemplateFilter(template, filters)
		assert.False(t, result)
	})
}

func TestExptTemplateManagerImpl_applyTemplateFilters(t *testing.T) {
	mgr := &ExptTemplateManagerImpl{}

	templates := []*entity.ExptTemplate{
		{
			Meta: &entity.ExptTemplateMeta{
				ID:   1,
				Name: "template1",
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID: 100,
			},
		},
		{
			Meta: &entity.ExptTemplateMeta{
				ID:   2,
				Name: "template2",
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID: 200,
			},
		},
		{
			Meta: &entity.ExptTemplateMeta{
				ID:   3,
				Name: "template3",
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID: 300,
			},
		},
	}

	t.Run("filters为nil，返回所有", func(t *testing.T) {
		result := mgr.applyTemplateFilters(templates, nil)
		assert.Len(t, result, 3)
	})

	t.Run("应用EvalSetIDs筛选", func(t *testing.T) {
		filters := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				EvalSetIDs: []int64{100, 300},
			},
		}
		result := mgr.applyTemplateFilters(templates, filters)
		assert.Len(t, result, 2)
		ids := []int64{}
		for _, t := range result {
			ids = append(ids, t.GetID())
		}
		assert.ElementsMatch(t, []int64{1, 3}, ids)
	})
}

func TestExptTemplateManagerImpl_applyTemplateOrderBy(t *testing.T) {
	mgr := &ExptTemplateManagerImpl{}

	templates := []*entity.ExptTemplate{
		{
			Meta: &entity.ExptTemplateMeta{
				ID: 1,
			},
			BaseInfo: &entity.BaseInfo{
				UpdatedAt: gptr.Of(int64(100)),
				CreatedAt: gptr.Of(int64(10)),
			},
		},
		{
			Meta: &entity.ExptTemplateMeta{
				ID: 2,
			},
			BaseInfo: &entity.BaseInfo{
				UpdatedAt: gptr.Of(int64(200)),
				CreatedAt: gptr.Of(int64(20)),
			},
		},
		{
			Meta: &entity.ExptTemplateMeta{
				ID: 3,
			},
			BaseInfo: &entity.BaseInfo{
				UpdatedAt: gptr.Of(int64(150)),
				CreatedAt: gptr.Of(int64(5)),
			},
		},
	}

	t.Run("orderBys为空，不排序", func(t *testing.T) {
		copyTemplates := make([]*entity.ExptTemplate, len(templates))
		copy(copyTemplates, templates)
		mgr.applyTemplateOrderBy(copyTemplates, nil)
		assert.Equal(t, int64(1), copyTemplates[0].GetID())
		assert.Equal(t, int64(2), copyTemplates[1].GetID())
		assert.Equal(t, int64(3), copyTemplates[2].GetID())
	})

	t.Run("按UpdatedAt升序", func(t *testing.T) {
		copyTemplates := make([]*entity.ExptTemplate, len(templates))
		copy(copyTemplates, templates)
		mgr.applyTemplateOrderBy(copyTemplates, []*entity.OrderBy{
			{
				Field: gptr.Of(entity.OrderByUpdatedAt),
				IsAsc: gptr.Of(true),
			},
		})
		assert.Equal(t, int64(1), copyTemplates[0].GetID())
		assert.Equal(t, int64(3), copyTemplates[1].GetID())
		assert.Equal(t, int64(2), copyTemplates[2].GetID())
	})

	t.Run("按UpdatedAt降序", func(t *testing.T) {
		copyTemplates := make([]*entity.ExptTemplate, len(templates))
		copy(copyTemplates, templates)
		mgr.applyTemplateOrderBy(copyTemplates, []*entity.OrderBy{
			{
				Field: gptr.Of(entity.OrderByUpdatedAt),
				IsAsc: gptr.Of(false),
			},
		})
		assert.Equal(t, int64(2), copyTemplates[0].GetID())
		assert.Equal(t, int64(3), copyTemplates[1].GetID())
		assert.Equal(t, int64(1), copyTemplates[2].GetID())
	})

	t.Run("按CreatedAt升序", func(t *testing.T) {
		copyTemplates := make([]*entity.ExptTemplate, len(templates))
		copy(copyTemplates, templates)
		mgr.applyTemplateOrderBy(copyTemplates, []*entity.OrderBy{
			{
				Field: gptr.Of(entity.OrderByCreatedAt),
				IsAsc: gptr.Of(true),
			},
		})
		assert.Equal(t, int64(3), copyTemplates[0].GetID())
		assert.Equal(t, int64(1), copyTemplates[1].GetID())
		assert.Equal(t, int64(2), copyTemplates[2].GetID())
	})

	t.Run("按CreatedAt降序", func(t *testing.T) {
		copyTemplates := make([]*entity.ExptTemplate, len(templates))
		copy(copyTemplates, templates)
		mgr.applyTemplateOrderBy(copyTemplates, []*entity.OrderBy{
			{
				Field: gptr.Of(entity.OrderByCreatedAt),
				IsAsc: gptr.Of(false),
			},
		})
		assert.Equal(t, int64(2), copyTemplates[0].GetID())
		assert.Equal(t, int64(1), copyTemplates[1].GetID())
		assert.Equal(t, int64(3), copyTemplates[2].GetID())
	})
}

func Test_taskToExptTemplate(t *testing.T) {
	t.Run("task为nil返回nil", func(t *testing.T) {
		result := taskToExptTemplate(nil, 100)
		assert.Nil(t, result)
	})

	t.Run("task.ID为nil返回nil", func(t *testing.T) {
		task := &taskdomain.Task{}
		result := taskToExptTemplate(task, 100)
		assert.Nil(t, result)
	})

	t.Run("正常转换-基本场景", func(t *testing.T) {
		taskID := int64(12345)
		task := &taskdomain.Task{
			ID:   &taskID,
			Name: "test-task",
		}
		result := taskToExptTemplate(task, 100)
		assert.NotNil(t, result)
		assert.Equal(t, -taskID, result.GetID())
		assert.Equal(t, int64(100), result.GetSpaceID())
		assert.Equal(t, "test-task", result.GetName())
		assert.Equal(t, entity.ExptType_Online, result.GetExptType())
		assert.Equal(t, entity.SourceType_AutoTask, result.ExptSource.SourceType)
		assert.Equal(t, "12345", result.ExptSource.SourceID)
	})

	t.Run("正常转换-带BaseInfo", func(t *testing.T) {
		taskID := int64(12345)
		createdAt := int64(1234567890000)
		updatedAt := int64(1234567891000)
		userID := "test-user"
		task := &taskdomain.Task{
			ID:   &taskID,
			Name: "test-task",
			BaseInfo: &observability_common.BaseInfo{
				CreatedAt: &createdAt,
				UpdatedAt: &updatedAt,
				CreatedBy: &observability_common.UserInfo{
					UserID: &userID,
				},
				UpdatedBy: &observability_common.UserInfo{
					UserID: &userID,
				},
			},
		}
		result := taskToExptTemplate(task, 100)
		assert.NotNil(t, result)
		assert.NotNil(t, result.BaseInfo)
		assert.Equal(t, createdAt, *result.BaseInfo.CreatedAt)
		assert.Equal(t, updatedAt, *result.BaseInfo.UpdatedAt)
		assert.Equal(t, userID, *result.BaseInfo.CreatedBy.UserID)
		assert.Equal(t, userID, *result.BaseInfo.UpdatedBy.UserID)
	})

	t.Run("正常转换-带Rule", func(t *testing.T) {
		taskID := int64(12345)
		task := &taskdomain.Task{
			ID:   &taskID,
			Name: "test-task",
			Rule: &taskdomain.Rule{
				Sampler: &taskdomain.Sampler{
					IsCycle: gptr.Of(true),
				},
			},
		}
		result := taskToExptTemplate(task, 100)
		assert.NotNil(t, result)
		assert.NotNil(t, result.ExptSource.Scheduler)
		assert.NotNil(t, result.ExptSource.Sampler)
		assert.NotNil(t, result.ExptSource.Sampler.IsCycle)
		assert.True(t, *result.ExptSource.Sampler.IsCycle)
	})

	t.Run("正常转换-带TaskConfig", func(t *testing.T) {
		taskID := int64(12345)
		task := &taskdomain.Task{
			ID:   &taskID,
			Name: "test-task",
			TaskConfig: &taskdomain.TaskConfig{
				AutoEvaluateConfigs: []*taskdomain.AutoEvaluateConfig{
					{
						EvaluatorID:        1,
						EvaluatorVersionID: 1001,
					},
				},
			},
		}
		result := taskToExptTemplate(task, 100)
		assert.NotNil(t, result)
		assert.NotNil(t, result.TripleConfig)
		assert.NotNil(t, result.TemplateConf)
		assert.NotNil(t, result.TemplateConf.ConnectorConf.EvaluatorsConf)
		assert.Len(t, result.TemplateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf, 1)
	})
}

func Test_autoEvaluateConfigsToExptTemplateConf(t *testing.T) {
	t.Run("taskConfig为nil返回nil", func(t *testing.T) {
		triple, connector := autoEvaluateConfigsToExptTemplateConf(nil)
		assert.Nil(t, triple)
		assert.NotNil(t, connector)
	})

	t.Run("AutoEvaluateConfigs为空返回nil", func(t *testing.T) {
		taskConfig := &taskdomain.TaskConfig{}
		triple, connector := autoEvaluateConfigsToExptTemplateConf(taskConfig)
		assert.Nil(t, triple)
		assert.NotNil(t, connector)
	})

	t.Run("AutoEvaluateConfigs包含nil元素", func(t *testing.T) {
		taskConfig := &taskdomain.TaskConfig{
			AutoEvaluateConfigs: []*taskdomain.AutoEvaluateConfig{
				nil,
				{
					EvaluatorID:        1,
					EvaluatorVersionID: 1001,
				},
			},
		}
		triple, connector := autoEvaluateConfigsToExptTemplateConf(taskConfig)
		assert.NotNil(t, triple)
		assert.NotNil(t, connector)
		assert.NotNil(t, connector.EvaluatorsConf)
		assert.Len(t, connector.EvaluatorsConf.EvaluatorConf, 1)
	})

	t.Run("AutoEvaluateConfigs包含无效元素", func(t *testing.T) {
		taskConfig := &taskdomain.TaskConfig{
			AutoEvaluateConfigs: []*taskdomain.AutoEvaluateConfig{
				{
					EvaluatorID:        1,
					EvaluatorVersionID: 0, // 无效的版本ID
				},
				{
					EvaluatorID:        0, // 无效的评估器ID
					EvaluatorVersionID: 0, // 无效的版本ID
				},
			},
		}
		triple, connector := autoEvaluateConfigsToExptTemplateConf(taskConfig)
		assert.Nil(t, triple)
		assert.NotNil(t, connector)
		assert.Nil(t, connector.EvaluatorsConf)
	})

	t.Run("正常转换-多个评估器", func(t *testing.T) {
		taskConfig := &taskdomain.TaskConfig{
			AutoEvaluateConfigs: []*taskdomain.AutoEvaluateConfig{
				{
					EvaluatorID:        1,
					EvaluatorVersionID: 1001,
					FieldMappings: []*taskdomain.EvaluateFieldMapping{
						{
							FieldSchema: &observability_dataset.FieldSchema{
								Key: gptr.Of("field1"),
							},
							TraceFieldKey: "trace_field1",
						},
					},
				},
				{
					EvaluatorID:        2,
					EvaluatorVersionID: 1002,
					FieldMappings: []*taskdomain.EvaluateFieldMapping{
						{
							FieldSchema: &observability_dataset.FieldSchema{
								Name: gptr.Of("field2"),
							},
							EvalSetName: gptr.Of("eval_set_field"),
						},
					},
				},
			},
		}
		triple, connector := autoEvaluateConfigsToExptTemplateConf(taskConfig)
		assert.NotNil(t, triple)
		assert.NotNil(t, connector)
		assert.NotNil(t, connector.EvaluatorsConf)
		assert.Len(t, connector.EvaluatorsConf.EvaluatorConf, 2)
		assert.Len(t, triple.EvaluatorIDVersionItems, 2)
		assert.Len(t, triple.EvaluatorVersionIds, 2)
	})
}

func Test_extractEvaluatorVersionIDs(t *testing.T) {
	t.Run("正常提取", func(t *testing.T) {
		items := []*entity.EvaluatorIDVersionItem{
			{EvaluatorVersionID: 1001},
			{EvaluatorVersionID: 1002},
			nil,
			{EvaluatorVersionID: 0},
		}
		result := extractEvaluatorVersionIDs(items)
		assert.Len(t, result, 2)
		assert.ElementsMatch(t, []int64{1001, 1002}, result)
	})
}

func Test_evaluateFieldMappingsToIngressConf(t *testing.T) {
	t.Run("空mappings返回空配置", func(t *testing.T) {
		result := evaluateFieldMappingsToIngressConf(nil)
		assert.NotNil(t, result)
		assert.NotNil(t, result.EvalSetAdapter)
		assert.Empty(t, result.EvalSetAdapter.FieldConfs)
	})

	t.Run("mappings为空返回空配置", func(t *testing.T) {
		result := evaluateFieldMappingsToIngressConf([]*taskdomain.EvaluateFieldMapping{})
		assert.NotNil(t, result)
		assert.NotNil(t, result.EvalSetAdapter)
		assert.Empty(t, result.EvalSetAdapter.FieldConfs)
	})

	t.Run("mappings包含nil元素", func(t *testing.T) {
		mappings := []*taskdomain.EvaluateFieldMapping{
			nil,
			{
				FieldSchema: &observability_dataset.FieldSchema{
					Key: gptr.Of("test_field"),
				},
				TraceFieldKey: "trace_field",
			},
		}
		result := evaluateFieldMappingsToIngressConf(mappings)
		assert.NotNil(t, result)
		assert.NotNil(t, result.EvalSetAdapter)
		assert.Len(t, result.EvalSetAdapter.FieldConfs, 1)
	})

	t.Run("mappings包含无效元素", func(t *testing.T) {
		mappings := []*taskdomain.EvaluateFieldMapping{
			{
				FieldSchema:   &observability_dataset.FieldSchema{}, // 无Key和Name
				TraceFieldKey: "trace_field",
			},
			{
				FieldSchema: &observability_dataset.FieldSchema{
					Key: gptr.Of(""), // 空Key
				},
				TraceFieldKey: "trace_field",
			},
		}
		result := evaluateFieldMappingsToIngressConf(mappings)
		assert.NotNil(t, result)
		assert.NotNil(t, result.EvalSetAdapter)
		assert.Empty(t, result.EvalSetAdapter.FieldConfs)
	})

	t.Run("正常转换-TraceFieldKey", func(t *testing.T) {
		mappings := []*taskdomain.EvaluateFieldMapping{
			{
				FieldSchema: &observability_dataset.FieldSchema{
					Key: gptr.Of("field1"),
				},
				TraceFieldKey: "trace_field1",
			},
		}
		result := evaluateFieldMappingsToIngressConf(mappings)
		assert.NotNil(t, result)
		assert.NotNil(t, result.EvalSetAdapter)
		assert.Len(t, result.EvalSetAdapter.FieldConfs, 1)
		assert.Equal(t, "field1", result.EvalSetAdapter.FieldConfs[0].FieldName)
		assert.Equal(t, "trace_field1", result.EvalSetAdapter.FieldConfs[0].FromField)
	})

	t.Run("正常转换-EvalSetName", func(t *testing.T) {
		mappings := []*taskdomain.EvaluateFieldMapping{
			{
				FieldSchema: &observability_dataset.FieldSchema{
					Name: gptr.Of("field2"),
				},
				EvalSetName: gptr.Of("eval_set_field"),
			},
		}
		result := evaluateFieldMappingsToIngressConf(mappings)
		assert.NotNil(t, result)
		assert.NotNil(t, result.EvalSetAdapter)
		assert.Len(t, result.EvalSetAdapter.FieldConfs, 1)
		assert.Equal(t, "field2", result.EvalSetAdapter.FieldConfs[0].FieldName)
		assert.Equal(t, "eval_set_field", result.EvalSetAdapter.FieldConfs[0].FromField)
	})

	t.Run("正常转换-多个字段", func(t *testing.T) {
		mappings := []*taskdomain.EvaluateFieldMapping{
			{
				FieldSchema: &observability_dataset.FieldSchema{
					Key: gptr.Of("field1"),
				},
				TraceFieldKey: "trace_field1",
			},
			{
				FieldSchema: &observability_dataset.FieldSchema{
					Name: gptr.Of("field2"),
				},
				EvalSetName: gptr.Of("eval_set_field"),
			},
		}
		result := evaluateFieldMappingsToIngressConf(mappings)
		assert.NotNil(t, result)
		assert.NotNil(t, result.EvalSetAdapter)
		assert.Len(t, result.EvalSetAdapter.FieldConfs, 2)
	})
}

func Test_taskRuleToExptScheduler(t *testing.T) {
	t.Run("rule为nil返回nil", func(t *testing.T) {
		result := taskRuleToExptScheduler(nil)
		assert.Nil(t, result)
	})

	t.Run("sampler和effectiveTime都为nil返回nil", func(t *testing.T) {
		rule := &taskdomain.Rule{}
		result := taskRuleToExptScheduler(rule)
		assert.Nil(t, result)
	})

	t.Run("只有sampler", func(t *testing.T) {
		rule := &taskdomain.Rule{
			Sampler: &taskdomain.Sampler{
				IsCycle: gptr.Of(true),
			},
		}
		result := taskRuleToExptScheduler(rule)
		assert.NotNil(t, result)
		assert.True(t, *result.Enabled)
	})

	t.Run("只有effectiveTime", func(t *testing.T) {
		startAt := int64(1234567890000)
		endAt := int64(1234567891000)
		rule := &taskdomain.Rule{
			EffectiveTime: &taskdomain.EffectiveTime{
				StartAt: &startAt,
				EndAt:   &endAt,
			},
		}
		result := taskRuleToExptScheduler(rule)
		assert.NotNil(t, result)
		assert.Equal(t, startAt, *result.StartTime)
		assert.Equal(t, endAt, *result.EndTime)
	})

	t.Run("sampler和effectiveTime都有", func(t *testing.T) {
		startAt := int64(1234567890000)
		rule := &taskdomain.Rule{
			Sampler: &taskdomain.Sampler{
				IsCycle:       gptr.Of(true),
				CycleTimeUnit: gptr.Of(taskdomain.TimeUnitDay),
			},
			EffectiveTime: &taskdomain.EffectiveTime{
				StartAt: &startAt,
			},
		}
		result := taskRuleToExptScheduler(rule)
		assert.NotNil(t, result)
		assert.True(t, *result.Enabled)
		assert.Equal(t, startAt, *result.StartTime)
	})

	t.Run("sampler.IsCycle为false", func(t *testing.T) {
		rule := &taskdomain.Rule{
			Sampler: &taskdomain.Sampler{
				IsCycle: gptr.Of(false),
			},
		}
		result := taskRuleToExptScheduler(rule)
		assert.NotNil(t, result)
		assert.False(t, *result.Enabled)
	})
}

func Test_convertTaskFrequency(t *testing.T) {
	t.Run("sampler为nil返回nil", func(t *testing.T) {
		result := convertTaskFrequency(nil, nil)
		assert.Nil(t, result)
	})

	t.Run("IsCycle为nil返回nil", func(t *testing.T) {
		sampler := &taskdomain.Sampler{}
		result := convertTaskFrequency(sampler, nil)
		assert.Nil(t, result)
	})

	t.Run("IsCycle为false返回nil", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle: gptr.Of(false),
		}
		result := convertTaskFrequency(sampler, nil)
		assert.Nil(t, result)
	})

	t.Run("TimeUnit为Day返回every_day", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitDay),
		}
		result := convertTaskFrequency(sampler, nil)
		assert.NotNil(t, result)
		assert.Equal(t, "every_day", *result)
	})

	t.Run("TimeUnit为Null返回every_day", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitNull),
		}
		result := convertTaskFrequency(sampler, nil)
		assert.NotNil(t, result)
		assert.Equal(t, "every_day", *result)
	})

	t.Run("TimeUnit为空返回every_day", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle: gptr.Of(true),
		}
		result := convertTaskFrequency(sampler, nil)
		assert.NotNil(t, result)
		assert.Equal(t, "every_day", *result)
	})

	t.Run("TimeUnit为Week", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitWeek),
		}
		// 测试周一
		startAt := int64(1704067200000) // 2024-01-01 00:00:00 UTC (周一)
		effectiveTime := &taskdomain.EffectiveTime{
			StartAt: &startAt,
		}
		result := convertTaskFrequency(sampler, effectiveTime)
		assert.NotNil(t, result)
		assert.Equal(t, "monday", *result)

		// 测试周日
		startAt = int64(1704585600000) // 2024-01-07 00:00:00 UTC (周日)
		effectiveTime = &taskdomain.EffectiveTime{
			StartAt: &startAt,
		}
		result = convertTaskFrequency(sampler, effectiveTime)
		assert.NotNil(t, result)
		assert.Equal(t, "sunday", *result)
	})

	t.Run("TimeUnit为Week但effectiveTime为nil返回nil", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitWeek),
		}
		result := convertTaskFrequency(sampler, nil)
		assert.Nil(t, result)
	})

	t.Run("TimeUnit为Week但StartAt为0返回nil", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitWeek),
		}
		startAt := int64(0)
		effectiveTime := &taskdomain.EffectiveTime{
			StartAt: &startAt,
		}
		result := convertTaskFrequency(sampler, effectiveTime)
		assert.Nil(t, result)
	})

	t.Run("TimeUnit为其他值返回nil", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of("invalid"),
		}
		result := convertTaskFrequency(sampler, nil)
		assert.Nil(t, result)
	})
}

func Test_spanFilterFieldsFromTaskRule(t *testing.T) {
	t.Run("sf为nil返回nil", func(t *testing.T) {
		result := spanFilterFieldsFromTaskRule(nil)
		assert.Nil(t, result)
	})

	t.Run("正常转换-只有基本字段", func(t *testing.T) {
		platformType := observability_common.PlatformTypeCozeloop
		spanListType := observability_common.SpanListTypeRootSpan
		sf := &taskfilter.SpanFilterFields{
			PlatformType: &platformType,
			SpanListType: &spanListType,
		}
		result := spanFilterFieldsFromTaskRule(sf)
		assert.NotNil(t, result)
		assert.Equal(t, platformType, *result.PlatformType)
		assert.Equal(t, spanListType, *result.SpanListType)
	})

	t.Run("正常转换-包含过滤器", func(t *testing.T) {
		platformType := observability_common.PlatformTypeCozeloop
		spanListType := observability_common.SpanListTypeRootSpan
		queryAndOr := taskfilter.QueryRelationAnd
		fieldType := taskfilter.FieldTypeString
		queryType := taskfilter.QueryTypeEq
		subQueryAndOr := taskfilter.QueryRelationOr
		sf := &taskfilter.SpanFilterFields{
			PlatformType: &platformType,
			SpanListType: &spanListType,
			Filters: &taskfilter.FilterFields{
				QueryAndOr: &queryAndOr,
				FilterFields: []*taskfilter.FilterField{
					{
						FieldName:  gptr.Of("field1"),
						FieldType:  &fieldType,
						Values:     []string{"value1", "value2"},
						QueryType:  &queryType,
						QueryAndOr: &queryAndOr,
						SubFilter: &taskfilter.FilterFields{
							QueryAndOr: &subQueryAndOr,
							FilterFields: []*taskfilter.FilterField{
								{
									FieldName: gptr.Of("sub_field"),
									Values:    []string{"sub_value"},
								},
							},
						},
					},
				},
			},
		}
		result := spanFilterFieldsFromTaskRule(sf)
		assert.NotNil(t, result)
		assert.Equal(t, platformType, *result.PlatformType)
		assert.Equal(t, spanListType, *result.SpanListType)
		assert.NotNil(t, result.Filters)
		assert.Equal(t, queryAndOr, *result.Filters.QueryAndOr)
		assert.Len(t, result.Filters.FilterFields, 1)
	})
}

func Test_filterFieldsFromTaskRule(t *testing.T) {
	t.Run("ff为nil返回nil", func(t *testing.T) {
		result := filterFieldsFromTaskRule(nil)
		assert.Nil(t, result)
	})

	t.Run("正常转换-只有QueryAndOr", func(t *testing.T) {
		queryAndOr := taskfilter.QueryRelationAnd
		ff := &taskfilter.FilterFields{
			QueryAndOr: &queryAndOr,
		}
		result := filterFieldsFromTaskRule(ff)
		assert.NotNil(t, result)
		assert.Equal(t, queryAndOr, *result.QueryAndOr)
	})

	t.Run("正常转换-包含FilterFields", func(t *testing.T) {
		queryAndOr := taskfilter.QueryRelationAnd
		fieldType := taskfilter.FieldTypeString
		queryType := taskfilter.QueryTypeEq
		ff := &taskfilter.FilterFields{
			QueryAndOr: &queryAndOr,
			FilterFields: []*taskfilter.FilterField{
				{
					FieldName: gptr.Of("field1"),
					FieldType: &fieldType,
					Values:    []string{"value1"},
					QueryType: &queryType,
				},
				nil, // 测试nil元素
				{
					FieldName: gptr.Of("field2"),
					Values:    []string{"value2"},
				},
			},
		}
		result := filterFieldsFromTaskRule(ff)
		assert.NotNil(t, result)
		assert.Equal(t, queryAndOr, *result.QueryAndOr)
		assert.Len(t, result.FilterFields, 2)
	})
}

func Test_filterFieldFromTaskRule(t *testing.T) {
	t.Run("f为nil返回nil", func(t *testing.T) {
		result := filterFieldFromTaskRule(nil)
		assert.Nil(t, result)
	})

	t.Run("正常转换-基本字段", func(t *testing.T) {
		fieldType := taskfilter.FieldTypeString
		queryType := taskfilter.QueryTypeEq
		queryAndOr := taskfilter.QueryRelationAnd
		f := &taskfilter.FilterField{
			FieldName:  gptr.Of("field1"),
			FieldType:  &fieldType,
			Values:     []string{"value1", "value2"},
			QueryType:  &queryType,
			QueryAndOr: &queryAndOr,
		}
		result := filterFieldFromTaskRule(f)
		assert.NotNil(t, result)
		assert.Equal(t, "field1", *result.FieldName)
		assert.Equal(t, fieldType, *result.FieldType)
		assert.Equal(t, []string{"value1", "value2"}, result.Values)
		assert.Equal(t, queryType, *result.QueryType)
		assert.Equal(t, queryAndOr, *result.QueryAndOr)
	})

	t.Run("正常转换-包含SubFilter", func(t *testing.T) {
		fieldType := taskfilter.FieldTypeString
		queryType := taskfilter.QueryTypeEq
		subQueryAndOr := taskfilter.QueryRelationOr
		f := &taskfilter.FilterField{
			FieldName: gptr.Of("field1"),
			FieldType: &fieldType,
			Values:    []string{"value1"},
			QueryType: &queryType,
			SubFilter: &taskfilter.FilterFields{
				QueryAndOr: &subQueryAndOr,
			},
		}
		result := filterFieldFromTaskRule(f)
		assert.NotNil(t, result)
		assert.Equal(t, "field1", *result.FieldName)
		assert.NotNil(t, result.SubFilter)
		assert.Equal(t, subQueryAndOr, *result.SubFilter.QueryAndOr)
	})
}

func Test_extractSpanFilterFieldsFromPipeline(t *testing.T) {
	t.Run("p为nil返回nil", func(t *testing.T) {
		result := extractSpanFilterFieldsFromPipeline(nil)
		assert.Nil(t, result)
	})

	t.Run("Flow为nil返回nil", func(t *testing.T) {
		p := &entity.Pipeline{}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.Nil(t, result)
	})

	t.Run("Flow.Nodes为空返回nil", func(t *testing.T) {
		p := &entity.Pipeline{
			Flow: &entity.FlowSchema{},
		}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.Nil(t, result)
	})

	t.Run("Nodes中无data_reflow节点返回nil", func(t *testing.T) {
		p := &entity.Pipeline{
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{
						NodeTemplateType: "other_type",
					},
				},
			},
		}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.Nil(t, result)
	})

	t.Run("data_reflow节点无Refs返回nil", func(t *testing.T) {
		p := &entity.Pipeline{
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{
						NodeTemplateType: "data_reflow",
					},
				},
			},
		}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.Nil(t, result)
	})

	t.Run("data_reflow节点无task Ref返回nil", func(t *testing.T) {
		p := &entity.Pipeline{
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{
						NodeTemplateType: "data_reflow",
						Refs: map[string]*entity.NodeRef{
							"other": {Content: "{}"},
						},
					},
				},
			},
		}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.Nil(t, result)
	})

	t.Run("data_reflow节点task Ref为空返回nil", func(t *testing.T) {
		p := &entity.Pipeline{
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{
						NodeTemplateType: "data_reflow",
						Refs: map[string]*entity.NodeRef{
							"task": {Content: ""},
						},
					},
				},
			},
		}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.Nil(t, result)
	})

	t.Run("正常提取", func(t *testing.T) {
		taskJSON := `{
			"rule": {
				"span_filters": {
					"platform_type": "TCE",
					"span_list_type": "normal",
					"filters": {
						"query_and_or": "and",
						"filter_fields": [
							{
								"field_name": "field1",
								"values": ["value1"]
							}
						]
					}
				}
			}
		}`
		p := &entity.Pipeline{
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{
						NodeTemplateType: "data_reflow",
						Refs: map[string]*entity.NodeRef{
							"task": {Content: taskJSON},
						},
					},
				},
			},
		}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.NotNil(t, result)
		assert.Equal(t, "TCE", *result.PlatformType)
		assert.Equal(t, "normal", *result.SpanListType)
	})
}

func Test_extractSchedulerFromPipeline(t *testing.T) {
	t.Run("p为nil返回nil", func(t *testing.T) {
		result := extractSchedulerFromPipeline(nil)
		assert.Nil(t, result)
	})

	t.Run("Scheduler为nil返回nil", func(t *testing.T) {
		p := &entity.Pipeline{}
		result := extractSchedulerFromPipeline(p)
		assert.Nil(t, result)
	})

	t.Run("正常提取", func(t *testing.T) {
		enabled := true
		p := &entity.Pipeline{
			Scheduler: &entity.Scheduler{
				Enabled: &enabled,
			},
		}
		result := extractSchedulerFromPipeline(p)
		assert.NotNil(t, result)
		assert.True(t, *result.Enabled)
	})
}

func Test_parseSpanFilterFieldsFromTaskJSON(t *testing.T) {
	t.Run("空content返回nil", func(t *testing.T) {
		result := parseSpanFilterFieldsFromTaskJSON("")
		assert.Nil(t, result)
	})

	t.Run("invalid JSON返回nil", func(t *testing.T) {
		result := parseSpanFilterFieldsFromTaskJSON("invalid json")
		assert.Nil(t, result)
	})

	t.Run("valid JSON但无rule返回nil", func(t *testing.T) {
		result := parseSpanFilterFieldsFromTaskJSON(`{"other":"field"}`)
		assert.Nil(t, result)
	})

	t.Run("无span_filters返回nil", func(t *testing.T) {
		result := parseSpanFilterFieldsFromTaskJSON(`{"rule": {}}`)
		assert.Nil(t, result)
	})

	t.Run("正常解析", func(t *testing.T) {
		jsonContent := `{
			"rule": {
				"span_filters": {
					"platform_type": "TCE",
					"span_list_type": "normal",
					"filters": {
						"query_and_or": "and",
						"filter_fields": [
							{
								"field_name": "field1",
								"field_type": "string",
								"values": ["value1", "value2"],
								"query_type": "eq",
								"query_and_or": "and",
								"sub_filter": {
									"query_and_or": "or"
								}
							}
						]
					}
				}
			}
		}`
		result := parseSpanFilterFieldsFromTaskJSON(jsonContent)
		assert.NotNil(t, result)
		assert.Equal(t, "TCE", *result.PlatformType)
		assert.Equal(t, "normal", *result.SpanListType)
		assert.NotNil(t, result.Filters)
		assert.Equal(t, "and", *result.Filters.QueryAndOr)
		assert.Len(t, result.Filters.FilterFields, 1)
	})

	t.Run("只包含基本字段", func(t *testing.T) {
		jsonContent := `{
			"rule": {
				"span_filters": {
					"platform_type": "TCE",
					"span_list_type": "normal"
				}
			}
		}`
		result := parseSpanFilterFieldsFromTaskJSON(jsonContent)
		assert.NotNil(t, result)
		assert.Equal(t, "TCE", *result.PlatformType)
		assert.Equal(t, "normal", *result.SpanListType)
		assert.Nil(t, result.Filters)
	})
}

func Test_parseDataReflowTaskJSON_sampler(t *testing.T) {
	t.Run("仅sampler", func(t *testing.T) {
		content := `{"rule":{"sampler":{"sample_rate":0.1,"sample_size":100,"is_cycle":false}}}`
		r := parseDataReflowTaskJSON(content)
		assert.NotNil(t, r)
		assert.Nil(t, r.SpanFilterFields)
		assert.NotNil(t, r.Sampler)
		assert.InDelta(t, 0.1, *r.Sampler.SampleRate, 1e-9)
		assert.NotNil(t, r.Sampler.SampleSize)
		assert.Equal(t, int64(100), *r.Sampler.SampleSize)
		assert.NotNil(t, r.Sampler.IsCycle)
		assert.False(t, *r.Sampler.IsCycle)
		assert.Nil(t, parseSpanFilterFieldsFromTaskJSON(content))
	})
}

func TestExptTemplateManagerImpl_enrichExptSourceFromPipeline(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPipelineAdapter := mocks.NewMockIPipelineListAdapter(ctrl)
	mgr := &ExptTemplateManagerImpl{
		pipelineRPCAdapter: mockPipelineAdapter,
	}

	ctx := context.Background()
	spaceID := int64(100)

	t.Run("pipelineRPCAdapter为nil，直接返回", func(t *testing.T) {
		mgrNoAdapter := &ExptTemplateManagerImpl{}
		err := mgrNoAdapter.enrichExptSourceFromPipeline(ctx, nil, spaceID)
		assert.NoError(t, err)
	})

	t.Run("templates为空，直接返回", func(t *testing.T) {
		err := mgr.enrichExptSourceFromPipeline(ctx, []*entity.ExptTemplate{}, spaceID)
		assert.NoError(t, err)
	})

	t.Run("没有需要查询的pipeline，直接返回", func(t *testing.T) {
		templates := []*entity.ExptTemplate{
			{ExptSource: nil},
			{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Evaluation}},
			// AutoTask 不再触发 ListPipeline，仅 Workflow + 合法 pipeline id 会查询
			{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_AutoTask, SourceID: "123"}},
			{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: ""}},
			{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "invalid"}},
			{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "0"}},
		}
		err := mgr.enrichExptSourceFromPipeline(ctx, templates, spaceID)
		assert.NoError(t, err)
	})

	t.Run("ListPipelineFlow失败，返回错误", func(t *testing.T) {
		templates := []*entity.ExptTemplate{
			{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "1"}},
		}
		mockPipelineAdapter.EXPECT().ListPipelineFlow(ctx, gomock.Any()).Return(nil, errors.New("rpc error"))
		err := mgr.enrichExptSourceFromPipeline(ctx, templates, spaceID)
		assert.Error(t, err)
	})

	t.Run("ListPipelineFlow返回nil或空，直接返回", func(t *testing.T) {
		templates := []*entity.ExptTemplate{
			{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "1"}},
		}
		mockPipelineAdapter.EXPECT().ListPipelineFlow(ctx, gomock.Any()).Return(nil, nil)
		err := mgr.enrichExptSourceFromPipeline(ctx, templates, spaceID)
		assert.NoError(t, err)

		mockPipelineAdapter.EXPECT().ListPipelineFlow(ctx, gomock.Any()).Return(&rpc.ListPipelineFlowResponse{Items: []*entity.Pipeline{}}, nil)
		err = mgr.enrichExptSourceFromPipeline(ctx, templates, spaceID)
		assert.NoError(t, err)
	})

	t.Run("pipeline ID为nil，跳过处理", func(t *testing.T) {
		templates := []*entity.ExptTemplate{
			{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "1"}},
		}
		pipeline := &entity.Pipeline{
			ID: nil,
		}
		mockPipelineAdapter.EXPECT().ListPipelineFlow(ctx, gomock.Any()).Return(&rpc.ListPipelineFlowResponse{Items: []*entity.Pipeline{pipeline}}, nil)
		err := mgr.enrichExptSourceFromPipeline(ctx, templates, spaceID)
		assert.NoError(t, err)
		assert.Nil(t, templates[0].ExptSource.SpanFilterFields)
		assert.Nil(t, templates[0].ExptSource.Sampler)
		assert.Nil(t, templates[0].ExptSource.Scheduler)
	})

	t.Run("成功处理，填充span filter和scheduler", func(t *testing.T) {
		template1 := &entity.ExptTemplate{
			ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "1"},
		}
		template2 := &entity.ExptTemplate{
			ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "1"},
		}
		templates := []*entity.ExptTemplate{template1, template2}

		taskContent := `{
			"rule": {
				"span_filters": {
					"span_list_type": "all",
					"platform_type": "web",
					"filters": {
						"query_and_or": "and",
						"filter_fields": [
							{
								"field_name": "user_id",
								"field_type": "string",
								"values": ["123"],
								"query_type": "eq",
								"query_and_or": "and"
							}
						]
					}
				},
				"sampler": {
					"sample_rate": 0.5,
					"is_cycle": true,
					"cycle_time_unit": "day"
				}
			}
		}`

		pipeline := &entity.Pipeline{
			ID: gptr.Of(int64(1)),
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{
						NodeTemplateType: "data_reflow",
						Refs: map[string]*entity.NodeRef{
							"task": {
								Content: taskContent,
							},
						},
					},
				},
			},
			Scheduler: &entity.Scheduler{
				Enabled:   gptr.Of(true),
				Frequency: gptr.Of("daily"),
				TriggerAt: gptr.Of(int64(0)),
				StartTime: gptr.Of(int64(1700000000000)),
				EndTime:   gptr.Of(int64(1700000000000)),
			},
		}

		mockPipelineAdapter.EXPECT().ListPipelineFlow(ctx, gomock.Any()).Return(&rpc.ListPipelineFlowResponse{Items: []*entity.Pipeline{pipeline}}, nil)
		err := mgr.enrichExptSourceFromPipeline(ctx, templates, spaceID)
		assert.NoError(t, err)

		assert.NotNil(t, template1.ExptSource.SpanFilterFields)
		assert.NotNil(t, template1.ExptSource.Scheduler)
		assert.NotNil(t, template2.ExptSource.SpanFilterFields)
		assert.NotNil(t, template2.ExptSource.Scheduler)
		assert.Equal(t, "all", *template1.ExptSource.SpanFilterFields.SpanListType)
		assert.Equal(t, "web", *template1.ExptSource.SpanFilterFields.PlatformType)
		assert.True(t, *template1.ExptSource.Scheduler.Enabled)
		assert.Equal(t, "daily", *template1.ExptSource.Scheduler.Frequency)
		assert.NotNil(t, template1.ExptSource.Sampler)
		assert.NotNil(t, template1.ExptSource.Sampler.SampleRate)
		assert.InDelta(t, 0.5, *template1.ExptSource.Sampler.SampleRate, 1e-9)
		assert.NotNil(t, template1.ExptSource.Sampler.IsCycle)
		assert.True(t, *template1.ExptSource.Sampler.IsCycle)
		assert.NotNil(t, template1.ExptSource.Sampler.CycleTimeUnit)
		assert.Equal(t, "day", *template1.ExptSource.Sampler.CycleTimeUnit)
	})

	t.Run("pipeline没有data_reflow节点，不填充数据", func(t *testing.T) {
		template := &entity.ExptTemplate{
			ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "1"},
		}
		templates := []*entity.ExptTemplate{template}

		pipeline := &entity.Pipeline{
			ID: gptr.Of(int64(1)),
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{
						NodeTemplateType: "other_type",
					},
				},
			},
		}

		mockPipelineAdapter.EXPECT().ListPipelineFlow(ctx, gomock.Any()).Return(&rpc.ListPipelineFlowResponse{Items: []*entity.Pipeline{pipeline}}, nil)
		err := mgr.enrichExptSourceFromPipeline(ctx, templates, spaceID)
		assert.NoError(t, err)

		assert.Nil(t, template.ExptSource.SpanFilterFields)
		assert.Nil(t, template.ExptSource.Scheduler)
	})

	t.Run("pipeline没有scheduler，只填充span filter", func(t *testing.T) {
		template := &entity.ExptTemplate{
			ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "1"},
		}
		templates := []*entity.ExptTemplate{template}

		taskContent := `{
			"rule": {
				"span_filters": {
					"span_list_type": "all"
				}
			}
		}`

		pipeline := &entity.Pipeline{
			ID: gptr.Of(int64(1)),
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{
						NodeTemplateType: "data_reflow",
						Refs: map[string]*entity.NodeRef{
							"task": {
								Content: taskContent,
							},
						},
					},
				},
			},
		}

		mockPipelineAdapter.EXPECT().ListPipelineFlow(ctx, gomock.Any()).Return(&rpc.ListPipelineFlowResponse{Items: []*entity.Pipeline{pipeline}}, nil)
		err := mgr.enrichExptSourceFromPipeline(ctx, templates, spaceID)
		assert.NoError(t, err)

		assert.NotNil(t, template.ExptSource.SpanFilterFields)
		assert.Equal(t, "all", *template.ExptSource.SpanFilterFields.SpanListType)
		assert.Nil(t, template.ExptSource.Scheduler)
	})

	t.Run("pipeline有data_reflow节点但无task ref，不填充数据", func(t *testing.T) {
		template := &entity.ExptTemplate{
			ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "1"},
		}
		templates := []*entity.ExptTemplate{template}

		pipeline := &entity.Pipeline{
			ID: gptr.Of(int64(1)),
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{
						NodeTemplateType: "data_reflow",
						Refs:             map[string]*entity.NodeRef{},
					},
				},
			},
		}

		mockPipelineAdapter.EXPECT().ListPipelineFlow(ctx, gomock.Any()).Return(&rpc.ListPipelineFlowResponse{Items: []*entity.Pipeline{pipeline}}, nil)
		err := mgr.enrichExptSourceFromPipeline(ctx, templates, spaceID)
		assert.NoError(t, err)

		assert.Nil(t, template.ExptSource.SpanFilterFields)
		assert.Nil(t, template.ExptSource.Scheduler)
	})

	t.Run("pipeline有data_reflow节点但task内容为空，不填充数据", func(t *testing.T) {
		template := &entity.ExptTemplate{
			ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "1"},
		}
		templates := []*entity.ExptTemplate{template}

		pipeline := &entity.Pipeline{
			ID: gptr.Of(int64(1)),
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{
						NodeTemplateType: "data_reflow",
						Refs: map[string]*entity.NodeRef{
							"task": {
								Content: "",
							},
						},
					},
				},
			},
		}

		mockPipelineAdapter.EXPECT().ListPipelineFlow(ctx, gomock.Any()).Return(&rpc.ListPipelineFlowResponse{Items: []*entity.Pipeline{pipeline}}, nil)
		err := mgr.enrichExptSourceFromPipeline(ctx, templates, spaceID)
		assert.NoError(t, err)

		assert.Nil(t, template.ExptSource.SpanFilterFields)
		assert.Nil(t, template.ExptSource.Scheduler)
	})

	t.Run("pipeline有data_reflow节点但task内容为无效JSON，不填充数据", func(t *testing.T) {
		template := &entity.ExptTemplate{
			ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "1"},
		}
		templates := []*entity.ExptTemplate{template}

		pipeline := &entity.Pipeline{
			ID: gptr.Of(int64(1)),
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{
						NodeTemplateType: "data_reflow",
						Refs: map[string]*entity.NodeRef{
							"task": {
								Content: "invalid json",
							},
						},
					},
				},
			},
		}

		mockPipelineAdapter.EXPECT().ListPipelineFlow(ctx, gomock.Any()).Return(&rpc.ListPipelineFlowResponse{Items: []*entity.Pipeline{pipeline}}, nil)
		err := mgr.enrichExptSourceFromPipeline(ctx, templates, spaceID)
		assert.NoError(t, err)

		assert.Nil(t, template.ExptSource.SpanFilterFields)
		assert.Nil(t, template.ExptSource.Scheduler)
	})
}

func TestExptTemplateManagerImpl_ListOnline(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockTaskAdapter := mocks.NewMockITaskRPCAdapter(ctrl)
	mockPipelineAdapter := mocks.NewMockIPipelineListAdapter(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:                mockRepo,
		taskRPCAdapter:              mockTaskAdapter,
		pipelineRPCAdapter:          mockPipelineAdapter,
		evaluatorService:            mockEvalSvc,
		evalTargetService:           mockTargetSvc,
		evaluationSetService:        mockEvalSetSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
	}

	ctx := context.Background()
	page := int32(1)
	pageSize := int32(10)
	spaceID := int64(100)
	session := &entity.Session{UserID: "u1"}

	t.Run("ListTasks失败，返回错误", func(t *testing.T) {
		mockTaskAdapter.EXPECT().ListTasks(ctx, gomock.Any()).Return(nil, nil, errors.New("list tasks fail"))
		templates, total, err := mgr.ListOnline(ctx, page, pageSize, spaceID, nil, nil, session)
		assert.Error(t, err)
		assert.Nil(t, templates)
		assert.Equal(t, int64(0), total)
	})

	t.Run("成功查询，无数据", func(t *testing.T) {
		mockTaskAdapter.EXPECT().ListTasks(ctx, gomock.Any()).Return([]*taskdomain.Task{}, gptr.Of(int64(0)), nil)
		mockRepo.EXPECT().List(ctx, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTemplate{}, int64(0), nil)
		templates, total, err := mgr.ListOnline(ctx, page, pageSize, spaceID, nil, nil, session)
		assert.NoError(t, err)
		assert.Len(t, templates, 0)
		assert.Equal(t, int64(0), total)
	})

	t.Run("templateRepo.List失败，返回错误", func(t *testing.T) {
		mockTaskAdapter.EXPECT().ListTasks(ctx, gomock.Any()).Return([]*taskdomain.Task{}, gptr.Of(int64(0)), nil)
		mockRepo.EXPECT().List(ctx, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("list templates fail"))
		templates, total, err := mgr.ListOnline(ctx, page, pageSize, spaceID, nil, nil, session)
		assert.Error(t, err)
		assert.Nil(t, templates)
		assert.Equal(t, int64(0), total)
	})

	t.Run("enrichExptSourceFromPipeline失败，返回错误", func(t *testing.T) {
		mockTaskAdapter.EXPECT().ListTasks(ctx, gomock.Any()).Return([]*taskdomain.Task{}, gptr.Of(int64(0)), nil)
		mockRepo.EXPECT().List(ctx, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTemplate{
			{
				Meta: &entity.ExptTemplateMeta{
					ID:          1,
					WorkspaceID: spaceID,
					ExptType:    entity.ExptType_Online,
				},
				ExptSource: &entity.ExptSource{
					SourceType: entity.SourceType_Workflow,
					SourceID:   "1",
				},
			},
		}, int64(1), nil)
		mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
		mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()
		mockPipelineAdapter.EXPECT().ListPipelineFlow(ctx, gomock.Any()).Return(nil, errors.New("list pipeline flow fail"))
		templates, total, err := mgr.ListOnline(ctx, page, pageSize, spaceID, nil, nil, session)
		assert.Error(t, err)
		assert.Nil(t, templates)
		assert.Equal(t, int64(0), total)
	})

	t.Run("成功查询，包含task转换的模板和DB模板", func(t *testing.T) {
		// Mock ListTasks 返回一个 task
		task := &taskdomain.Task{
			ID:   gptr.Of(int64(1)),
			Name: "test task",
			BaseInfo: &observability_common.BaseInfo{
				CreatedAt: gptr.Of(int64(1700000000000)),
				UpdatedAt: gptr.Of(int64(1700000000000)),
				CreatedBy: &observability_common.UserInfo{
					UserID: gptr.Of("u1"),
				},
				UpdatedBy: &observability_common.UserInfo{
					UserID: gptr.Of("u1"),
				},
			},
			TaskConfig: &taskdomain.TaskConfig{
				AutoEvaluateConfigs: []*taskdomain.AutoEvaluateConfig{
					{
						EvaluatorID:        1,
						EvaluatorVersionID: 101,
						FieldMappings: []*taskdomain.EvaluateFieldMapping{
							{
								TraceFieldKey: "test_field",
								FieldSchema: &observability_dataset.FieldSchema{
									Key:  gptr.Of("test_key"),
									Name: gptr.Of("test_name"),
								},
							},
						},
					},
				},
			},
			Rule: &taskdomain.Rule{
				Sampler: &taskdomain.Sampler{
					IsCycle:       gptr.Of(true),
					CycleTimeUnit: gptr.Of(taskdomain.TimeUnitDay),
				},
				EffectiveTime: &taskdomain.EffectiveTime{
					StartAt: gptr.Of(int64(1700000000000)),
					EndAt:   gptr.Of(int64(1700000000000)),
				},
			},
		}
		mockTaskAdapter.EXPECT().ListTasks(ctx, gomock.Any()).Return([]*taskdomain.Task{task}, gptr.Of(int64(1)), nil)

		// Mock templateRepo.List 返回一个 DB 模板
		dbTemplate := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:          2,
				WorkspaceID: spaceID,
				Name:        "test db template",
				ExptType:    entity.ExptType_Online,
			},
			ExptSource: &entity.ExptSource{
				SourceType: entity.SourceType_Evaluation,
				SourceID:   "2",
			},
		}
		mockRepo.EXPECT().List(ctx, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTemplate{dbTemplate}, int64(1), nil)

		// Mock mgetExptTupleByID 返回空结果
		mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
		mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

		templates, total, err := mgr.ListOnline(ctx, page, pageSize, spaceID, nil, nil, session)
		assert.NoError(t, err)
		assert.Len(t, templates, 2)
		assert.Equal(t, int64(2), total)
	})

	t.Run("内存分页测试，超出范围", func(t *testing.T) {
		// Mock ListTasks 返回一个 task
		task := &taskdomain.Task{
			ID:   gptr.Of(int64(1)),
			Name: "test task",
		}
		mockTaskAdapter.EXPECT().ListTasks(ctx, gomock.Any()).Return([]*taskdomain.Task{task}, gptr.Of(int64(1)), nil)

		// Mock templateRepo.List 返回空结果
		mockRepo.EXPECT().List(ctx, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTemplate{}, int64(0), nil)

		// Mock mgetExptTupleByID 返回空结果
		mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
		mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

		// 页码超出范围
		templates, total, err := mgr.ListOnline(ctx, int32(2), pageSize, spaceID, nil, nil, session)
		assert.NoError(t, err)
		assert.Len(t, templates, 0)
		assert.Equal(t, int64(1), total)
	})

	t.Run("带筛选条件的查询", func(t *testing.T) {
		// Mock ListTasks 返回一个 task
		task := &taskdomain.Task{
			ID:   gptr.Of(int64(1)),
			Name: "test task",
			BaseInfo: &observability_common.BaseInfo{
				CreatedBy: &observability_common.UserInfo{
					UserID: gptr.Of("u1"),
				},
			},
		}
		mockTaskAdapter.EXPECT().ListTasks(ctx, gomock.Any()).Return([]*taskdomain.Task{task}, gptr.Of(int64(1)), nil).Times(2)

		// Mock templateRepo.List 返回空结果
		mockRepo.EXPECT().List(ctx, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTemplate{}, int64(0), nil).Times(2)

		// Mock mgetExptTupleByID 返回空结果
		mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
		mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

		// 带创建者筛选
		filter := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				CreatedBy: []string{"u1"},
			},
		}
		templates, total, err := mgr.ListOnline(ctx, page, pageSize, spaceID, filter, nil, session)
		assert.NoError(t, err)
		assert.Len(t, templates, 1)
		assert.Equal(t, int64(1), total)

		// 带不匹配的创建者筛选
		filter.Includes.CreatedBy = []string{"u2"}
		templates, total, err = mgr.ListOnline(ctx, page, pageSize, spaceID, filter, nil, session)
		assert.NoError(t, err)
		assert.Len(t, templates, 0)
		assert.Equal(t, int64(0), total)
	})

	t.Run("带排序的查询", func(t *testing.T) {
		// Mock ListTasks 返回两个 task
		task1 := &taskdomain.Task{
			ID:   gptr.Of(int64(1)),
			Name: "task 1",
			BaseInfo: &observability_common.BaseInfo{
				CreatedAt: gptr.Of(int64(1700000000000)),
			},
		}
		task2 := &taskdomain.Task{
			ID:   gptr.Of(int64(2)),
			Name: "task 2",
			BaseInfo: &observability_common.BaseInfo{
				CreatedAt: gptr.Of(int64(1700000000001)),
			},
		}
		mockTaskAdapter.EXPECT().ListTasks(ctx, gomock.Any()).Return([]*taskdomain.Task{task1, task2}, gptr.Of(int64(2)), nil)

		// Mock templateRepo.List 返回空结果
		mockRepo.EXPECT().List(ctx, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTemplate{}, int64(0), nil)

		// Mock mgetExptTupleByID 返回空结果
		mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
		mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

		// 按创建时间升序排序
		orderBy := &entity.OrderBy{
			Field: gptr.Of(entity.OrderByCreatedAt),
			IsAsc: gptr.Of(true),
		}
		templates, total, err := mgr.ListOnline(ctx, page, pageSize, spaceID, nil, []*entity.OrderBy{orderBy}, session)
		assert.NoError(t, err)
		assert.Len(t, templates, 2)
		assert.Equal(t, int64(2), total)
	})

	t.Run("带Excludes筛选", func(t *testing.T) {
		task := &taskdomain.Task{
			ID:   gptr.Of(int64(1)),
			Name: "test task",
			BaseInfo: &observability_common.BaseInfo{
				CreatedBy: &observability_common.UserInfo{UserID: gptr.Of("u1")},
			},
		}
		mockTaskAdapter.EXPECT().ListTasks(ctx, gomock.Any()).Return([]*taskdomain.Task{task}, gptr.Of(int64(1)), nil)
		mockRepo.EXPECT().List(ctx, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTemplate{}, int64(0), nil)
		mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
		mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
		mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, nil).AnyTimes()

		filter := &entity.ExptTemplateListFilter{
			Excludes: &entity.ExptTemplateFilterFields{
				CreatedBy: []string{"u1"},
			},
		}
		templates, total, err := mgr.ListOnline(ctx, page, pageSize, spaceID, filter, nil, session)
		assert.NoError(t, err)
		assert.Len(t, templates, 0)
		assert.Equal(t, int64(0), total)
	})
}

func Test_convertTaskFrequency_WeekDays(t *testing.T) {
	// Monday=1706572800000 (2024-01-30 Tue is actually... let's compute)
	// We need deterministic timestamps for each weekday
	// time.Date(2024, 1, 29, 0, 0, 0, 0, time.UTC) is Monday
	mondayMS := time.Date(2024, 1, 29, 0, 0, 0, 0, time.UTC).UnixMilli()
	tuesdayMS := time.Date(2024, 1, 30, 0, 0, 0, 0, time.UTC).UnixMilli()
	wednesdayMS := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC).UnixMilli()
	thursdayMS := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	fridayMS := time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC).UnixMilli()
	saturdayMS := time.Date(2024, 2, 3, 0, 0, 0, 0, time.UTC).UnixMilli()
	sundayMS := time.Date(2024, 2, 4, 0, 0, 0, 0, time.UTC).UnixMilli()

	tests := []struct {
		name     string
		startAt  int64
		expected string
	}{
		{"Monday", mondayMS, "monday"},
		{"Tuesday", tuesdayMS, "tuesday"},
		{"Wednesday", wednesdayMS, "wednesday"},
		{"Thursday", thursdayMS, "thursday"},
		{"Friday", fridayMS, "friday"},
		{"Saturday", saturdayMS, "saturday"},
		{"Sunday", sundayMS, "sunday"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler := &taskdomain.Sampler{
				IsCycle:       gptr.Of(true),
				CycleTimeUnit: gptr.Of(taskdomain.TimeUnitWeek),
			}
			effectiveTime := &taskdomain.EffectiveTime{
				StartAt: gptr.Of(tt.startAt),
			}
			result := convertTaskFrequency(sampler, effectiveTime)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expected, *result)
		})
	}

	t.Run("week_nil_effective_time", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitWeek),
		}
		result := convertTaskFrequency(sampler, nil)
		assert.Nil(t, result)
	})

	t.Run("week_zero_start_at", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitWeek),
		}
		effectiveTime := &taskdomain.EffectiveTime{
			StartAt: gptr.Of(int64(0)),
		}
		result := convertTaskFrequency(sampler, effectiveTime)
		assert.Nil(t, result)
	})

	t.Run("unknown_time_unit", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnit("month")),
		}
		result := convertTaskFrequency(sampler, nil)
		assert.Nil(t, result)
	})

	t.Run("nil_sampler", func(t *testing.T) {
		result := convertTaskFrequency(nil, nil)
		assert.Nil(t, result)
	})

	t.Run("not_cycle", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle: gptr.Of(false),
		}
		result := convertTaskFrequency(sampler, nil)
		assert.Nil(t, result)
	})

	t.Run("day_unit", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitDay),
		}
		result := convertTaskFrequency(sampler, nil)
		assert.NotNil(t, result)
		assert.Equal(t, "every_day", *result)
	})

	t.Run("null_unit", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitNull),
		}
		result := convertTaskFrequency(sampler, nil)
		assert.NotNil(t, result)
		assert.Equal(t, "every_day", *result)
	})

	t.Run("empty_unit", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle: gptr.Of(true),
		}
		result := convertTaskFrequency(sampler, nil)
		assert.NotNil(t, result)
		assert.Equal(t, "every_day", *result)
	})
}

func TestExptTemplateManagerImpl_Create_GenIDError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockIdgen := idgenmocks.NewMockIIDGenerator(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
		idgen:        mockIdgen,
	}

	ctx := context.Background()
	param := newBasicCreateParam()
	session := &entity.Session{UserID: "u1"}

	mockRepo.EXPECT().GetByName(ctx, param.Name, param.SpaceID).Return(nil, false, nil)
	mockIdgen.EXPECT().GenID(ctx).Return(int64(0), errors.New("gen id fail"))

	got, err := mgr.Create(ctx, param, session)
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestExptTemplateManagerImpl_Create_TemplateConfValidError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockIdgen := idgenmocks.NewMockIIDGenerator(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
		idgen:        mockIdgen,
	}

	ctx := context.Background()
	param := newBasicCreateParam()
	param.TemplateConf = &entity.ExptTemplateConfiguration{
		ConnectorConf: entity.Connector{
			EvaluatorsConf: &entity.EvaluatorsConf{
				EvaluatorConf: []*entity.EvaluatorConf{
					{EvaluatorVersionID: 0},
				},
			},
		},
	}
	session := &entity.Session{UserID: "u1"}

	mockRepo.EXPECT().GetByName(ctx, param.Name, param.SpaceID).Return(nil, false, nil)

	got, err := mgr.Create(ctx, param, session)
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestExptTemplateManagerImpl_Create_WithCreateEvalTargetParam(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockIdgen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)
	mockLWT := lwtmocks.NewMockILatestWriteTracker(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:                mockRepo,
		idgen:                       mockIdgen,
		evalTargetService:           mockTargetSvc,
		evaluatorService:            mockEvalSvc,
		evaluationSetService:        mockEvalSetSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
		lwt:                         mockLWT,
	}

	ctx := context.Background()
	param := newBasicCreateParam()
	param.CreateEvalTargetParam = &entity.CreateEvalTargetParam{
		SourceTargetID:      gptr.Of("src-1"),
		SourceTargetVersion: gptr.Of("v1"),
		EvalTargetType:      gptr.Of(entity.EvalTargetTypeLoopPrompt),
		CustomEvalTarget: &entity.CustomEvalTarget{
			ID:        gptr.Of("c1"),
			Name:      gptr.Of("custom"),
			AvatarURL: gptr.Of("http://img"),
			Ext:       map[string]string{"k": "v"},
		},
	}
	param.TemplateConf = &entity.ExptTemplateConfiguration{
		ConnectorConf: entity.Connector{
			TargetConf: &entity.TargetConf{TargetVersionID: 1},
		},
	}
	param.EvaluatorIDVersionItems = []*entity.EvaluatorIDVersionItem{
		{EvaluatorID: 1, Version: "v1", EvaluatorVersionID: 101},
	}
	session := &entity.Session{UserID: "u1"}

	mockRepo.EXPECT().GetByName(ctx, param.Name, param.SpaceID).Return(nil, false, nil)
	mockIdgen.EXPECT().GenID(ctx).Return(int64(10001), nil)
	mockTargetSvc.EXPECT().CreateEvalTarget(gomock.Any(), param.SpaceID, "src-1", "v1", entity.EvalTargetTypeLoopPrompt, gomock.Any()).Return(int64(20), int64(21), nil)
	mockRepo.EXPECT().Create(ctx, gomock.Any(), gomock.Any()).Return(nil)
	mockLWT.EXPECT().SetWriteFlag(ctx, platestwrite.ResourceTypeExptTemplate, int64(10001)).AnyTimes()
	mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	got, err := mgr.Create(ctx, param, session)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, int64(20), got.GetTargetID())
	assert.Equal(t, int64(21), got.GetTargetVersionID())
	assert.Equal(t, int64(21), got.TemplateConf.ConnectorConf.TargetConf.TargetVersionID)
}

func TestExptTemplateManagerImpl_Create_RepoCreateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockIdgen := idgenmocks.NewMockIIDGenerator(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
		idgen:        mockIdgen,
	}

	ctx := context.Background()
	param := newBasicCreateParam()
	session := &entity.Session{UserID: "u1"}

	mockRepo.EXPECT().GetByName(ctx, param.Name, param.SpaceID).Return(nil, false, nil)
	mockIdgen.EXPECT().GenID(ctx).Return(int64(10001), nil)
	mockRepo.EXPECT().Create(ctx, gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	got, err := mgr.Create(ctx, param, session)
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestExptTemplateManagerImpl_UpdateMeta_WithCronActivate(t *testing.T) {
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

	t.Run("CronActivate_true_with_existing_ExptInfo", func(t *testing.T) {
		cronVal := true
		param := &entity.UpdateExptTemplateMetaParam{
			TemplateID:   templateID,
			SpaceID:      spaceID,
			CronActivate: &cronVal,
		}

		existing := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{
				ID:          templateID,
				WorkspaceID: spaceID,
				Name:        "tpl",
			},
			ExptInfo: &entity.ExptInfo{
				CreatedExptCount: 5,
				LatestExptID:     200,
			},
		}

		mockRepo.EXPECT().GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).Return(existing, nil)
		mockRepo.EXPECT().UpdateFields(ctx, templateID, gomock.AssignableToTypeOf(map[string]any{})).DoAndReturn(func(_ context.Context, _ int64, fields map[string]any) error {
			assert.Equal(t, true, fields["cron_activate"])
			assert.NotNil(t, fields["expt_info"])
			return nil
		})

		updated := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{ID: templateID, WorkspaceID: spaceID, Name: "tpl"},
		}
		mockRepo.EXPECT().GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).Return(updated, nil)

		got, err := mgr.UpdateMeta(ctx, param, session)
		assert.NoError(t, err)
		assert.NotNil(t, got)
	})

	t.Run("CronActivate_true_with_nil_ExptInfo", func(t *testing.T) {
		cronVal := true
		param := &entity.UpdateExptTemplateMetaParam{
			TemplateID:   templateID,
			SpaceID:      spaceID,
			CronActivate: &cronVal,
		}

		existing := &entity.ExptTemplate{
			Meta:     &entity.ExptTemplateMeta{ID: templateID, WorkspaceID: spaceID, Name: "tpl"},
			ExptInfo: nil,
		}

		mockRepo.EXPECT().GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).Return(existing, nil)
		mockRepo.EXPECT().UpdateFields(ctx, templateID, gomock.AssignableToTypeOf(map[string]any{})).Return(nil)
		updated := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{ID: templateID, WorkspaceID: spaceID, Name: "tpl"},
		}
		mockRepo.EXPECT().GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).Return(updated, nil)

		got, err := mgr.UpdateMeta(ctx, param, session)
		assert.NoError(t, err)
		assert.NotNil(t, got)
	})

	t.Run("nil_session", func(t *testing.T) {
		param := &entity.UpdateExptTemplateMetaParam{
			TemplateID:  templateID,
			SpaceID:     spaceID,
			Description: "desc",
		}

		existing := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{ID: templateID, WorkspaceID: spaceID, Name: "tpl"},
		}

		mockRepo.EXPECT().GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).Return(existing, nil)
		mockRepo.EXPECT().UpdateFields(ctx, templateID, gomock.AssignableToTypeOf(map[string]any{})).DoAndReturn(func(_ context.Context, _ int64, fields map[string]any) error {
			_, hasUpdatedBy := fields["updated_by"]
			assert.False(t, hasUpdatedBy)
			return nil
		})
		updated := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{ID: templateID, WorkspaceID: spaceID, Name: "tpl"},
		}
		mockRepo.EXPECT().GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).Return(updated, nil)

		got, err := mgr.UpdateMeta(ctx, param, nil)
		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Nil(t, got.BaseInfo.UpdatedBy)
	})

	t.Run("UpdateMeta_NameCheck_Error", func(t *testing.T) {
		param := &entity.UpdateExptTemplateMetaParam{
			TemplateID: templateID,
			SpaceID:    spaceID,
			Name:       "new-name",
		}

		existing := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{ID: templateID, WorkspaceID: spaceID, Name: "old-name"},
		}

		mockRepo.EXPECT().GetByID(ctx, templateID, gomock.AssignableToTypeOf(&spaceID)).Return(existing, nil)
		mockRepo.EXPECT().GetByName(ctx, "new-name", spaceID).Return(nil, false, errors.New("db err"))

		_, err := mgr.UpdateMeta(ctx, param, session)
		assert.Error(t, err)
	})
}

func TestExptTemplateManagerImpl_resolveTargetForCreate_Errors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mgr := &ExptTemplateManagerImpl{
		evalTargetService: mockTargetSvc,
	}

	ctx := context.Background()

	t.Run("CreateEvalTarget_error", func(t *testing.T) {
		param := &entity.CreateExptTemplateParam{
			SpaceID: 100,
			CreateEvalTargetParam: &entity.CreateEvalTargetParam{
				SourceTargetID:      gptr.Of("src-id"),
				SourceTargetVersion: gptr.Of("v1"),
				EvalTargetType:      gptr.Of(entity.EvalTargetTypeLoopPrompt),
			},
		}
		mockTargetSvc.EXPECT().CreateEvalTarget(gomock.Any(), int64(100), "src-id", "v1", entity.EvalTargetTypeLoopPrompt, gomock.Any()).Return(int64(0), int64(0), errors.New("create fail"))

		_, _, _, err := mgr.resolveTargetForCreate(ctx, param)
		assert.Error(t, err)
	})

	t.Run("GetEvalTarget_error", func(t *testing.T) {
		param := &entity.CreateExptTemplateParam{
			SpaceID:  200,
			TargetID: 30,
		}
		mockTargetSvc.EXPECT().GetEvalTarget(gomock.Any(), int64(30)).Return(nil, errors.New("get fail"))

		_, _, _, err := mgr.resolveTargetForCreate(ctx, param)
		assert.Error(t, err)
	})

	t.Run("GetEvalTarget_nil", func(t *testing.T) {
		param := &entity.CreateExptTemplateParam{
			SpaceID:  200,
			TargetID: 30,
		}
		mockTargetSvc.EXPECT().GetEvalTarget(gomock.Any(), int64(30)).Return(nil, nil)

		_, _, _, err := mgr.resolveTargetForCreate(ctx, param)
		assert.Error(t, err)
	})
}

func TestExptTemplateManagerImpl_matchesTemplateFilter_ExtendedBranches(t *testing.T) {
	mgr := &ExptTemplateManagerImpl{}

	t.Run("includes_UpdatedBy_match", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			Meta:     &entity.ExptTemplateMeta{ID: 1, ExptType: entity.ExptType_Online},
			BaseInfo: &entity.BaseInfo{UpdatedBy: &entity.UserInfo{UserID: gptr.Of("u2")}},
		}
		filter := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				UpdatedBy: []string{"u2"},
			},
		}
		assert.True(t, mgr.matchesTemplateFilter(tpl, filter))
	})

	t.Run("includes_UpdatedBy_no_match", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			Meta:     &entity.ExptTemplateMeta{ID: 1, ExptType: entity.ExptType_Online},
			BaseInfo: &entity.BaseInfo{UpdatedBy: &entity.UserInfo{UserID: gptr.Of("u3")}},
		}
		filter := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				UpdatedBy: []string{"u2"},
			},
		}
		assert.False(t, mgr.matchesTemplateFilter(tpl, filter))
	})

	t.Run("includes_TargetIDs_match", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			Meta:         &entity.ExptTemplateMeta{ID: 1, ExptType: entity.ExptType_Online},
			TripleConfig: &entity.ExptTemplateTuple{TargetID: 20},
		}
		filter := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				TargetIDs: []int64{20},
			},
		}
		assert.True(t, mgr.matchesTemplateFilter(tpl, filter))
	})

	t.Run("includes_TargetIDs_no_match", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			Meta:         &entity.ExptTemplateMeta{ID: 1, ExptType: entity.ExptType_Online},
			TripleConfig: &entity.ExptTemplateTuple{TargetID: 20},
		}
		filter := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				TargetIDs: []int64{30},
			},
		}
		assert.False(t, mgr.matchesTemplateFilter(tpl, filter))
	})

	t.Run("includes_TargetType_match", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			Meta:         &entity.ExptTemplateMeta{ID: 1, ExptType: entity.ExptType_Online},
			TripleConfig: &entity.ExptTemplateTuple{TargetType: entity.EvalTargetTypeLoopPrompt},
		}
		filter := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				TargetType: []int64{int64(entity.EvalTargetTypeLoopPrompt)},
			},
		}
		assert.True(t, mgr.matchesTemplateFilter(tpl, filter))
	})

	t.Run("includes_ExptType_no_match", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{ID: 1, ExptType: entity.ExptType_Online},
		}
		filter := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				ExptType: []int64{int64(entity.ExptType_Offline)},
			},
		}
		assert.False(t, mgr.matchesTemplateFilter(tpl, filter))
	})

	t.Run("includes_FuzzyName_match", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{ID: 1, Name: "My Test Template", ExptType: entity.ExptType_Online},
		}
		filter := &entity.ExptTemplateListFilter{
			FuzzyName: "test",
			Includes:  &entity.ExptTemplateFilterFields{},
		}
		assert.True(t, mgr.matchesTemplateFilter(tpl, filter))
	})

	t.Run("includes_FuzzyName_no_match", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{ID: 1, Name: "My Template", ExptType: entity.ExptType_Online},
		}
		filter := &entity.ExptTemplateListFilter{
			FuzzyName: "xyz",
			Includes:  &entity.ExptTemplateFilterFields{},
		}
		assert.False(t, mgr.matchesTemplateFilter(tpl, filter))
	})

	t.Run("excludes_UpdatedBy", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			Meta:     &entity.ExptTemplateMeta{ID: 1, ExptType: entity.ExptType_Online},
			BaseInfo: &entity.BaseInfo{UpdatedBy: &entity.UserInfo{UserID: gptr.Of("u2")}},
		}
		filter := &entity.ExptTemplateListFilter{
			Excludes: &entity.ExptTemplateFilterFields{
				UpdatedBy: []string{"u2"},
			},
		}
		assert.False(t, mgr.matchesTemplateFilter(tpl, filter))
	})

	t.Run("excludes_EvalSetIDs", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			Meta:         &entity.ExptTemplateMeta{ID: 1, ExptType: entity.ExptType_Online},
			TripleConfig: &entity.ExptTemplateTuple{EvalSetID: 10},
		}
		filter := &entity.ExptTemplateListFilter{
			Excludes: &entity.ExptTemplateFilterFields{
				EvalSetIDs: []int64{10},
			},
		}
		assert.False(t, mgr.matchesTemplateFilter(tpl, filter))
	})

	t.Run("excludes_TargetIDs", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			Meta:         &entity.ExptTemplateMeta{ID: 1, ExptType: entity.ExptType_Online},
			TripleConfig: &entity.ExptTemplateTuple{TargetID: 20},
		}
		filter := &entity.ExptTemplateListFilter{
			Excludes: &entity.ExptTemplateFilterFields{
				TargetIDs: []int64{20},
			},
		}
		assert.False(t, mgr.matchesTemplateFilter(tpl, filter))
	})

	t.Run("excludes_TargetType", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			Meta:         &entity.ExptTemplateMeta{ID: 1, ExptType: entity.ExptType_Online},
			TripleConfig: &entity.ExptTemplateTuple{TargetType: entity.EvalTargetTypeLoopPrompt},
		}
		filter := &entity.ExptTemplateListFilter{
			Excludes: &entity.ExptTemplateFilterFields{
				TargetType: []int64{int64(entity.EvalTargetTypeLoopPrompt)},
			},
		}
		assert.False(t, mgr.matchesTemplateFilter(tpl, filter))
	})

	t.Run("excludes_ExptType", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{ID: 1, ExptType: entity.ExptType_Online},
		}
		filter := &entity.ExptTemplateListFilter{
			Excludes: &entity.ExptTemplateFilterFields{
				ExptType: []int64{int64(entity.ExptType_Online)},
			},
		}
		assert.False(t, mgr.matchesTemplateFilter(tpl, filter))
	})

	t.Run("excludes_CronActivate_true", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			Meta:     &entity.ExptTemplateMeta{ID: 1, ExptType: entity.ExptType_Online},
			ExptInfo: &entity.ExptInfo{CronActivate: true},
		}
		filter := &entity.ExptTemplateListFilter{
			Excludes: &entity.ExptTemplateFilterFields{
				CronActivate: []int64{1},
			},
		}
		assert.False(t, mgr.matchesTemplateFilter(tpl, filter))
	})

	t.Run("excludes_CronActivate_false", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{ID: 1, ExptType: entity.ExptType_Online},
		}
		filter := &entity.ExptTemplateListFilter{
			Excludes: &entity.ExptTemplateFilterFields{
				CronActivate: []int64{0},
			},
		}
		assert.False(t, mgr.matchesTemplateFilter(tpl, filter))
	})

	t.Run("includes_EvalSetIDs", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			Meta:         &entity.ExptTemplateMeta{ID: 1, ExptType: entity.ExptType_Online},
			TripleConfig: &entity.ExptTemplateTuple{EvalSetID: 10},
		}
		filter := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				EvalSetIDs: []int64{10},
			},
		}
		assert.True(t, mgr.matchesTemplateFilter(tpl, filter))
	})

	t.Run("includes_EvalSetIDs_no_match", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			Meta:         &entity.ExptTemplateMeta{ID: 1, ExptType: entity.ExptType_Online},
			TripleConfig: &entity.ExptTemplateTuple{EvalSetID: 10},
		}
		filter := &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				EvalSetIDs: []int64{99},
			},
		}
		assert.False(t, mgr.matchesTemplateFilter(tpl, filter))
	})
}

func TestExptTemplateManagerImpl_MGet_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockLWT := lwtmocks.NewMockILatestWriteTracker(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
		lwt:          mockLWT,
	}

	ctx := context.Background()
	spaceID := int64(100)
	session := &entity.Session{UserID: "u1"}

	mockLWT.EXPECT().CheckWriteFlagByID(ctx, platestwrite.ResourceTypeExptTemplate, int64(1)).Return(false)
	mockRepo.EXPECT().MGetByID(ctx, []int64{1}, spaceID).Return(nil, errors.New("db error"))

	_, err := mgr.MGet(ctx, []int64{1}, spaceID, session)
	assert.Error(t, err)
}

func TestExptTemplateManagerImpl_MGet_EmptyResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockLWT := lwtmocks.NewMockILatestWriteTracker(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo: mockRepo,
		lwt:          mockLWT,
	}

	ctx := context.Background()
	spaceID := int64(100)
	session := &entity.Session{UserID: "u1"}

	mockLWT.EXPECT().CheckWriteFlagByID(ctx, platestwrite.ResourceTypeExptTemplate, int64(1)).Return(false)
	mockRepo.EXPECT().MGetByID(ctx, []int64{1}, spaceID).Return([]*entity.ExptTemplate{}, nil)

	got, err := mgr.MGet(ctx, []int64{1}, spaceID, session)
	assert.NoError(t, err)
	assert.Empty(t, got)
}

func TestExptTemplateManagerImpl_List_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mgr := &ExptTemplateManagerImpl{templateRepo: mockRepo}

	ctx := context.Background()
	spaceID := int64(100)

	mockRepo.EXPECT().List(ctx, int32(1), int32(10), gomock.Nil(), gomock.Nil(), spaceID).Return(nil, int64(0), errors.New("db error"))

	_, _, err := mgr.List(ctx, 1, 10, spaceID, nil, nil, &entity.Session{UserID: "u1"})
	assert.Error(t, err)
}

func TestExptTemplateManagerImpl_List_MgetTupleError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:                mockRepo,
		evaluatorService:            mockEvalSvc,
		evalTargetService:           mockTargetSvc,
		evaluationSetService:        mockEvalSetSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
	}

	ctx := context.Background()
	spaceID := int64(100)

	templates := []*entity.ExptTemplate{
		{
			Meta:         &entity.ExptTemplateMeta{ID: 1, WorkspaceID: spaceID},
			TripleConfig: &entity.ExptTemplateTuple{EvalSetID: 10, EvalSetVersionID: 11, EvaluatorVersionIds: []int64{101}},
		},
	}
	mockRepo.EXPECT().List(ctx, int32(1), int32(10), gomock.Nil(), gomock.Nil(), spaceID).Return(templates, int64(1), nil)
	mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
	mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(spaceID), gomock.Any(), gptr.Of(false)).Return(nil, nil).AnyTimes()
	mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()
	mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, errors.New("batch error"))

	_, _, err := mgr.List(ctx, 1, 10, spaceID, nil, nil, &entity.Session{UserID: "u1"})
	assert.Error(t, err)
}

func TestExptTemplateManagerImpl_applyTemplateOrderBy_UpdatedAt(t *testing.T) {
	mgr := &ExptTemplateManagerImpl{}

	templates := []*entity.ExptTemplate{
		{
			Meta:     &entity.ExptTemplateMeta{ID: 1},
			BaseInfo: &entity.BaseInfo{UpdatedAt: gptr.Of(int64(300))},
		},
		{
			Meta:     &entity.ExptTemplateMeta{ID: 2},
			BaseInfo: &entity.BaseInfo{UpdatedAt: gptr.Of(int64(100))},
		},
		{
			Meta:     &entity.ExptTemplateMeta{ID: 3},
			BaseInfo: &entity.BaseInfo{UpdatedAt: gptr.Of(int64(200))},
		},
	}

	t.Run("asc", func(t *testing.T) {
		tpls := make([]*entity.ExptTemplate, len(templates))
		copy(tpls, templates)
		orderBys := []*entity.OrderBy{
			{Field: gptr.Of(entity.OrderByUpdatedAt), IsAsc: gptr.Of(true)},
		}
		mgr.applyTemplateOrderBy(tpls, orderBys)
		assert.Equal(t, int64(2), tpls[0].GetID())
		assert.Equal(t, int64(3), tpls[1].GetID())
		assert.Equal(t, int64(1), tpls[2].GetID())
	})

	t.Run("desc", func(t *testing.T) {
		tpls := make([]*entity.ExptTemplate, len(templates))
		copy(tpls, templates)
		orderBys := []*entity.OrderBy{
			{Field: gptr.Of(entity.OrderByUpdatedAt), IsAsc: gptr.Of(false)},
		}
		mgr.applyTemplateOrderBy(tpls, orderBys)
		assert.Equal(t, int64(1), tpls[0].GetID())
		assert.Equal(t, int64(3), tpls[1].GetID())
		assert.Equal(t, int64(2), tpls[2].GetID())
	})

	t.Run("nil_BaseInfo", func(t *testing.T) {
		tpls := []*entity.ExptTemplate{
			{Meta: &entity.ExptTemplateMeta{ID: 1}},
			{Meta: &entity.ExptTemplateMeta{ID: 2}, BaseInfo: &entity.BaseInfo{UpdatedAt: gptr.Of(int64(100))}},
		}
		orderBys := []*entity.OrderBy{
			{Field: gptr.Of(entity.OrderByUpdatedAt), IsAsc: gptr.Of(true)},
		}
		mgr.applyTemplateOrderBy(tpls, orderBys)
		assert.Equal(t, int64(1), tpls[0].GetID())
		assert.Equal(t, int64(2), tpls[1].GetID())
	})

	t.Run("unknown_field", func(t *testing.T) {
		tpls := []*entity.ExptTemplate{
			{Meta: &entity.ExptTemplateMeta{ID: 2}},
			{Meta: &entity.ExptTemplateMeta{ID: 1}},
		}
		orderBys := []*entity.OrderBy{
			{Field: gptr.Of("unknown_field"), IsAsc: gptr.Of(true)},
		}
		mgr.applyTemplateOrderBy(tpls, orderBys)
		assert.Equal(t, int64(2), tpls[0].GetID())
		assert.Equal(t, int64(1), tpls[1].GetID())
	})
}

func Test_taskToExptTemplate_NilBaseInfoFields(t *testing.T) {
	t.Run("nil_task", func(t *testing.T) {
		result := taskToExptTemplate(nil, 100)
		assert.Nil(t, result)
	})

	t.Run("nil_ID", func(t *testing.T) {
		task := &taskdomain.Task{ID: nil}
		result := taskToExptTemplate(task, 100)
		assert.Nil(t, result)
	})

	t.Run("nil_BaseInfo", func(t *testing.T) {
		task := &taskdomain.Task{
			ID:       gptr.Of(int64(1)),
			Name:     "test",
			BaseInfo: nil,
		}
		result := taskToExptTemplate(task, 100)
		assert.NotNil(t, result)
		assert.NotNil(t, result.BaseInfo)
	})

	t.Run("BaseInfo_with_nil_CreatedBy", func(t *testing.T) {
		task := &taskdomain.Task{
			ID:   gptr.Of(int64(1)),
			Name: "test",
			BaseInfo: &observability_common.BaseInfo{
				CreatedAt: gptr.Of(int64(1000)),
				UpdatedAt: gptr.Of(int64(2000)),
				CreatedBy: nil,
				UpdatedBy: nil,
			},
		}
		result := taskToExptTemplate(task, 100)
		assert.NotNil(t, result)
		assert.Nil(t, result.BaseInfo.CreatedBy)
		assert.Nil(t, result.BaseInfo.UpdatedBy)
	})

	t.Run("nil_TaskConfig", func(t *testing.T) {
		task := &taskdomain.Task{
			ID:         gptr.Of(int64(1)),
			Name:       "test",
			TaskConfig: nil,
		}
		result := taskToExptTemplate(task, 100)
		assert.NotNil(t, result)
		assert.Nil(t, result.TripleConfig)
	})

	t.Run("nil_Rule", func(t *testing.T) {
		task := &taskdomain.Task{
			ID:   gptr.Of(int64(1)),
			Name: "test",
			Rule: nil,
		}
		result := taskToExptTemplate(task, 100)
		assert.NotNil(t, result)
		assert.Nil(t, result.ExptSource.Scheduler)
	})

	t.Run("Rule_with_SpanFilters", func(t *testing.T) {
		pt := observability_common.PlatformType("web")
		task := &taskdomain.Task{
			ID:   gptr.Of(int64(1)),
			Name: "test",
			Rule: &taskdomain.Rule{
				SpanFilters: &taskfilter.SpanFilterFields{
					PlatformType: &pt,
				},
			},
		}
		result := taskToExptTemplate(task, 100)
		assert.NotNil(t, result)
		assert.NotNil(t, result.ExptSource.SpanFilterFields)
		assert.Equal(t, "web", *result.ExptSource.SpanFilterFields.PlatformType)
	})
}

func Test_autoEvaluateConfigsToExptTemplateConf_NilConfig(t *testing.T) {
	t.Run("nil_TaskConfig", func(t *testing.T) {
		triple, connector := autoEvaluateConfigsToExptTemplateConf(nil)
		assert.Nil(t, triple)
		assert.Nil(t, connector.EvaluatorsConf)
	})

	t.Run("empty_AutoEvaluateConfigs", func(t *testing.T) {
		tc := &taskdomain.TaskConfig{AutoEvaluateConfigs: []*taskdomain.AutoEvaluateConfig{}}
		triple, connector := autoEvaluateConfigsToExptTemplateConf(tc)
		assert.Nil(t, triple)
		assert.Nil(t, connector.EvaluatorsConf)
	})

	t.Run("nil_and_invalid_configs_filtered", func(t *testing.T) {
		tc := &taskdomain.TaskConfig{
			AutoEvaluateConfigs: []*taskdomain.AutoEvaluateConfig{
				nil,
				{EvaluatorID: 1, EvaluatorVersionID: 0},
				{EvaluatorID: 2, EvaluatorVersionID: -1},
			},
		}
		triple, connector := autoEvaluateConfigsToExptTemplateConf(tc)
		assert.Nil(t, triple)
		assert.Nil(t, connector.EvaluatorsConf)
	})

	t.Run("with_FieldMappings_EvalSetName", func(t *testing.T) {
		tc := &taskdomain.TaskConfig{
			AutoEvaluateConfigs: []*taskdomain.AutoEvaluateConfig{
				{
					EvaluatorID:        1,
					EvaluatorVersionID: 101,
					FieldMappings: []*taskdomain.EvaluateFieldMapping{
						{
							TraceFieldKey: "trace_key",
							EvalSetName:   gptr.Of("eval_set_col"),
							FieldSchema: &observability_dataset.FieldSchema{
								Key: gptr.Of("field_key"),
							},
						},
					},
				},
			},
		}
		triple, connector := autoEvaluateConfigsToExptTemplateConf(tc)
		assert.NotNil(t, triple)
		assert.NotNil(t, connector.EvaluatorsConf)
		assert.Len(t, connector.EvaluatorsConf.EvaluatorConf, 1)
		assert.Len(t, connector.EvaluatorsConf.EvaluatorConf[0].IngressConf.EvalSetAdapter.FieldConfs, 1)
		assert.Equal(t, "eval_set_col", connector.EvaluatorsConf.EvaluatorConf[0].IngressConf.EvalSetAdapter.FieldConfs[0].FromField)
	})

	t.Run("with_FieldMappings_NameFallback", func(t *testing.T) {
		tc := &taskdomain.TaskConfig{
			AutoEvaluateConfigs: []*taskdomain.AutoEvaluateConfig{
				{
					EvaluatorID:        1,
					EvaluatorVersionID: 101,
					FieldMappings: []*taskdomain.EvaluateFieldMapping{
						{
							TraceFieldKey: "trace_key",
							FieldSchema: &observability_dataset.FieldSchema{
								Key:  nil,
								Name: gptr.Of("field_name"),
							},
						},
					},
				},
			},
		}
		triple, connector := autoEvaluateConfigsToExptTemplateConf(tc)
		assert.NotNil(t, triple)
		assert.Len(t, connector.EvaluatorsConf.EvaluatorConf[0].IngressConf.EvalSetAdapter.FieldConfs, 1)
		assert.Equal(t, "field_name", connector.EvaluatorsConf.EvaluatorConf[0].IngressConf.EvalSetAdapter.FieldConfs[0].FieldName)
	})
}

func TestExptTemplateManagerImpl_buildFieldMappingConfigAndEnableScoreWeight_NilConf(t *testing.T) {
	mgr := &ExptTemplateManagerImpl{}

	t.Run("nil_templateConf", func(t *testing.T) {
		tpl := &entity.ExptTemplate{}
		mgr.buildFieldMappingConfigAndEnableScoreWeight(tpl, nil)
		assert.Nil(t, tpl.FieldMappingConfig)
	})

	t.Run("no_ScoreWeight", func(t *testing.T) {
		tpl := &entity.ExptTemplate{}
		conf := &entity.ExptTemplateConfiguration{
			ConnectorConf: entity.Connector{
				EvaluatorsConf: &entity.EvaluatorsConf{
					EvaluatorConf: []*entity.EvaluatorConf{
						{
							EvaluatorVersionID: 101,
							IngressConf: &entity.EvaluatorIngressConf{
								EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
							},
						},
					},
				},
			},
		}
		mgr.buildFieldMappingConfigAndEnableScoreWeight(tpl, conf)
		assert.False(t, conf.ConnectorConf.EvaluatorsConf.EnableScoreWeight)
	})

	t.Run("nil_IngressConf_skipped", func(t *testing.T) {
		tpl := &entity.ExptTemplate{}
		conf := &entity.ExptTemplateConfiguration{
			ConnectorConf: entity.Connector{
				EvaluatorsConf: &entity.EvaluatorsConf{
					EvaluatorConf: []*entity.EvaluatorConf{
						{EvaluatorVersionID: 101, IngressConf: nil},
					},
				},
			},
		}
		mgr.buildFieldMappingConfigAndEnableScoreWeight(tpl, conf)
		assert.Empty(t, tpl.FieldMappingConfig.EvaluatorFieldMapping)
	})

	t.Run("ScoreWeight_zero", func(t *testing.T) {
		tpl := &entity.ExptTemplate{}
		conf := &entity.ExptTemplateConfiguration{
			ConnectorConf: entity.Connector{
				EvaluatorsConf: &entity.EvaluatorsConf{
					EvaluatorConf: []*entity.EvaluatorConf{
						{
							EvaluatorVersionID: 101,
							ScoreWeight:        gptr.Of(0.0),
							IngressConf: &entity.EvaluatorIngressConf{
								EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
							},
						},
					},
				},
			},
		}
		mgr.buildFieldMappingConfigAndEnableScoreWeight(tpl, conf)
		assert.False(t, conf.ConnectorConf.EvaluatorsConf.EnableScoreWeight)
	})
}

func TestExptTemplateManagerImpl_resolveAndFillEvaluatorVersionIDs_SpaceMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mgr := &ExptTemplateManagerImpl{evaluatorService: mockEvalSvc}

	ctx := context.Background()
	spaceID := int64(100)

	items := []*entity.EvaluatorIDVersionItem{
		{EvaluatorID: 1, Version: "v1", EvaluatorVersionID: 0},
	}

	ev := &entity.Evaluator{
		ID:                     1,
		EvaluatorType:          entity.EvaluatorTypePrompt,
		Builtin:                false,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{},
	}
	ev.PromptEvaluatorVersion.SetID(101)
	ev.PromptEvaluatorVersion.SetVersion("v1")
	ev.SetSpaceID(int64(999))

	mockEvalSvc.EXPECT().BatchGetEvaluatorByIDAndVersion(ctx, gomock.Any()).Return([]*entity.Evaluator{ev}, nil)

	err := mgr.resolveAndFillEvaluatorVersionIDs(ctx, spaceID, nil, items)
	assert.Error(t, err)
}

func TestExptTemplateManagerImpl_resolveAndFillEvaluatorVersionIDs_NothingToResolve(t *testing.T) {
	mgr := &ExptTemplateManagerImpl{}
	ctx := context.Background()

	items := []*entity.EvaluatorIDVersionItem{
		{EvaluatorID: 1, Version: "v1", EvaluatorVersionID: 101},
		nil,
	}

	err := mgr.resolveAndFillEvaluatorVersionIDs(ctx, 100, nil, items)
	assert.NoError(t, err)
}

func TestExptTemplateManagerImpl_resolveAndFillEvaluatorVersionIDs_TemplateConfNormalPairExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mgr := &ExptTemplateManagerImpl{evaluatorService: mockEvalSvc}

	ctx := context.Background()
	spaceID := int64(100)

	items := []*entity.EvaluatorIDVersionItem{
		{EvaluatorID: 1, Version: "v1", EvaluatorVersionID: 0},
	}

	templateConf := &entity.ExptTemplateConfiguration{
		ConnectorConf: entity.Connector{
			EvaluatorsConf: &entity.EvaluatorsConf{
				EvaluatorConf: []*entity.EvaluatorConf{
					{EvaluatorID: 1, Version: "v1", EvaluatorVersionID: 0},
					{EvaluatorID: 2, Version: "v2", EvaluatorVersionID: 0},
					nil,
					{EvaluatorID: 0, Version: "", EvaluatorVersionID: 0},
					{EvaluatorID: 3, Version: "v3", EvaluatorVersionID: 333},
				},
			},
		},
	}

	ev1 := &entity.Evaluator{
		ID:                     1,
		EvaluatorType:          entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{},
	}
	ev1.PromptEvaluatorVersion.SetID(101)
	ev1.PromptEvaluatorVersion.SetVersion("v1")
	ev1.SetSpaceID(spaceID)

	ev2 := &entity.Evaluator{
		ID:                     2,
		EvaluatorType:          entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{},
	}
	ev2.PromptEvaluatorVersion.SetID(102)
	ev2.PromptEvaluatorVersion.SetVersion("v2")
	ev2.SetSpaceID(spaceID)

	mockEvalSvc.EXPECT().BatchGetEvaluatorByIDAndVersion(ctx, gomock.Any()).Return([]*entity.Evaluator{ev1, ev2}, nil)

	err := mgr.resolveAndFillEvaluatorVersionIDs(ctx, spaceID, templateConf, items)
	assert.NoError(t, err)

	assert.Equal(t, int64(101), items[0].EvaluatorVersionID)
	assert.Equal(t, int64(101), templateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf[0].EvaluatorVersionID)
	assert.Equal(t, int64(102), templateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf[1].EvaluatorVersionID)
	assert.Equal(t, int64(333), templateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf[4].EvaluatorVersionID)
}

func Test_taskRuleToExptScheduler_EffectiveTimeOnly(t *testing.T) {
	t.Run("only_effective_time_with_start_and_end", func(t *testing.T) {
		rule := &taskdomain.Rule{
			EffectiveTime: &taskdomain.EffectiveTime{
				StartAt: gptr.Of(int64(1700000000000)),
				EndAt:   gptr.Of(int64(1700000001000)),
			},
		}
		result := taskRuleToExptScheduler(rule)
		assert.NotNil(t, result)
		assert.Equal(t, int64(1700000000000), *result.StartTime)
		assert.Equal(t, int64(1700000001000), *result.EndTime)
	})

	t.Run("sampler_not_cycle_with_effective_time", func(t *testing.T) {
		rule := &taskdomain.Rule{
			Sampler: &taskdomain.Sampler{
				IsCycle: gptr.Of(false),
			},
			EffectiveTime: &taskdomain.EffectiveTime{
				StartAt: gptr.Of(int64(1700000000000)),
			},
		}
		result := taskRuleToExptScheduler(rule)
		assert.NotNil(t, result)
		assert.NotNil(t, result.Enabled)
		assert.False(t, *result.Enabled)
	})

	t.Run("sampler_zero_effective_time", func(t *testing.T) {
		rule := &taskdomain.Rule{
			EffectiveTime: &taskdomain.EffectiveTime{
				StartAt: gptr.Of(int64(0)),
				EndAt:   gptr.Of(int64(0)),
			},
		}
		result := taskRuleToExptScheduler(rule)
		assert.Nil(t, result)
	})

	t.Run("sampler_and_effective_nil", func(t *testing.T) {
		rule := &taskdomain.Rule{
			Sampler:       nil,
			EffectiveTime: nil,
		}
		result := taskRuleToExptScheduler(rule)
		assert.Nil(t, result)
	})
}

func TestExptTemplateManagerImpl_enrichExptSourceFromPipeline_RespNilItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPipelineAdapter := mocks.NewMockIPipelineListAdapter(ctrl)
	mgr := &ExptTemplateManagerImpl{pipelineRPCAdapter: mockPipelineAdapter}

	ctx := context.Background()
	spaceID := int64(100)

	t.Run("resp_nil", func(t *testing.T) {
		templates := []*entity.ExptTemplate{
			{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "1"}},
		}
		mockPipelineAdapter.EXPECT().ListPipelineFlow(ctx, gomock.Any()).Return(nil, nil)
		err := mgr.enrichExptSourceFromPipeline(ctx, templates, spaceID)
		assert.NoError(t, err)
	})

	t.Run("pipeline_nil_in_items", func(t *testing.T) {
		templates := []*entity.ExptTemplate{
			{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "1"}},
		}
		mockPipelineAdapter.EXPECT().ListPipelineFlow(ctx, gomock.Any()).Return(&rpc.ListPipelineFlowResponse{
			Items: []*entity.Pipeline{nil},
		}, nil)
		err := mgr.enrichExptSourceFromPipeline(ctx, templates, spaceID)
		assert.NoError(t, err)
	})

	t.Run("pipeline_no_matching_template", func(t *testing.T) {
		templates := []*entity.ExptTemplate{
			{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "1"}},
		}
		mockPipelineAdapter.EXPECT().ListPipelineFlow(ctx, gomock.Any()).Return(&rpc.ListPipelineFlowResponse{
			Items: []*entity.Pipeline{
				{ID: gptr.Of(int64(999))},
			},
		}, nil)
		err := mgr.enrichExptSourceFromPipeline(ctx, templates, spaceID)
		assert.NoError(t, err)
	})

	t.Run("duplicate_pipeline_IDs_deduped", func(t *testing.T) {
		templates := []*entity.ExptTemplate{
			{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "1"}},
			{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "1"}},
		}
		mockPipelineAdapter.EXPECT().ListPipelineFlow(ctx, gomock.Any()).Return(&rpc.ListPipelineFlowResponse{
			Items: []*entity.Pipeline{
				{ID: gptr.Of(int64(1)), Scheduler: &entity.Scheduler{Enabled: gptr.Of(true)}},
			},
		}, nil)
		err := mgr.enrichExptSourceFromPipeline(ctx, templates, spaceID)
		assert.NoError(t, err)
		assert.NotNil(t, templates[0].ExptSource.Scheduler)
		assert.NotNil(t, templates[1].ExptSource.Scheduler)
	})
}

func Test_parseSpanFilterFieldsFromTaskJSON_WithSubFilter(t *testing.T) {
	taskJSON := `{
		"rule": {
			"span_filters": {
				"platform_type": "TCE",
				"span_list_type": "normal",
				"filters": {
					"query_and_or": "and",
					"filter_fields": [
						{
							"field_name": "field1",
							"field_type": "string",
							"values": ["v1", "v2"],
							"query_type": "in",
							"query_and_or": "or",
							"sub_filter": {
								"query_and_or": "and",
								"filter_fields": [
									{"field_name": "sub1"}
								]
							}
						},
						null
					]
				}
			}
		}
	}`
	result := parseSpanFilterFieldsFromTaskJSON(taskJSON)
	assert.NotNil(t, result)
	assert.Equal(t, "TCE", *result.PlatformType)
	assert.Equal(t, "normal", *result.SpanListType)
	assert.NotNil(t, result.Filters)
	assert.Equal(t, "and", *result.Filters.QueryAndOr)
	assert.Len(t, result.Filters.FilterFields, 1)
	ff := result.Filters.FilterFields[0]
	assert.Equal(t, "field1", *ff.FieldName)
	assert.Equal(t, "string", *ff.FieldType)
	assert.Equal(t, []string{"v1", "v2"}, ff.Values)
	assert.Equal(t, "in", *ff.QueryType)
	assert.Equal(t, "or", *ff.QueryAndOr)
	assert.NotNil(t, ff.SubFilter)
	assert.Equal(t, "and", *ff.SubFilter.QueryAndOr)
	assert.Len(t, ff.SubFilter.FilterFields, 1)
	assert.Equal(t, "sub1", *ff.SubFilter.FilterFields[0].FieldName)
}

func Test_parseSpanFilterFieldsFromTaskJSON_NoFilters(t *testing.T) {
	taskJSON := `{
		"rule": {
			"span_filters": {
				"platform_type": "TCE"
			}
		}
	}`
	result := parseSpanFilterFieldsFromTaskJSON(taskJSON)
	assert.NotNil(t, result)
	assert.Nil(t, result.Filters)
}

func Test_parseSpanFilterFieldsFromTaskJSON_WithExtendedFilterFields(t *testing.T) {
	taskJSON := `{
		"rule": {
			"span_filters": {
				"platform_type": "inner_prompt",
				"span_list_type": "all_span",
				"filters": {
					"query_and_or": "and",
					"filter_fields": [
						{
							"field_name": "deployment_env",
							"query_type": "not_in",
							"values": ["boe"],
							"is_custom": false,
							"extra_info": {"source": "preset"}
						},
						{
							"sub_filter": {
								"query_and_or": "and",
								"filter_fields": [
									{
										"field_name": "duration",
										"logic_field_name_type": "duration",
										"field_type": "long",
										"query_type": "gte",
										"values": ["12"],
										"is_custom": false
									},
									{
										"field_name": "latency_first_resp",
										"logic_field_name_type": "latency_first_resp",
										"field_type": "long",
										"query_type": "gte",
										"values": ["1000"],
										"is_custom": false
									}
								]
							}
						}
					]
				}
			}
		}
	}`
	result := parseSpanFilterFieldsFromTaskJSON(taskJSON)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Filters)
	assert.Len(t, result.Filters.FilterFields, 2)

	first := result.Filters.FilterFields[0]
	assert.Equal(t, "deployment_env", *first.FieldName)
	assert.NotNil(t, first.IsCustom)
	assert.False(t, *first.IsCustom)
	assert.Equal(t, "preset", first.ExtraInfo["source"])

	second := result.Filters.FilterFields[1]
	assert.NotNil(t, second.SubFilter)
	assert.Len(t, second.SubFilter.FilterFields, 2)
	assert.Equal(t, "duration", *second.SubFilter.FilterFields[0].FieldName)
	assert.Equal(t, "duration", *second.SubFilter.FilterFields[0].LogicFieldNameType)
	assert.Equal(t, "latency_first_resp", *second.SubFilter.FilterFields[1].LogicFieldNameType)
}

func TestExptTemplateManagerImpl_packTemplateTupleID_WithRefs(t *testing.T) {
	mgr := &ExptTemplateManagerImpl{}

	t.Run("from_EvaluatorVersionRef", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			TripleConfig: &entity.ExptTemplateTuple{EvalSetID: 10, EvalSetVersionID: 20},
			EvaluatorVersionRef: []*entity.ExptTemplateEvaluatorVersionRef{
				{EvaluatorID: 1, EvaluatorVersionID: 101},
				{EvaluatorID: 2, EvaluatorVersionID: 0},
			},
		}
		tupleID := mgr.packTemplateTupleID(tpl)
		assert.ElementsMatch(t, []int64{101}, tupleID.EvaluatorVersionIDs)
	})

	t.Run("no_target", func(t *testing.T) {
		tpl := &entity.ExptTemplate{
			TripleConfig: &entity.ExptTemplateTuple{EvalSetID: 10, EvalSetVersionID: 20, TargetID: 0, TargetVersionID: 0},
		}
		tupleID := mgr.packTemplateTupleID(tpl)
		assert.Nil(t, tupleID.VersionedTargetID)
	})
}

func Test_convertTaskFrequency_AllBranches(t *testing.T) {
	t.Run("nil_sampler", func(t *testing.T) {
		result := convertTaskFrequency(nil, nil)
		assert.Nil(t, result)
	})

	t.Run("not_cycle", func(t *testing.T) {
		sampler := &taskdomain.Sampler{IsCycle: gptr.Of(false)}
		result := convertTaskFrequency(sampler, nil)
		assert.Nil(t, result)
	})

	t.Run("cycle_day", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitDay),
		}
		result := convertTaskFrequency(sampler, nil)
		assert.NotNil(t, result)
		assert.Equal(t, "every_day", *result)
	})

	t.Run("cycle_null_unit", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitNull),
		}
		result := convertTaskFrequency(sampler, nil)
		assert.NotNil(t, result)
		assert.Equal(t, "every_day", *result)
	})

	t.Run("cycle_empty_unit", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle: gptr.Of(true),
		}
		result := convertTaskFrequency(sampler, nil)
		assert.NotNil(t, result)
		assert.Equal(t, "every_day", *result)
	})

	t.Run("cycle_week_no_effective_time", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitWeek),
		}
		result := convertTaskFrequency(sampler, nil)
		assert.Nil(t, result)
	})

	t.Run("cycle_week_zero_start", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitWeek),
		}
		et := &taskdomain.EffectiveTime{StartAt: gptr.Of(int64(0))}
		result := convertTaskFrequency(sampler, et)
		assert.Nil(t, result)
	})

	t.Run("cycle_week_monday", func(t *testing.T) {
		monday := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitWeek),
		}
		et := &taskdomain.EffectiveTime{StartAt: gptr.Of(monday.UnixMilli())}
		result := convertTaskFrequency(sampler, et)
		assert.NotNil(t, result)
		assert.Equal(t, "monday", *result)
	})

	t.Run("cycle_week_tuesday", func(t *testing.T) {
		tuesday := time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitWeek),
		}
		et := &taskdomain.EffectiveTime{StartAt: gptr.Of(tuesday.UnixMilli())}
		result := convertTaskFrequency(sampler, et)
		assert.NotNil(t, result)
		assert.Equal(t, "tuesday", *result)
	})

	t.Run("cycle_week_wednesday", func(t *testing.T) {
		wednesday := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitWeek),
		}
		et := &taskdomain.EffectiveTime{StartAt: gptr.Of(wednesday.UnixMilli())}
		result := convertTaskFrequency(sampler, et)
		assert.NotNil(t, result)
		assert.Equal(t, "wednesday", *result)
	})

	t.Run("cycle_week_thursday", func(t *testing.T) {
		thursday := time.Date(2024, 1, 4, 12, 0, 0, 0, time.UTC)
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitWeek),
		}
		et := &taskdomain.EffectiveTime{StartAt: gptr.Of(thursday.UnixMilli())}
		result := convertTaskFrequency(sampler, et)
		assert.NotNil(t, result)
		assert.Equal(t, "thursday", *result)
	})

	t.Run("cycle_week_friday", func(t *testing.T) {
		friday := time.Date(2024, 1, 5, 12, 0, 0, 0, time.UTC)
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitWeek),
		}
		et := &taskdomain.EffectiveTime{StartAt: gptr.Of(friday.UnixMilli())}
		result := convertTaskFrequency(sampler, et)
		assert.NotNil(t, result)
		assert.Equal(t, "friday", *result)
	})

	t.Run("cycle_week_saturday", func(t *testing.T) {
		saturday := time.Date(2024, 1, 6, 12, 0, 0, 0, time.UTC)
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitWeek),
		}
		et := &taskdomain.EffectiveTime{StartAt: gptr.Of(saturday.UnixMilli())}
		result := convertTaskFrequency(sampler, et)
		assert.NotNil(t, result)
		assert.Equal(t, "saturday", *result)
	})

	t.Run("cycle_week_sunday", func(t *testing.T) {
		sunday := time.Date(2024, 1, 7, 12, 0, 0, 0, time.UTC)
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of(taskdomain.TimeUnitWeek),
		}
		et := &taskdomain.EffectiveTime{StartAt: gptr.Of(sunday.UnixMilli())}
		result := convertTaskFrequency(sampler, et)
		assert.NotNil(t, result)
		assert.Equal(t, "sunday", *result)
	})

	t.Run("unknown_time_unit", func(t *testing.T) {
		sampler := &taskdomain.Sampler{
			IsCycle:       gptr.Of(true),
			CycleTimeUnit: gptr.Of("month"),
		}
		result := convertTaskFrequency(sampler, nil)
		assert.Nil(t, result)
	})
}

func Test_spanFilterFieldsFromTaskRule_WithFilters(t *testing.T) {
	t.Run("nil_input", func(t *testing.T) {
		result := spanFilterFieldsFromTaskRule(nil)
		assert.Nil(t, result)
	})

	t.Run("with_platform_and_span_list_type", func(t *testing.T) {
		pt := observability_common.PlatformType("web")
		slt := observability_common.SpanListType("root")
		sf := &taskfilter.SpanFilterFields{
			PlatformType: &pt,
			SpanListType: &slt,
		}
		result := spanFilterFieldsFromTaskRule(sf)
		assert.NotNil(t, result)
		assert.Equal(t, "web", *result.PlatformType)
		assert.Equal(t, "root", *result.SpanListType)
		assert.Nil(t, result.Filters)
	})

	t.Run("with_filters", func(t *testing.T) {
		qao := "and"
		ft := taskfilter.FieldType("string")
		sf := &taskfilter.SpanFilterFields{
			Filters: &taskfilter.FilterFields{
				QueryAndOr: &qao,
				FilterFields: []*taskfilter.FilterField{
					{
						FieldName: gptr.Of("f1"),
						FieldType: &ft,
						Values:    []string{"v1"},
					},
				},
			},
		}
		result := spanFilterFieldsFromTaskRule(sf)
		assert.NotNil(t, result)
		assert.NotNil(t, result.Filters)
		assert.Equal(t, "and", *result.Filters.QueryAndOr)
		assert.Len(t, result.Filters.FilterFields, 1)
		assert.Equal(t, "f1", *result.Filters.FilterFields[0].FieldName)
	})
}

func Test_evaluateFieldMappingsToIngressConf_EdgeCases(t *testing.T) {
	t.Run("nil_mapping_in_list", func(t *testing.T) {
		mappings := []*taskdomain.EvaluateFieldMapping{nil}
		result := evaluateFieldMappingsToIngressConf(mappings)
		assert.NotNil(t, result)
		assert.Empty(t, result.EvalSetAdapter.FieldConfs)
	})

	t.Run("nil_FieldSchema", func(t *testing.T) {
		mappings := []*taskdomain.EvaluateFieldMapping{
			{TraceFieldKey: "tk", FieldSchema: nil},
		}
		result := evaluateFieldMappingsToIngressConf(mappings)
		assert.NotNil(t, result)
		assert.Empty(t, result.EvalSetAdapter.FieldConfs)
	})

	t.Run("FieldSchema_with_empty_key_and_nil_name", func(t *testing.T) {
		mappings := []*taskdomain.EvaluateFieldMapping{
			{
				TraceFieldKey: "tk",
				FieldSchema: &observability_dataset.FieldSchema{
					Key:  gptr.Of(""),
					Name: nil,
				},
			},
		}
		result := evaluateFieldMappingsToIngressConf(mappings)
		assert.NotNil(t, result)
		assert.Empty(t, result.EvalSetAdapter.FieldConfs)
	})

	t.Run("FieldSchema_key_nil_name_empty", func(t *testing.T) {
		mappings := []*taskdomain.EvaluateFieldMapping{
			{
				TraceFieldKey: "tk",
				FieldSchema: &observability_dataset.FieldSchema{
					Key:  nil,
					Name: gptr.Of(""),
				},
			},
		}
		result := evaluateFieldMappingsToIngressConf(mappings)
		assert.NotNil(t, result)
		assert.Empty(t, result.EvalSetAdapter.FieldConfs)
	})

	t.Run("trace_field_key_used_when_no_eval_set_name", func(t *testing.T) {
		mappings := []*taskdomain.EvaluateFieldMapping{
			{
				TraceFieldKey: "trace_key_1",
				FieldSchema: &observability_dataset.FieldSchema{
					Key: gptr.Of("field_key"),
				},
			},
		}
		result := evaluateFieldMappingsToIngressConf(mappings)
		assert.NotNil(t, result)
		assert.Len(t, result.EvalSetAdapter.FieldConfs, 1)
		assert.Equal(t, "trace_key_1", result.EvalSetAdapter.FieldConfs[0].FromField)
		assert.Equal(t, "field_key", result.EvalSetAdapter.FieldConfs[0].FieldName)
	})
}

func Test_extractSpanFilterFieldsFromPipeline_NodeBranches(t *testing.T) {
	t.Run("nil_pipeline", func(t *testing.T) {
		result := extractSpanFilterFieldsFromPipeline(nil)
		assert.Nil(t, result)
	})

	t.Run("nil_flow", func(t *testing.T) {
		p := &entity.Pipeline{Flow: nil}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.Nil(t, result)
	})

	t.Run("empty_nodes", func(t *testing.T) {
		p := &entity.Pipeline{Flow: &entity.FlowSchema{Nodes: []*entity.Node{}}}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.Nil(t, result)
	})

	t.Run("nil_node_in_list", func(t *testing.T) {
		p := &entity.Pipeline{Flow: &entity.FlowSchema{Nodes: []*entity.Node{nil}}}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.Nil(t, result)
	})

	t.Run("non_data_reflow_node", func(t *testing.T) {
		p := &entity.Pipeline{
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{NodeTemplateType: "other_type"},
				},
			},
		}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.Nil(t, result)
	})

	t.Run("data_reflow_node_nil_refs", func(t *testing.T) {
		p := &entity.Pipeline{
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{NodeTemplateType: "data_reflow", Refs: nil},
				},
			},
		}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.Nil(t, result)
	})

	t.Run("data_reflow_node_no_task_ref", func(t *testing.T) {
		p := &entity.Pipeline{
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{
						NodeTemplateType: "data_reflow",
						Refs: map[string]*entity.NodeRef{
							"other": {Content: "{}"},
						},
					},
				},
			},
		}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.Nil(t, result)
	})

	t.Run("data_reflow_node_nil_task_ref", func(t *testing.T) {
		p := &entity.Pipeline{
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{
						NodeTemplateType: "data_reflow",
						Refs: map[string]*entity.NodeRef{
							"task": nil,
						},
					},
				},
			},
		}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.Nil(t, result)
	})

	t.Run("data_reflow_node_empty_content", func(t *testing.T) {
		p := &entity.Pipeline{
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{
						NodeTemplateType: "data_reflow",
						Refs: map[string]*entity.NodeRef{
							"task": {Content: ""},
						},
					},
				},
			},
		}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.Nil(t, result)
	})

	t.Run("data_reflow_node_with_valid_task_json", func(t *testing.T) {
		taskJSON := `{"rule":{"span_filters":{"platform_type":"TCE","span_list_type":"root"}}}`
		p := &entity.Pipeline{
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{
						NodeTemplateType: "data_reflow",
						Refs: map[string]*entity.NodeRef{
							"task": {Content: taskJSON},
						},
					},
				},
			},
		}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.NotNil(t, result)
		assert.Equal(t, "TCE", *result.PlatformType)
		assert.Equal(t, "root", *result.SpanListType)
	})

	t.Run("data_reflow_node_invalid_json", func(t *testing.T) {
		p := &entity.Pipeline{
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{
						NodeTemplateType: "data_reflow",
						Refs: map[string]*entity.NodeRef{
							"task": {Content: "not-json"},
						},
					},
				},
			},
		}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.Nil(t, result)
	})

	t.Run("skips_non_data_reflow_then_finds_data_reflow", func(t *testing.T) {
		taskJSON := `{"rule":{"span_filters":{"platform_type":"web"}}}`
		p := &entity.Pipeline{
			Flow: &entity.FlowSchema{
				Nodes: []*entity.Node{
					{NodeTemplateType: "other"},
					nil,
					{
						NodeTemplateType: "data_reflow",
						Refs: map[string]*entity.NodeRef{
							"task": {Content: taskJSON},
						},
					},
				},
			},
		}
		result := extractSpanFilterFieldsFromPipeline(p)
		assert.NotNil(t, result)
		assert.Equal(t, "web", *result.PlatformType)
	})
}

func Test_parseSpanFilterFieldsFromTaskJSON_EdgeCases(t *testing.T) {
	t.Run("invalid_json", func(t *testing.T) {
		result := parseSpanFilterFieldsFromTaskJSON("{bad json")
		assert.Nil(t, result)
	})

	t.Run("nil_rule", func(t *testing.T) {
		result := parseSpanFilterFieldsFromTaskJSON(`{}`)
		assert.Nil(t, result)
	})

	t.Run("nil_span_filters", func(t *testing.T) {
		result := parseSpanFilterFieldsFromTaskJSON(`{"rule":{}}`)
		assert.Nil(t, result)
	})

	t.Run("empty_filter_fields_in_filters", func(t *testing.T) {
		taskJSON := `{"rule":{"span_filters":{"filters":{"query_and_or":"and","filter_fields":[]}}}}`
		result := parseSpanFilterFieldsFromTaskJSON(taskJSON)
		assert.NotNil(t, result)
		assert.NotNil(t, result.Filters)
		assert.Equal(t, "and", *result.Filters.QueryAndOr)
		assert.Empty(t, result.Filters.FilterFields)
	})
}

func TestExptTemplateManagerImpl_ListOnline_MgetTupleForTaskError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockTaskAdapter := mocks.NewMockITaskRPCAdapter(ctrl)
	mockPipelineAdapter := mocks.NewMockIPipelineListAdapter(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:                mockRepo,
		taskRPCAdapter:              mockTaskAdapter,
		pipelineRPCAdapter:          mockPipelineAdapter,
		evaluatorService:            mockEvalSvc,
		evalTargetService:           mockTargetSvc,
		evaluationSetService:        mockEvalSetSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
	}

	ctx := context.Background()
	spaceID := int64(100)
	session := &entity.Session{UserID: "u1"}

	task := &taskdomain.Task{
		ID:   gptr.Of(int64(1)),
		Name: "task1",
		TaskConfig: &taskdomain.TaskConfig{
			AutoEvaluateConfigs: []*taskdomain.AutoEvaluateConfig{
				{EvaluatorID: 1, EvaluatorVersionID: 101},
			},
		},
	}
	mockTaskAdapter.EXPECT().ListTasks(ctx, gomock.Any()).Return([]*taskdomain.Task{task}, gptr.Of(int64(1)), nil)
	mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, errors.New("batch eval error"))
	mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	_, _, err := mgr.ListOnline(ctx, 1, 10, spaceID, nil, nil, session)
	assert.Error(t, err)
}

func TestExptTemplateManagerImpl_ListOnline_MgetTupleForDBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockTaskAdapter := mocks.NewMockITaskRPCAdapter(ctrl)
	mockPipelineAdapter := mocks.NewMockIPipelineListAdapter(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:                mockRepo,
		taskRPCAdapter:              mockTaskAdapter,
		pipelineRPCAdapter:          mockPipelineAdapter,
		evaluatorService:            mockEvalSvc,
		evalTargetService:           mockTargetSvc,
		evaluationSetService:        mockEvalSetSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
	}

	ctx := context.Background()
	spaceID := int64(100)
	session := &entity.Session{UserID: "u1"}

	mockTaskAdapter.EXPECT().ListTasks(ctx, gomock.Any()).Return([]*taskdomain.Task{}, gptr.Of(int64(0)), nil)

	dbTpl := &entity.ExptTemplate{
		Meta:         &entity.ExptTemplateMeta{ID: 2, WorkspaceID: spaceID, ExptType: entity.ExptType_Online},
		TripleConfig: &entity.ExptTemplateTuple{EvalSetID: 10, EvalSetVersionID: 11, EvaluatorVersionIds: []int64{101}},
	}
	mockRepo.EXPECT().List(ctx, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTemplate{dbTpl}, int64(1), nil)
	mockEvalSvc.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, gomock.Any(), true).Return(nil, errors.New("batch eval error"))
	mockEvalSetVerSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	mockEvalSetSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	mockTargetSvc.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), spaceID, gomock.Any(), true).Return(nil, nil).AnyTimes()

	_, _, err := mgr.ListOnline(ctx, 1, 10, spaceID, nil, nil, session)
	assert.Error(t, err)
}

func TestExptTemplateManagerImpl_ListOnline_NilTaskInList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockIExptTemplateRepo(ctrl)
	mockTaskAdapter := mocks.NewMockITaskRPCAdapter(ctrl)
	mockEvalSvc := svcmocks.NewMockEvaluatorService(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalSetSvc := svcmocks.NewMockIEvaluationSetService(ctrl)
	mockEvalSetVerSvc := svcmocks.NewMockEvaluationSetVersionService(ctrl)

	mgr := &ExptTemplateManagerImpl{
		templateRepo:                mockRepo,
		taskRPCAdapter:              mockTaskAdapter,
		evaluatorService:            mockEvalSvc,
		evalTargetService:           mockTargetSvc,
		evaluationSetService:        mockEvalSetSvc,
		evaluationSetVersionService: mockEvalSetVerSvc,
	}

	ctx := context.Background()
	spaceID := int64(100)
	session := &entity.Session{UserID: "u1"}

	mockTaskAdapter.EXPECT().ListTasks(ctx, gomock.Any()).Return([]*taskdomain.Task{nil, {ID: nil}}, gptr.Of(int64(2)), nil)
	mockRepo.EXPECT().List(ctx, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTemplate{}, int64(0), nil)

	templates, total, err := mgr.ListOnline(ctx, 1, 10, spaceID, nil, nil, session)
	assert.NoError(t, err)
	assert.Len(t, templates, 0)
	assert.Equal(t, int64(0), total)
}

func TestExptTemplateManagerImpl_enrichExptSourceFromPipeline_NilAdapter(t *testing.T) {
	mgr := &ExptTemplateManagerImpl{pipelineRPCAdapter: nil}
	templates := []*entity.ExptTemplate{
		{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "1"}},
	}
	err := mgr.enrichExptSourceFromPipeline(context.Background(), templates, 100)
	assert.NoError(t, err)
}

func TestExptTemplateManagerImpl_enrichExptSourceFromPipeline_NoPipelineIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPipelineAdapter := mocks.NewMockIPipelineListAdapter(ctrl)
	mgr := &ExptTemplateManagerImpl{pipelineRPCAdapter: mockPipelineAdapter}

	templates := []*entity.ExptTemplate{
		{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_AutoTask, SourceID: "1"}},
		{ExptSource: nil},
		{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: ""}},
		{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "not_a_number"}},
	}
	err := mgr.enrichExptSourceFromPipeline(context.Background(), templates, 100)
	assert.NoError(t, err)
}

func TestExptTemplateManagerImpl_enrichExptSourceFromPipeline_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPipelineAdapter := mocks.NewMockIPipelineListAdapter(ctrl)
	mgr := &ExptTemplateManagerImpl{pipelineRPCAdapter: mockPipelineAdapter}

	ctx := context.Background()
	templates := []*entity.ExptTemplate{
		{ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Workflow, SourceID: "1"}},
	}
	mockPipelineAdapter.EXPECT().ListPipelineFlow(ctx, gomock.Any()).Return(nil, errors.New("rpc error"))
	err := mgr.enrichExptSourceFromPipeline(ctx, templates, 100)
	assert.Error(t, err)
}

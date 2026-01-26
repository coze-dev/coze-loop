// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	idgenMocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	lwtMocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	rpcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/utils"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestExptResultServiceImpl_MGetStats(t *testing.T) {
	tests := []struct {
		name    string
		exptIDs []int64
		spaceID int64
		session *entity.Session
		setup   func(mockExptStatsRepo *repoMocks.MockIExptStatsRepo)
		want    []*entity.ExptStats
		wantErr bool
	}{
		{
			name:    "正常获取多个实验统计",
			exptIDs: []int64{1, 2},
			spaceID: 100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptStatsRepo *repoMocks.MockIExptStatsRepo) {
				mockExptStatsRepo.EXPECT().
					MGet(gomock.Any(), []int64{1, 2}, int64(100)).
					Return([]*entity.ExptStats{
						{
							ID:      1,
							ExptID:  1,
							SpaceID: 100,
						},
						{
							ID:      2,
							ExptID:  2,
							SpaceID: 100,
						},
					}, nil).
					Times(1)
			},
			want: []*entity.ExptStats{
				{
					ID:      1,
					ExptID:  1,
					SpaceID: 100,
				},
				{
					ID:      2,
					ExptID:  2,
					SpaceID: 100,
				},
			},
			wantErr: false,
		},
		{
			name:    "获取空列表",
			exptIDs: []int64{},
			spaceID: 100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptStatsRepo *repoMocks.MockIExptStatsRepo) {
				mockExptStatsRepo.EXPECT().
					MGet(gomock.Any(), []int64{}, int64(100)).
					Return([]*entity.ExptStats{}, nil).
					Times(1)
			},
			want:    []*entity.ExptStats{},
			wantErr: false,
		},
		{
			name:    "数据库错误",
			exptIDs: []int64{1},
			spaceID: 100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptStatsRepo *repoMocks.MockIExptStatsRepo) {
				mockExptStatsRepo.EXPECT().
					MGet(gomock.Any(), []int64{1}, int64(100)).
					Return(nil, fmt.Errorf("db error")).
					Times(1)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
			svc := ExptResultServiceImpl{
				ExptStatsRepo: mockExptStatsRepo,
			}

			tt.setup(mockExptStatsRepo)

			got, err := svc.MGetStats(context.Background(), tt.exptIDs, tt.spaceID, tt.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("MGetStats() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("MGetStats() got length = %v, want %v", len(got), len(tt.want))
					return
				}
				for i := range got {
					if got[i].ID != tt.want[i].ID {
						t.Errorf("MGetStats() got[%d].ID = %v, want %v", i, got[i].ID, tt.want[i].ID)
					}
					if got[i].ExptID != tt.want[i].ExptID {
						t.Errorf("MGetStats() got[%d].ExptID = %v, want %v", i, got[i].ExptID, tt.want[i].ExptID)
					}
					if got[i].SpaceID != tt.want[i].SpaceID {
						t.Errorf("MGetStats() got[%d].SpaceID = %v, want %v", i, got[i].SpaceID, tt.want[i].SpaceID)
					}
				}
			}
		})
	}
}

func TestExptResultServiceImpl_GetStats(t *testing.T) {
	tests := []struct {
		name    string
		exptID  int64
		spaceID int64
		session *entity.Session
		setup   func(mockExptStatsRepo *repoMocks.MockIExptStatsRepo)
		want    *entity.ExptStats
		wantErr bool
	}{
		{
			name:    "正常获取单个实验统计",
			exptID:  1,
			spaceID: 100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptStatsRepo *repoMocks.MockIExptStatsRepo) {
				mockExptStatsRepo.EXPECT().
					MGet(gomock.Any(), []int64{1}, int64(100)).
					Return([]*entity.ExptStats{
						{
							ID:      1,
							ExptID:  1,
							SpaceID: 100,
						},
					}, nil).
					Times(1)
			},
			want: &entity.ExptStats{
				ID:      1,
				ExptID:  1,
				SpaceID: 100,
			},
			wantErr: false,
		},
		{
			name:    "数据库错误",
			exptID:  1,
			spaceID: 100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptStatsRepo *repoMocks.MockIExptStatsRepo) {
				mockExptStatsRepo.EXPECT().
					MGet(gomock.Any(), []int64{1}, int64(100)).
					Return(nil, fmt.Errorf("db error")).
					Times(1)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
			svc := ExptResultServiceImpl{
				ExptStatsRepo: mockExptStatsRepo,
			}

			tt.setup(mockExptStatsRepo)

			got, err := svc.GetStats(context.Background(), tt.exptID, tt.spaceID, tt.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetStats() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.ID != tt.want.ID {
					t.Errorf("GetStats() got.ID = %v, want %v", got.ID, tt.want.ID)
				}
				if got.ExptID != tt.want.ExptID {
					t.Errorf("GetStats() got.ExptID = %v, want %v", got.ExptID, tt.want.ExptID)
				}
				if got.SpaceID != tt.want.SpaceID {
					t.Errorf("GetStats() got.SpaceID = %v, want %v", got.SpaceID, tt.want.SpaceID)
				}
			}
		})
	}
}

func TestExptResultServiceImpl_CreateStats(t *testing.T) {
	tests := []struct {
		name    string
		stats   *entity.ExptStats
		session *entity.Session
		setup   func(mockExptStatsRepo *repoMocks.MockIExptStatsRepo)
		wantErr bool
	}{
		{
			name: "正常创建统计",
			stats: &entity.ExptStats{
				ID:      1,
				ExptID:  1,
				SpaceID: 100,
			},
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptStatsRepo *repoMocks.MockIExptStatsRepo) {
				mockExptStatsRepo.EXPECT().
					Create(gomock.Any(), &entity.ExptStats{
						ID:      1,
						ExptID:  1,
						SpaceID: 100,
					}).
					Return(nil).
					Times(1)
			},
			wantErr: false,
		},
		{
			name: "数据库错误",
			stats: &entity.ExptStats{
				ID:      1,
				ExptID:  1,
				SpaceID: 100,
			},
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptStatsRepo *repoMocks.MockIExptStatsRepo) {
				mockExptStatsRepo.EXPECT().
					Create(gomock.Any(), &entity.ExptStats{
						ID:      1,
						ExptID:  1,
						SpaceID: 100,
					}).
					Return(fmt.Errorf("db error")).
					Times(1)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
			svc := ExptResultServiceImpl{
				ExptStatsRepo: mockExptStatsRepo,
			}

			tt.setup(mockExptStatsRepo)

			err := svc.CreateStats(context.Background(), tt.stats, tt.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateStats() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptResultServiceImpl_getExptColumnsEvalTarget(t *testing.T) {
	t.Run("skip experiments without eval target", func(t *testing.T) {
		svc := ExptResultServiceImpl{}
		expts := []*entity.Experiment{
			{
				ID:              1,
				TargetVersionID: 0, // ContainsEvalTarget == false
			},
		}

		got, err := svc.getExptColumnsEvalTarget(context.Background(), int64(100), expts, false)
		assert.NoError(t, err)
		assert.Len(t, got, 0)
	})

	t.Run("experiment with eval target but without trajectory support", func(t *testing.T) {
		svc := ExptResultServiceImpl{}
		expts := []*entity.Experiment{
			{
				ID:              2,
				TargetVersionID: 1,                            // ContainsEvalTarget == true
				TargetType:      entity.EvalTargetTypeCozeBot, // SupptTrajectory == false
			},
		}

		got, err := svc.getExptColumnsEvalTarget(context.Background(), int64(100), expts, false)
		assert.NoError(t, err)
		if assert.Len(t, got, 1) {
			assert.Equal(t, int64(2), got[0].ExptID)
			// actual_output + 4 metrics
			assert.Len(t, got[0].Columns, 1+len(columnsEvalTargetMtr))
			assert.Equal(t, consts.ReportColumnNameEvalTargetActualOutput, got[0].Columns[0].Name)

			// should not contain trajectory column
			for _, c := range got[0].Columns {
				assert.NotEqual(t, consts.ReportColumnNameEvalTargetTrajectory, c.Name)
			}
		}
	})

	t.Run("experiment with eval target and trajectory support, fullTrajectory=true", func(t *testing.T) {
		svc := ExptResultServiceImpl{}
		expts := []*entity.Experiment{
			{
				ID:              3,
				TargetVersionID: 1,                                    // ContainsEvalTarget == true
				TargetType:      entity.EvalTargetTypeVolcengineAgent, // SupptTrajectory == true
			},
		}

		got, err := svc.getExptColumnsEvalTarget(context.Background(), int64(100), expts, true)
		assert.NoError(t, err)
		if assert.Len(t, got, 1) {
			assert.Equal(t, int64(3), got[0].ExptID)
			// actual_output + trajectory + 4 metrics
			assert.Len(t, got[0].Columns, 1+1+len(columnsEvalTargetMtr))
			assert.Equal(t, consts.ReportColumnNameEvalTargetActualOutput, got[0].Columns[0].Name)
			assert.Equal(t, consts.ReportColumnNameEvalTargetTrajectory, got[0].Columns[1].Name)
		}
	})

	t.Run("experiment with eval target and trajectory support, fullTrajectory=false", func(t *testing.T) {
		svc := ExptResultServiceImpl{}
		expts := []*entity.Experiment{
			{
				ID:              4,
				TargetVersionID: 1,                                    // ContainsEvalTarget == true
				TargetType:      entity.EvalTargetTypeVolcengineAgent, // SupptTrajectory == true
			},
		}

		got, err := svc.getExptColumnsEvalTarget(context.Background(), int64(100), expts, false)
		assert.NoError(t, err)
		if assert.Len(t, got, 1) {
			assert.Equal(t, int64(4), got[0].ExptID)
			// actual_output + trajectory + 4 metrics（只要 SupptTrajectory=true 就会返回 trajectory 列，不受 fullTrajectory 参数影响）
			assert.Len(t, got[0].Columns, 1+1+len(columnsEvalTargetMtr))
			assert.Equal(t, consts.ReportColumnNameEvalTargetActualOutput, got[0].Columns[0].Name)
			assert.Equal(t, consts.ReportColumnNameEvalTargetTrajectory, got[0].Columns[1].Name)
		}
	})
}

func TestExptResultServiceImpl_GetExptItemTurnResults(t *testing.T) {
	tests := []struct {
		name    string
		exptID  int64
		itemID  int64
		spaceID int64
		session *entity.Session
		setup   func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo)
		want    []*entity.ExptTurnResult
		wantErr bool
	}{
		{
			name:    "正常获取实验结果",
			exptID:  1,
			itemID:  1,
			spaceID: 100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo) {
				// 设置 GetItemTurnResults 的 mock
				mockExptTurnResultRepo.EXPECT().
					GetItemTurnResults(gomock.Any(), int64(1), int64(1), int64(100)).
					Return([]*entity.ExptTurnResult{
						{
							ID:     1,
							ExptID: 1,
							ItemID: 1,
						},
					}, nil).
					Times(1)

				// 设置 BatchGetTurnEvaluatorResultRef 的 mock
				mockExptTurnResultRepo.EXPECT().
					BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(100), []int64{1}).
					Return([]*entity.ExptTurnEvaluatorResultRef{
						{
							ExptTurnResultID:   1,
							EvaluatorVersionID: 1,
						},
					}, nil).
					Times(1)
			},
			want: []*entity.ExptTurnResult{
				{
					ID:     1,
					ExptID: 1,
					ItemID: 1,
					EvaluatorResults: &entity.EvaluatorResults{
						EvalVerIDToResID: map[int64]int64{
							1: 1,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "获取空结果",
			exptID:  1,
			itemID:  1,
			spaceID: 100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo) {
				mockExptTurnResultRepo.EXPECT().
					GetItemTurnResults(gomock.Any(), int64(1), int64(1), int64(100)).
					Return([]*entity.ExptTurnResult{}, nil).
					Times(1)

				mockExptTurnResultRepo.EXPECT().
					BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(100), []int64{}).
					Return([]*entity.ExptTurnEvaluatorResultRef{}, nil).
					Times(1)
			},
			want:    []*entity.ExptTurnResult{},
			wantErr: false,
		},
		{
			name:    "数据库错误",
			exptID:  1,
			itemID:  1,
			spaceID: 100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo) {
				mockExptTurnResultRepo.EXPECT().
					GetItemTurnResults(gomock.Any(), int64(1), int64(1), int64(100)).
					Return(nil, fmt.Errorf("db error")).
					Times(1)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
			svc := ExptResultServiceImpl{
				ExptTurnResultRepo: mockExptTurnResultRepo,
			}

			tt.setup(mockExptTurnResultRepo)

			got, err := svc.GetExptItemTurnResults(context.Background(), tt.exptID, tt.itemID, tt.spaceID, tt.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetExptItemTurnResults() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("GetExptItemTurnResults() got length = %v, want %v", len(got), len(tt.want))
					return
				}
				for i := range got {
					if got[i].ID != tt.want[i].ID {
						t.Errorf("GetExptItemTurnResults() got[%d].ID = %v, want %v", i, got[i].ID, tt.want[i].ID)
					}
					if got[i].ExptID != tt.want[i].ExptID {
						t.Errorf("GetExptItemTurnResults() got[%d].ExptID = %v, want %v", i, got[i].ExptID, tt.want[i].ExptID)
					}
					if got[i].ItemID != tt.want[i].ItemID {
						t.Errorf("GetExptItemTurnResults() got[%d].ItemID = %v, want %v", i, got[i].ItemID, tt.want[i].ItemID)
					}
				}
			}
		})
	}
}

func TestExptResultServiceImpl_CalculateStats(t *testing.T) {
	tests := []struct {
		name    string
		exptID  int64
		spaceID int64
		session *entity.Session
		setup   func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockExptItemResultRepo *repoMocks.MockIExptItemResultRepo)
		want    *entity.ExptCalculateStats
		wantErr bool
	}{
		{
			name:    "正常计算统计",
			exptID:  1,
			spaceID: 100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockExptItemResultRepo *repoMocks.MockIExptItemResultRepo) {
				mockExptTurnResultRepo.EXPECT().
					ListTurnResult(
						gomock.Any(),
						int64(100),
						int64(1),
						gomock.Any(),
						gomock.Any(),
						false,
					).
					Return([]*entity.ExptTurnResult{
						{
							ID:     1,
							Status: int32(entity.TurnRunState_Success),
						},
						{
							ID:     2,
							Status: int32(entity.TurnRunState_Fail),
						},
					}, int64(2), nil).
					Times(1)
				mockExptItemResultRepo.EXPECT().
					ListItemResultsByExptID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*entity.ExptItemResult{
						{
							ItemID: 1,
							Status: entity.ItemRunState_Success,
						},
						{
							ItemID: 2,
							Status: entity.ItemRunState_Fail,
						},
					}, int64(2), nil).
					AnyTimes()
			},
			want: &entity.ExptCalculateStats{
				SuccessItemCnt: 1,
				FailItemCnt:    1,
			},
			wantErr: false,
		},
		{
			name:    "数据库错误",
			exptID:  1,
			spaceID: 100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockExptItemResultRepo *repoMocks.MockIExptItemResultRepo) {
				mockExptItemResultRepo.EXPECT().
					ListItemResultsByExptID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, int64(0), fmt.Errorf("db error")).
					Times(1)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "处理中状态",
			exptID:  1,
			spaceID: 100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockExptItemResultRepo *repoMocks.MockIExptItemResultRepo) {
				mockExptTurnResultRepo.EXPECT().
					ListTurnResult(
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
					).
					Return([]*entity.ExptTurnResult{
						{
							ID:     1,
							Status: int32(entity.TurnRunState_Processing),
						},
						{
							ID:     2,
							Status: int32(entity.TurnRunState_Queueing),
						},
					}, int64(2), nil).
					Times(1)
				mockExptItemResultRepo.EXPECT().
					ListItemResultsByExptID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*entity.ExptItemResult{
						{
							ItemID: 1,
							Status: entity.ItemRunState_Processing,
						},
						{
							ItemID: 2,
							Status: entity.ItemRunState_Queueing,
						},
					}, int64(2), nil).
					AnyTimes()
			},
			want: &entity.ExptCalculateStats{
				ProcessingItemCnt: 1,
				PendingItemCnt:    1,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
			mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
			svc := ExptResultServiceImpl{
				ExptTurnResultRepo: mockExptTurnResultRepo,
				ExptItemResultRepo: mockExptItemResultRepo,
			}

			tt.setup(mockExptTurnResultRepo, mockExptItemResultRepo)

			got, err := svc.CalculateStats(context.Background(), tt.exptID, tt.spaceID, tt.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("CalculateStats() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.SuccessItemCnt != tt.want.SuccessItemCnt {
					t.Errorf("CalculateStats() got.SuccessItemCnt = %v, want %v", got.SuccessItemCnt, tt.want.SuccessItemCnt)
				}
				if got.FailItemCnt != tt.want.FailItemCnt {
					t.Errorf("CalculateStats() got.FailItemCnt = %v, want %v", got.FailItemCnt, tt.want.FailItemCnt)
				}
				if got.ProcessingItemCnt != tt.want.ProcessingItemCnt {
					t.Errorf("CalculateStats() got.ProcessingItemCnt = %v, want %v", got.ProcessingItemCnt, tt.want.ProcessingItemCnt)
				}
				if got.PendingItemCnt != tt.want.PendingItemCnt {
					t.Errorf("CalculateStats() got.PendingItemCnt = %v, want %v", got.PendingItemCnt, tt.want.PendingItemCnt)
				}
			}
		})
	}
}

func TestExptResultServiceImpl_GetIncompleteTurns(t *testing.T) {
	tests := []struct {
		name    string
		exptID  int64
		spaceID int64
		session *entity.Session
		setup   func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo)
		want    []*entity.ItemTurnID
		wantErr bool
	}{
		{
			name:    "successful_get_incomplete_turns",
			exptID:  1,
			spaceID: 100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo) {
				// First page with queueing and processing turns
				mockExptTurnResultRepo.EXPECT().
					ListTurnResult(gomock.Any(), int64(100), int64(1), nil, gomock.Any(), false).
					Return([]*entity.ExptTurnResult{
						{
							TurnID: 1,
							ItemID: 10,
							Status: int32(entity.TurnRunState_Queueing),
						},
						{
							TurnID: 2,
							ItemID: 20,
							Status: int32(entity.TurnRunState_Processing),
						},
						{
							TurnID: 3,
							ItemID: 30,
							Status: int32(entity.TurnRunState_Success),
						},
					}, int64(3), nil).
					Times(1)
			},
			want: []*entity.ItemTurnID{
				{TurnID: 1, ItemID: 10},
				{TurnID: 2, ItemID: 20},
			},
			wantErr: false,
		},
		{
			name:    "no_incomplete_turns",
			exptID:  1,
			spaceID: 100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo) {
				mockExptTurnResultRepo.EXPECT().
					ListTurnResult(gomock.Any(), int64(100), int64(1), nil, gomock.Any(), false).
					Return([]*entity.ExptTurnResult{
						{
							TurnID: 1,
							ItemID: 10,
							Status: int32(entity.TurnRunState_Success),
						},
						{
							TurnID: 2,
							ItemID: 20,
							Status: int32(entity.TurnRunState_Fail),
						},
					}, int64(2), nil).
					Times(1)
			},
			want:    []*entity.ItemTurnID{},
			wantErr: false,
		},
		{
			name:    "multiple_pages_with_incomplete_turns",
			exptID:  1,
			spaceID: 100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo) {
				// First page
				mockExptTurnResultRepo.EXPECT().
					ListTurnResult(gomock.Any(), int64(100), int64(1), nil, gomock.Any(), false).
					Return([]*entity.ExptTurnResult{
						{
							TurnID: 1,
							ItemID: 10,
							Status: int32(entity.TurnRunState_Queueing),
						},
					}, int64(3), nil).
					Times(1)

				// Second page
				mockExptTurnResultRepo.EXPECT().
					ListTurnResult(gomock.Any(), int64(100), int64(1), nil, gomock.Any(), false).
					Return([]*entity.ExptTurnResult{
						{
							TurnID: 2,
							ItemID: 20,
							Status: int32(entity.TurnRunState_Processing),
						},
					}, int64(3), nil).
					Times(1)

				// Third page - empty result to break loop
				mockExptTurnResultRepo.EXPECT().
					ListTurnResult(gomock.Any(), int64(100), int64(1), nil, gomock.Any(), false).
					Return([]*entity.ExptTurnResult{}, int64(3), nil).
					Times(1)
			},
			want: []*entity.ItemTurnID{
				{TurnID: 1, ItemID: 10},
				{TurnID: 2, ItemID: 20},
			},
			wantErr: false,
		},
		{
			name:    "database_error",
			exptID:  1,
			spaceID: 100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo) {
				mockExptTurnResultRepo.EXPECT().
					ListTurnResult(gomock.Any(), int64(100), int64(1), nil, gomock.Any(), false).
					Return(nil, int64(0), fmt.Errorf("database connection failed")).
					Times(1)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
			svc := ExptResultServiceImpl{
				ExptTurnResultRepo: mockExptTurnResultRepo,
			}

			tt.setup(mockExptTurnResultRepo)

			got, err := svc.GetIncompleteTurns(context.Background(), tt.exptID, tt.spaceID, tt.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetIncompleteTurns() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != len(tt.want) {
				t.Errorf("GetIncompleteTurns() got %v incomplete turns, want %v", len(got), len(tt.want))
			}
			if !tt.wantErr {
				for i, wantTurn := range tt.want {
					if i >= len(got) {
						t.Errorf("GetIncompleteTurns() missing turn at index %d", i)
						continue
					}
					if got[i].TurnID != wantTurn.TurnID || got[i].ItemID != wantTurn.ItemID {
						t.Errorf("GetIncompleteTurns() got[%d] = {RecordID: %v, ItemID: %v}, want {RecordID: %v, ItemID: %v}",
							i, got[i].TurnID, got[i].ItemID, wantTurn.TurnID, wantTurn.ItemID)
					}
				}
			}
		})
	}
}

func TestExptResultServiceImpl_MGetExperimentResult(t *testing.T) {
	tests := []struct {
		name    string
		param   *entity.MGetExperimentResultParam
		setup   func(ctrl *gomock.Controller) ExptResultServiceImpl
		want    []*entity.ColumnEvaluator
		wantErr bool
	}{
		{
			name: "正常获取实验结果 - 无ck - 无filter",
			param: &entity.MGetExperimentResultParam{
				SpaceID: 100,
				ExptIDs: []int64{1},
			},
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
				mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
				mockMetric := metricsMocks.NewMockExptMetric(ctrl)
				mockLWT := lwtMocks.NewMockILatestWriteTracker(ctrl)
				mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
				mockExptTurnResultFilterRepo := repoMocks.NewMockIExptTurnResultFilterRepo(ctrl)
				mockEvaluatorService := svcMocks.NewMockEvaluatorService(ctrl)
				mockEvaluationSetItemService := svcMocks.NewMockEvaluationSetItemService(ctrl)
				mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)
				mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
				mockEvaluationSetService := svcMocks.NewMockIEvaluationSetService(ctrl)
				mockEvaluationSetVersionService := svcMocks.NewMockEvaluationSetVersionService(ctrl)
				mockExptAnnotateRepo := repoMocks.NewMockIExptAnnotateRepo(ctrl)
				mockTagRPCAdapter := rpcMocks.NewMockITagRPCAdapter(ctrl)

				mockExperimentRepo.EXPECT().MGetByID(gomock.Any(), []int64{1}, int64(100)).Return([]*entity.Experiment{{
					ID:               1,
					EvalSetID:        1,
					EvalSetVersionID: 1,
					ExptType:         entity.ExptType_Offline,
				}}, nil).AnyTimes()
				mockExperimentRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{EvalSetVersionID: 1}, nil).AnyTimes()
				mockExptTurnResultRepo.EXPECT().ListTurnResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{{ID: 1, ItemID: 1}}, int64(1), nil)
				mockMetric.EXPECT().EmitGetExptResult(gomock.Any(), gomock.Any()).AnyTimes()
				mockLWT.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
				mockExptStatsRepo.EXPECT().MGet(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptStats{}, nil).AnyTimes()
				mockExperimentRepo.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptEvaluatorRef{}, nil).AnyTimes()
				mockEvaluatorService.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.Evaluator{
					{
						ID:            1,
						Name:          "test_evaluator",
						Description:   "test description",
						EvaluatorType: entity.EvaluatorTypePrompt,
						PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
							ID:      1,
							Version: "v1",
						},
					},
				}, nil).AnyTimes()
				mockEvaluationSetItemService.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{}, nil).AnyTimes()
				mockEvaluatorRecordService.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvaluatorRecord{}, nil).AnyTimes()
				mockEvalTargetService.EXPECT().BatchGetRecordByIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvalTargetRecord{}, nil).AnyTimes()
				mockEvaluationSetService.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSet{}, nil).AnyTimes()
				mockEvaluationSetService.EXPECT().QueryItemSnapshotMappings(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ItemSnapshotFieldMapping{
					{
						FieldKey:      "field_key_string",
						MappingKey:    "string_map",
						MappingSubKey: "subkey_string",
					},
					{
						FieldKey:      "field_key_int",
						MappingKey:    "int_map",
						MappingSubKey: "subkey_int",
					},
					{
						FieldKey:      "field_key_float",
						MappingKey:    "float_map",
						MappingSubKey: "subkey_float",
					},
					{
						FieldKey:      "field_key_bool",
						MappingKey:    "bool_map",
						MappingSubKey: "subkey_bool",
					},
				}, "2025-01-01", nil).AnyTimes()
				mockEvaluationSetVersionService.EXPECT().GetEvaluationSetVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSetVersion{}, nil, nil).AnyTimes()
				mockExptItemResultRepo.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{}, nil).AnyTimes()
				mockExptTurnResultRepo.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnEvaluatorResultRef{}, nil).AnyTimes()
				mockExptItemResultRepo.EXPECT().GetItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{}, nil).AnyTimes()
				mockExptTurnResultFilterRepo.EXPECT().QueryItemIDStates(gomock.Any(), gomock.Any()).Return(map[int64]entity.ItemRunState{}, int64(0), nil).AnyTimes()
				mockExptTurnResultFilterRepo.EXPECT().GetExptTurnResultFilterKeyMappings(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultFilterKeyMapping{
					{
						SpaceID:   100,
						ExptID:    1,
						FromField: "1",
						ToKey:     "key1",
						FieldType: entity.FieldTypeEvaluator,
					},
				}, nil).AnyTimes()
				mockMetric.EXPECT().EmitExptTurnResultFilterQueryLatency(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				mockExptAnnotateRepo.EXPECT().BatchGetExptTurnResultTagRefs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultTagRef{
					{
						ID:       1,
						SpaceID:  1,
						ExptID:   1,
						TagKeyID: 1,
					},
				}, nil).AnyTimes()
				mockExptAnnotateRepo.EXPECT().GetExptTurnAnnotateRecordRefsByTurnResultIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnAnnotateRecordRef{}, nil).AnyTimes()
				mockExptAnnotateRepo.EXPECT().GetAnnotateRecordsByIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.AnnotateRecord{}, nil).AnyTimes()
				mockTagRPCAdapter.EXPECT().BatchGetTagInfo(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[int64]*entity.TagInfo{
					1: {
						TagKeyId:       1,
						TagKeyName:     "123",
						Description:    "123",
						InActive:       false,
						TagContentType: "",
						TagValues:      nil,
						TagContentSpec: nil,
						TagStatus:      "",
					},
				}, nil).AnyTimes()

				return ExptResultServiceImpl{
					ExptTurnResultRepo:          mockExptTurnResultRepo,
					ExperimentRepo:              mockExperimentRepo,
					ExptStatsRepo:               mockExptStatsRepo,
					Metric:                      mockMetric,
					lwt:                         mockLWT,
					ExptItemResultRepo:          mockExptItemResultRepo,
					evaluatorService:            mockEvaluatorService,
					evaluationSetItemService:    mockEvaluationSetItemService,
					evaluatorRecordService:      mockEvaluatorRecordService,
					evalTargetService:           mockEvalTargetService,
					evaluationSetService:        mockEvaluationSetService,
					evaluationSetVersionService: mockEvaluationSetVersionService,
					ExptAnnotateRepo:            mockExptAnnotateRepo,
					tagRPCAdapter:               mockTagRPCAdapter,
				}
			},
			want:    []*entity.ColumnEvaluator{},
			wantErr: false,
		},
		{
			name: "正常获取离线实验结果 - 无ck - 有参数",
			param: &entity.MGetExperimentResultParam{
				SpaceID: 100,
				ExptIDs: []int64{1},
			},
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
				mockMetric := metricsMocks.NewMockExptMetric(ctrl)
				mockLWT := lwtMocks.NewMockILatestWriteTracker(ctrl)
				mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
				mockEvaluatorService := svcMocks.NewMockEvaluatorService(ctrl)
				mockEvaluationSetItemService := svcMocks.NewMockEvaluationSetItemService(ctrl)
				mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)
				mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
				mockEvaluationSetService := svcMocks.NewMockIEvaluationSetService(ctrl)
				mockEvaluationSetVersionService := svcMocks.NewMockEvaluationSetVersionService(ctrl)
				mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
				mockExptAnnotateRepo := repoMocks.NewMockIExptAnnotateRepo(ctrl)
				mockTagRPCAdapter := rpcMocks.NewMockITagRPCAdapter(ctrl)

				mockExperimentRepo.EXPECT().MGetByID(gomock.Any(), []int64{1}, int64(100)).Return([]*entity.Experiment{{
					ID:               1,
					EvalSetID:        1,
					EvalSetVersionID: 1,
					ExptType:         entity.ExptType_Offline,
				}}, nil).AnyTimes()
				mockExperimentRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{
					EvalSetVersionID: 1,
					EvalSetID:        1,
					ExptType:         entity.ExptType_Offline,
				}, nil).AnyTimes()
				mockExptTurnResultRepo.EXPECT().ListTurnResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{{ID: 1, ItemID: 1}}, int64(1), nil)
				mockMetric.EXPECT().EmitGetExptResult(gomock.Any(), gomock.Any()).AnyTimes()
				mockLWT.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
				mockExptStatsRepo.EXPECT().MGet(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptStats{}, nil).AnyTimes()
				mockExperimentRepo.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptEvaluatorRef{
					{
						EvaluatorVersionID: 1,
						EvaluatorID:        1,
					},
				}, nil).AnyTimes()
				mockEvaluatorService.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.Evaluator{
					{
						ID:            1,
						Name:          "test_evaluator",
						Description:   "test description",
						EvaluatorType: entity.EvaluatorTypePrompt,
						PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
							ID:      1,
							Version: "v1",
						},
					},
				}, nil).AnyTimes()
				mockEvaluationSetItemService.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{}, nil).AnyTimes()
				mockEvaluatorRecordService.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvaluatorRecord{}, nil).AnyTimes()
				mockEvalTargetService.EXPECT().BatchGetRecordByIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvalTargetRecord{}, nil).AnyTimes()
				mockEvaluationSetService.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{},
						},
					},
				}, nil).AnyTimes()
				mockEvaluationSetVersionService.EXPECT().GetEvaluationSetVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSetVersion{
					EvaluationSetSchema: &entity.EvaluationSetSchema{
						FieldSchemas: []*entity.FieldSchema{},
					},
				}, nil, nil).AnyTimes()
				mockExptItemResultRepo.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{
					{
						ItemID: 1,
						Status: 1,
					},
				}, nil).AnyTimes()
				mockExptTurnResultRepo.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnEvaluatorResultRef{}, nil).AnyTimes()
				mockExptItemResultRepo.EXPECT().GetItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{}, nil).AnyTimes()
				mockExptAnnotateRepo.EXPECT().BatchGetExptTurnResultTagRefs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultTagRef{
					{
						ID:       1,
						SpaceID:  1,
						ExptID:   1,
						TagKeyID: 1,
					},
				}, nil).AnyTimes()
				mockExptAnnotateRepo.EXPECT().GetExptTurnAnnotateRecordRefsByTurnResultIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnAnnotateRecordRef{}, nil).AnyTimes()
				mockExptAnnotateRepo.EXPECT().GetAnnotateRecordsByIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.AnnotateRecord{}, nil).AnyTimes()
				mockTagRPCAdapter.EXPECT().BatchGetTagInfo(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[int64]*entity.TagInfo{}, nil).AnyTimes()

				return ExptResultServiceImpl{
					ExptTurnResultRepo:          mockExptTurnResultRepo,
					ExperimentRepo:              mockExperimentRepo,
					ExptStatsRepo:               mockExptStatsRepo,
					Metric:                      mockMetric,
					lwt:                         mockLWT,
					ExptItemResultRepo:          mockExptItemResultRepo,
					evaluatorService:            mockEvaluatorService,
					evaluationSetItemService:    mockEvaluationSetItemService,
					evaluatorRecordService:      mockEvaluatorRecordService,
					evalTargetService:           mockEvalTargetService,
					evaluationSetService:        mockEvaluationSetService,
					evaluationSetVersionService: mockEvaluationSetVersionService,
					ExptAnnotateRepo:            mockExptAnnotateRepo,
					tagRPCAdapter:               mockTagRPCAdapter,
				}
			},
			want: []*entity.ColumnEvaluator{
				{
					EvaluatorVersionID: 1,
					EvaluatorID:        1,
					EvaluatorType:      entity.EvaluatorTypePrompt,
					Name:               gptr.Of("test_evaluator"),
					Version:            gptr.Of("v1"),
					Description:        gptr.Of("test description"),
					Builtin:            gptr.Of(false),
				},
			},
			wantErr: false,
		},
		{
			name: "获取实验失败",
			param: &entity.MGetExperimentResultParam{
				SpaceID: 100,
				ExptIDs: []int64{1},
			},
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
				mockMetric := metricsMocks.NewMockExptMetric(ctrl)
				mockLWT := lwtMocks.NewMockILatestWriteTracker(ctrl)
				mockEvaluationSetService := svcMocks.NewMockIEvaluationSetService(ctrl)

				mockExperimentRepo.EXPECT().MGetByID(gomock.Any(), []int64{1}, int64(100)).Return(nil, fmt.Errorf("get experiment error"))
				mockMetric.EXPECT().EmitGetExptResult(gomock.Any(), gomock.Any()).AnyTimes()
				mockLWT.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()

				return ExptResultServiceImpl{
					ExperimentRepo:       mockExperimentRepo,
					Metric:               mockMetric,
					lwt:                  mockLWT,
					evaluationSetService: mockEvaluationSetService,
				}
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "获取轮次结果失败",
			param: &entity.MGetExperimentResultParam{
				SpaceID: 100,
				ExptIDs: []int64{1},
			},
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
				mockMetric := metricsMocks.NewMockExptMetric(ctrl)
				mockLWT := lwtMocks.NewMockILatestWriteTracker(ctrl)
				mockEvaluatorService := svcMocks.NewMockEvaluatorService(ctrl)
				mockEvaluationSetService := svcMocks.NewMockIEvaluationSetService(ctrl)
				mockEvaluationSetVersionService := svcMocks.NewMockEvaluationSetVersionService(ctrl)
				mockExptAnnotateRepo := repoMocks.NewMockIExptAnnotateRepo(ctrl)
				mockTagRPCAdapter := rpcMocks.NewMockITagRPCAdapter(ctrl)

				mockExperimentRepo.EXPECT().MGetByID(gomock.Any(), []int64{1}, int64(100)).Return([]*entity.Experiment{{
					ID:               1,
					EvalSetVersionID: 1,
					EvalSetID:        1,
					ExptType:         entity.ExptType_Offline,
				}}, nil)
				mockExptTurnResultRepo.EXPECT().ListTurnResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), fmt.Errorf("list turn result error"))
				mockMetric.EXPECT().EmitGetExptResult(gomock.Any(), gomock.Any()).AnyTimes()
				mockLWT.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
				mockExperimentRepo.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptEvaluatorRef{}, nil)
				mockEvaluatorService.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.Evaluator{}, nil).AnyTimes()
				mockEvaluationSetService.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{},
						},
					},
				}, nil)
				mockEvaluationSetVersionService.EXPECT().GetEvaluationSetVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSetVersion{
					EvaluationSetSchema: &entity.EvaluationSetSchema{
						FieldSchemas: []*entity.FieldSchema{},
					},
				}, nil, nil).AnyTimes()
				mockExptAnnotateRepo.EXPECT().BatchGetExptTurnResultTagRefs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultTagRef{
					{
						ID:       1,
						SpaceID:  1,
						ExptID:   1,
						TagKeyID: 1,
					},
				}, nil).AnyTimes()
				mockExptAnnotateRepo.EXPECT().GetExptTurnAnnotateRecordRefsByTurnResultIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnAnnotateRecordRef{}, nil).AnyTimes()
				mockExptAnnotateRepo.EXPECT().GetAnnotateRecordsByIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.AnnotateRecord{}, nil).AnyTimes()
				mockTagRPCAdapter.EXPECT().BatchGetTagInfo(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[int64]*entity.TagInfo{}, nil).AnyTimes()

				return ExptResultServiceImpl{
					ExptTurnResultRepo:          mockExptTurnResultRepo,
					ExperimentRepo:              mockExperimentRepo,
					Metric:                      mockMetric,
					lwt:                         mockLWT,
					evaluatorService:            mockEvaluatorService,
					evaluationSetService:        mockEvaluationSetService,
					evaluationSetVersionService: mockEvaluationSetVersionService,
					ExptAnnotateRepo:            mockExptAnnotateRepo,
					tagRPCAdapter:               mockTagRPCAdapter,
				}
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "在线实验对比场景",
			param: &entity.MGetExperimentResultParam{
				SpaceID:    100,
				ExptIDs:    []int64{1, 2},
				BaseExptID: ptr.Of(int64(1)),
			},
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
				mockMetric := metricsMocks.NewMockExptMetric(ctrl)
				mockLWT := lwtMocks.NewMockILatestWriteTracker(ctrl)
				mockEvaluatorService := svcMocks.NewMockEvaluatorService(ctrl)
				mockEvaluationSetService := svcMocks.NewMockIEvaluationSetService(ctrl)
				mockEvaluationSetVersionService := svcMocks.NewMockEvaluationSetVersionService(ctrl)
				mockExptAnnotateRepo := repoMocks.NewMockIExptAnnotateRepo(ctrl)
				mockTagRPCAdapter := rpcMocks.NewMockITagRPCAdapter(ctrl)

				mockExperimentRepo.EXPECT().MGetByID(gomock.Any(), []int64{1, 2}, int64(100)).Return([]*entity.Experiment{
					{
						ID:               1,
						ExptType:         entity.ExptType_Online,
						EvalSetVersionID: 1,
						EvalSetID:        1,
					},
					{
						ID:               2,
						ExptType:         entity.ExptType_Online,
						EvalSetVersionID: 1,
						EvalSetID:        1,
					},
				}, nil)
				mockMetric.EXPECT().EmitGetExptResult(gomock.Any(), gomock.Any()).AnyTimes()
				mockLWT.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
				mockExperimentRepo.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptEvaluatorRef{}, nil)
				mockEvaluatorService.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.Evaluator{}, nil).AnyTimes()
				mockEvaluationSetService.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{},
						},
					},
				}, nil)
				mockEvaluationSetVersionService.EXPECT().GetEvaluationSetVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSetVersion{
					EvaluationSetSchema: &entity.EvaluationSetSchema{
						FieldSchemas: []*entity.FieldSchema{},
					},
				}, nil, nil).AnyTimes()
				mockExptAnnotateRepo.EXPECT().BatchGetExptTurnResultTagRefs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultTagRef{
					{
						ID:       1,
						SpaceID:  1,
						ExptID:   1,
						TagKeyID: 1,
					},
				}, nil).AnyTimes()
				mockExptAnnotateRepo.EXPECT().GetExptTurnAnnotateRecordRefsByTurnResultIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnAnnotateRecordRef{}, nil).AnyTimes()
				mockExptAnnotateRepo.EXPECT().GetAnnotateRecordsByIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.AnnotateRecord{}, nil).AnyTimes()
				mockTagRPCAdapter.EXPECT().BatchGetTagInfo(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[int64]*entity.TagInfo{}, nil).AnyTimes()

				return ExptResultServiceImpl{
					ExperimentRepo:              mockExperimentRepo,
					Metric:                      mockMetric,
					lwt:                         mockLWT,
					evaluatorService:            mockEvaluatorService,
					evaluationSetService:        mockEvaluationSetService,
					evaluationSetVersionService: mockEvaluationSetVersionService,
					ExptAnnotateRepo:            mockExptAnnotateRepo,
					tagRPCAdapter:               mockTagRPCAdapter,
				}
			},
			want:    []*entity.ColumnEvaluator{},
			wantErr: false,
		},
		{
			name: "正常获取离线实验结果 - 有ck - 有参数",
			param: &entity.MGetExperimentResultParam{
				SpaceID:        100,
				ExptIDs:        []int64{1},
				UseAccelerator: true,
				BaseExptID:     ptr.Of(int64(1)),
				FilterAccelerators: map[int64]*entity.ExptTurnResultFilterAccelerator{
					1: {
						ExptID:  1,
						SpaceID: 100,
						MapCond: &entity.ExptTurnResultFilterMapCond{
							EvalTargetDataFilters: []*entity.FieldFilter{
								{
									Key:    "actual_output",
									Op:     "=",
									Values: []any{"1"},
								},
							},
							EvaluatorScoreFilters: []*entity.FieldFilter{
								{
									Key:    "key1",
									Op:     "=",
									Values: []any{1.0},
								},
							},
						},
						KeywordSearch: &entity.KeywordFilter{
							Keyword: ptr.Of("test"),
							ItemSnapshotFilter: &entity.ItemSnapshotFilter{
								StringMapFilters: []*entity.FieldFilter{
									{
										Key:    "field_key_string",
										Op:     "=",
										Values: []any{"1"},
									},
									{
										Key:    "field_key_int",
										Op:     "=",
										Values: []any{1},
									},
									{
										Key:    "field_key_float",
										Op:     "=",
										Values: []any{1.0},
									},
									{
										Key:    "field_key_bool",
										Op:     "=",
										Values: []any{"true"},
									},
								},
							},
						},
						ItemSnapshotCond: &entity.ItemSnapshotFilter{
							StringMapFilters: []*entity.FieldFilter{
								{
									Key:    "field_key_string",
									Op:     "=",
									Values: []any{"1"},
								},
								{
									Key:    "field_key_int",
									Op:     "=",
									Values: []any{1},
								},
								{
									Key:    "field_key_float",
									Op:     "=",
									Values: []any{1.0},
								},
								{
									Key:    "field_key_bool",
									Op:     "=",
									Values: []any{"true"},
								},
							},
						},
					},
				},
			},
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
				mockMetric := metricsMocks.NewMockExptMetric(ctrl)
				mockLWT := lwtMocks.NewMockILatestWriteTracker(ctrl)
				mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
				mockExptTurnResultFilterRepo := repoMocks.NewMockIExptTurnResultFilterRepo(ctrl)
				mockEvaluatorService := svcMocks.NewMockEvaluatorService(ctrl)
				mockEvaluationSetItemService := svcMocks.NewMockEvaluationSetItemService(ctrl)
				mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)
				mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
				mockEvaluationSetService := svcMocks.NewMockIEvaluationSetService(ctrl)
				mockEvaluationSetVersionService := svcMocks.NewMockEvaluationSetVersionService(ctrl)
				mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
				mockExptAnnotateRepo := repoMocks.NewMockIExptAnnotateRepo(ctrl)
				mockTagRPCAdapter := rpcMocks.NewMockITagRPCAdapter(ctrl)

				mockExperimentRepo.EXPECT().MGetByID(gomock.Any(), []int64{1}, int64(100)).Return([]*entity.Experiment{{
					ID:               1,
					EvalSetVersionID: 1,
					EvalSetID:        1,
					ExptType:         entity.ExptType_Offline,
				}}, nil).AnyTimes()
				mockExperimentRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{
					EvalSetVersionID: 1,
					EvalSetID:        1,
					ExptType:         entity.ExptType_Offline,
				}, nil).AnyTimes()
				mockMetric.EXPECT().EmitGetExptResult(gomock.Any(), gomock.Any()).AnyTimes()
				mockLWT.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
				mockExptStatsRepo.EXPECT().MGet(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptStats{}, nil).AnyTimes()
				mockExperimentRepo.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptEvaluatorRef{
					{
						EvaluatorVersionID: 1,
						EvaluatorID:        1,
					},
				}, nil).AnyTimes()
				mockEvaluatorService.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.Evaluator{
					{
						ID:            1,
						Name:          "test_evaluator",
						Description:   "test description",
						EvaluatorType: entity.EvaluatorTypePrompt,
						PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
							ID:      1,
							Version: "v1",
						},
					},
				}, nil).AnyTimes()
				mockEvaluationSetItemService.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{}, nil).AnyTimes()
				mockEvaluatorRecordService.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvaluatorRecord{}, nil).AnyTimes()
				mockEvalTargetService.EXPECT().BatchGetRecordByIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvalTargetRecord{}, nil).AnyTimes()
				mockEvaluationSetService.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSet{}, nil).AnyTimes()
				mockEvaluationSetService.EXPECT().QueryItemSnapshotMappings(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ItemSnapshotFieldMapping{
					{
						FieldKey:      "field_key_string",
						MappingKey:    "string_map",
						MappingSubKey: "subkey_string",
					},
					{
						FieldKey:      "field_key_int",
						MappingKey:    "int_map",
						MappingSubKey: "subkey_int",
					},
					{
						FieldKey:      "field_key_float",
						MappingKey:    "float_map",
						MappingSubKey: "subkey_float",
					},
					{
						FieldKey:      "field_key_bool",
						MappingKey:    "bool_map",
						MappingSubKey: "subkey_bool",
					},
				}, "2025-01-01", nil).AnyTimes()
				mockEvaluationSetVersionService.EXPECT().GetEvaluationSetVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSetVersion{}, nil, nil).AnyTimes()
				mockExptItemResultRepo.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{}, nil).AnyTimes()
				mockExptTurnResultRepo.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnEvaluatorResultRef{}, nil).AnyTimes()
				mockExptItemResultRepo.EXPECT().GetItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{}, nil).AnyTimes()
				mockExptTurnResultRepo.EXPECT().ListTurnResultByItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{
					{
						ID:     1,
						ItemID: 1,
					},
				}, int64(0), nil).AnyTimes()
				mockExptTurnResultFilterRepo.EXPECT().QueryItemIDStates(gomock.Any(), gomock.Any()).Return(map[int64]entity.ItemRunState{}, int64(0), nil).Return(
					map[int64]entity.ItemRunState{1: 1}, int64(1), nil,
				).AnyTimes()
				mockExptTurnResultFilterRepo.EXPECT().GetExptTurnResultFilterKeyMappings(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultFilterKeyMapping{
					{
						SpaceID:   100,
						ExptID:    1,
						FromField: "1",
						ToKey:     "key1",
						FieldType: entity.FieldTypeEvaluator,
					},
				}, nil).AnyTimes()
				mockMetric.EXPECT().EmitExptTurnResultFilterQueryLatency(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				mockExptAnnotateRepo.EXPECT().BatchGetExptTurnResultTagRefs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultTagRef{
					{
						ID:       1,
						SpaceID:  1,
						ExptID:   1,
						TagKeyID: 1,
					},
				}, nil).AnyTimes()
				mockExptAnnotateRepo.EXPECT().GetExptTurnAnnotateRecordRefsByTurnResultIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnAnnotateRecordRef{}, nil).AnyTimes()
				mockExptAnnotateRepo.EXPECT().GetAnnotateRecordsByIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.AnnotateRecord{}, nil).AnyTimes()
				mockTagRPCAdapter.EXPECT().BatchGetTagInfo(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[int64]*entity.TagInfo{}, nil).AnyTimes()

				return ExptResultServiceImpl{
					ExptTurnResultRepo:          mockExptTurnResultRepo,
					ExperimentRepo:              mockExperimentRepo,
					ExptStatsRepo:               mockExptStatsRepo,
					Metric:                      mockMetric,
					lwt:                         mockLWT,
					ExptItemResultRepo:          mockExptItemResultRepo,
					evaluatorService:            mockEvaluatorService,
					evaluationSetItemService:    mockEvaluationSetItemService,
					evaluatorRecordService:      mockEvaluatorRecordService,
					evalTargetService:           mockEvalTargetService,
					evaluationSetService:        mockEvaluationSetService,
					evaluationSetVersionService: mockEvaluationSetVersionService,
					exptTurnResultFilterRepo:    mockExptTurnResultFilterRepo,
					ExptAnnotateRepo:            mockExptAnnotateRepo,
					tagRPCAdapter:               mockTagRPCAdapter,
				}
			},
			want: []*entity.ColumnEvaluator{
				{
					EvaluatorVersionID: 1,
					EvaluatorID:        1,
					EvaluatorType:      entity.EvaluatorTypePrompt,
					Name:               gptr.Of("test_evaluator"),
					Version:            gptr.Of("v1"),
					Description:        gptr.Of("test description"),
					Builtin:            gptr.Of(false),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := tt.setup(ctrl)
			got, err := svc.MGetExperimentResult(context.Background(), tt.param)
			if (err != nil) != tt.wantErr {
				t.Errorf("MGetExperimentResult() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got.ColumnEvaluators, tt.want) {
				t.Errorf("MGetExperimentResult() got = %v, want %v", got.ColumnEvaluators, tt.want)
			}
		})
	}
}

func TestExptResultServiceImpl_RecordItemRunLogs(t *testing.T) {
	tests := []struct {
		name      string
		exptID    int64
		exptRunID int64
		itemID    int64
		spaceID   int64
		session   *entity.Session
		setup     func(ctrl *gomock.Controller) ExptResultServiceImpl
		wantErr   bool
	}{
		{
			name:      "正常记录运行日志",
			exptID:    1,
			exptRunID: 1,
			itemID:    1,
			spaceID:   100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
				mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)
				mockPublisher := eventsMocks.NewMockExptEventPublisher(ctrl)
				mockIdgen := idgenMocks.NewMockIIDGenerator(ctrl)

				// GetItemRunLog mock
				mockExptItemResultRepo.EXPECT().
					GetItemRunLog(gomock.Any(), int64(1), int64(1), int64(1), int64(100)).
					Return(&entity.ExptItemResultRunLog{Status: 1, ResultState: int32(entity.ExptItemResultStateLogged)}, nil)
				mockExptItemResultRepo.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{
					{
						ID:     1,
						ItemID: 1,
						Status: entity.ItemRunState_Processing,
					},
				}, nil)

				// GetItemTurnRunLogs mock
				mockExptTurnResultRepo.EXPECT().
					GetItemTurnRunLogs(gomock.Any(), int64(1), int64(1), int64(1), int64(100)).
					Return([]*entity.ExptTurnResultRunLog{{
						TurnID:             1,
						Status:             entity.TurnRunState_Success,
						EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{1: 1}},
					}}, nil)

				// GetItemTurnResults mock
				mockExptItemResultRepo.EXPECT().
					GetItemTurnResults(gomock.Any(), int64(100), int64(1), int64(1)).
					Return([]*entity.ExptTurnResult{{
						ID:     1,
						TurnID: 1,
						Status: int32(entity.TurnRunState_Success),
					}}, nil)

				// idgen mock
				mockIdgen.EXPECT().
					GenMultiIDs(gomock.Any(), 1).
					Return([]int64{1}, nil)

				// CreateTurnEvaluatorRefs mock
				mockExptTurnResultRepo.EXPECT().
					CreateTurnEvaluatorRefs(gomock.Any(), gomock.Any()).
					Return(nil)

				// SaveTurnResults mock
				mockExptTurnResultRepo.EXPECT().
					SaveTurnResults(gomock.Any(), gomock.Any()).
					Return(nil)

				// UpdateItemsResult mock
				mockExptItemResultRepo.EXPECT().
					UpdateItemsResult(gomock.Any(), int64(100), int64(1), []int64{1}, gomock.Any()).
					Return(nil)

				// UpdateItemRunLog mock
				mockExptItemResultRepo.EXPECT().
					UpdateItemRunLog(gomock.Any(), int64(1), int64(1), []int64{1}, gomock.Any(), int64(100)).
					Return(nil)

				// ArithOperateCount mock
				mockExptStatsRepo.EXPECT().
					ArithOperateCount(gomock.Any(), int64(1), int64(100), gomock.Any()).
					Return(nil)

				return ExptResultServiceImpl{
					ExptItemResultRepo:     mockExptItemResultRepo,
					ExptTurnResultRepo:     mockExptTurnResultRepo,
					ExptStatsRepo:          mockExptStatsRepo,
					evaluatorRecordService: mockEvaluatorRecordService,
					publisher:              mockPublisher,
					idgen:                  mockIdgen,
				}
			},
			wantErr: false,
		},
		{
			name:      "获取运行日志失败",
			exptID:    1,
			exptRunID: 1,
			itemID:    1,
			spaceID:   100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)

				// GetItemRunLog mock 返回错误
				mockExptItemResultRepo.EXPECT().
					GetItemRunLog(gomock.Any(), int64(1), int64(1), int64(1), int64(100)).
					Return(nil, fmt.Errorf("get item run log error"))

				return ExptResultServiceImpl{
					ExptItemResultRepo: mockExptItemResultRepo,
				}
			},
			wantErr: true,
		},
		{
			name:      "获取轮次运行日志失败",
			exptID:    1,
			exptRunID: 1,
			itemID:    1,
			spaceID:   100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)

				// GetItemRunLog mock
				mockExptItemResultRepo.EXPECT().
					GetItemRunLog(gomock.Any(), int64(1), int64(1), int64(1), int64(100)).
					Return(&entity.ExptItemResultRunLog{Status: 1, ResultState: int32(entity.ExptItemResultStateLogged)}, nil)

				// GetItemTurnRunLogs mock 返回错误
				mockExptTurnResultRepo.EXPECT().
					GetItemTurnRunLogs(gomock.Any(), int64(1), int64(1), int64(1), int64(100)).
					Return(nil, fmt.Errorf("get turn run logs error"))

				return ExptResultServiceImpl{
					ExptItemResultRepo: mockExptItemResultRepo,
					ExptTurnResultRepo: mockExptTurnResultRepo,
				}
			},
			wantErr: true,
		},
		{
			name:      "获取轮次结果失败",
			exptID:    1,
			exptRunID: 1,
			itemID:    1,
			spaceID:   100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)

				// GetItemRunLog mock
				mockExptItemResultRepo.EXPECT().
					GetItemRunLog(gomock.Any(), int64(1), int64(1), int64(1), int64(100)).
					Return(&entity.ExptItemResultRunLog{Status: 1, ResultState: int32(entity.ExptItemResultStateLogged)}, nil)

				// GetItemTurnRunLogs mock
				mockExptTurnResultRepo.EXPECT().
					GetItemTurnRunLogs(gomock.Any(), int64(1), int64(1), int64(1), int64(100)).
					Return([]*entity.ExptTurnResultRunLog{{TurnID: 1, Status: entity.TurnRunState_Success, EvaluatorResultIds: nil}}, nil)

				// GetItemTurnResults mock 返回错误
				mockExptItemResultRepo.EXPECT().
					GetItemTurnResults(gomock.Any(), int64(100), int64(1), int64(1)).
					Return(nil, fmt.Errorf("get turn results error"))

				return ExptResultServiceImpl{
					ExptItemResultRepo: mockExptItemResultRepo,
					ExptTurnResultRepo: mockExptTurnResultRepo,
				}
			},
			wantErr: true,
		},
		{
			name:      "保存轮次结果失败",
			exptID:    1,
			exptRunID: 1,
			itemID:    1,
			spaceID:   100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)

				// GetItemRunLog mock
				mockExptItemResultRepo.EXPECT().
					GetItemRunLog(gomock.Any(), int64(1), int64(1), int64(1), int64(100)).
					Return(&entity.ExptItemResultRunLog{Status: 1, ResultState: int32(entity.ExptItemResultStateLogged)}, nil)

				// BatchGet mock
				mockExptItemResultRepo.EXPECT().
					BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*entity.ExptItemResult{
						{
							ID:     1,
							ItemID: 1,
							Status: entity.ItemRunState_Processing,
						},
					}, nil)

				// GetItemTurnRunLogs mock
				mockExptTurnResultRepo.EXPECT().
					GetItemTurnRunLogs(gomock.Any(), int64(1), int64(1), int64(1), int64(100)).
					Return([]*entity.ExptTurnResultRunLog{{TurnID: 1, Status: entity.TurnRunState_Success, EvaluatorResultIds: nil}}, nil)

				// GetItemTurnResults mock
				mockExptItemResultRepo.EXPECT().
					GetItemTurnResults(gomock.Any(), int64(100), int64(1), int64(1)).
					Return([]*entity.ExptTurnResult{{
						ID:     1,
						TurnID: 1,
						Status: int32(entity.TurnRunState_Success),
					}}, nil)

				// SaveTurnResults mock 返回错误
				mockExptTurnResultRepo.EXPECT().
					SaveTurnResults(gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("save turn results error"))

				return ExptResultServiceImpl{
					ExptItemResultRepo: mockExptItemResultRepo,
					ExptTurnResultRepo: mockExptTurnResultRepo,
				}
			},
			wantErr: true,
		},
		{
			name:      "更新项目结果失败",
			exptID:    1,
			exptRunID: 1,
			itemID:    1,
			spaceID:   100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)

				// GetItemRunLog mock
				mockExptItemResultRepo.EXPECT().
					GetItemRunLog(gomock.Any(), int64(1), int64(1), int64(1), int64(100)).
					Return(&entity.ExptItemResultRunLog{Status: 1, ResultState: int32(entity.ExptItemResultStateLogged)}, nil)

				// BatchGet mock
				mockExptItemResultRepo.EXPECT().
					BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*entity.ExptItemResult{
						{
							ID:     1,
							ItemID: 1,
							Status: entity.ItemRunState_Processing,
						},
					}, nil)

				// GetItemTurnRunLogs mock
				mockExptTurnResultRepo.EXPECT().
					GetItemTurnRunLogs(gomock.Any(), int64(1), int64(1), int64(1), int64(100)).
					Return([]*entity.ExptTurnResultRunLog{{TurnID: 1, Status: entity.TurnRunState_Success, EvaluatorResultIds: nil}}, nil)

				// GetItemTurnResults mock
				mockExptItemResultRepo.EXPECT().
					GetItemTurnResults(gomock.Any(), int64(100), int64(1), int64(1)).
					Return([]*entity.ExptTurnResult{{
						ID:     1,
						TurnID: 1,
						Status: int32(entity.TurnRunState_Success),
					}}, nil)

				// SaveTurnResults mock
				mockExptTurnResultRepo.EXPECT().
					SaveTurnResults(gomock.Any(), gomock.Any()).
					Return(nil)

				// UpdateItemsResult mock 返回错误
				mockExptItemResultRepo.EXPECT().
					UpdateItemsResult(gomock.Any(), int64(100), int64(1), []int64{1}, gomock.Any()).
					Return(fmt.Errorf("update items result error"))

				return ExptResultServiceImpl{
					ExptItemResultRepo: mockExptItemResultRepo,
					ExptTurnResultRepo: mockExptTurnResultRepo,
				}
			},
			wantErr: true,
		},
		{
			name:      "更新运行日志失败",
			exptID:    1,
			exptRunID: 1,
			itemID:    1,
			spaceID:   100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)

				// GetItemRunLog mock
				mockExptItemResultRepo.EXPECT().
					GetItemRunLog(gomock.Any(), int64(1), int64(1), int64(1), int64(100)).
					Return(&entity.ExptItemResultRunLog{Status: 1, ResultState: int32(entity.ExptItemResultStateLogged)}, nil)

				// BatchGet mock
				mockExptItemResultRepo.EXPECT().
					BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*entity.ExptItemResult{
						{
							ID:     1,
							ItemID: 1,
							Status: entity.ItemRunState_Processing,
						},
					}, nil)

				// GetItemTurnRunLogs mock
				mockExptTurnResultRepo.EXPECT().
					GetItemTurnRunLogs(gomock.Any(), int64(1), int64(1), int64(1), int64(100)).
					Return([]*entity.ExptTurnResultRunLog{{TurnID: 1, Status: entity.TurnRunState_Success, EvaluatorResultIds: nil}}, nil)

				// GetItemTurnResults mock
				mockExptItemResultRepo.EXPECT().
					GetItemTurnResults(gomock.Any(), int64(100), int64(1), int64(1)).
					Return([]*entity.ExptTurnResult{{
						ID:     1,
						TurnID: 1,
						Status: int32(entity.TurnRunState_Success),
					}}, nil)

				// SaveTurnResults mock
				mockExptTurnResultRepo.EXPECT().
					SaveTurnResults(gomock.Any(), gomock.Any()).
					Return(nil)

				// UpdateItemsResult mock
				mockExptItemResultRepo.EXPECT().
					UpdateItemsResult(gomock.Any(), int64(100), int64(1), []int64{1}, gomock.Any()).
					Return(nil)

				// UpdateItemRunLog mock 返回错误
				mockExptItemResultRepo.EXPECT().
					UpdateItemRunLog(gomock.Any(), int64(1), int64(1), []int64{1}, gomock.Any(), int64(100)).
					Return(fmt.Errorf("update run log error"))

				return ExptResultServiceImpl{
					ExptItemResultRepo: mockExptItemResultRepo,
					ExptTurnResultRepo: mockExptTurnResultRepo,
				}
			},
			wantErr: true,
		},
		{
			name:      "统计操作失败",
			exptID:    1,
			exptRunID: 1,
			itemID:    1,
			spaceID:   100,
			session: &entity.Session{
				UserID: "test",
			},
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)

				// GetItemRunLog mock
				mockExptItemResultRepo.EXPECT().
					GetItemRunLog(gomock.Any(), int64(1), int64(1), int64(1), int64(100)).
					Return(&entity.ExptItemResultRunLog{Status: 1, ResultState: int32(entity.ExptItemResultStateLogged)}, nil)

				// BatchGet mock
				mockExptItemResultRepo.EXPECT().
					BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*entity.ExptItemResult{
						{
							ID:     1,
							ItemID: 1,
							Status: entity.ItemRunState_Processing,
						},
					}, nil)

				// GetItemTurnRunLogs mock
				mockExptTurnResultRepo.EXPECT().
					GetItemTurnRunLogs(gomock.Any(), int64(1), int64(1), int64(1), int64(100)).
					Return([]*entity.ExptTurnResultRunLog{{TurnID: 1, Status: entity.TurnRunState_Success, EvaluatorResultIds: nil}}, nil)

				// GetItemTurnResults mock
				mockExptItemResultRepo.EXPECT().
					GetItemTurnResults(gomock.Any(), int64(100), int64(1), int64(1)).
					Return([]*entity.ExptTurnResult{{
						ID:     1,
						TurnID: 1,
						Status: int32(entity.TurnRunState_Success),
					}}, nil)

				// SaveTurnResults mock
				mockExptTurnResultRepo.EXPECT().
					SaveTurnResults(gomock.Any(), gomock.Any()).
					Return(nil)

				// UpdateItemsResult mock
				mockExptItemResultRepo.EXPECT().
					UpdateItemsResult(gomock.Any(), int64(100), int64(1), []int64{1}, gomock.Any()).
					Return(nil)

				// UpdateItemRunLog mock
				mockExptItemResultRepo.EXPECT().
					UpdateItemRunLog(gomock.Any(), int64(1), int64(1), []int64{1}, gomock.Any(), int64(100)).
					Return(nil)

				// ArithOperateCount mock 返回错误
				mockExptStatsRepo.EXPECT().
					ArithOperateCount(gomock.Any(), int64(1), int64(100), gomock.Any()).
					Return(fmt.Errorf("stats operation error"))

				return ExptResultServiceImpl{
					ExptItemResultRepo: mockExptItemResultRepo,
					ExptTurnResultRepo: mockExptTurnResultRepo,
					ExptStatsRepo:      mockExptStatsRepo,
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := tt.setup(ctrl)

			// RecordItemRunLogs 内部会通过 ExperimentRepo.GetByID 加载实验配置，用于判断是否启用加权分数。
			// 旧单测未初始化 ExperimentRepo，导致新增逻辑下出现 nil pointer。
			// 这里为所有用例统一注入一个 mock ExperimentRepo，并让 GetByID 返回 nil 实验，跳过加权逻辑。
			mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
			svc.ExperimentRepo = mockExperimentRepo
			mockExperimentRepo.EXPECT().
				GetByID(gomock.Any(), tt.exptID, tt.spaceID).
				Return((*entity.Experiment)(nil), nil).
				AnyTimes()

			_, err := svc.RecordItemRunLogs(context.Background(), tt.exptID, tt.exptRunID, tt.itemID, tt.spaceID)
			if (err != nil) != tt.wantErr {
				t.Errorf("RecordItemRunLogs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewExptResultService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建所有依赖的 mock
	mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockExptTurnResultFilterRepo := repoMocks.NewMockIExptTurnResultFilterRepo(ctrl)
	mockMetric := metricsMocks.NewMockExptMetric(ctrl)
	mockLWT := lwtMocks.NewMockILatestWriteTracker(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorService := svcMocks.NewMockEvaluatorService(ctrl)
	mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
	mockEvaluationSetVersionService := svcMocks.NewMockEvaluationSetVersionService(ctrl)
	mockEvaluationSetService := svcMocks.NewMockIEvaluationSetService(ctrl)
	mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)
	mockEvaluationSetItemService := svcMocks.NewMockEvaluationSetItemService(ctrl)
	mockPublisher := eventsMocks.NewMockExptEventPublisher(ctrl)
	mockTagAdapter := rpcMocks.NewMockITagRPCAdapter(ctrl)
	mockAnnotateRepo := repoMocks.NewMockIExptAnnotateRepo(ctrl)
	svc := NewExptResultService(
		mockExptItemResultRepo,
		mockExptTurnResultRepo,
		mockAnnotateRepo,
		mockExptStatsRepo,
		mockExperimentRepo,
		mockMetric,
		mockLWT,
		mockIDGen,
		mockExptTurnResultFilterRepo,
		mockEvaluatorService,
		mockEvalTargetService,
		mockEvaluationSetVersionService,
		mockEvaluationSetService,
		mockEvaluatorRecordService,
		mockEvaluationSetItemService,
		mockPublisher,
		mockTagAdapter,
		nil,
	)

	impl, ok := svc.(*ExptResultServiceImpl)
	if !ok {
		t.Fatalf("NewExptResultService should return *ExptResultServiceImpl")
	}

	// 断言每个依赖都被正确赋值
	if impl.ExptItemResultRepo != mockExptItemResultRepo {
		t.Errorf("ExptItemResultRepo not set correctly")
	}
	if impl.ExptTurnResultRepo != mockExptTurnResultRepo {
		t.Errorf("ExptTurnResultRepo not set correctly")
	}
	if impl.ExptStatsRepo != mockExptStatsRepo {
		t.Errorf("ExptStatsRepo not set correctly")
	}
	if impl.ExperimentRepo != mockExperimentRepo {
		t.Errorf("ExperimentRepo not set correctly")
	}
	if impl.Metric != mockMetric {
		t.Errorf("Metric not set correctly")
	}
	if impl.lwt != mockLWT {
		t.Errorf("lwt not set correctly")
	}
	if impl.idgen != mockIDGen {
		t.Errorf("idgen not set correctly")
	}
	if impl.evaluatorService != mockEvaluatorService {
		t.Errorf("evaluatorService not set correctly")
	}
	if impl.evalTargetService != mockEvalTargetService {
		t.Errorf("evalTargetService not set correctly")
	}
	if impl.evaluationSetVersionService != mockEvaluationSetVersionService {
		t.Errorf("evaluationSetVersionService not set correctly")
	}
	if impl.evaluationSetService != mockEvaluationSetService {
		t.Errorf("evaluationSetService not set correctly")
	}
	if impl.evaluatorRecordService != mockEvaluatorRecordService {
		t.Errorf("evaluatorRecordService not set correctly")
	}
	if impl.evaluationSetItemService != mockEvaluationSetItemService {
		t.Errorf("evaluationSetItemService not set correctly")
	}
	if impl.publisher != mockPublisher {
		t.Errorf("publisher not set correctly")
	}
}

func TestExptResultServiceImpl_ManualUpsertExptTurnResultFilter(t *testing.T) {
	// 定义测试用例
	tests := []struct {
		name    string
		spaceID int64
		exptID  int64
		itemIDs []int64
		setup   func(
			mockLWT *lwtMocks.MockILatestWriteTracker,
			mockExperimentRepo *repoMocks.MockIExperimentRepo,
			mockFilterRepo *repoMocks.MockIExptTurnResultFilterRepo,
			mockPublisher *eventsMocks.MockExptEventPublisher,
			mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo,
		)
		wantErr bool
	}{
		{
			name:    "成功场景-正常插入和发布事件",
			spaceID: 100,
			exptID:  1,
			itemIDs: []int64{10, 11},
			setup: func(mockLWT *lwtMocks.MockILatestWriteTracker, mockExperimentRepo *repoMocks.MockIExperimentRepo, mockFilterRepo *repoMocks.MockIExptTurnResultFilterRepo, mockPublisher *eventsMocks.MockExptEventPublisher, mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo) {
				// 模拟写标志检查
				mockLWT.EXPECT().CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeExperiment, int64(1)).Return(false)
				// 模拟获取实验信息
				mockExperimentRepo.EXPECT().MGetByID(gomock.Any(), []int64{1}, int64(100)).Return([]*entity.Experiment{
					{
						ID:      1,
						SpaceID: 100,
						EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{
							{EvaluatorVersionID: 101},
							{EvaluatorVersionID: 102},
						},
					},
				}, nil)
				// 模拟插入Filter Key Mappings
				mockFilterRepo.EXPECT().InsertExptTurnResultFilterKeyMappings(gomock.Any(), gomock.Any()).Return(nil)
				// 模拟发布事件
				mockPublisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gptr.Of(time.Second*3)).Return(nil)
				mockExptAnnotateRepo.EXPECT().GetExptTurnResultTagRefs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultTagRef{
					{
						ID:          1,
						SpaceID:     100,
						ExptID:      1,
						TagKeyID:    10,
						TotalCnt:    10,
						CompleteCnt: 10,
					},
				}, nil)
			},
			wantErr: false,
		},
		{
			name:    "失败场景-实验不存在",
			spaceID: 100,
			exptID:  2,
			itemIDs: []int64{10},
			setup: func(mockLWT *lwtMocks.MockILatestWriteTracker, mockExperimentRepo *repoMocks.MockIExperimentRepo, mockFilterRepo *repoMocks.MockIExptTurnResultFilterRepo, mockPublisher *eventsMocks.MockExptEventPublisher, mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo) {
				// 模拟写标志检查
				mockLWT.EXPECT().CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeExperiment, int64(2)).Return(false)
				// 模拟返回空实验列表
				mockExperimentRepo.EXPECT().MGetByID(gomock.Any(), []int64{2}, int64(100)).Return([]*entity.Experiment{}, nil)
			},
			wantErr: true,
		},
		{
			name:    "失败场景-插入Filter Key Mappings失败",
			spaceID: 100,
			exptID:  3,
			itemIDs: []int64{10},
			setup: func(mockLWT *lwtMocks.MockILatestWriteTracker, mockExperimentRepo *repoMocks.MockIExperimentRepo, mockFilterRepo *repoMocks.MockIExptTurnResultFilterRepo, mockPublisher *eventsMocks.MockExptEventPublisher, mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo) {
				// 模拟写标志检查
				mockLWT.EXPECT().CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeExperiment, int64(3)).Return(false)
				// 模拟获取实验信息
				mockExperimentRepo.EXPECT().MGetByID(gomock.Any(), []int64{3}, int64(100)).Return([]*entity.Experiment{
					{
						ID:      3,
						SpaceID: 100,
						EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{
							{EvaluatorVersionID: 101},
						},
					},
				}, nil)
				mockExptAnnotateRepo.EXPECT().GetExptTurnResultTagRefs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultTagRef{
					{
						ID:          1,
						SpaceID:     100,
						ExptID:      1,
						TagKeyID:    10,
						TotalCnt:    10,
						CompleteCnt: 10,
					},
				}, nil)
				// 模拟插入失败
				mockFilterRepo.EXPECT().InsertExptTurnResultFilterKeyMappings(gomock.Any(), gomock.Any()).Return(fmt.Errorf("db insert error"))
			},
			wantErr: true,
		},
	}

	// 循环执行测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// 创建Mocks
			mockLWT := lwtMocks.NewMockILatestWriteTracker(ctrl)
			mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
			mockFilterRepo := repoMocks.NewMockIExptTurnResultFilterRepo(ctrl)
			mockPublisher := eventsMocks.NewMockExptEventPublisher(ctrl)
			mockExptAnnotateRepo := repoMocks.NewMockIExptAnnotateRepo(ctrl)

			// 实例化被测服务
			svc := ExptResultServiceImpl{
				lwt:                      mockLWT,
				ExperimentRepo:           mockExperimentRepo,
				exptTurnResultFilterRepo: mockFilterRepo,
				publisher:                mockPublisher,
				ExptAnnotateRepo:         mockExptAnnotateRepo,
			}

			// 设置Mock期望
			tt.setup(mockLWT, mockExperimentRepo, mockFilterRepo, mockPublisher, mockExptAnnotateRepo)

			// 调用被测方法
			err := svc.ManualUpsertExptTurnResultFilter(context.Background(), tt.spaceID, tt.exptID, tt.itemIDs)

			// 断言结果
			if (err != nil) != tt.wantErr {
				t.Errorf("ManualUpsertExptTurnResultFilter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPayloadBuilder_BuildTurnResultFilter(t *testing.T) {
	// 定义测试用例
	mockCreateDate, _ := time.Parse("2006-01-02", "2025-01-01")
	tests := []struct {
		name    string
		setup   func(ctrl *gomock.Controller) *PayloadBuilder
		want    []*entity.ExptTurnResultFilterEntity
		wantErr bool
	}{
		{
			name: "成功场景-离线实验",
			setup: func(ctrl *gomock.Controller) *PayloadBuilder {
				// 创建 Mocks
				mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
				mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)
				mockExptAnnotateRepo := repoMocks.NewMockIExptAnnotateRepo(ctrl)

				// 定义模拟数据
				spaceID := int64(100)
				baselineExptID := int64(1)
				now := time.Now()

				// 设置 Mock 期望
				// 1. ExperimentRepo.GetByID
				mockExperimentRepo.EXPECT().GetByID(gomock.Any(), baselineExptID, spaceID).Return(&entity.Experiment{
					ID:               baselineExptID,
					SpaceID:          spaceID,
					ExptType:         entity.ExptType_Offline, // 离线实验
					StartAt:          &now,
					EvalSetVersionID: 101,
				}, nil)

				// 2. buildEvaluatorResult -> ExptTurnResultRepo.BatchGetTurnEvaluatorResultRef
				mockExptTurnResultRepo.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), spaceID, []int64{10}).Return([]*entity.ExptTurnEvaluatorResultRef{
					{ExptTurnResultID: 10, EvaluatorResultID: 1001, EvaluatorVersionID: 201},
				}, nil)

				// 3. buildEvaluatorResult -> EvaluatorRecordService.BatchGetEvaluatorRecord
				mockEvaluatorRecordService.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{1001}, false).Return([]*entity.EvaluatorRecord{
					{
						ID:                 1001,
						EvaluatorVersionID: 201,
						EvaluatorOutputData: &entity.EvaluatorOutputData{
							EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.9)},
						},
					},
				}, nil)

				// 4. buildTargetOutput -> EvalTargetService.BatchGetRecordByIDs
				mockEvalTargetService.EXPECT().BatchGetRecordByIDs(gomock.Any(), spaceID, []int64{40}).Return([]*entity.EvalTargetRecord{
					{
						ID: 40,
						EvalTargetOutputData: &entity.EvalTargetOutputData{
							OutputFields: map[string]*entity.Content{"actual_output": {Text: ptr.Of("some output")}},
						},
					},
				}, nil)
				mockExptAnnotateRepo.EXPECT().GetExptTurnAnnotateRecordRefsByTurnResultIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnAnnotateRecordRef{
					{
						ID:               1,
						ExptTurnResultID: 10,
						SpaceID:          100,
						ExptID:           1,
						TagKeyID:         10,
						AnnotateRecordID: 10,
					},
					{
						ID:               1,
						ExptTurnResultID: 10,
						SpaceID:          100,
						ExptID:           1,
						TagKeyID:         11,
						AnnotateRecordID: 11,
					},
					{
						ID:               1,
						ExptTurnResultID: 10,
						SpaceID:          100,
						ExptID:           1,
						TagKeyID:         12,
						AnnotateRecordID: 12,
					},
					{
						ID:               1,
						ExptTurnResultID: 10,
						SpaceID:          100,
						ExptID:           1,
						TagKeyID:         13,
						AnnotateRecordID: 13,
					},
				}, nil).AnyTimes()
				mockExptAnnotateRepo.EXPECT().GetAnnotateRecordsByIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.AnnotateRecord{
					{
						ID:           10,
						ExperimentID: 1,
						SpaceID:      100,
						TagKeyID:     10,
						TagValueID:   0,
						AnnotateData: &entity.AnnotateData{
							Score:          ptr.Of(float64(1)),
							TagContentType: entity.TagContentTypeContinuousNumber,
						},
					},
					{
						ID:           13,
						ExperimentID: 1,
						SpaceID:      100,
						TagKeyID:     13,
						TagValueID:   456,
						AnnotateData: &entity.AnnotateData{
							TagContentType: entity.TagContentTypeCategorical,
						},
					},
					{
						ID:           11,
						ExperimentID: 1,
						SpaceID:      100,
						TagKeyID:     11,
						TagValueID:   123,
						AnnotateData: &entity.AnnotateData{
							TagContentType: entity.TagContentTypeBoolean,
						},
					},
					{
						ID:           12,
						ExperimentID: 1,
						SpaceID:      100,
						TagKeyID:     12,
						TagValueID:   0,
						AnnotateData: &entity.AnnotateData{
							TextValue:      ptr.Of("text"),
							TagContentType: entity.TagContentTypeFreeText,
						},
					},
				}, nil).AnyTimes()

				// 创建 PayloadBuilder 实例
				return &PayloadBuilder{
					BaselineExptID:       baselineExptID,
					SpaceID:              spaceID,
					BaseExptTurnResultDO: []*entity.ExptTurnResult{{ID: 10, ItemID: 20, TurnID: 30, TargetResultID: 40}},
					BaseExptItemResultDO: []*entity.ExptItemResult{{ItemID: 20, ItemIdx: 1, Status: entity.ItemRunState_Success}},
					ExptTurnResultFilterKeyMappingEvaluatorMap: map[string]*entity.ExptTurnResultFilterKeyMapping{
						"201": {ToKey: "eval_score_key"},
						"10":  {ToKey: "eval_score_key"},
						"11":  {ToKey: "eval_score_key"},
						"12":  {ToKey: "eval_score_key"},
						"13":  {ToKey: "eval_score_key"},
					},
					ExperimentRepo:         mockExperimentRepo,
					ExptTurnResultRepo:     mockExptTurnResultRepo,
					EvalTargetService:      mockEvalTargetService,
					EvaluatorRecordService: mockEvaluatorRecordService,
					ExptAnnotateRepo:       mockExptAnnotateRepo,
				}
			},
			want: []*entity.ExptTurnResultFilterEntity{
				{
					SpaceID:          100,
					ExptID:           1,
					ItemID:           20,
					TurnID:           30,
					ItemIdx:          1,
					Status:           entity.ItemRunState_Success,
					EvalTargetData:   map[string]string{"actual_output": "some output"},
					EvaluatorScore:   map[string]float64{"eval_score_key": 0.9},
					AnnotationFloat:  map[string]float64{},
					AnnotationBool:   map[string]bool{},
					AnnotationString: map[string]string{},
					CreatedDate:      mockCreateDate,
					EvalSetVersionID: 101,
				},
			},
			wantErr: false,
		},
		{
			name: "失败场景-获取实验信息失败",
			setup: func(ctrl *gomock.Controller) *PayloadBuilder {
				mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
				dbErr := errors.New("database error")
				mockExperimentRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, dbErr)

				return &PayloadBuilder{
					BaselineExptID: 1,
					SpaceID:        100,
					ExperimentRepo: mockExperimentRepo,
				}
			},
			want:    nil,
			wantErr: true,
		},
	}

	// 循环执行测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// 初始化 PayloadBuilder
			builder := tt.setup(ctrl)

			// 调用被测方法
			got, err := builder.BuildTurnResultFilter(context.Background())

			// 断言错误
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildTurnResultFilter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// 断言结果
			// 由于结果中包含时间戳，直接比较会失败，这里特殊处理
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Fatalf("BuildTurnResultFilter() got len = %d, want len %d", len(got), len(tt.want))
				}
				for i := range got {
					if got[i].SpaceID != tt.want[i].SpaceID {
						t.Errorf("BuildTurnResultFilter() got[%d].SpaceID = %d, want[%d].SpaceID %d", i, got[i].SpaceID, i, tt.want[i].SpaceID)
					}
					if got[i].ExptID != tt.want[i].ExptID {
						t.Errorf("BuildTurnResultFilter() got[%d].ExptID = %d, want[%d].ExptID %d", i, got[i].ExptID, i, tt.want[i].ExptID)
					}
					if got[i].ItemID != tt.want[i].ItemID {
						t.Errorf("BuildTurnResultFilter() got[%d].ItemID = %d, want[%d].ItemID %d", i, got[i].ItemID, i, tt.want[i].ItemID)
					}
				}
			}
		})
	}
}

func TestExptResultServiceImpl_UpsertExptTurnResultFilter(t *testing.T) {
	type args struct {
		spaceID int64
		exptID  int64
		itemIDs []int64
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	mockFilterRepo := repoMocks.NewMockIExptTurnResultFilterRepo(ctrl)
	mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
	mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)
	mockExptAnnotateRepo := repoMocks.NewMockIExptAnnotateRepo(ctrl)
	tests := []struct {
		name    string
		args    args
		setup   func()
		wantErr bool
	}{{
		name: "正常更新过滤条件",
		args: args{
			spaceID: 100,
			exptID:  1,
			itemIDs: []int64{1, 2},
		},
		setup: func() {
			mockExptTurnResultRepo = repoMocks.NewMockIExptTurnResultRepo(ctrl)
			mockExptItemResultRepo = repoMocks.NewMockIExptItemResultRepo(ctrl)
			mockFilterRepo = repoMocks.NewMockIExptTurnResultFilterRepo(ctrl)
			mockExperimentRepo = repoMocks.NewMockIExperimentRepo(ctrl)
			mockEvalTargetService = svcMocks.NewMockIEvalTargetService(ctrl)
			mockEvaluatorRecordService = svcMocks.NewMockEvaluatorRecordService(ctrl)
			now := time.Now()
			// 设置实验信息Mock
			mockExperimentRepo.EXPECT().GetByID(gomock.Any(), int64(1), int64(100)).Return(&entity.Experiment{
				ID:               1,
				SpaceID:          100,
				ExptType:         entity.ExptType_Offline, // 离线实验
				StartAt:          &now,
				EvalSetVersionID: 101,
			}, nil)

			mockExptTurnResultRepo.EXPECT().ListTurnResultByItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]*entity.ExptTurnResult{{ID: 1, ItemID: 1}, {ID: 2, ItemID: 2}}, int64(2), nil)
			mockExptItemResultRepo.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]*entity.ExptItemResult{{ItemID: 1}, {ItemID: 2}}, nil)
			mockFilterRepo.EXPECT().GetExptTurnResultFilterKeyMappings(gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]*entity.ExptTurnResultFilterKeyMapping{}, nil)

			// 更精确匹配Save方法的参数验证
			mockFilterRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
			// 定义模拟数据
			spaceID := int64(100)
			baselineExptID := int64(1)

			// 设置 Mock 期望
			// 1. ExperimentRepo.GetByID
			mockExperimentRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{
				ID:               baselineExptID,
				SpaceID:          spaceID,
				ExptType:         entity.ExptType_Offline, // 离线实验
				StartAt:          &now,
				EvalSetVersionID: 101,
			}, nil).AnyTimes()

			// 2. buildEvaluatorResult -> ExptTurnResultRepo.BatchGetTurnEvaluatorResultRef
			mockExptTurnResultRepo.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnEvaluatorResultRef{
				{ExptTurnResultID: 10, EvaluatorResultID: 1001, EvaluatorVersionID: 201},
			}, nil)

			// 3. buildEvaluatorResult -> EvaluatorRecordService.BatchGetEvaluatorRecord
			mockEvaluatorRecordService.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvaluatorRecord{
				{
					ID:                 1001,
					EvaluatorVersionID: 201,
					EvaluatorOutputData: &entity.EvaluatorOutputData{
						EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.9)},
					},
				},
			}, nil)

			// 4. buildTargetOutput -> EvalTargetService.BatchGetRecordByIDs
			mockEvalTargetService.EXPECT().BatchGetRecordByIDs(gomock.Any(), spaceID, gomock.Any()).Return([]*entity.EvalTargetRecord{
				{
					ID: 40,
					EvalTargetOutputData: &entity.EvalTargetOutputData{
						OutputFields: map[string]*entity.Content{"actual_output": {Text: ptr.Of("some output")}},
					},
				},
			}, nil)
			mockExptAnnotateRepo.EXPECT().GetExptTurnAnnotateRecordRefsByTurnResultIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnAnnotateRecordRef{
				{
					ID:       1,
					SpaceID:  100,
					ExptID:   1,
					TagKeyID: 10,
				},
			}, nil).AnyTimes()
			mockExptAnnotateRepo.EXPECT().GetAnnotateRecordsByIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.AnnotateRecord{
				{
					ID:       1,
					SpaceID:  100,
					TagKeyID: 10,
				},
			}, nil).AnyTimes()
		},
		wantErr: false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			// 此处原代码调用 NewExptResultService 时部分参数为 nil，实际项目中需根据情况补充
			// 以下为修正后的调用示例，实际使用时需根据 NewExptResultService 函数定义完善参数

			svc := &ExptResultServiceImpl{
				ExptTurnResultRepo:       mockExptTurnResultRepo,
				ExptItemResultRepo:       mockExptItemResultRepo,
				exptTurnResultFilterRepo: mockFilterRepo,
				ExperimentRepo:           mockExperimentRepo,
				evalTargetService:        mockEvalTargetService,
				evaluatorRecordService:   mockEvaluatorRecordService,
				ExptAnnotateRepo:         mockExptAnnotateRepo,
			}
			if err := svc.UpsertExptTurnResultFilter(context.Background(), tt.args.spaceID, tt.args.exptID, tt.args.itemIDs); (err != nil) != tt.wantErr {
				t.Errorf("UpsertExptTurnResultFilter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptResultServiceImpl_CompareExptTurnResultFilters(t *testing.T) {
	type args struct {
		spaceID    int64
		exptID     int64
		itemIDs    []int64
		retryTimes int32
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	mockFilterRepo := repoMocks.NewMockIExptTurnResultFilterRepo(ctrl)
	mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
	mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)
	mockMetric := metricsMocks.NewMockExptMetric(ctrl)
	mockLWT := lwtMocks.NewMockILatestWriteTracker(ctrl)
	mockIDGen := idgenMocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorService := svcMocks.NewMockEvaluatorService(ctrl)
	mockEvaluationSetVersionService := svcMocks.NewMockEvaluationSetVersionService(ctrl)
	mockEvaluationSetService := svcMocks.NewMockIEvaluationSetService(ctrl)
	mockEvaluationSetItemService := svcMocks.NewMockEvaluationSetItemService(ctrl)
	mockPublisher := eventsMocks.NewMockExptEventPublisher(ctrl)
	mockExptAnnotateRepo := repoMocks.NewMockIExptAnnotateRepo(ctrl)

	svc := &ExptResultServiceImpl{
		ExptTurnResultRepo:          mockExptTurnResultRepo,
		ExptItemResultRepo:          mockExptItemResultRepo,
		exptTurnResultFilterRepo:    mockFilterRepo,
		ExperimentRepo:              mockExperimentRepo,
		evalTargetService:           mockEvalTargetService,
		evaluatorRecordService:      mockEvaluatorRecordService,
		evaluationSetItemService:    mockEvaluationSetItemService,
		publisher:                   mockPublisher,
		lwt:                         mockLWT,
		evaluatorService:            mockEvaluatorService,
		evaluationSetVersionService: mockEvaluationSetVersionService,
		evaluationSetService:        mockEvaluationSetService,
		Metric:                      mockMetric,
		idgen:                       mockIDGen,
		ExptAnnotateRepo:            mockExptAnnotateRepo,
	}

	now := time.Now()

	defaultSetup := func() {
		// 设置 ExptAnnotateRepo Mock 避免 PayloadBuilder 构建时的 panic
		mockExptAnnotateRepo.EXPECT().GetExptTurnAnnotateRecordRefsByTurnResultIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnAnnotateRecordRef{}, nil).AnyTimes()
		mockExptAnnotateRepo.EXPECT().GetAnnotateRecordsByIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.AnnotateRecord{}, nil).AnyTimes()

		// 设置实验信息Mock
		mockExperimentRepo.EXPECT().MGetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.Experiment{{
			ID:               1,
			SpaceID:          100,
			ExptType:         entity.ExptType_Offline, // 离线实验
			StartAt:          &now,
			EvalSetVersionID: 101,
		}}, nil).AnyTimes()
		mockExperimentRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{
			ID:               1,
			SpaceID:          100,
			ExptType:         entity.ExptType_Offline, // 离线实验
			StartAt:          &now,
			EvalSetVersionID: 101,
		}, nil).AnyTimes()
		mockFilterRepo.EXPECT().GetByExptIDItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultFilterEntity{
			{
				SpaceID: 100,
				ExptID:  1,
				ItemID:  1,
				ItemIdx: 1,
				TurnID:  1,
				Status:  1,
				EvalTargetData: map[string]string{
					"actual_output": "some output",
				},
				EvaluatorScore: map[string]float64{
					"key1": 0.9,
				},
				EvaluatorScoreCorrected: true,
				EvalSetVersionID:        1,
			},
		}, nil).AnyTimes()
		mockMetric.EXPECT().EmitExptTurnResultFilterQueryLatency(gomock.Any(), gomock.Any(), gomock.Any()).Return().AnyTimes()
		mockFilterRepo.EXPECT().GetExptTurnResultFilterKeyMappings(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultFilterKeyMapping{
			{
				SpaceID:   100,
				ExptID:    1,
				FromField: "1",
				ToKey:     "key1",
				FieldType: entity.FieldTypeEvaluator,
			},
		}, nil).AnyTimes()
		mockExptTurnResultRepo.EXPECT().ListTurnResultByItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{
			{
				ID:             1,
				ExptID:         1,
				ItemID:         1,
				TurnID:         1,
				Status:         1,
				TargetResultID: 1,
			},
		}, int64(1), nil).AnyTimes()
		mockExptItemResultRepo.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{
			{
				ID:     1,
				ExptID: 1,
				ItemID: 1,
				Status: 1,
			},
		}, nil).AnyTimes()
		mockExperimentRepo.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptEvaluatorRef{
			{
				EvaluatorVersionID: 1,
				EvaluatorID:        1,
			},
		}, nil).AnyTimes()
		mockEvaluatorService.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.Evaluator{
			{
				ID:            1,
				Name:          "test_evaluator",
				Description:   "test description",
				EvaluatorType: entity.EvaluatorTypePrompt,
				PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
					ID:      1,
					Version: "v1",
				},
			},
		}, nil).AnyTimes()
		mockEvaluationSetItemService.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{
			{
				EvaluationSetID: 1,
				SchemaID:        1,
				ItemID:          1,
				ItemKey:         "1",
				Turns: []*entity.Turn{
					{
						ID: 1,
						FieldDataList: []*entity.FieldData{
							{
								Key:  "actual_output",
								Name: "actual_output",
								Content: &entity.Content{
									Text: ptr.Of("some output"),
								},
							},
						},
					},
				},
			},
		}, nil).AnyTimes()
		mockEvaluatorRecordService.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvaluatorRecord{
			{
				ID:                 1,
				SpaceID:            0,
				ExperimentID:       1,
				ItemID:             1,
				TurnID:             1,
				EvaluatorVersionID: 1,
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: ptr.Of(float64(9)),
					},
				},
			},
		}, nil).AnyTimes()
		mockEvalTargetService.EXPECT().BatchGetRecordByIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvalTargetRecord{
			{
				ID:                  1,
				SpaceID:             1,
				TargetID:            1,
				TargetVersionID:     1,
				ExperimentRunID:     1,
				ItemID:              1,
				TurnID:              1,
				EvalTargetInputData: nil,

				EvalTargetOutputData: &entity.EvalTargetOutputData{
					OutputFields: map[string]*entity.Content{
						"actual_output": {
							Text: ptr.Of("some output"),
						},
					},
				},
			},
		}, nil).AnyTimes()
		mockEvaluationSetService.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSet{}, nil).AnyTimes()
		mockEvaluationSetService.EXPECT().QueryItemSnapshotMappings(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ItemSnapshotFieldMapping{
			{
				FieldKey:      "field_key_string",
				MappingKey:    "string_map",
				MappingSubKey: "subkey_string",
			},
			{
				FieldKey:      "field_key_int",
				MappingKey:    "int_map",
				MappingSubKey: "subkey_int",
			},
			{
				FieldKey:      "field_key_float",
				MappingKey:    "float_map",
				MappingSubKey: "subkey_float",
			},
			{
				FieldKey:      "field_key_bool",
				MappingKey:    "bool_map",
				MappingSubKey: "subkey_bool",
			},
		}, "2025-01-01", nil).AnyTimes()
		mockEvaluationSetVersionService.EXPECT().GetEvaluationSetVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSetVersion{}, nil, nil).AnyTimes()
		mockExptItemResultRepo.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptItemResult{}, nil).AnyTimes()
		mockExptTurnResultRepo.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnEvaluatorResultRef{
			{
				ID:                 1,
				SpaceID:            1,
				ExptTurnResultID:   1,
				EvaluatorVersionID: 1,
				EvaluatorResultID:  1,
				ExptID:             1,
			},
		}, nil).AnyTimes()
		mockExptItemResultRepo.EXPECT().GetItemTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{}, nil).AnyTimes()
		mockExptTurnResultRepo.EXPECT().ListTurnResultByItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{
			{
				ID:     1,
				ItemID: 1,
			},
		}, int64(0), nil).AnyTimes()
		mockFilterRepo.EXPECT().QueryItemIDStates(gomock.Any(), gomock.Any()).Return(
			map[int64]entity.ItemRunState{int64(1): entity.ItemRunState_Success}, int64(1), nil,
		).AnyTimes()
		mockFilterRepo.EXPECT().GetExptTurnResultFilterKeyMappings(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultFilterKeyMapping{
			{
				SpaceID:   100,
				ExptID:    1,
				FromField: "1",
				ToKey:     "key1",
				FieldType: entity.FieldTypeEvaluator,
			},
		}, nil).AnyTimes()
		mockMetric.EXPECT().EmitExptTurnResultFilterCheck(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return().AnyTimes()
		mockPublisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	}

	tests := []struct {
		name    string
		args    args
		setup   func()
		wantErr bool
	}{
		{
			name: "正常比较过滤条件, retryTimes超过",
			args: args{
				spaceID:    100,
				exptID:     1,
				itemIDs:    []int64{1, 2},
				retryTimes: 3,
			},
			setup:   defaultSetup,
			wantErr: false,
		},
		{
			name: "正常比较过滤条件, retryTimes=0",
			args: args{
				spaceID:    100,
				exptID:     1,
				itemIDs:    []int64{1, 2},
				retryTimes: 0,
			},
			setup:   defaultSetup,
			wantErr: false,
		},
		// 新增测试用例：基于现有架构稍微增加覆盖率
		{
			name: "过滤器不存在场景测试",
			args: args{
				spaceID:    100,
				exptID:     2, // 使用不同的 exptID
				itemIDs:    []int64{2},
				retryTimes: 3,
			},
			setup: func() {
				// 基于 defaultSetup，但针对不同的 exptID 设置空过滤器
				defaultSetup()

				// 覆盖过滤器设置，使其为空（模拟过滤器不存在的情况）
				mockFilterRepo.EXPECT().GetByExptIDItemIDs(gomock.Any(), "100", "2", gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultFilterEntity{}, nil).AnyTimes()

				// 设置 TurnResult 存在，确保会进入 for 循环
				mockExptTurnResultRepo.EXPECT().ListTurnResultByItemIDs(gomock.Any(), int64(100), int64(2), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{
					{
						ID:     2,
						ExptID: 2,
						ItemID: 2,
						TurnID: 1,
						Status: 1,
					},
				}, int64(1), nil).AnyTimes()

				// 设置实验信息
				mockExperimentRepo.EXPECT().MGetByID(gomock.Any(), []int64{2}, int64(100)).Return([]*entity.Experiment{{
					ID:               2,
					SpaceID:          100,
					ExptType:         entity.ExptType_Offline,
					StartAt:          &now,
					EvalSetVersionID: 101,
				}}, nil).AnyTimes()

				// 验证指标上报 - 过滤器不存在且重试次数超过最大值
				mockMetric.EXPECT().EmitExptTurnResultFilterCheck(int64(100), false, false, true, true).Return().AnyTimes()
			},
			wantErr: false,
		},
		{
			name: "itemIDs为空时获取所有item",
			args: args{
				spaceID:    100,
				exptID:     3,
				itemIDs:    []int64{}, // 空的itemIDs
				retryTimes: 0,
			},
			setup: func() {
				defaultSetup()

				// 设置实验信息
				mockExperimentRepo.EXPECT().MGetByID(gomock.Any(), []int64{3}, int64(100)).Return([]*entity.Experiment{{
					ID:               3,
					SpaceID:          100,
					ExptType:         entity.ExptType_Offline,
					StartAt:          &now,
					EvalSetVersionID: 101,
				}}, nil).AnyTimes()

				// 模拟获取所有item的调用
				mockExptItemResultRepo.EXPECT().ListItemResultsByExptID(gomock.Any(), int64(3), int64(100), entity.Page{}, false).Return([]*entity.ExptItemResult{
					{
						ID:     1,
						ExptID: 3,
						ItemID: 10,
						Status: 1,
					},
					{
						ID:     2,
						ExptID: 3,
						ItemID: 20,
						Status: 1,
					},
				}, int64(2), nil).Times(1)

				// 设置过滤器查询
				mockFilterRepo.EXPECT().GetByExptIDItemIDs(gomock.Any(), "100", "3", gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultFilterEntity{
					{
						SpaceID: 100,
						ExptID:  3,
						ItemID:  10,
						ItemIdx: 1,
						TurnID:  1,
						Status:  1,
						EvalTargetData: map[string]string{
							"actual_output": "some output",
						},
						EvaluatorScore: map[string]float64{
							"key1": 0.9,
						},
						EvaluatorScoreCorrected: true,
						EvalSetVersionID:        1,
					},
					{
						SpaceID: 100,
						ExptID:  3,
						ItemID:  20,
						ItemIdx: 2,
						TurnID:  1,
						Status:  1,
						EvalTargetData: map[string]string{
							"actual_output": "some output",
						},
						EvaluatorScore: map[string]float64{
							"key1": 0.9,
						},
						EvaluatorScoreCorrected: true,
						EvalSetVersionID:        1,
					},
				}, nil).AnyTimes()

				// 设置TurnResult查询
				mockExptTurnResultRepo.EXPECT().ListTurnResultByItemIDs(gomock.Any(), int64(100), int64(3), []int64{10, 20}, gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResult{
					{
						ID:     10,
						ExptID: 3,
						ItemID: 10,
						TurnID: 1,
						Status: 1,
					},
					{
						ID:     20,
						ExptID: 3,
						ItemID: 20,
						TurnID: 1,
						Status: 1,
					},
				}, int64(2), nil).AnyTimes()

				// 验证指标上报
				mockMetric.EXPECT().EmitExptTurnResultFilterCheck(int64(100), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return().AnyTimes()
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			ctx := context.Background()
			err := svc.CompareExptTurnResultFilters(ctx, tt.args.spaceID, tt.args.exptID, tt.args.itemIDs, tt.args.retryTimes)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompareExptTurnResultFilters() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExptResultServiceImpl_ListTurnResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock dependencies
	mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockMetric := metricsMocks.NewMockExptMetric(ctrl)
	mockLwt := lwtMocks.NewMockILatestWriteTracker(ctrl)
	mockIdgen := idgenMocks.NewMockIIDGenerator(ctrl)
	mockExptTurnResultFilterRepo := repoMocks.NewMockIExptTurnResultFilterRepo(ctrl)
	mockEvaluatorService := svcMocks.NewMockEvaluatorService(ctrl)
	mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
	mockEvaluationSetVersionService := svcMocks.NewMockEvaluationSetVersionService(ctrl)
	mockEvaluationSetService := svcMocks.NewMockIEvaluationSetService(ctrl)
	mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)
	mockEvaluationSetItemService := svcMocks.NewMockEvaluationSetItemService(ctrl)
	mockPublisher := eventsMocks.NewMockExptEventPublisher(ctrl)

	service := ExptResultServiceImpl{
		ExptItemResultRepo:          mockExptItemResultRepo,
		ExptTurnResultRepo:          mockExptTurnResultRepo,
		ExptStatsRepo:               mockExptStatsRepo,
		ExperimentRepo:              mockExperimentRepo,
		Metric:                      mockMetric,
		lwt:                         mockLwt,
		idgen:                       mockIdgen,
		exptTurnResultFilterRepo:    mockExptTurnResultFilterRepo,
		evalTargetService:           mockEvalTargetService,
		evaluationSetVersionService: mockEvaluationSetVersionService,
		evaluationSetService:        mockEvaluationSetService,
		evaluatorService:            mockEvaluatorService,
		evaluatorRecordService:      mockEvaluatorRecordService,
		evaluationSetItemService:    mockEvaluationSetItemService,
		publisher:                   mockPublisher,
	}

	now := time.Now()

	tests := []struct {
		name                        string
		param                       *entity.MGetExperimentResultParam
		expt                        *entity.Experiment
		setup                       func()
		expectedTurnResults         []*entity.ExptTurnResult
		expectedItemID2ItemRunState map[int64]entity.ItemRunState
		expectedTotal               int64
		expectedError               error
	}{
		{
			name: "UseAccelerator=false, 正常流程",
			param: &entity.MGetExperimentResultParam{
				SpaceID:        100,
				ExptIDs:        []int64{1},
				BaseExptID:     gptr.Of(int64(1)),
				UseAccelerator: false,
				Page:           entity.NewPage(1, 20),
			},
			expt: &entity.Experiment{
				ID:       1,
				SpaceID:  100,
				ExptType: entity.ExptType_Offline,
				StartAt:  &now,
			},
			setup: func() {
				mockExptTurnResultRepo.EXPECT().
					ListTurnResult(gomock.Any(), int64(100), int64(1), nil, gomock.Any(), false).
					Return([]*entity.ExptTurnResult{
						{
							ID:      1,
							SpaceID: 100,
							ExptID:  1,
							ItemID:  10,
							TurnID:  20,
							Status:  int32(entity.TurnRunState_Success),
						},
					}, int64(1), nil).
					Times(1)

				// 添加 BatchGet mock 期望
				mockExptItemResultRepo.EXPECT().
					BatchGet(gomock.Any(), int64(100), int64(1), []int64{10}).
					Return([]*entity.ExptItemResult{
						{
							ID:      1,
							ItemID:  10,
							SpaceID: 100,
							ExptID:  1,
							ItemIdx: 1,
						},
					}, nil).
					Times(1)
			},
			expectedTurnResults: []*entity.ExptTurnResult{
				{
					ID:      1,
					SpaceID: 100,
					ExptID:  1,
					ItemID:  10,
					TurnID:  20,
					Status:  int32(entity.TurnRunState_Success),
				},
			},
			expectedItemID2ItemRunState: nil,
			expectedTotal:               1,
			expectedError:               nil,
		},
		{
			name: "UseAccelerator=false, 数据库错误",
			param: &entity.MGetExperimentResultParam{
				SpaceID:        100,
				ExptIDs:        []int64{1},
				BaseExptID:     gptr.Of(int64(1)),
				UseAccelerator: false,
				Page:           entity.NewPage(1, 20),
			},
			expt: &entity.Experiment{
				ID:       1,
				SpaceID:  100,
				ExptType: entity.ExptType_Offline,
				StartAt:  &now,
			},
			setup: func() {
				mockExptTurnResultRepo.EXPECT().
					ListTurnResult(gomock.Any(), int64(100), int64(1), nil, gomock.Any(), false).
					Return(nil, int64(0), errors.New("database error")).
					Times(1)
			},
			expectedTurnResults:         nil,
			expectedItemID2ItemRunState: nil,
			expectedTotal:               0,
			expectedError:               errors.New("database error"),
		},
		{
			name: "UseAccelerator=true, 无过滤器",
			param: &entity.MGetExperimentResultParam{
				SpaceID:            100,
				ExptIDs:            []int64{1},
				BaseExptID:         gptr.Of(int64(1)),
				UseAccelerator:     true,
				FilterAccelerators: map[int64]*entity.ExptTurnResultFilterAccelerator{},
				Page:               entity.NewPage(1, 20),
			},
			expt: &entity.Experiment{
				ID:       1,
				SpaceID:  100,
				ExptType: entity.ExptType_Offline,
				StartAt:  &now,
			},
			setup: func() {
				mockExptItemResultRepo.EXPECT().
					ListItemResultsByExptID(gomock.Any(), int64(1), int64(100), gomock.Any(), false).
					Return([]*entity.ExptItemResult{
						{
							ID:      1,
							ItemID:  10,
							SpaceID: 100,
							ExptID:  1,
						},
					}, int64(1), nil).
					Times(1)

				mockExptTurnResultRepo.EXPECT().
					ListTurnResultByItemIDs(gomock.Any(), int64(100), int64(1), []int64{10}, entity.Page{}, false).
					Return([]*entity.ExptTurnResult{
						{
							ID:      1,
							SpaceID: 100,
							ExptID:  1,
							ItemID:  10,
							TurnID:  20,
							Status:  int32(entity.TurnRunState_Success),
						},
					}, int64(1), nil).
					Times(1)

				// 添加 BatchGet mock 期望
				mockExptItemResultRepo.EXPECT().
					BatchGet(gomock.Any(), int64(100), int64(1), []int64{10}).
					Return([]*entity.ExptItemResult{
						{
							ID:      1,
							ItemID:  10,
							SpaceID: 100,
							ExptID:  1,
							ItemIdx: 1,
						},
					}, nil).
					Times(1)
			},
			expectedTurnResults: []*entity.ExptTurnResult{
				{
					ID:      1,
					SpaceID: 100,
					ExptID:  1,
					ItemID:  10,
					TurnID:  20,
					Status:  int32(entity.TurnRunState_Success),
				},
			},
			expectedItemID2ItemRunState: nil,
			expectedTotal:               1,
			expectedError:               nil,
		},
		{
			name: "UseAccelerator=true, 有过滤器",
			param: &entity.MGetExperimentResultParam{
				SpaceID:        100,
				ExptIDs:        []int64{1},
				BaseExptID:     gptr.Of(int64(1)),
				UseAccelerator: true,
				FilterAccelerators: map[int64]*entity.ExptTurnResultFilterAccelerator{
					1: {
						SpaceID: 100,
						ExptID:  1,
						ItemIDs: []*entity.FieldFilter{
							{Key: "test"},
						},
					},
				},
				Page: entity.NewPage(1, 20),
			},
			expt: &entity.Experiment{
				ID:               1,
				SpaceID:          100,
				ExptType:         entity.ExptType_Offline,
				StartAt:          &now,
				EvalSetVersionID: 5,
			},
			setup: func() {
				mockExptTurnResultFilterRepo.EXPECT().
					QueryItemIDStates(gomock.Any(), gomock.Any()).
					Return(map[int64]entity.ItemRunState{
						10: entity.ItemRunState_Success,
					}, int64(1), nil).
					Times(1)

				mockMetric.EXPECT().
					EmitExptTurnResultFilterQueryLatency(int64(100), gomock.Any(), false).
					Times(1)

				mockExptTurnResultRepo.EXPECT().
					ListTurnResultByItemIDs(gomock.Any(), int64(100), int64(1), []int64{10}, entity.Page{}, false).
					Return([]*entity.ExptTurnResult{
						{
							ID:      1,
							SpaceID: 100,
							ExptID:  1,
							ItemID:  10,
							TurnID:  20,
							Status:  int32(entity.TurnRunState_Success),
						},
					}, int64(1), nil).
					Times(1)

				// 添加 BatchGet mock 期望
				mockExptItemResultRepo.EXPECT().
					BatchGet(gomock.Any(), int64(100), int64(1), []int64{10}).
					Return([]*entity.ExptItemResult{
						{
							ID:      1,
							ItemID:  10,
							SpaceID: 100,
							ExptID:  1,
							ItemIdx: 1,
						},
					}, nil).
					Times(1)
			},
			expectedTurnResults: []*entity.ExptTurnResult{
				{
					ID:      1,
					SpaceID: 100,
					ExptID:  1,
					ItemID:  10,
					TurnID:  20,
					Status:  int32(entity.TurnRunState_Success),
				},
			},
			expectedItemID2ItemRunState: map[int64]entity.ItemRunState{
				10: entity.ItemRunState_Success,
			},
			expectedTotal: 1,
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			turnResults, itemID2ItemRunState, total, err := service.ListTurnResult(context.Background(), tt.param, tt.expt)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedTurnResults, turnResults)
			assert.Equal(t, tt.expectedItemID2ItemRunState, itemID2ItemRunState)
			assert.Equal(t, tt.expectedTotal, total)
		})
	}
}

func TestExptResultServiceImpl_ListTurnResult_EdgeCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock dependencies
	mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockMetric := metricsMocks.NewMockExptMetric(ctrl)
	mockLwt := lwtMocks.NewMockILatestWriteTracker(ctrl)
	mockIdgen := idgenMocks.NewMockIIDGenerator(ctrl)
	mockExptTurnResultFilterRepo := repoMocks.NewMockIExptTurnResultFilterRepo(ctrl)
	mockEvaluatorService := svcMocks.NewMockEvaluatorService(ctrl)
	mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
	mockEvaluationSetVersionService := svcMocks.NewMockEvaluationSetVersionService(ctrl)
	mockEvaluationSetService := svcMocks.NewMockIEvaluationSetService(ctrl)
	mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)
	mockEvaluationSetItemService := svcMocks.NewMockEvaluationSetItemService(ctrl)
	mockPublisher := eventsMocks.NewMockExptEventPublisher(ctrl)

	service := ExptResultServiceImpl{
		ExptItemResultRepo:          mockExptItemResultRepo,
		ExptTurnResultRepo:          mockExptTurnResultRepo,
		ExptStatsRepo:               mockExptStatsRepo,
		ExperimentRepo:              mockExperimentRepo,
		Metric:                      mockMetric,
		lwt:                         mockLwt,
		idgen:                       mockIdgen,
		exptTurnResultFilterRepo:    mockExptTurnResultFilterRepo,
		evalTargetService:           mockEvalTargetService,
		evaluationSetVersionService: mockEvaluationSetVersionService,
		evaluationSetService:        mockEvaluationSetService,
		evaluatorService:            mockEvaluatorService,
		evaluatorRecordService:      mockEvaluatorRecordService,
		evaluationSetItemService:    mockEvaluationSetItemService,
		publisher:                   mockPublisher,
	}

	now := time.Now()

	t.Run("UseAccelerator=false, 有过滤器", func(t *testing.T) {
		param := &entity.MGetExperimentResultParam{
			SpaceID:        100,
			ExptIDs:        []int64{1},
			BaseExptID:     gptr.Of(int64(1)),
			UseAccelerator: false,
			Filters: map[int64]*entity.ExptTurnResultFilter{
				1: {
					TrunRunStateFilters: []*entity.TurnRunStateFilter{
						{
							Status:   []entity.TurnRunState{entity.TurnRunState_Success},
							Operator: "=",
						},
					},
				},
			},
			Page: entity.NewPage(1, 20),
		}

		expt := &entity.Experiment{
			ID:       1,
			SpaceID:  100,
			ExptType: entity.ExptType_Offline,
			StartAt:  &now,
		}

		expectedFilter := &entity.ExptTurnResultFilter{
			TrunRunStateFilters: []*entity.TurnRunStateFilter{
				{
					Status:   []entity.TurnRunState{entity.TurnRunState_Success},
					Operator: "=",
				},
			},
		}

		mockExptTurnResultRepo.EXPECT().
			ListTurnResult(gomock.Any(), int64(100), int64(1), expectedFilter, gomock.Any(), false).
			Return([]*entity.ExptTurnResult{
				{
					ID:      1,
					SpaceID: 100,
					ExptID:  1,
					ItemID:  10,
					TurnID:  20,
					Status:  int32(entity.TurnRunState_Success),
				},
			}, int64(1), nil).
			Times(1)

		// 添加 BatchGet mock 期望
		mockExptItemResultRepo.EXPECT().
			BatchGet(gomock.Any(), int64(100), int64(1), []int64{10}).
			Return([]*entity.ExptItemResult{
				{
					ID:      1,
					ItemID:  10,
					SpaceID: 100,
					ExptID:  1,
					ItemIdx: 1,
				},
			}, nil).
			Times(1)

		turnResults, itemID2ItemRunState, total, err := service.ListTurnResult(context.Background(), param, expt)

		assert.NoError(t, err)
		assert.Len(t, turnResults, 1)
		assert.Equal(t, int64(1), turnResults[0].ID)
		assert.Nil(t, itemID2ItemRunState)
		assert.Equal(t, int64(1), total)
	})
}

func TestParseTurnKey(t *testing.T) {
	tests := []struct {
		name          string
		turnKey       string
		want          *TurnKeyComponents
		wantErr       bool
		expectedError string
	}{
		// 正常场景
		{
			name:    "正常解析-基本数值",
			turnKey: "123_456_789_012",
			want: &TurnKeyComponents{
				SpaceID: 123,
				ExptID:  456,
				ItemID:  789,
				TurnID:  12,
			},
			wantErr: false,
		},
		{
			name:    "正常解析-零值",
			turnKey: "0_0_0_0",
			want: &TurnKeyComponents{
				SpaceID: 0,
				ExptID:  0,
				ItemID:  0,
				TurnID:  0,
			},
			wantErr: false,
		},
		{
			name:    "正常解析-大数值",
			turnKey: "999999999_888888888_777777777_666666666",
			want: &TurnKeyComponents{
				SpaceID: 999999999,
				ExptID:  888888888,
				ItemID:  777777777,
				TurnID:  666666666,
			},
			wantErr: false,
		},
		{
			name:    "正常解析-最大int64值",
			turnKey: "9223372036854775807_9223372036854775807_9223372036854775807_9223372036854775807",
			want: &TurnKeyComponents{
				SpaceID: 9223372036854775807,
				ExptID:  9223372036854775807,
				ItemID:  9223372036854775807,
				TurnID:  9223372036854775807,
			},
			wantErr: false,
		},
		{
			name:    "正常解析-负数值",
			turnKey: "-1_-2_-3_-4",
			want: &TurnKeyComponents{
				SpaceID: -1,
				ExptID:  -2,
				ItemID:  -3,
				TurnID:  -4,
			},
			wantErr: false,
		},
		{
			name:    "正常解析-混合正负数",
			turnKey: "-1_2_-3_4",
			want: &TurnKeyComponents{
				SpaceID: -1,
				ExptID:  2,
				ItemID:  -3,
				TurnID:  4,
			},
			wantErr: false,
		},
		{
			name:    "正常解析-最小int64值",
			turnKey: "-9223372036854775808_-9223372036854775808_-9223372036854775808_-9223372036854775808",
			want: &TurnKeyComponents{
				SpaceID: -9223372036854775808,
				ExptID:  -9223372036854775808,
				ItemID:  -9223372036854775808,
				TurnID:  -9223372036854775808,
			},
			wantErr: false,
		},
		// 错误场景 - 格式错误
		{
			name:          "空字符串",
			turnKey:       "",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid turnKey format:",
		},
		{
			name:          "无分隔符",
			turnKey:       "123456789012",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid turnKey format:",
		},
		{
			name:          "分隔符不足-1个",
			turnKey:       "123_456",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid turnKey format:",
		},
		{
			name:          "分隔符不足-2个",
			turnKey:       "123_456_789",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid turnKey format:",
		},
		{
			name:          "分隔符过多",
			turnKey:       "123_456_789_012_345",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid turnKey format:",
		},
		// 错误场景 - 数值解析错误
		{
			name:          "spaceID非数字",
			turnKey:       "abc_456_789_012",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid spaceID in turnKey:",
		},
		{
			name:          "exptID非数字",
			turnKey:       "123_abc_789_012",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid exptID in turnKey:",
		},
		{
			name:          "itemID非数字",
			turnKey:       "123_456_abc_012",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid itemID in turnKey:",
		},
		{
			name:          "turnID非数字",
			turnKey:       "123_456_789_abc",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid turnID in turnKey:",
		},
		{
			name:          "spaceID为空",
			turnKey:       "_456_789_012",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid spaceID in turnKey:",
		},
		{
			name:          "exptID为空",
			turnKey:       "123__789_012",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid exptID in turnKey:",
		},
		{
			name:          "itemID为空",
			turnKey:       "123_456__012",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid itemID in turnKey:",
		},
		{
			name:          "turnID为空",
			turnKey:       "123_456_789_",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid turnID in turnKey:",
		},
		{
			name:          "spaceID超出int64范围",
			turnKey:       "92233720368547758080_456_789_012",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid spaceID in turnKey:",
		},
		{
			name:          "包含浮点数",
			turnKey:       "123.5_456_789_012",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid spaceID in turnKey:",
		},
		{
			name:          "包含特殊字符",
			turnKey:       "123@_456_789_012",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid spaceID in turnKey:",
		},
		{
			name:          "包含空格",
			turnKey:       "123 _456_789_012",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid spaceID in turnKey:",
		},
		{
			name:          "包含制表符",
			turnKey:       "123\t_456_789_012",
			want:          nil,
			wantErr:       true,
			expectedError: "invalid spaceID in turnKey:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTurnKey(tt.turnKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTurnKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseTurnKey() expected error but got none")
					return
				}
				if tt.expectedError != "" && !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("ParseTurnKey() error = %v, expected to contain %v", err, tt.expectedError)
				}
				if got != nil {
					t.Errorf("ParseTurnKey() expected nil result when error occurs, got %v", got)
				}
			} else {
				if err != nil {
					t.Errorf("ParseTurnKey() unexpected error = %v", err)
					return
				}
				if got == nil {
					t.Errorf("ParseTurnKey() expected non-nil result, got nil")
					return
				}
				if got.SpaceID != tt.want.SpaceID {
					t.Errorf("ParseTurnKey() got.SpaceID = %v, want %v", got.SpaceID, tt.want.SpaceID)
				}
				if got.ExptID != tt.want.ExptID {
					t.Errorf("ParseTurnKey() got.ExptID = %v, want %v", got.ExptID, tt.want.ExptID)
				}
				if got.ItemID != tt.want.ItemID {
					t.Errorf("ParseTurnKey() got.ItemID = %v, want %v", got.ItemID, tt.want.ItemID)
				}
				if got.TurnID != tt.want.TurnID {
					t.Errorf("ParseTurnKey() got.RecordID = %v, want %v", got.TurnID, tt.want.TurnID)
				}
			}
		})
	}
}

func TestNewPayloadBuilder_ExtFieldAndItemRunState(t *testing.T) {
	tests := []struct {
		name                string
		baselineItemResults []*entity.ExptItemResult
		baselineTurnResults []*entity.ExptTurnResult
		itemID2ItemRunState map[int64]entity.ItemRunState
		wantExt             map[string]string
		wantRunState        entity.ItemRunState
	}{
		{
			name: "Ext字段有值且itemID2ItemRunState存在",
			baselineItemResults: []*entity.ExptItemResult{
				{
					ItemID:  1,
					ItemIdx: 0,
					Status:  entity.ItemRunState_Success,
					Ext: map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
				},
			},
			baselineTurnResults: []*entity.ExptTurnResult{
				{
					ID:      1,
					ItemID:  1,
					TurnID:  0,
					TurnIdx: 0,
				},
			},
			itemID2ItemRunState: map[int64]entity.ItemRunState{
				1: entity.ItemRunState_Processing,
			},
			wantExt: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			wantRunState: entity.ItemRunState_Processing,
		},
		{
			name: "Ext字段为空map且itemID2ItemRunState不存在",
			baselineItemResults: []*entity.ExptItemResult{
				{
					ItemID:  1,
					ItemIdx: 0,
					Status:  entity.ItemRunState_Success,
					Ext:     map[string]string{},
				},
			},
			baselineTurnResults: []*entity.ExptTurnResult{
				{
					ID:      1,
					ItemID:  1,
					TurnID:  0,
					TurnIdx: 0,
				},
			},
			itemID2ItemRunState: map[int64]entity.ItemRunState{},
			wantExt:             nil,
			wantRunState:        entity.ItemRunState_Success,
		},
		{
			name: "Ext字段为nil且itemID2ItemRunState不存在",
			baselineItemResults: []*entity.ExptItemResult{
				{
					ItemID:  1,
					ItemIdx: 0,
					Status:  entity.ItemRunState_Fail,
					Ext:     nil,
				},
			},
			baselineTurnResults: []*entity.ExptTurnResult{
				{
					ID:      1,
					ItemID:  1,
					TurnID:  0,
					TurnIdx: 0,
				},
			},
			itemID2ItemRunState: map[int64]entity.ItemRunState{},
			wantExt:             nil,
			wantRunState:        entity.ItemRunState_Fail,
		},
		{
			name: "Ext字段有值且itemID2ItemRunState不存在",
			baselineItemResults: []*entity.ExptItemResult{
				{
					ItemID:  1,
					ItemIdx: 0,
					Status:  entity.ItemRunState_Success,
					Ext: map[string]string{
						"span_id": "span-123",
					},
				},
			},
			baselineTurnResults: []*entity.ExptTurnResult{
				{
					ID:      1,
					ItemID:  1,
					TurnID:  0,
					TurnIdx: 0,
				},
			},
			itemID2ItemRunState: map[int64]entity.ItemRunState{},
			wantExt: map[string]string{
				"span_id": "span-123",
			},
			wantRunState: entity.ItemRunState_Success,
		},
		{
			name: "多个ItemResult，Ext字段和itemID2ItemRunState混合",
			baselineItemResults: []*entity.ExptItemResult{
				{
					ItemID:  1,
					ItemIdx: 0,
					Status:  entity.ItemRunState_Success,
					Ext: map[string]string{
						"key1": "value1",
					},
				},
				{
					ItemID:  2,
					ItemIdx: 1,
					Status:  entity.ItemRunState_Fail,
					Ext:     map[string]string{},
				},
			},
			baselineTurnResults: []*entity.ExptTurnResult{
				{
					ID:      1,
					ItemID:  1,
					TurnID:  0,
					TurnIdx: 0,
				},
				{
					ID:      2,
					ItemID:  2,
					TurnID:  0,
					TurnIdx: 0,
				},
			},
			itemID2ItemRunState: map[int64]entity.ItemRunState{
				1: entity.ItemRunState_Processing,
			},
			wantExt: map[string]string{
				"key1": "value1",
			},
			wantRunState: entity.ItemRunState_Processing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// 创建必要的 mocks
			mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
			mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
			mockExptAnnotateRepo := repoMocks.NewMockIExptAnnotateRepo(ctrl)
			mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
			mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)
			mockEvaluationSetItemService := svcMocks.NewMockEvaluationSetItemService(ctrl)

			// 创建参数
			param := &entity.MGetExperimentResultParam{
				SpaceID: 100,
				ExptIDs: []int64{1},
			}

			// 调用 NewPayloadBuilder
			builder := NewPayloadBuilder(
				context.Background(),
				param,
				1,
				tt.baselineTurnResults,
				tt.baselineItemResults,
				mockExperimentRepo,
				mockExptTurnResultRepo,
				mockExptAnnotateRepo,
				mockEvalTargetService,
				mockEvaluatorRecordService,
				mockEvaluationSetItemService,
				nil,
				nil,
				nil,
				tt.itemID2ItemRunState,
			)

			// 验证结果
			assert.NotNil(t, builder)
			assert.NotNil(t, builder.ItemResults)

			// 验证第一个 ItemResult 的 Ext 字段和 RunState
			if len(builder.ItemResults) > 0 {
				firstItemResult := builder.ItemResults[0]
				assert.NotNil(t, firstItemResult.SystemInfo)

				// 验证 Ext 字段
				if tt.wantExt == nil {
					assert.Nil(t, firstItemResult.Ext)
				} else {
					assert.NotNil(t, firstItemResult.Ext)
					assert.Equal(t, tt.wantExt, firstItemResult.Ext)
				}

				// 验证 RunState
				assert.Equal(t, tt.wantRunState, firstItemResult.SystemInfo.RunState)
			}

			// 如果有多个 ItemResult，验证第二个
			if len(builder.ItemResults) > 1 && len(tt.baselineItemResults) > 1 {
				secondItemResult := builder.ItemResults[1]
				assert.NotNil(t, secondItemResult.SystemInfo)
				// 第二个 ItemResult 的 Ext 应该是空的（因为 baselineItemResults[1].Ext 是空 map）
				assert.Nil(t, secondItemResult.Ext)
				// 第二个 ItemResult 的 RunState 应该是 baselineItemResults[1].Status（因为 itemID2ItemRunState 中没有 2）
				assert.Equal(t, tt.baselineItemResults[1].Status, secondItemResult.SystemInfo.RunState)
			}
		})
	}
}

func TestExptResultBuilder_buildTargetOutput(t *testing.T) {
	tests := []struct {
		name           string
		exptType       entity.ExptType
		fullTrajectory bool
		setup          func(ctrl *gomock.Controller) (*ExptResultBuilder, *svcMocks.MockIEvalTargetService)
		wantErr        bool
		checkFunc      func(t *testing.T, builder *ExptResultBuilder)
	}{
		{
			name:           "Online experiment should skip buildTargetOutput",
			exptType:       entity.ExptType_Online,
			fullTrajectory: false,
			setup: func(ctrl *gomock.Controller) (*ExptResultBuilder, *svcMocks.MockIEvalTargetService) {
				mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
				builder := &ExptResultBuilder{
					exptDO: &entity.Experiment{
						ID:       1,
						ExptType: entity.ExptType_Online,
					},
					SpaceID:           100,
					turnResultDO:      []*entity.ExptTurnResult{},
					evalTargetService: mockEvalTargetService,
					FullTrajectory:    false,
				}
				return builder, mockEvalTargetService
			},
			wantErr: false,
			checkFunc: func(t *testing.T, builder *ExptResultBuilder) {
				assert.Nil(t, builder.turnResultID2TargetOutput)
			},
		},
		{
			name:           "FullTrajectory=false should trim trajectory field",
			exptType:       entity.ExptType_Offline,
			fullTrajectory: false,
			setup: func(ctrl *gomock.Controller) (*ExptResultBuilder, *svcMocks.MockIEvalTargetService) {
				mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
				builder := &ExptResultBuilder{
					exptDO: &entity.Experiment{
						ID:       1,
						ExptType: entity.ExptType_Offline,
					},
					SpaceID: 100,
					turnResultDO: []*entity.ExptTurnResult{
						{
							ID:             10,
							TargetResultID: 1,
						},
					},
					evalTargetService: mockEvalTargetService,
					FullTrajectory:    false,
				}
				// 创建一个有效的 JSON 对象作为 trajectory
				fullTrajectoryJSON := `{"id":"trace-1","root_step":{"step_id":"step-1","type":"tool_call","content":"very long content that should be trimmed"}}`
				mockEvalTargetService.EXPECT().
					BatchGetRecordByIDs(gomock.Any(), int64(100), []int64{1}).
					Return([]*entity.EvalTargetRecord{
						{
							ID: 1,
							EvalTargetOutputData: &entity.EvalTargetOutputData{
								OutputFields: map[string]*entity.Content{
									"actual_output": {
										Text: gptr.Of("test output"),
									},
									consts.EvalTargetOutputFieldKeyTrajectory: {
										Text: gptr.Of(fullTrajectoryJSON),
									},
								},
							},
						},
					}, nil)
				return builder, mockEvalTargetService
			},
			wantErr: false,
			checkFunc: func(t *testing.T, builder *ExptResultBuilder) {
				assert.NotNil(t, builder.turnResultID2TargetOutput)
				targetOutput, ok := builder.turnResultID2TargetOutput[10]
				assert.True(t, ok)
				assert.NotNil(t, targetOutput)
				assert.NotNil(t, targetOutput.EvalTargetRecord)
				assert.NotNil(t, targetOutput.EvalTargetRecord.EvalTargetOutputData)
				// trajectory 字段应该被剪裁而不是删除
				trajectoryContent, hasTrajectory := targetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields[consts.EvalTargetOutputFieldKeyTrajectory]
				assert.True(t, hasTrajectory, "trajectory field should exist when FullTrajectory=false, but should be trimmed")
				assert.NotNil(t, trajectoryContent)
				assert.NotNil(t, trajectoryContent.Text)
				// 验证内容已被剪裁（使用 generateJsonObjectPreview）
				originalJSON := `{"id":"trace-1","root_step":{"step_id":"step-1","type":"tool_call","content":"very long content that should be trimmed"}}`
				expectedPreview := utils.GenerateJsonObjectPreview(originalJSON)
				assert.Equal(t, expectedPreview, *trajectoryContent.Text, "trajectory should be trimmed using generateJsonObjectPreview")
				// actual_output 字段应该保留
				_, hasActualOutput := targetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields["actual_output"]
				assert.True(t, hasActualOutput, "actual_output field should be preserved")
			},
		},
		{
			name:           "FullTrajectory=true should preserve trajectory field",
			exptType:       entity.ExptType_Offline,
			fullTrajectory: true,
			setup: func(ctrl *gomock.Controller) (*ExptResultBuilder, *svcMocks.MockIEvalTargetService) {
				mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
				builder := &ExptResultBuilder{
					exptDO: &entity.Experiment{
						ID:       1,
						ExptType: entity.ExptType_Offline,
					},
					SpaceID: 100,
					turnResultDO: []*entity.ExptTurnResult{
						{
							ID:             10,
							TargetResultID: 1,
						},
					},
					evalTargetService: mockEvalTargetService,
					FullTrajectory:    true,
				}
				mockEvalTargetService.EXPECT().
					BatchGetRecordByIDs(gomock.Any(), int64(100), []int64{1}).
					Return([]*entity.EvalTargetRecord{
						{
							ID: 1,
							EvalTargetOutputData: &entity.EvalTargetOutputData{
								OutputFields: map[string]*entity.Content{
									"actual_output": {
										Text: gptr.Of("test output"),
									},
									consts.EvalTargetOutputFieldKeyTrajectory: {
										Text: gptr.Of("test trajectory"),
									},
								},
							},
						},
					}, nil)
				return builder, mockEvalTargetService
			},
			wantErr: false,
			checkFunc: func(t *testing.T, builder *ExptResultBuilder) {
				assert.NotNil(t, builder.turnResultID2TargetOutput)
				targetOutput, ok := builder.turnResultID2TargetOutput[10]
				assert.True(t, ok)
				assert.NotNil(t, targetOutput)
				assert.NotNil(t, targetOutput.EvalTargetRecord)
				assert.NotNil(t, targetOutput.EvalTargetRecord.EvalTargetOutputData)
				// trajectory 字段应该保留
				_, hasTrajectory := targetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields[consts.EvalTargetOutputFieldKeyTrajectory]
				assert.True(t, hasTrajectory, "trajectory field should be preserved when FullTrajectory=true")
			},
		},
		{
			name:           "FullTrajectory=false, nil OutputFields should not panic",
			exptType:       entity.ExptType_Offline,
			fullTrajectory: false,
			setup: func(ctrl *gomock.Controller) (*ExptResultBuilder, *svcMocks.MockIEvalTargetService) {
				mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
				builder := &ExptResultBuilder{
					exptDO: &entity.Experiment{
						ID:       1,
						ExptType: entity.ExptType_Offline,
					},
					SpaceID: 100,
					turnResultDO: []*entity.ExptTurnResult{
						{
							ID:             10,
							TargetResultID: 1,
						},
					},
					evalTargetService: mockEvalTargetService,
					FullTrajectory:    false,
				}
				mockEvalTargetService.EXPECT().
					BatchGetRecordByIDs(gomock.Any(), int64(100), []int64{1}).
					Return([]*entity.EvalTargetRecord{
						{
							ID: 1,
							EvalTargetOutputData: &entity.EvalTargetOutputData{
								OutputFields: nil,
							},
						},
					}, nil)
				return builder, mockEvalTargetService
			},
			wantErr: false,
			checkFunc: func(t *testing.T, builder *ExptResultBuilder) {
				assert.NotNil(t, builder.turnResultID2TargetOutput)
				targetOutput, ok := builder.turnResultID2TargetOutput[10]
				assert.True(t, ok)
				assert.NotNil(t, targetOutput)
			},
		},
		{
			name:           "FullTrajectory=false, invalid JSON should not be modified",
			exptType:       entity.ExptType_Offline,
			fullTrajectory: false,
			setup: func(ctrl *gomock.Controller) (*ExptResultBuilder, *svcMocks.MockIEvalTargetService) {
				mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
				builder := &ExptResultBuilder{
					exptDO: &entity.Experiment{
						ID:       1,
						ExptType: entity.ExptType_Offline,
					},
					SpaceID: 100,
					turnResultDO: []*entity.ExptTurnResult{
						{
							ID:             10,
							TargetResultID: 1,
						},
					},
					evalTargetService: mockEvalTargetService,
					FullTrajectory:    false,
				}
				// 创建一个无效的 JSON（不是对象格式）
				invalidJSON := `"not a json object"`
				mockEvalTargetService.EXPECT().
					BatchGetRecordByIDs(gomock.Any(), int64(100), []int64{1}).
					Return([]*entity.EvalTargetRecord{
						{
							ID: 1,
							EvalTargetOutputData: &entity.EvalTargetOutputData{
								OutputFields: map[string]*entity.Content{
									"actual_output": {
										Text: gptr.Of("test output"),
									},
									consts.EvalTargetOutputFieldKeyTrajectory: {
										Text: gptr.Of(invalidJSON),
									},
								},
							},
						},
					}, nil)
				return builder, mockEvalTargetService
			},
			wantErr: false,
			checkFunc: func(t *testing.T, builder *ExptResultBuilder) {
				assert.NotNil(t, builder.turnResultID2TargetOutput)
				targetOutput, ok := builder.turnResultID2TargetOutput[10]
				assert.True(t, ok)
				assert.NotNil(t, targetOutput)
				assert.NotNil(t, targetOutput.EvalTargetRecord)
				assert.NotNil(t, targetOutput.EvalTargetRecord.EvalTargetOutputData)
				// trajectory 字段应该存在，但内容不变（因为不是有效的 JSON 对象）
				trajectoryContent, hasTrajectory := targetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields[consts.EvalTargetOutputFieldKeyTrajectory]
				assert.True(t, hasTrajectory, "trajectory field should exist")
				assert.NotNil(t, trajectoryContent)
				assert.NotNil(t, trajectoryContent.Text)
				// 内容应该保持不变（因为 generateJsonObjectPreview 对无效 JSON 返回空字符串）
				assert.Equal(t, `"not a json object"`, *trajectoryContent.Text, "invalid JSON should not be modified")
			},
		},
		{
			name:           "FullTrajectory=false, empty Text should not panic",
			exptType:       entity.ExptType_Offline,
			fullTrajectory: false,
			setup: func(ctrl *gomock.Controller) (*ExptResultBuilder, *svcMocks.MockIEvalTargetService) {
				mockEvalTargetService := svcMocks.NewMockIEvalTargetService(ctrl)
				builder := &ExptResultBuilder{
					exptDO: &entity.Experiment{
						ID:       1,
						ExptType: entity.ExptType_Offline,
					},
					SpaceID: 100,
					turnResultDO: []*entity.ExptTurnResult{
						{
							ID:             10,
							TargetResultID: 1,
						},
					},
					evalTargetService: mockEvalTargetService,
					FullTrajectory:    false,
				}
				emptyText := ""
				mockEvalTargetService.EXPECT().
					BatchGetRecordByIDs(gomock.Any(), int64(100), []int64{1}).
					Return([]*entity.EvalTargetRecord{
						{
							ID: 1,
							EvalTargetOutputData: &entity.EvalTargetOutputData{
								OutputFields: map[string]*entity.Content{
									"actual_output": {
										Text: gptr.Of("test output"),
									},
									consts.EvalTargetOutputFieldKeyTrajectory: {
										Text: &emptyText,
									},
								},
							},
						},
					}, nil)
				return builder, mockEvalTargetService
			},
			wantErr: false,
			checkFunc: func(t *testing.T, builder *ExptResultBuilder) {
				assert.NotNil(t, builder.turnResultID2TargetOutput)
				targetOutput, ok := builder.turnResultID2TargetOutput[10]
				assert.True(t, ok)
				assert.NotNil(t, targetOutput)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			builder, _ := tt.setup(ctrl)
			err := builder.buildTargetOutput(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkFunc != nil {
					tt.checkFunc(t, builder)
				}
			}
		})
	}
}

func TestExptResultBuilder_buildEvaluatorResult(t *testing.T) {
	tests := []struct {
		name           string
		fullTrajectory bool
		setup          func(ctrl *gomock.Controller) (*ExptResultBuilder, *repoMocks.MockIExptTurnResultRepo, *svcMocks.MockEvaluatorRecordService)
		wantErr        bool
		checkFunc      func(t *testing.T, builder *ExptResultBuilder)
	}{
		{
			name:           "FullTrajectory=false should trim trajectory in InputFields",
			fullTrajectory: false,
			setup: func(ctrl *gomock.Controller) (*ExptResultBuilder, *repoMocks.MockIExptTurnResultRepo, *svcMocks.MockEvaluatorRecordService) {
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)

				fullTrajectoryJSON := `{"id":"trace-1","root_step":{"step_id":"step-1","type":"tool_call","content":"very long content that should be trimmed"}}`

				builder := &ExptResultBuilder{
					exptDO: &entity.Experiment{
						ID:       1,
						ExptType: entity.ExptType_Offline,
					},
					SpaceID: 100,
					turnResultDO: []*entity.ExptTurnResult{
						{
							ID: 10,
						},
					},
					ExptTurnResultRepo:     mockExptTurnResultRepo,
					evaluatorRecordService: mockEvaluatorRecordService,
					FullTrajectory:         false,
				}

				mockExptTurnResultRepo.EXPECT().
					BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(100), []int64{10}).
					Return([]*entity.ExptTurnEvaluatorResultRef{
						{ExptTurnResultID: 10, EvaluatorResultID: 1001, EvaluatorVersionID: 201},
					}, nil)

				mockEvaluatorRecordService.EXPECT().
					BatchGetEvaluatorRecord(gomock.Any(), []int64{1001}, false).
					Return([]*entity.EvaluatorRecord{
						{
							ID:                 1001,
							EvaluatorVersionID: 201,
							EvaluatorInputData: &entity.EvaluatorInputData{
								InputFields: map[string]*entity.Content{
									consts.EvalTargetOutputFieldKeyTrajectory: {
										Text: gptr.Of(fullTrajectoryJSON),
									},
								},
							},
						},
					}, nil)

				return builder, mockExptTurnResultRepo, mockEvaluatorRecordService
			},
			wantErr: false,
			checkFunc: func(t *testing.T, builder *ExptResultBuilder) {
				assert.NotNil(t, builder.turnResultID2EvaluatorVersionID2Result)
				evaluatorRecords, ok := builder.turnResultID2EvaluatorVersionID2Result[10]
				assert.True(t, ok)
				assert.NotNil(t, evaluatorRecords)
				evaluatorRecord, ok := evaluatorRecords[201]
				assert.True(t, ok)
				assert.NotNil(t, evaluatorRecord)
				assert.NotNil(t, evaluatorRecord.EvaluatorInputData)
				assert.NotNil(t, evaluatorRecord.EvaluatorInputData.InputFields)

				trajectoryContent, hasTrajectory := evaluatorRecord.EvaluatorInputData.InputFields[consts.EvalTargetOutputFieldKeyTrajectory]
				assert.True(t, hasTrajectory, "trajectory field should exist")
				assert.NotNil(t, trajectoryContent)
				assert.NotNil(t, trajectoryContent.Text)

				// 验证内容已被剪裁
				originalJSON := `{"id":"trace-1","root_step":{"step_id":"step-1","type":"tool_call","content":"very long content that should be trimmed"}}`
				expectedPreview := utils.GenerateJsonObjectPreview(originalJSON)
				expectedTrimmed := utils.GenerateTextPreview(expectedPreview)
				assert.Equal(t, expectedTrimmed, *trajectoryContent.Text, "trajectory should be trimmed")
			},
		},
		{
			name:           "FullTrajectory=false should trim trajectory in EvaluateTargetOutputFields",
			fullTrajectory: false,
			setup: func(ctrl *gomock.Controller) (*ExptResultBuilder, *repoMocks.MockIExptTurnResultRepo, *svcMocks.MockEvaluatorRecordService) {
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)

				fullTrajectoryJSON := `{"id":"trace-2","root_step":{"step_id":"step-2","type":"message","content":"another very long content"}}`

				builder := &ExptResultBuilder{
					exptDO: &entity.Experiment{
						ID:       1,
						ExptType: entity.ExptType_Offline,
					},
					SpaceID: 100,
					turnResultDO: []*entity.ExptTurnResult{
						{
							ID: 10,
						},
					},
					ExptTurnResultRepo:     mockExptTurnResultRepo,
					evaluatorRecordService: mockEvaluatorRecordService,
					FullTrajectory:         false,
				}

				mockExptTurnResultRepo.EXPECT().
					BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(100), []int64{10}).
					Return([]*entity.ExptTurnEvaluatorResultRef{
						{ExptTurnResultID: 10, EvaluatorResultID: 1001, EvaluatorVersionID: 201},
					}, nil)

				mockEvaluatorRecordService.EXPECT().
					BatchGetEvaluatorRecord(gomock.Any(), []int64{1001}, false).
					Return([]*entity.EvaluatorRecord{
						{
							ID:                 1001,
							EvaluatorVersionID: 201,
							EvaluatorInputData: &entity.EvaluatorInputData{
								EvaluateTargetOutputFields: map[string]*entity.Content{
									consts.EvalTargetOutputFieldKeyTrajectory: {
										Text: gptr.Of(fullTrajectoryJSON),
									},
								},
							},
						},
					}, nil)

				return builder, mockExptTurnResultRepo, mockEvaluatorRecordService
			},
			wantErr: false,
			checkFunc: func(t *testing.T, builder *ExptResultBuilder) {
				assert.NotNil(t, builder.turnResultID2EvaluatorVersionID2Result)
				evaluatorRecords, ok := builder.turnResultID2EvaluatorVersionID2Result[10]
				assert.True(t, ok)
				assert.NotNil(t, evaluatorRecords)
				evaluatorRecord, ok := evaluatorRecords[201]
				assert.True(t, ok)
				assert.NotNil(t, evaluatorRecord)
				assert.NotNil(t, evaluatorRecord.EvaluatorInputData)
				assert.NotNil(t, evaluatorRecord.EvaluatorInputData.EvaluateTargetOutputFields)

				trajectoryContent, hasTrajectory := evaluatorRecord.EvaluatorInputData.EvaluateTargetOutputFields[consts.EvalTargetOutputFieldKeyTrajectory]
				assert.True(t, hasTrajectory, "trajectory field should exist")
				assert.NotNil(t, trajectoryContent)
				assert.NotNil(t, trajectoryContent.Text)

				// 验证内容已被剪裁
				originalJSON := `{"id":"trace-2","root_step":{"step_id":"step-2","type":"message","content":"another very long content"}}`
				expectedPreview := utils.GenerateJsonObjectPreview(originalJSON)
				expectedTrimmed := utils.GenerateTextPreview(expectedPreview)
				assert.Equal(t, expectedTrimmed, *trajectoryContent.Text, "trajectory should be trimmed")
			},
		},
		{
			name:           "FullTrajectory=false should trim trajectory in both InputFields and EvaluateTargetOutputFields",
			fullTrajectory: false,
			setup: func(ctrl *gomock.Controller) (*ExptResultBuilder, *repoMocks.MockIExptTurnResultRepo, *svcMocks.MockEvaluatorRecordService) {
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)

				fullTrajectoryJSON1 := `{"id":"trace-1","root_step":{"step_id":"step-1"}}`
				fullTrajectoryJSON2 := `{"id":"trace-2","root_step":{"step_id":"step-2"}}`

				builder := &ExptResultBuilder{
					exptDO: &entity.Experiment{
						ID:       1,
						ExptType: entity.ExptType_Offline,
					},
					SpaceID: 100,
					turnResultDO: []*entity.ExptTurnResult{
						{
							ID: 10,
						},
					},
					ExptTurnResultRepo:     mockExptTurnResultRepo,
					evaluatorRecordService: mockEvaluatorRecordService,
					FullTrajectory:         false,
				}

				mockExptTurnResultRepo.EXPECT().
					BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(100), []int64{10}).
					Return([]*entity.ExptTurnEvaluatorResultRef{
						{ExptTurnResultID: 10, EvaluatorResultID: 1001, EvaluatorVersionID: 201},
					}, nil)

				mockEvaluatorRecordService.EXPECT().
					BatchGetEvaluatorRecord(gomock.Any(), []int64{1001}, false).
					Return([]*entity.EvaluatorRecord{
						{
							ID:                 1001,
							EvaluatorVersionID: 201,
							EvaluatorInputData: &entity.EvaluatorInputData{
								InputFields: map[string]*entity.Content{
									consts.EvalTargetOutputFieldKeyTrajectory: {
										Text: gptr.Of(fullTrajectoryJSON1),
									},
								},
								EvaluateTargetOutputFields: map[string]*entity.Content{
									consts.EvalTargetOutputFieldKeyTrajectory: {
										Text: gptr.Of(fullTrajectoryJSON2),
									},
								},
							},
						},
					}, nil)

				return builder, mockExptTurnResultRepo, mockEvaluatorRecordService
			},
			wantErr: false,
			checkFunc: func(t *testing.T, builder *ExptResultBuilder) {
				assert.NotNil(t, builder.turnResultID2EvaluatorVersionID2Result)
				evaluatorRecords, ok := builder.turnResultID2EvaluatorVersionID2Result[10]
				assert.True(t, ok)
				assert.NotNil(t, evaluatorRecords)
				evaluatorRecord, ok := evaluatorRecords[201]
				assert.True(t, ok)
				assert.NotNil(t, evaluatorRecord)

				// 验证 InputFields 中的 trajectory 被剪裁
				trajectoryContent1, hasTrajectory1 := evaluatorRecord.EvaluatorInputData.InputFields[consts.EvalTargetOutputFieldKeyTrajectory]
				assert.True(t, hasTrajectory1)
				assert.NotNil(t, trajectoryContent1.Text)
				expectedPreview1 := utils.GenerateJsonObjectPreview(`{"id":"trace-1","root_step":{"step_id":"step-1"}}`)
				expectedTrimmed1 := utils.GenerateTextPreview(expectedPreview1)
				assert.Equal(t, expectedTrimmed1, *trajectoryContent1.Text)

				// 验证 EvaluateTargetOutputFields 中的 trajectory 被剪裁
				trajectoryContent2, hasTrajectory2 := evaluatorRecord.EvaluatorInputData.EvaluateTargetOutputFields[consts.EvalTargetOutputFieldKeyTrajectory]
				assert.True(t, hasTrajectory2)
				assert.NotNil(t, trajectoryContent2.Text)
				expectedPreview2 := utils.GenerateJsonObjectPreview(`{"id":"trace-2","root_step":{"step_id":"step-2"}}`)
				expectedTrimmed2 := utils.GenerateTextPreview(expectedPreview2)
				assert.Equal(t, expectedTrimmed2, *trajectoryContent2.Text)
			},
		},
		{
			name:           "FullTrajectory=true should preserve trajectory",
			fullTrajectory: true,
			setup: func(ctrl *gomock.Controller) (*ExptResultBuilder, *repoMocks.MockIExptTurnResultRepo, *svcMocks.MockEvaluatorRecordService) {
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)

				fullTrajectoryJSON := `{"id":"trace-1","root_step":{"step_id":"step-1"}}`

				builder := &ExptResultBuilder{
					exptDO: &entity.Experiment{
						ID:       1,
						ExptType: entity.ExptType_Offline,
					},
					SpaceID: 100,
					turnResultDO: []*entity.ExptTurnResult{
						{
							ID: 10,
						},
					},
					ExptTurnResultRepo:     mockExptTurnResultRepo,
					evaluatorRecordService: mockEvaluatorRecordService,
					FullTrajectory:         true,
				}

				mockExptTurnResultRepo.EXPECT().
					BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(100), []int64{10}).
					Return([]*entity.ExptTurnEvaluatorResultRef{
						{ExptTurnResultID: 10, EvaluatorResultID: 1001, EvaluatorVersionID: 201},
					}, nil)

				mockEvaluatorRecordService.EXPECT().
					BatchGetEvaluatorRecord(gomock.Any(), []int64{1001}, false).
					Return([]*entity.EvaluatorRecord{
						{
							ID:                 1001,
							EvaluatorVersionID: 201,
							EvaluatorInputData: &entity.EvaluatorInputData{
								InputFields: map[string]*entity.Content{
									consts.EvalTargetOutputFieldKeyTrajectory: {
										Text: gptr.Of(fullTrajectoryJSON),
									},
								},
							},
						},
					}, nil)

				return builder, mockExptTurnResultRepo, mockEvaluatorRecordService
			},
			wantErr: false,
			checkFunc: func(t *testing.T, builder *ExptResultBuilder) {
				assert.NotNil(t, builder.turnResultID2EvaluatorVersionID2Result)
				evaluatorRecords, ok := builder.turnResultID2EvaluatorVersionID2Result[10]
				assert.True(t, ok)
				assert.NotNil(t, evaluatorRecords)
				evaluatorRecord, ok := evaluatorRecords[201]
				assert.True(t, ok)
				assert.NotNil(t, evaluatorRecord)

				// 验证 trajectory 未被剪裁（保持原样）
				trajectoryContent, hasTrajectory := evaluatorRecord.EvaluatorInputData.InputFields[consts.EvalTargetOutputFieldKeyTrajectory]
				assert.True(t, hasTrajectory)
				assert.NotNil(t, trajectoryContent)
				assert.Equal(t, `{"id":"trace-1","root_step":{"step_id":"step-1"}}`, *trajectoryContent.Text, "trajectory should be preserved when FullTrajectory=true")
			},
		},
		{
			name:           "FullTrajectory=false with invalid JSON should use GenerateTextPreview directly",
			fullTrajectory: false,
			setup: func(ctrl *gomock.Controller) (*ExptResultBuilder, *repoMocks.MockIExptTurnResultRepo, *svcMocks.MockEvaluatorRecordService) {
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)

				invalidJSON := `"not a json object"`

				builder := &ExptResultBuilder{
					exptDO: &entity.Experiment{
						ID:       1,
						ExptType: entity.ExptType_Offline,
					},
					SpaceID: 100,
					turnResultDO: []*entity.ExptTurnResult{
						{
							ID: 10,
						},
					},
					ExptTurnResultRepo:     mockExptTurnResultRepo,
					evaluatorRecordService: mockEvaluatorRecordService,
					FullTrajectory:         false,
				}

				mockExptTurnResultRepo.EXPECT().
					BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(100), []int64{10}).
					Return([]*entity.ExptTurnEvaluatorResultRef{
						{ExptTurnResultID: 10, EvaluatorResultID: 1001, EvaluatorVersionID: 201},
					}, nil)

				mockEvaluatorRecordService.EXPECT().
					BatchGetEvaluatorRecord(gomock.Any(), []int64{1001}, false).
					Return([]*entity.EvaluatorRecord{
						{
							ID:                 1001,
							EvaluatorVersionID: 201,
							EvaluatorInputData: &entity.EvaluatorInputData{
								InputFields: map[string]*entity.Content{
									consts.EvalTargetOutputFieldKeyTrajectory: {
										Text: gptr.Of(invalidJSON),
									},
								},
							},
						},
					}, nil)

				return builder, mockExptTurnResultRepo, mockEvaluatorRecordService
			},
			wantErr: false,
			checkFunc: func(t *testing.T, builder *ExptResultBuilder) {
				assert.NotNil(t, builder.turnResultID2EvaluatorVersionID2Result)
				evaluatorRecords, ok := builder.turnResultID2EvaluatorVersionID2Result[10]
				assert.True(t, ok)
				assert.NotNil(t, evaluatorRecords)
				evaluatorRecord, ok := evaluatorRecords[201]
				assert.True(t, ok)
				assert.NotNil(t, evaluatorRecord)

				// 验证无效 JSON 时直接使用 GenerateTextPreview
				trajectoryContent, hasTrajectory := evaluatorRecord.EvaluatorInputData.InputFields[consts.EvalTargetOutputFieldKeyTrajectory]
				assert.True(t, hasTrajectory)
				assert.NotNil(t, trajectoryContent)
				expectedTrimmed := utils.GenerateTextPreview(`"not a json object"`)
				assert.Equal(t, expectedTrimmed, *trajectoryContent.Text, "invalid JSON should be trimmed using GenerateTextPreview directly")
			},
		},
		{
			name:           "FullTrajectory=false with nil InputFields should not panic",
			fullTrajectory: false,
			setup: func(ctrl *gomock.Controller) (*ExptResultBuilder, *repoMocks.MockIExptTurnResultRepo, *svcMocks.MockEvaluatorRecordService) {
				mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)

				builder := &ExptResultBuilder{
					exptDO: &entity.Experiment{
						ID:       1,
						ExptType: entity.ExptType_Offline,
					},
					SpaceID: 100,
					turnResultDO: []*entity.ExptTurnResult{
						{
							ID: 10,
						},
					},
					ExptTurnResultRepo:     mockExptTurnResultRepo,
					evaluatorRecordService: mockEvaluatorRecordService,
					FullTrajectory:         false,
				}

				mockExptTurnResultRepo.EXPECT().
					BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(100), []int64{10}).
					Return([]*entity.ExptTurnEvaluatorResultRef{
						{ExptTurnResultID: 10, EvaluatorResultID: 1001, EvaluatorVersionID: 201},
					}, nil)

				mockEvaluatorRecordService.EXPECT().
					BatchGetEvaluatorRecord(gomock.Any(), []int64{1001}, false).
					Return([]*entity.EvaluatorRecord{
						{
							ID:                 1001,
							EvaluatorVersionID: 201,
							EvaluatorInputData: &entity.EvaluatorInputData{
								InputFields: nil,
							},
						},
					}, nil)

				return builder, mockExptTurnResultRepo, mockEvaluatorRecordService
			},
			wantErr: false,
			checkFunc: func(t *testing.T, builder *ExptResultBuilder) {
				assert.NotNil(t, builder.turnResultID2EvaluatorVersionID2Result)
				evaluatorRecords, ok := builder.turnResultID2EvaluatorVersionID2Result[10]
				assert.True(t, ok)
				assert.NotNil(t, evaluatorRecords)
				evaluatorRecord, ok := evaluatorRecords[201]
				assert.True(t, ok)
				assert.NotNil(t, evaluatorRecord)
				assert.Nil(t, evaluatorRecord.EvaluatorInputData.InputFields)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			builder, _, _ := tt.setup(ctrl)
			err := builder.buildEvaluatorResult(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkFunc != nil {
					tt.checkFunc(t, builder)
				}
			}
		})
	}
}

// TestExptResultServiceImpl_RecordItemRunLogs_ScoreWeights 测试 RecordItemRunLogs 中构建 scoreWeights 的逻辑（181-194行）
func TestExptResultServiceImpl_RecordItemRunLogs_ScoreWeights(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	exptID := int64(1)
	exptRunID := int64(1)
	itemID := int64(1)
	spaceID := int64(100)

	t.Run("启用加权分数，构建权重映射", func(t *testing.T) {
		mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
		mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
		mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
		mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)
		mockPublisher := eventsMocks.NewMockExptEventPublisher(ctrl)
		mockIdgen := idgenMocks.NewMockIIDGenerator(ctrl)
		mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)

		service := ExptResultServiceImpl{
			ExptItemResultRepo:     mockExptItemResultRepo,
			ExptTurnResultRepo:     mockExptTurnResultRepo,
			ExptStatsRepo:          mockExptStatsRepo,
			evaluatorRecordService: mockEvaluatorRecordService,
			publisher:              mockPublisher,
			idgen:                  mockIdgen,
			ExperimentRepo:         mockExperimentRepo,
		}

		// Mock GetItemRunLog
		mockExptItemResultRepo.EXPECT().
			GetItemRunLog(ctx, exptID, exptRunID, itemID, spaceID).
			Return(&entity.ExptItemResultRunLog{
				Status:      1,
				ResultState: int32(entity.ExptItemResultStateLogged),
			}, nil)

		// Mock BatchGet
		mockExptItemResultRepo.EXPECT().
			BatchGet(ctx, spaceID, exptID, []int64{itemID}).
			Return([]*entity.ExptItemResult{
				{ID: 1, ItemID: itemID, Status: entity.ItemRunState_Processing},
			}, nil)

		// Mock GetItemTurnRunLogs
		mockExptTurnResultRepo.EXPECT().
			GetItemTurnRunLogs(ctx, exptID, exptRunID, itemID, spaceID).
			Return([]*entity.ExptTurnResultRunLog{
				{
					TurnID:             1,
					Status:             entity.TurnRunState_Success,
					EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{101: 1}},
				},
			}, nil)

		// Mock GetItemTurnResults
		mockExptItemResultRepo.EXPECT().
			GetItemTurnResults(ctx, spaceID, exptID, itemID).
			Return([]*entity.ExptTurnResult{
				{ID: 1, TurnID: 1, Status: int32(entity.TurnRunState_Success)},
			}, nil)

		// Mock GetByID - 返回启用加权分数的实验配置
		weight1 := 0.6
		weight2 := 0.4
		mockExperimentRepo.EXPECT().
			GetByID(ctx, exptID, spaceID).
			Return(&entity.Experiment{
				ID: exptID,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						EvaluatorsConf: &entity.EvaluatorsConf{
							EnableScoreWeight: true,
							EvaluatorConf: []*entity.EvaluatorConf{
								{
									EvaluatorVersionID: 101,
									ScoreWeight:        &weight1,
								},
								{
									EvaluatorVersionID: 102,
									ScoreWeight:        &weight2,
								},
								{
									EvaluatorVersionID: 103,
									ScoreWeight:        nil, // nil 权重应该被跳过
								},
								{
									EvaluatorVersionID: 104,
									ScoreWeight:        gptr.Of(0.0), // 0 权重应该被跳过
								},
							},
						},
					},
				},
			}, nil)

		// Mock BatchGetEvaluatorRecord
		mockEvaluatorRecordService.EXPECT().
			BatchGetEvaluatorRecord(ctx, []int64{1}, false).
			Return([]*entity.EvaluatorRecord{
				{
					ID:                 1,
					EvaluatorVersionID: 101,
					EvaluatorOutputData: &entity.EvaluatorOutputData{
						EvaluatorResult: &entity.EvaluatorResult{
							Score: gptr.Of(0.8),
						},
					},
				},
			}, nil)

		// Mock idgen
		mockIdgen.EXPECT().
			GenMultiIDs(ctx, 1).
			Return([]int64{1}, nil)

		// Mock CreateTurnEvaluatorRefs
		mockExptTurnResultRepo.EXPECT().
			CreateTurnEvaluatorRefs(ctx, gomock.Any()).
			Return(nil)

		// Mock SaveTurnResults
		mockExptTurnResultRepo.EXPECT().
			SaveTurnResults(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, results []*entity.ExptTurnResult) error {
				// 验证加权分数被正确计算
				if len(results) > 0 && results[0].WeightedScore != nil {
					// 权重为 0.6，分数为 0.8，加权分数应该是 0.8 * 0.6 / 0.6 = 0.8
					assert.NotNil(t, results[0].WeightedScore)
				}
				return nil
			})

		// Mock UpdateItemsResult
		mockExptItemResultRepo.EXPECT().
			UpdateItemsResult(ctx, spaceID, exptID, []int64{itemID}, gomock.Any()).
			Return(nil)

		// Mock UpdateItemRunLog
		mockExptItemResultRepo.EXPECT().
			UpdateItemRunLog(ctx, exptID, exptRunID, []int64{itemID}, gomock.Any(), spaceID).
			Return(nil)

		// Mock ArithOperateCount
		mockExptStatsRepo.EXPECT().
			ArithOperateCount(ctx, exptID, spaceID, gomock.Any()).
			Return(nil)

		_, err := service.RecordItemRunLogs(ctx, exptID, exptRunID, itemID, spaceID)
		assert.NoError(t, err)
	})

	t.Run("未启用加权分数，不构建权重映射", func(t *testing.T) {
		mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
		mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
		mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
		mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)
		mockPublisher := eventsMocks.NewMockExptEventPublisher(ctrl)
		mockIdgen := idgenMocks.NewMockIIDGenerator(ctrl)
		mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)

		service := ExptResultServiceImpl{
			ExptItemResultRepo:     mockExptItemResultRepo,
			ExptTurnResultRepo:     mockExptTurnResultRepo,
			ExptStatsRepo:          mockExptStatsRepo,
			evaluatorRecordService: mockEvaluatorRecordService,
			publisher:              mockPublisher,
			idgen:                  mockIdgen,
			ExperimentRepo:         mockExperimentRepo,
		}

		// Mock GetItemRunLog
		mockExptItemResultRepo.EXPECT().
			GetItemRunLog(ctx, exptID, exptRunID, itemID, spaceID).
			Return(&entity.ExptItemResultRunLog{
				Status:      1,
				ResultState: int32(entity.ExptItemResultStateLogged),
			}, nil)

		// Mock BatchGet
		mockExptItemResultRepo.EXPECT().
			BatchGet(ctx, spaceID, exptID, []int64{itemID}).
			Return([]*entity.ExptItemResult{
				{ID: 1, ItemID: itemID, Status: entity.ItemRunState_Processing},
			}, nil)

		// Mock GetItemTurnRunLogs
		mockExptTurnResultRepo.EXPECT().
			GetItemTurnRunLogs(ctx, exptID, exptRunID, itemID, spaceID).
			Return([]*entity.ExptTurnResultRunLog{
				{
					TurnID:             1,
					Status:             entity.TurnRunState_Success,
					EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{101: 1}},
				},
			}, nil)

		// Mock GetItemTurnResults
		mockExptItemResultRepo.EXPECT().
			GetItemTurnResults(ctx, spaceID, exptID, itemID).
			Return([]*entity.ExptTurnResult{
				{ID: 1, TurnID: 1, Status: int32(entity.TurnRunState_Success)},
			}, nil)

		// Mock GetByID - 返回未启用加权分数的实验配置
		mockExperimentRepo.EXPECT().
			GetByID(ctx, exptID, spaceID).
			Return(&entity.Experiment{
				ID: exptID,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						EvaluatorsConf: &entity.EvaluatorsConf{
							EnableScoreWeight: false, // 未启用加权分数
						},
					},
				},
			}, nil)

		// Mock idgen
		mockIdgen.EXPECT().
			GenMultiIDs(ctx, 1).
			Return([]int64{1}, nil)

		// Mock CreateTurnEvaluatorRefs
		mockExptTurnResultRepo.EXPECT().
			CreateTurnEvaluatorRefs(ctx, gomock.Any()).
			Return(nil)

		// Mock SaveTurnResults
		mockExptTurnResultRepo.EXPECT().
			SaveTurnResults(ctx, gomock.Any()).
			Return(nil)

		// Mock UpdateItemsResult
		mockExptItemResultRepo.EXPECT().
			UpdateItemsResult(ctx, spaceID, exptID, []int64{itemID}, gomock.Any()).
			Return(nil)

		// Mock UpdateItemRunLog
		mockExptItemResultRepo.EXPECT().
			UpdateItemRunLog(ctx, exptID, exptRunID, []int64{itemID}, gomock.Any(), spaceID).
			Return(nil)

		// Mock ArithOperateCount
		mockExptStatsRepo.EXPECT().
			ArithOperateCount(ctx, exptID, spaceID, gomock.Any()).
			Return(nil)

		_, err := service.RecordItemRunLogs(ctx, exptID, exptRunID, itemID, spaceID)
		assert.NoError(t, err)
	})
}

// TestExptResultServiceImpl_RecordItemRunLogs_CalculateWeightedScore 测试 RecordItemRunLogs 中计算加权分数的逻辑（216-242行）
func TestExptResultServiceImpl_RecordItemRunLogs_CalculateWeightedScore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	exptID := int64(1)
	exptRunID := int64(1)
	itemID := int64(1)
	spaceID := int64(100)

	t.Run("成功计算加权分数", func(t *testing.T) {
		mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
		mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
		mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
		mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)
		mockPublisher := eventsMocks.NewMockExptEventPublisher(ctrl)
		mockIdgen := idgenMocks.NewMockIIDGenerator(ctrl)
		mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)

		service := ExptResultServiceImpl{
			ExptItemResultRepo:     mockExptItemResultRepo,
			ExptTurnResultRepo:     mockExptTurnResultRepo,
			ExptStatsRepo:          mockExptStatsRepo,
			evaluatorRecordService: mockEvaluatorRecordService,
			publisher:              mockPublisher,
			idgen:                  mockIdgen,
			ExperimentRepo:         mockExperimentRepo,
		}

		// Mock GetItemRunLog
		mockExptItemResultRepo.EXPECT().
			GetItemRunLog(ctx, exptID, exptRunID, itemID, spaceID).
			Return(&entity.ExptItemResultRunLog{
				Status:      1,
				ResultState: int32(entity.ExptItemResultStateLogged),
			}, nil)

		// Mock BatchGet
		mockExptItemResultRepo.EXPECT().
			BatchGet(ctx, spaceID, exptID, []int64{itemID}).
			Return([]*entity.ExptItemResult{
				{ID: 1, ItemID: itemID, Status: entity.ItemRunState_Processing},
			}, nil)

		// Mock GetItemTurnRunLogs - 包含评估器结果ID
		mockExptTurnResultRepo.EXPECT().
			GetItemTurnRunLogs(ctx, exptID, exptRunID, itemID, spaceID).
			Return([]*entity.ExptTurnResultRunLog{
				{
					TurnID:             1,
					Status:             entity.TurnRunState_Success,
					EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{101: 1, 102: 2}},
				},
			}, nil)

		// Mock GetItemTurnResults
		mockExptItemResultRepo.EXPECT().
			GetItemTurnResults(ctx, spaceID, exptID, itemID).
			Return([]*entity.ExptTurnResult{
				{ID: 1, TurnID: 1, Status: int32(entity.TurnRunState_Success)},
			}, nil)

		// Mock GetByID - 返回启用加权分数的实验配置
		weight1 := 0.6
		weight2 := 0.4
		mockExperimentRepo.EXPECT().
			GetByID(ctx, exptID, spaceID).
			Return(&entity.Experiment{
				ID: exptID,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						EvaluatorsConf: &entity.EvaluatorsConf{
							EnableScoreWeight: true,
							EvaluatorConf: []*entity.EvaluatorConf{
								{EvaluatorVersionID: 101, ScoreWeight: &weight1},
								{EvaluatorVersionID: 102, ScoreWeight: &weight2},
							},
						},
					},
				},
			}, nil)

		// Mock BatchGetEvaluatorRecord - 返回两个评估器记录
		// 注意：由于 map 遍历顺序不确定，使用 gomock.Any() 匹配参数顺序
		score1 := 0.8
		score2 := 0.9
		mockEvaluatorRecordService.EXPECT().
			BatchGetEvaluatorRecord(ctx, gomock.Any(), false).
			DoAndReturn(func(_ context.Context, ids []int64, _ bool) ([]*entity.EvaluatorRecord, error) {
				// 根据传入的ID顺序返回对应的记录
				records := make([]*entity.EvaluatorRecord, 0, len(ids))
				for _, id := range ids {
					switch id {
					case 1:
						records = append(records, &entity.EvaluatorRecord{
							ID:                 1,
							EvaluatorVersionID: 101,
							EvaluatorOutputData: &entity.EvaluatorOutputData{
								EvaluatorResult: &entity.EvaluatorResult{
									Score: &score1,
								},
							},
						})
					case 2:
						records = append(records, &entity.EvaluatorRecord{
							ID:                 2,
							EvaluatorVersionID: 102,
							EvaluatorOutputData: &entity.EvaluatorOutputData{
								EvaluatorResult: &entity.EvaluatorResult{
									Score: &score2,
								},
							},
						})
					}
				}
				return records, nil
			})

		// Mock idgen
		mockIdgen.EXPECT().
			GenMultiIDs(ctx, 2).
			Return([]int64{1, 2}, nil)

		// Mock CreateTurnEvaluatorRefs
		mockExptTurnResultRepo.EXPECT().
			CreateTurnEvaluatorRefs(ctx, gomock.Any()).
			Return(nil)

		// Mock SaveTurnResults - 验证加权分数被正确计算
		mockExptTurnResultRepo.EXPECT().
			SaveTurnResults(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, results []*entity.ExptTurnResult) error {
				if assert.Len(t, results, 1) {
					// 加权分数 = (0.8 * 0.6 + 0.9 * 0.4) / (0.6 + 0.4) = (0.48 + 0.36) / 1.0 = 0.84
					if assert.NotNil(t, results[0].WeightedScore) {
						expected := 0.84
						assert.InDelta(t, expected, *results[0].WeightedScore, 0.01)
					}
				}
				return nil
			})

		// Mock UpdateItemsResult
		mockExptItemResultRepo.EXPECT().
			UpdateItemsResult(ctx, spaceID, exptID, []int64{itemID}, gomock.Any()).
			Return(nil)

		// Mock UpdateItemRunLog
		mockExptItemResultRepo.EXPECT().
			UpdateItemRunLog(ctx, exptID, exptRunID, []int64{itemID}, gomock.Any(), spaceID).
			Return(nil)

		// Mock ArithOperateCount
		mockExptStatsRepo.EXPECT().
			ArithOperateCount(ctx, exptID, spaceID, gomock.Any()).
			Return(nil)

		_, err := service.RecordItemRunLogs(ctx, exptID, exptRunID, itemID, spaceID)
		assert.NoError(t, err)
	})

	t.Run("BatchGetEvaluatorRecord 失败，记录错误但不中断流程", func(t *testing.T) {
		mockExptItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
		mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
		mockExptStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
		mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)
		mockPublisher := eventsMocks.NewMockExptEventPublisher(ctrl)
		mockIdgen := idgenMocks.NewMockIIDGenerator(ctrl)
		mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)

		service := ExptResultServiceImpl{
			ExptItemResultRepo:     mockExptItemResultRepo,
			ExptTurnResultRepo:     mockExptTurnResultRepo,
			ExptStatsRepo:          mockExptStatsRepo,
			evaluatorRecordService: mockEvaluatorRecordService,
			publisher:              mockPublisher,
			idgen:                  mockIdgen,
			ExperimentRepo:         mockExperimentRepo,
		}

		// Mock GetItemRunLog
		mockExptItemResultRepo.EXPECT().
			GetItemRunLog(ctx, exptID, exptRunID, itemID, spaceID).
			Return(&entity.ExptItemResultRunLog{
				Status:      1,
				ResultState: int32(entity.ExptItemResultStateLogged),
			}, nil)

		// Mock BatchGet
		mockExptItemResultRepo.EXPECT().
			BatchGet(ctx, spaceID, exptID, []int64{itemID}).
			Return([]*entity.ExptItemResult{
				{ID: 1, ItemID: itemID, Status: entity.ItemRunState_Processing},
			}, nil)

		// Mock GetItemTurnRunLogs
		mockExptTurnResultRepo.EXPECT().
			GetItemTurnRunLogs(ctx, exptID, exptRunID, itemID, spaceID).
			Return([]*entity.ExptTurnResultRunLog{
				{
					TurnID:             1,
					Status:             entity.TurnRunState_Success,
					EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{101: 1}},
				},
			}, nil)

		// Mock GetItemTurnResults
		mockExptItemResultRepo.EXPECT().
			GetItemTurnResults(ctx, spaceID, exptID, itemID).
			Return([]*entity.ExptTurnResult{
				{ID: 1, TurnID: 1, Status: int32(entity.TurnRunState_Success)},
			}, nil)

		// Mock GetByID
		weight1 := 0.6
		mockExperimentRepo.EXPECT().
			GetByID(ctx, exptID, spaceID).
			Return(&entity.Experiment{
				ID: exptID,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						EvaluatorsConf: &entity.EvaluatorsConf{
							EnableScoreWeight: true,
							EvaluatorConf: []*entity.EvaluatorConf{
								{EvaluatorVersionID: 101, ScoreWeight: &weight1},
							},
						},
					},
				},
			}, nil)

		// Mock BatchGetEvaluatorRecord - 返回错误
		mockEvaluatorRecordService.EXPECT().
			BatchGetEvaluatorRecord(ctx, []int64{1}, false).
			Return(nil, errors.New("db error"))

		// Mock idgen
		mockIdgen.EXPECT().
			GenMultiIDs(ctx, 1).
			Return([]int64{1}, nil)

		// Mock CreateTurnEvaluatorRefs
		mockExptTurnResultRepo.EXPECT().
			CreateTurnEvaluatorRefs(ctx, gomock.Any()).
			Return(nil)

		// Mock SaveTurnResults - 加权分数应该为 nil
		mockExptTurnResultRepo.EXPECT().
			SaveTurnResults(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, results []*entity.ExptTurnResult) error {
				if assert.Len(t, results, 1) {
					assert.Nil(t, results[0].WeightedScore)
				}
				return nil
			})

		// Mock UpdateItemsResult
		mockExptItemResultRepo.EXPECT().
			UpdateItemsResult(ctx, spaceID, exptID, []int64{itemID}, gomock.Any()).
			Return(nil)

		// Mock UpdateItemRunLog
		mockExptItemResultRepo.EXPECT().
			UpdateItemRunLog(ctx, exptID, exptRunID, []int64{itemID}, gomock.Any(), spaceID).
			Return(nil)

		// Mock ArithOperateCount
		mockExptStatsRepo.EXPECT().
			ArithOperateCount(ctx, exptID, spaceID, gomock.Any()).
			Return(nil)

		_, err := service.RecordItemRunLogs(ctx, exptID, exptRunID, itemID, spaceID)
		assert.NoError(t, err) // 即使 BatchGetEvaluatorRecord 失败，流程也应该继续
	})
}

// TestCalculateWeightedScore 测试 calculateWeightedScore 函数（1442-1547行）
func TestCalculateWeightedScore(t *testing.T) {
	t.Run("空记录返回 nil", func(t *testing.T) {
		result := calculateWeightedScore(nil, nil)
		assert.Nil(t, result)

		result = calculateWeightedScore(map[int64]*entity.EvaluatorRecord{}, nil)
		assert.Nil(t, result)
	})

	t.Run("无权重配置，计算简单平均", func(t *testing.T) {
		score1 := 0.8
		score2 := 0.9
		score3 := 0.7
		records := map[int64]*entity.EvaluatorRecord{
			101: {
				EvaluatorVersionID: 101,
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: &score1,
					},
				},
			},
			102: {
				EvaluatorVersionID: 102,
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: &score2,
					},
				},
			},
			103: {
				EvaluatorVersionID: 103,
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: &score3,
					},
				},
			},
		}

		result := calculateWeightedScore(records, nil)
		assert.NotNil(t, result)
		expected := (0.8 + 0.9 + 0.7) / 3.0 // 0.8
		assert.InDelta(t, expected, *result, 0.01)
	})

	t.Run("有权重配置，计算加权平均", func(t *testing.T) {
		score1 := 0.8
		score2 := 0.9
		records := map[int64]*entity.EvaluatorRecord{
			101: {
				EvaluatorVersionID: 101,
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: &score1,
					},
				},
			},
			102: {
				EvaluatorVersionID: 102,
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: &score2,
					},
				},
			},
		}

		weights := map[int64]float64{
			101: 0.6,
			102: 0.4,
		}

		result := calculateWeightedScore(records, weights)
		assert.NotNil(t, result)
		expected := (0.8*0.6 + 0.9*0.4) / (0.6 + 0.4) // 0.84
		assert.InDelta(t, expected, *result, 0.01)
	})

	t.Run("优先使用修正分数", func(t *testing.T) {
		originalScore := 0.8
		correctionScore := 0.9
		records := map[int64]*entity.EvaluatorRecord{
			101: {
				EvaluatorVersionID: 101,
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: &originalScore,
						Correction: &entity.Correction{
							Score: &correctionScore,
						},
					},
				},
			},
		}

		result := calculateWeightedScore(records, nil)
		assert.NotNil(t, result)
		assert.InDelta(t, 0.9, *result, 0.01) // 应该使用修正分数 0.9
	})

	t.Run("包含 nil 记录，跳过", func(t *testing.T) {
		score1 := 0.8
		records := map[int64]*entity.EvaluatorRecord{
			101: {
				EvaluatorVersionID: 101,
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: &score1,
					},
				},
			},
			102: nil, // nil 记录应该被跳过
		}

		result := calculateWeightedScore(records, nil)
		assert.NotNil(t, result)
		assert.InDelta(t, 0.8, *result, 0.01)
	})

	t.Run("记录无分数，返回 nil", func(t *testing.T) {
		records := map[int64]*entity.EvaluatorRecord{
			101: {
				EvaluatorVersionID: 101,
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: nil, // 无分数
					},
				},
			},
		}

		result := calculateWeightedScore(records, nil)
		assert.Nil(t, result)
	})

	t.Run("权重为0或负数，跳过", func(t *testing.T) {
		score1 := 0.8
		score2 := 0.9
		records := map[int64]*entity.EvaluatorRecord{
			101: {
				EvaluatorVersionID: 101,
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: &score1,
					},
				},
			},
			102: {
				EvaluatorVersionID: 102,
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: &score2,
					},
				},
			},
		}

		weights := map[int64]float64{
			101: 0.6,
			102: 0.0, // 权重为0，应该被跳过
		}

		result := calculateWeightedScore(records, weights)
		assert.NotNil(t, result)
		// 只有 101 参与计算，加权分数 = 0.8 * 0.6 / 0.6 = 0.8
		assert.InDelta(t, 0.8, *result, 0.01)
	})

	t.Run("所有记录都被跳过，返回 nil", func(t *testing.T) {
		records := map[int64]*entity.EvaluatorRecord{
			101: {
				EvaluatorVersionID: 101,
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorResult: &entity.EvaluatorResult{
						Score: nil,
					},
				},
			},
		}

		weights := map[int64]float64{
			101: 0.0, // 权重为0
		}

		result := calculateWeightedScore(records, weights)
		assert.Nil(t, result)
	})
}

// TestExptResultBuilder_FillExptTurnResultFilters_RecalculateWeightedScore 测试 fillExptTurnResultFilters 中重新计算加权分数的逻辑（1211-1232行）
func TestExptResultBuilder_FillExptTurnResultFilters_RecalculateWeightedScore(t *testing.T) {
	ctx := context.Background()

	t.Run("WeightedScore 为 nil 且启用加权分数，重新计算", func(t *testing.T) {
		builder := &PayloadBuilder{
			SpaceID:        100,
			BaselineExptID: 1,
			BaseExptItemResultDO: []*entity.ExptItemResult{
				{ItemID: 1, ItemIdx: 1, Status: entity.ItemRunState_Success},
			},
			BaseExptTurnResultDO: []*entity.ExptTurnResult{
				{
					ID:            1,
					ItemID:        1,
					TurnID:        1,
					WeightedScore: nil, // 为 nil，需要重新计算
				},
			},
			ExptResultBuilders: []*ExptResultBuilder{
				{
					exptDO: &entity.Experiment{
						ID: 1,
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								EvaluatorsConf: &entity.EvaluatorsConf{
									EnableScoreWeight: true,
									EvaluatorConf: []*entity.EvaluatorConf{
										{
											EvaluatorVersionID: 101,
											ScoreWeight:        gptr.Of(0.6),
										},
										{
											EvaluatorVersionID: 102,
											ScoreWeight:        gptr.Of(0.4),
										},
									},
								},
							},
						},
					},
					turnResultID2EvaluatorVersionID2Result: map[int64]map[int64]*entity.EvaluatorRecord{
						1: {
							101: {
								EvaluatorVersionID: 101,
								EvaluatorOutputData: &entity.EvaluatorOutputData{
									EvaluatorResult: &entity.EvaluatorResult{
										Score: gptr.Of(0.8),
									},
								},
							},
							102: {
								EvaluatorVersionID: 102,
								EvaluatorOutputData: &entity.EvaluatorOutputData{
									EvaluatorResult: &entity.EvaluatorResult{
										Score: gptr.Of(0.9),
									},
								},
							},
						},
					},
				},
			},
		}

		err := builder.fillExptTurnResultFilters(ctx, nil, 1)
		assert.NoError(t, err)
		assert.Len(t, builder.ExptTurnResultFilters, 1)
		if assert.NotNil(t, builder.ExptTurnResultFilters[0].EvaluatorWeightedScore) {
			// 加权分数 = (0.8 * 0.6 + 0.9 * 0.4) / (0.6 + 0.4) = 0.84
			expected := 0.84
			assert.InDelta(t, expected, *builder.ExptTurnResultFilters[0].EvaluatorWeightedScore, 0.01)
		}
	})

	t.Run("WeightedScore 不为 nil，使用已有值", func(t *testing.T) {
		existingScore := 0.75
		builder := &PayloadBuilder{
			SpaceID:        100,
			BaselineExptID: 1,
			BaseExptItemResultDO: []*entity.ExptItemResult{
				{ItemID: 1, ItemIdx: 1, Status: entity.ItemRunState_Success},
			},
			BaseExptTurnResultDO: []*entity.ExptTurnResult{
				{
					ID:            1,
					ItemID:        1,
					TurnID:        1,
					WeightedScore: &existingScore, // 已有值，不重新计算
				},
			},
			ExptResultBuilders: []*ExptResultBuilder{
				{
					exptDO: &entity.Experiment{
						ID: 1,
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								EvaluatorsConf: &entity.EvaluatorsConf{
									EnableScoreWeight: true,
								},
							},
						},
					},
					turnResultID2EvaluatorVersionID2Result: map[int64]map[int64]*entity.EvaluatorRecord{
						1: {
							101: {
								EvaluatorVersionID: 101,
								EvaluatorOutputData: &entity.EvaluatorOutputData{
									EvaluatorResult: &entity.EvaluatorResult{
										Score: gptr.Of(0.8),
									},
								},
							},
						},
					},
				},
			},
		}

		err := builder.fillExptTurnResultFilters(ctx, nil, 1)
		assert.NoError(t, err)
		assert.Len(t, builder.ExptTurnResultFilters, 1)
		// 应该使用已有的加权分数
		assert.Equal(t, existingScore, *builder.ExptTurnResultFilters[0].EvaluatorWeightedScore)
	})

	t.Run("未启用加权分数，不重新计算", func(t *testing.T) {
		builder := &PayloadBuilder{
			SpaceID:        100,
			BaselineExptID: 1,
			BaseExptItemResultDO: []*entity.ExptItemResult{
				{ItemID: 1, ItemIdx: 1, Status: entity.ItemRunState_Success},
			},
			BaseExptTurnResultDO: []*entity.ExptTurnResult{
				{
					ID:            1,
					ItemID:        1,
					TurnID:        1,
					WeightedScore: nil,
				},
			},
			ExptResultBuilders: []*ExptResultBuilder{
				{
					exptDO: &entity.Experiment{
						ID: 1,
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								EvaluatorsConf: &entity.EvaluatorsConf{
									EnableScoreWeight: false, // 未启用
								},
							},
						},
					},
					turnResultID2EvaluatorVersionID2Result: map[int64]map[int64]*entity.EvaluatorRecord{
						1: {
							101: {
								EvaluatorVersionID: 101,
								EvaluatorOutputData: &entity.EvaluatorOutputData{
									EvaluatorResult: &entity.EvaluatorResult{
										Score: gptr.Of(0.8),
									},
								},
							},
						},
					},
				},
			},
		}

		err := builder.fillExptTurnResultFilters(ctx, nil, 1)
		assert.NoError(t, err)
		assert.Len(t, builder.ExptTurnResultFilters, 1)
		// 未启用加权分数，应该保持为 nil
		assert.Nil(t, builder.ExptTurnResultFilters[0].EvaluatorWeightedScore)
	})
}

// TestExptResultServiceImpl_RecalculateWeightedScore 测试 RecalculateWeightedScore 函数（2707-2802行）
func TestExptResultServiceImpl_RecalculateWeightedScore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	spaceID := int64(100)
	exptID := int64(1)
	itemID := int64(1)
	turnID := int64(1)

	t.Run("成功重新计算加权分数", func(t *testing.T) {
		mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
		mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
		mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)

		service := ExptResultServiceImpl{
			ExptTurnResultRepo:     mockExptTurnResultRepo,
			ExperimentRepo:         mockExperimentRepo,
			evaluatorRecordService: mockEvaluatorRecordService,
		}

		// Mock Get - 返回 turnResult
		turnResultID := int64(10)
		mockExptTurnResultRepo.EXPECT().
			Get(ctx, spaceID, exptID, itemID, turnID).
			Return(&entity.ExptTurnResult{
				ID: turnResultID,
			}, nil)

		// Mock GetByID - 返回启用加权分数的实验配置
		weight1 := 0.6
		weight2 := 0.4
		mockExperimentRepo.EXPECT().
			GetByID(ctx, exptID, spaceID).
			Return(&entity.Experiment{
				ID: exptID,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						EvaluatorsConf: &entity.EvaluatorsConf{
							EnableScoreWeight: true,
							EvaluatorConf: []*entity.EvaluatorConf{
								{EvaluatorVersionID: 101, ScoreWeight: &weight1},
								{EvaluatorVersionID: 102, ScoreWeight: &weight2},
							},
						},
					},
				},
			}, nil)

		// Mock BatchGetTurnEvaluatorResultRef
		mockExptTurnResultRepo.EXPECT().
			BatchGetTurnEvaluatorResultRef(ctx, spaceID, []int64{turnResultID}).
			Return([]*entity.ExptTurnEvaluatorResultRef{
				{EvaluatorResultID: 1, EvaluatorVersionID: 101},
				{EvaluatorResultID: 2, EvaluatorVersionID: 102},
			}, nil)

		// Mock BatchGetEvaluatorRecord
		score1 := 0.8
		score2 := 0.9
		mockEvaluatorRecordService.EXPECT().
			BatchGetEvaluatorRecord(ctx, []int64{1, 2}, false).
			Return([]*entity.EvaluatorRecord{
				{
					ID:                 1,
					EvaluatorVersionID: 101,
					EvaluatorOutputData: &entity.EvaluatorOutputData{
						EvaluatorResult: &entity.EvaluatorResult{
							Score: &score1,
						},
					},
				},
				{
					ID:                 2,
					EvaluatorVersionID: 102,
					EvaluatorOutputData: &entity.EvaluatorOutputData{
						EvaluatorResult: &entity.EvaluatorResult{
							Score: &score2,
						},
					},
				},
			}, nil)

		// Mock UpdateTurnResults - 验证加权分数被正确更新
		mockExptTurnResultRepo.EXPECT().
			UpdateTurnResults(ctx, exptID, gomock.Any(), spaceID, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ int64, itemTurnIDs []*entity.ItemTurnID, _ int64, updateFields map[string]any) error {
				assert.Len(t, itemTurnIDs, 1)
				assert.Equal(t, itemID, itemTurnIDs[0].ItemID)
				assert.Equal(t, turnID, itemTurnIDs[0].TurnID)
				if weightedScore, ok := updateFields["weighted_score"].(*float64); ok {
					assert.NotNil(t, weightedScore)
					expected := 0.84
					assert.InDelta(t, expected, *weightedScore, 0.01)
				}
				return nil
			})

		err := service.RecalculateWeightedScore(ctx, spaceID, exptID, itemID, turnID)
		assert.NoError(t, err)
	})

	t.Run("TurnResult 不存在，返回 nil", func(t *testing.T) {
		mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)

		service := ExptResultServiceImpl{
			ExptTurnResultRepo: mockExptTurnResultRepo,
		}

		mockExptTurnResultRepo.EXPECT().
			Get(ctx, spaceID, exptID, itemID, turnID).
			Return(nil, nil)

		err := service.RecalculateWeightedScore(ctx, spaceID, exptID, itemID, turnID)
		assert.NoError(t, err) // 应该返回 nil，不报错
	})

	t.Run("Experiment 不存在，返回 nil", func(t *testing.T) {
		mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
		mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)

		service := ExptResultServiceImpl{
			ExptTurnResultRepo: mockExptTurnResultRepo,
			ExperimentRepo:     mockExperimentRepo,
		}

		mockExptTurnResultRepo.EXPECT().
			Get(ctx, spaceID, exptID, itemID, turnID).
			Return(&entity.ExptTurnResult{ID: 10}, nil)

		mockExperimentRepo.EXPECT().
			GetByID(ctx, exptID, spaceID).
			Return(nil, nil)

		err := service.RecalculateWeightedScore(ctx, spaceID, exptID, itemID, turnID)
		assert.NoError(t, err) // 应该返回 nil，不报错
	})

	t.Run("未启用加权分数，返回 nil", func(t *testing.T) {
		mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
		mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)

		service := ExptResultServiceImpl{
			ExptTurnResultRepo: mockExptTurnResultRepo,
			ExperimentRepo:     mockExperimentRepo,
		}

		mockExptTurnResultRepo.EXPECT().
			Get(ctx, spaceID, exptID, itemID, turnID).
			Return(&entity.ExptTurnResult{ID: 10}, nil)

		mockExperimentRepo.EXPECT().
			GetByID(ctx, exptID, spaceID).
			Return(&entity.Experiment{
				ID: exptID,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						EvaluatorsConf: &entity.EvaluatorsConf{
							EnableScoreWeight: false, // 未启用
						},
					},
				},
			}, nil)

		err := service.RecalculateWeightedScore(ctx, spaceID, exptID, itemID, turnID)
		assert.NoError(t, err) // 应该返回 nil，不报错
	})

	t.Run("无评估器引用，返回 nil", func(t *testing.T) {
		mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
		mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)

		service := ExptResultServiceImpl{
			ExptTurnResultRepo: mockExptTurnResultRepo,
			ExperimentRepo:     mockExperimentRepo,
		}

		mockExptTurnResultRepo.EXPECT().
			Get(ctx, spaceID, exptID, itemID, turnID).
			Return(&entity.ExptTurnResult{ID: 10}, nil)

		weight1 := 0.6
		mockExperimentRepo.EXPECT().
			GetByID(ctx, exptID, spaceID).
			Return(&entity.Experiment{
				ID: exptID,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						EvaluatorsConf: &entity.EvaluatorsConf{
							EnableScoreWeight: true,
							EvaluatorConf: []*entity.EvaluatorConf{
								{EvaluatorVersionID: 101, ScoreWeight: &weight1},
							},
						},
					},
				},
			}, nil)

		mockExptTurnResultRepo.EXPECT().
			BatchGetTurnEvaluatorResultRef(ctx, spaceID, []int64{10}).
			Return([]*entity.ExptTurnEvaluatorResultRef{}, nil) // 空列表

		err := service.RecalculateWeightedScore(ctx, spaceID, exptID, itemID, turnID)
		assert.NoError(t, err) // 应该返回 nil，不报错
	})
}

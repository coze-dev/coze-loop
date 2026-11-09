// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestExptResultServiceImpl_UpsertExptTurnResultFilter_ErrorBranches(t *testing.T) {
	tests := []struct {
		name    string
		spaceID int64
		exptID  int64
		itemIDs []int64
		setup   func(ctrl *gomock.Controller) ExptResultServiceImpl
		wantErr bool
	}{
		{
			name:    "invalid_space_id",
			spaceID: 0,
			exptID:  1,
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				return ExptResultServiceImpl{}
			},
			wantErr: true,
		},
		{
			name:    "list_turn_result_error",
			spaceID: 100,
			exptID:  1,
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				turnRepo.EXPECT().
					ListTurnResultByItemIDs(gomock.Any(), int64(100), int64(1), nil, gomock.Any(), false).
					Return(nil, int64(0), errors.New("list failed"))
				return ExptResultServiceImpl{ExptTurnResultRepo: turnRepo}
			},
			wantErr: true,
		},
		{
			name:    "no_turn_result_returns_nil",
			spaceID: 100,
			exptID:  1,
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				turnRepo.EXPECT().
					ListTurnResultByItemIDs(gomock.Any(), int64(100), int64(1), nil, gomock.Any(), false).
					Return([]*entity.ExptTurnResult{}, int64(0), nil)
				return ExptResultServiceImpl{ExptTurnResultRepo: turnRepo}
			},
			wantErr: false,
		},
		{
			name:    "batch_get_item_result_error",
			spaceID: 100,
			exptID:  1,
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				turnRepo.EXPECT().
					ListTurnResultByItemIDs(gomock.Any(), int64(100), int64(1), nil, gomock.Any(), false).
					Return([]*entity.ExptTurnResult{{ID: 10, ItemID: 20, TurnID: 30}}, int64(1), nil)
				itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
				itemRepo.EXPECT().
					BatchGet(gomock.Any(), int64(100), int64(1), []int64{20}).
					Return(nil, errors.New("batch get failed"))
				return ExptResultServiceImpl{ExptTurnResultRepo: turnRepo, ExptItemResultRepo: itemRepo}
			},
			wantErr: true,
		},
		{
			name:    "get_filter_key_mappings_error",
			spaceID: 100,
			exptID:  1,
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				turnRepo.EXPECT().
					ListTurnResultByItemIDs(gomock.Any(), int64(100), int64(1), nil, gomock.Any(), false).
					Return([]*entity.ExptTurnResult{{ID: 10, ItemID: 20, TurnID: 30}}, int64(1), nil)
				itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
				itemRepo.EXPECT().
					BatchGet(gomock.Any(), int64(100), int64(1), []int64{20}).
					Return([]*entity.ExptItemResult{}, nil)
				filterRepo := repoMocks.NewMockIExptTurnResultFilterRepo(ctrl)
				filterRepo.EXPECT().
					GetExptTurnResultFilterKeyMappings(gomock.Any(), int64(100), int64(1)).
					Return(nil, errors.New("key mappings failed"))
				return ExptResultServiceImpl{
					ExptTurnResultRepo:       turnRepo,
					ExptItemResultRepo:       itemRepo,
					exptTurnResultFilterRepo: filterRepo,
				}
			},
			wantErr: true,
		},
		{
			name:    "build_turn_result_filter_error",
			spaceID: 100,
			exptID:  1,
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				turnRepo.EXPECT().
					ListTurnResultByItemIDs(gomock.Any(), int64(100), int64(1), nil, gomock.Any(), false).
					Return([]*entity.ExptTurnResult{{ID: 10, ItemID: 20, TurnID: 30}}, int64(1), nil)
				itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
				itemRepo.EXPECT().
					BatchGet(gomock.Any(), int64(100), int64(1), []int64{20}).
					Return([]*entity.ExptItemResult{}, nil)
				filterRepo := repoMocks.NewMockIExptTurnResultFilterRepo(ctrl)
				filterRepo.EXPECT().
					GetExptTurnResultFilterKeyMappings(gomock.Any(), int64(100), int64(1)).
					Return(nil, nil)
				exptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
				exptRepo.EXPECT().
					GetByID(gomock.Any(), int64(1), int64(100)).
					Return(nil, errors.New("get expt failed"))
				return ExptResultServiceImpl{
					ExptTurnResultRepo:       turnRepo,
					ExptItemResultRepo:       itemRepo,
					exptTurnResultFilterRepo: filterRepo,
					ExperimentRepo:           exptRepo,
				}
			},
			wantErr: true,
		},
		{
			name:    "save_filter_error",
			spaceID: 100,
			exptID:  1,
			setup: func(ctrl *gomock.Controller) ExptResultServiceImpl {
				turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
				turnRepo.EXPECT().
					ListTurnResultByItemIDs(gomock.Any(), int64(100), int64(1), nil, gomock.Any(), false).
					Return([]*entity.ExptTurnResult{{ID: 10, ItemID: 20, TurnID: 30}}, int64(1), nil)
				turnRepo.EXPECT().
					BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(100), []int64{10}).
					Return(nil, nil)
				itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
				itemRepo.EXPECT().
					BatchGet(gomock.Any(), int64(100), int64(1), []int64{20}).
					Return([]*entity.ExptItemResult{}, nil)
				filterRepo := repoMocks.NewMockIExptTurnResultFilterRepo(ctrl)
				filterRepo.EXPECT().
					GetExptTurnResultFilterKeyMappings(gomock.Any(), int64(100), int64(1)).
					Return(nil, nil)
				filterRepo.EXPECT().
					Save(gomock.Any(), gomock.Any()).
					Return(errors.New("save failed"))
				exptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
				exptRepo.EXPECT().
					GetByID(gomock.Any(), int64(1), int64(100)).
					Return(&entity.Experiment{ID: 1, SpaceID: 100, ExptType: entity.ExptType_Online}, nil)
				recordSvc := svcMocks.NewMockEvaluatorRecordService(ctrl)
				recordSvc.EXPECT().
					BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).
					Return(nil, nil)
				annotateRepo := repoMocks.NewMockIExptAnnotateRepo(ctrl)
				annotateRepo.EXPECT().
					GetExptTurnAnnotateRecordRefsByTurnResultIDs(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, nil)
				annotateRepo.EXPECT().
					GetAnnotateRecordsByIDs(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, nil)
				return ExptResultServiceImpl{
					ExptTurnResultRepo:       turnRepo,
					ExptItemResultRepo:       itemRepo,
					exptTurnResultFilterRepo: filterRepo,
					ExperimentRepo:           exptRepo,
					ExptAnnotateRepo:         annotateRepo,
					evaluatorRecordService:   recordSvc,
					scoreCalculator:          NewEvaluatorScoreCalculator(nil, nil),
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
			err := svc.UpsertExptTurnResultFilter(context.Background(), tt.spaceID, tt.exptID, tt.itemIDs)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

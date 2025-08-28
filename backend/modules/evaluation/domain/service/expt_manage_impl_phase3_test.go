// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// Phase 3: Skip target logic tests
func TestExptMangerImpl_packTupleID_WithoutTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()

	tests := []struct {
		name string
		expt *entity.Experiment
		want *entity.ExptTupleID
	}{
		{
			name: "experiment_with_target",
			expt: &entity.Experiment{
				EvalSetID:        1,
				EvalSetVersionID: 2,
				TargetID:         3,
				TargetVersionID:  4,
				EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{
					{EvaluatorVersionID: 10},
					{EvaluatorVersionID: 11},
				},
			},
			want: &entity.ExptTupleID{
				VersionedEvalSetID: &entity.VersionedEvalSetID{
					EvalSetID: 1,
					VersionID: 2,
				},
				VersionedTargetID: &entity.VersionedTargetID{
					TargetID:  3,
					VersionID: 4,
				},
				EvaluatorVersionIDs: []int64{10, 11},
			},
		},
		{
			name: "experiment_without_target_zero_ids",
			expt: &entity.Experiment{
				EvalSetID:        1,
				EvalSetVersionID: 2,
				TargetID:         0, // No target
				TargetVersionID:  0, // No target version
				EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{
					{EvaluatorVersionID: 10},
				},
			},
			want: &entity.ExptTupleID{
				VersionedEvalSetID: &entity.VersionedEvalSetID{
					EvalSetID: 1,
					VersionID: 2,
				},
				VersionedTargetID:   nil, // Should be nil when no target
				EvaluatorVersionIDs: []int64{10},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mgr.packTupleID(ctx, tt.expt)

			// Check VersionedEvalSetID
			if got.VersionedEvalSetID.EvalSetID != tt.want.VersionedEvalSetID.EvalSetID {
				t.Errorf("packTupleID() VersionedEvalSetID.EvalSetID = %v, want %v",
					got.VersionedEvalSetID.EvalSetID, tt.want.VersionedEvalSetID.EvalSetID)
			}
			if got.VersionedEvalSetID.VersionID != tt.want.VersionedEvalSetID.VersionID {
				t.Errorf("packTupleID() VersionedEvalSetID.VersionID = %v, want %v",
					got.VersionedEvalSetID.VersionID, tt.want.VersionedEvalSetID.VersionID)
			}

			// Check VersionedTargetID
			if tt.want.VersionedTargetID == nil {
				if got.VersionedTargetID != nil {
					t.Errorf("packTupleID() VersionedTargetID = %v, want nil", got.VersionedTargetID)
				}
			} else {
				if got.VersionedTargetID == nil {
					t.Errorf("packTupleID() VersionedTargetID = nil, want %v", tt.want.VersionedTargetID)
				} else {
					if got.VersionedTargetID.TargetID != tt.want.VersionedTargetID.TargetID {
						t.Errorf("packTupleID() VersionedTargetID.TargetID = %v, want %v",
							got.VersionedTargetID.TargetID, tt.want.VersionedTargetID.TargetID)
					}
					if got.VersionedTargetID.VersionID != tt.want.VersionedTargetID.VersionID {
						t.Errorf("packTupleID() VersionedTargetID.VersionID = %v, want %v",
							got.VersionedTargetID.VersionID, tt.want.VersionedTargetID.VersionID)
					}
				}
			}

			// Check EvaluatorVersionIDs
			if len(got.EvaluatorVersionIDs) != len(tt.want.EvaluatorVersionIDs) {
				t.Errorf("packTupleID() EvaluatorVersionIDs length = %v, want %v",
					len(got.EvaluatorVersionIDs), len(tt.want.EvaluatorVersionIDs))
			} else {
				for i, id := range got.EvaluatorVersionIDs {
					if id != tt.want.EvaluatorVersionIDs[i] {
						t.Errorf("packTupleID() EvaluatorVersionIDs[%d] = %v, want %v",
							i, id, tt.want.EvaluatorVersionIDs[i])
					}
				}
			}
		})
	}
}

func TestExptMangerImpl_getExptTupleByID_WithoutTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}

	tests := []struct {
		name       string
		tupleID    *entity.ExptTupleID
		setup      func()
		wantTarget bool
		wantErr    bool
	}{
		{
			name: "tuple_without_target",
			tupleID: &entity.ExptTupleID{
				VersionedEvalSetID: &entity.VersionedEvalSetID{
					EvalSetID: 1,
					VersionID: 2,
				},
				VersionedTargetID:   nil, // No target
				EvaluatorVersionIDs: []int64{10},
			},
			setup: func() {
				// No target service call expected

				mgr.evaluationSetVersionService.(*svcMocks.MockEvaluationSetVersionService).
					EXPECT().
					GetEvaluationSetVersion(ctx, int64(1), int64(2), gptr.Of(true)).
					Return(&entity.EvaluationSetVersion{ID: 2}, &entity.EvaluationSet{ID: 1}, nil)

				mgr.evaluatorService.(*svcMocks.MockEvaluatorService).
					EXPECT().
					BatchGetEvaluatorVersion(ctx, nil, []int64{10}, false).
					Return([]*entity.Evaluator{{ID: 10}}, nil)
			},
			wantTarget: false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			got, err := mgr.getExptTupleByID(ctx, tt.tupleID, 1, session)

			if (err != nil) != tt.wantErr {
				t.Errorf("getExptTupleByID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if tt.wantTarget && got.Target == nil {
					t.Errorf("getExptTupleByID() target = nil, want target")
				}
				if !tt.wantTarget && got.Target != nil {
					t.Errorf("getExptTupleByID() target = %v, want nil", got.Target)
				}
				if got.EvalSet == nil {
					t.Errorf("getExptTupleByID() evalSet = nil, want evalSet")
				}
			}
		})
	}
}

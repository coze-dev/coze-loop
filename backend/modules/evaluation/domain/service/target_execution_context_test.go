// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
)

type executionRecordRepo struct {
	repo.IEvalTargetRepo
	record *entity.EvalTargetRecord
	err    error
}

func (r executionRecordRepo) GetEvalTargetRecordByIDAndSpaceID(_ context.Context, _, _ int64) (*entity.EvalTargetRecord, error) {
	return r.record, r.err
}

type executionRunRepo struct {
	repo.IExptRunLogRepo
	runs map[int64]*entity.ExptRunLog
	err  error
}

func (r executionRunRepo) GetByRunID(_ context.Context, id int64) (*entity.ExptRunLog, error) {
	return r.runs[id], r.err
}

func TestGetExecutionContextUsesPersistedRunOwner(t *testing.T) {
	runs := executionRunRepo{runs: map[int64]*entity.ExptRunLog{
		21: {ID: 21, ExptRunID: 21, ExptID: 10, SpaceID: 2, CreatedBy: "initiator-a"},
		22: {ID: 22, ExptRunID: 22, ExptID: 10, SpaceID: 2, CreatedBy: "initiator-b"},
	}}
	for _, tt := range []struct {
		name  string
		runID int64
		owner string
	}{
		{"first run", 21, "initiator-a"}, {"another user reruns", 22, "initiator-b"}, {"same run append keeps owner", 21, "initiator-a"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := &EvalTargetServiceImpl{evalTargetRepo: executionRecordRepo{record: &entity.EvalTargetRecord{ID: 31, SpaceID: 1, ExperimentRunID: tt.runID, BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: gptr.Of("wrong-record-creator")}}}}, exptRunLogRepo: runs}
			got, err := svc.GetExecutionContext(session.WithCtxUser(context.Background(), &session.User{ID: "different-current-user"}), 1, 31)
			require.NoError(t, err)
			require.Equal(t, &entity.EvalTargetExecutionContext{WorkspaceID: 1, EvalTargetRecordID: 31, ExperimentID: 10, ExperimentRunID: tt.runID, ExperimentWorkspaceID: 2, InitiatorUserID: tt.owner}, got)
		})
	}
}

func TestGetExecutionContextRejectsMissingOrMismatchedIdentity(t *testing.T) {
	validRecord := &entity.EvalTargetRecord{ID: 31, SpaceID: 1, ExperimentRunID: 21}
	validRun := &entity.ExptRunLog{ID: 21, ExptRunID: 21, ExptID: 10, SpaceID: 2, CreatedBy: "initiator-a"}
	for _, tt := range []struct {
		name   string
		record *entity.EvalTargetRecord
		run    *entity.ExptRunLog
	}{
		{"missing or deleted record", nil, validRun},
		{"wrong record space", &entity.EvalTargetRecord{ID: 31, SpaceID: 9, ExperimentRunID: 21}, validRun},
		{"wrong record id", &entity.EvalTargetRecord{ID: 32, SpaceID: 1, ExperimentRunID: 21}, validRun},
		{"debug has no experiment run", &entity.EvalTargetRecord{ID: 31, SpaceID: 1}, validRun},
		{"missing or deleted run", validRecord, nil},
		{"wrong run id", validRecord, &entity.ExptRunLog{ID: 22, ExptRunID: 22, ExptID: 10, SpaceID: 2, CreatedBy: "other"}},
		{"missing run owner", validRecord, &entity.ExptRunLog{ID: 21, ExptRunID: 21, ExptID: 10, SpaceID: 2}},
		{"blank run owner", validRecord, &entity.ExptRunLog{ID: 21, ExptRunID: 21, ExptID: 10, SpaceID: 2, CreatedBy: "  "}},
		{"missing experiment", validRecord, &entity.ExptRunLog{ID: 21, ExptRunID: 21, SpaceID: 2, CreatedBy: "a"}},
		{"missing experiment space", validRecord, &entity.ExptRunLog{ID: 21, ExptRunID: 21, ExptID: 10, CreatedBy: "a"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := &EvalTargetServiceImpl{evalTargetRepo: executionRecordRepo{record: tt.record}, exptRunLogRepo: executionRunRepo{runs: map[int64]*entity.ExptRunLog{21: tt.run}}}
			got, err := svc.GetExecutionContext(context.Background(), 1, 31)
			require.Nil(t, got)
			require.ErrorContains(t, err, "601203004")
		})
	}
	for _, tt := range []struct {
		name              string
		recordErr, runErr error
	}{
		{"record storage error", errors.New("record unavailable"), nil},
		{"run storage error", nil, errors.New("run unavailable")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := &EvalTargetServiceImpl{evalTargetRepo: executionRecordRepo{record: validRecord, err: tt.recordErr}, exptRunLogRepo: executionRunRepo{err: tt.runErr}}
			got, err := svc.GetExecutionContext(context.Background(), 1, 31)
			require.Nil(t, got)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "601203004")
		})
	}
}

func TestGetExecutionContextValidatesLogicalRunID(t *testing.T) {
	svc := &EvalTargetServiceImpl{
		evalTargetRepo: executionRecordRepo{record: &entity.EvalTargetRecord{ID: 31, SpaceID: 1, ExperimentRunID: 21}},
		exptRunLogRepo: executionRunRepo{runs: map[int64]*entity.ExptRunLog{21: {ID: 99, ExptRunID: 21, ExptID: 10, SpaceID: 2, CreatedBy: "run-owner"}}},
	}
	got, err := svc.GetExecutionContext(context.Background(), 1, 31)
	require.NoError(t, err)
	require.Equal(t, int64(21), got.ExperimentRunID)
	require.Equal(t, "run-owner", got.InitiatorUserID)
}

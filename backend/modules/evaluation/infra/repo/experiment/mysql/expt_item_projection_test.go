// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	dbmocks "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type itemRunProjectionWriter interface {
	ApplyItemRunResults(context.Context, int64, int64, int64, int64, []*model.ExptTurnResult, []*model.ExptTurnEvaluatorResultRef, ...db.Option) (bool, error)
}

func TestApplyItemRunResults_AtomicScopeAndRollback(t *testing.T) {
	for _, failAt := range []string{"", "owner", "run-log", "turn-scope", "delete-refs", "turn-1", "turn-2", "insert-refs", "item", "result-state", "stats", "commit"} {
		t.Run(map[bool]string{true: "success", false: failAt}[failAt == ""], func(t *testing.T) {
			ctrl := gomock.NewController(t)
			gormDB, source, replica := newRunLogResolverTestDB(t)
			provider := dbmocks.NewMockProvider(ctrl)
			provider.EXPECT().NewSession(gomock.Any()).Return(gormDB)
			dao := &ExptTurnResultDAOImpl{provider: provider}
			writer, ok := any(dao).(itemRunProjectionWriter)
			require.True(t, ok, "item projection must be committed atomically")
			sentinel := errors.New("projection write unavailable")
			source.ExpectBegin()
			if failAt == "owner" {
				source.ExpectQuery("SELECT .*expt_item_result.*FOR UPDATE").WithArgs(int64(3), int64(1), int64(10), 1).WillReturnError(sentinel)
			} else {
				source.ExpectQuery("SELECT .*expt_item_result.*FOR UPDATE").WithArgs(int64(3), int64(1), int64(10), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "item_id", "status"}).AddRow(11, 3, 1, 103, 10, int32(entity.ItemRunState_Queueing)))
			}
			if failAt != "owner" {
				q := source.ExpectQuery("SELECT .*expt_item_result_run_log.*FOR UPDATE").WithArgs(int64(3), int64(1), int64(103), int64(10), 1)
				if failAt == "run-log" {
					q.WillReturnError(sentinel)
				} else {
					q.WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "item_id", "status", "result_state", "log_id"}).AddRow(12, 3, 1, 103, 10, int32(entity.ItemRunState_Fail), int32(entity.ExptItemResultStateLogged), "log"))
				}
			}
			readFailed := failAt == "owner" || failAt == "run-log"
			if !readFailed {
				q := source.ExpectQuery("SELECT .*expt_turn_result.*FOR UPDATE").WithArgs(int64(3), int64(1), int64(10), int64(21), int64(22))
				if failAt == "turn-scope" {
					q.WillReturnError(sentinel)
				} else {
					q.WillReturnRows(sqlmock.NewRows([]string{"id", "expt_run_id"}).AddRow(21, 101).AddRow(22, 103))
				}
			}
			writeSteps := []struct{ name, sql string }{
				{"delete-refs", "DELETE FROM `expt_turn_evaluator_result_ref`"},
				{"turn-1", "UPDATE `expt_turn_result` SET"},
				{"turn-2", "UPDATE `expt_turn_result` SET"},
				{"insert-refs", "INSERT INTO `expt_turn_evaluator_result_ref`"},
				{"item", "UPDATE `expt_item_result` SET"},
				{"result-state", "UPDATE `expt_item_result_run_log` SET"},
				{"stats", "UPDATE `expt_stats` SET .*fail_cnt.*pending_cnt"},
			}
			if !readFailed && failAt != "turn-scope" {
				for _, step := range writeSteps {
					e := source.ExpectExec(step.sql)
					if step.name == "delete-refs" {
						e.WithArgs(int64(3), int64(1), int64(21), int64(22))
					}
					if step.name == failAt {
						e.WillReturnError(sentinel)
						break
					}
					e.WillReturnResult(sqlmock.NewResult(1, 1))
				}
			}
			if failAt == "commit" {
				source.ExpectCommit().WillReturnError(sentinel)
			} else if failAt != "" {
				source.ExpectRollback()
			} else {
				source.ExpectCommit()
			}
			turns := []*model.ExptTurnResult{
				{ID: 21, SpaceID: 3, ExptID: 1, ExptRunID: 103, ItemID: 10, TurnID: 20, TargetResultID: 201, Status: int32(entity.TurnRunState_Fail)},
				{ID: 22, SpaceID: 3, ExptID: 1, ExptRunID: 103, ItemID: 10, TurnID: 21, TargetResultID: 202, Status: int32(entity.TurnRunState_Success), WeightedScore: gptr.Of(0.5)},
			}
			refs := []*model.ExptTurnEvaluatorResultRef{{ID: 71, SpaceID: 3, ExptID: 1, ExptTurnResultID: 22, EvaluatorVersionID: 401, EvaluatorResultID: 302, Alias_: "judge-a"}}
			applied, err := writer.ApplyItemRunResults(context.Background(), 1, 103, 10, 3, turns, refs)
			if failAt != "" {
				assert.ErrorIs(t, err, sentinel)
				assert.False(t, applied)
			} else {
				require.NoError(t, err)
				assert.True(t, applied)
			}
			require.NoError(t, source.ExpectationsWereMet())
			require.NoError(t, replica.ExpectationsWereMet())
		})
	}
}

func TestApplyItemRunResults_StaleAndDuplicateDoNotWrite(t *testing.T) {
	for _, stale := range []bool{true, false} {
		t.Run(map[bool]string{true: "stale run", false: "already resulted"}[stale], func(t *testing.T) {
			ctrl := gomock.NewController(t)
			gormDB, source, replica := newRunLogResolverTestDB(t)
			provider := dbmocks.NewMockProvider(ctrl)
			provider.EXPECT().NewSession(gomock.Any()).Return(gormDB)
			writer, ok := any(&ExptTurnResultDAOImpl{provider: provider}).(itemRunProjectionWriter)
			require.True(t, ok)
			source.ExpectBegin()
			owner := int64(103)
			if stale {
				owner = 104
			}
			source.ExpectQuery("SELECT .*expt_item_result.*FOR UPDATE").WithArgs(int64(3), int64(1), int64(10), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "expt_run_id"}).AddRow(11, owner))
			if !stale {
				source.ExpectQuery("SELECT .*expt_item_result_run_log.*FOR UPDATE").WithArgs(int64(3), int64(1), int64(103), int64(10), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "result_state"}).AddRow(12, int32(entity.ExptItemResultStateResulted)))
			}
			source.ExpectCommit()
			applied, err := writer.ApplyItemRunResults(context.Background(), 1, 103, 10, 3, []*model.ExptTurnResult{{ID: 21, SpaceID: 3, ExptID: 1, ExptRunID: 103, ItemID: 10}}, nil)
			require.NoError(t, err)
			assert.False(t, applied)
			require.NoError(t, source.ExpectationsWereMet())
			require.NoError(t, replica.ExpectationsWereMet())
		})
	}
}

func TestApplyItemRunResults_EmptyRefsAndLockedDelta(t *testing.T) {
	ctrl := gomock.NewController(t)
	gormDB, source, replica := newRunLogResolverTestDB(t)
	provider := dbmocks.NewMockProvider(ctrl)
	provider.EXPECT().NewSession(gomock.Any()).Return(gormDB)
	dao := &ExptTurnResultDAOImpl{provider: provider}
	source.ExpectBegin()
	source.ExpectQuery("SELECT .*expt_item_result.*FOR UPDATE").WithArgs(int64(3), int64(1), int64(10), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "expt_run_id", "status"}).AddRow(11, 103, int32(entity.ItemRunState_Fail)))
	source.ExpectQuery("SELECT .*expt_item_result_run_log.*FOR UPDATE").WithArgs(int64(3), int64(1), int64(103), int64(10), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "status", "result_state", "log_id"}).AddRow(12, int32(entity.ItemRunState_Fail), int32(entity.ExptItemResultStateLogged), "log"))
	source.ExpectQuery("SELECT .*expt_turn_result.*FOR UPDATE").WithArgs(int64(3), int64(1), int64(10), int64(21)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(21))
	source.ExpectExec("DELETE FROM `expt_turn_evaluator_result_ref`").WithArgs(int64(3), int64(1), int64(21)).WillReturnResult(sqlmock.NewResult(0, 2))
	source.ExpectExec("UPDATE `expt_turn_result` SET").WithArgs(nil, int64(103), "", int32(entity.TurnRunState_Fail), int64(0), nil, sqlmock.AnyArg(), int64(3), int64(1), int64(10), int64(21)).WillReturnResult(sqlmock.NewResult(0, 1))
	source.ExpectExec("UPDATE `expt_item_result` SET").WillReturnResult(sqlmock.NewResult(0, 1))
	source.ExpectExec("UPDATE `expt_item_result_run_log` SET").WillReturnResult(sqlmock.NewResult(0, 1))
	source.ExpectCommit()
	applied, err := dao.ApplyItemRunResults(context.Background(), 1, 103, 10, 3, []*model.ExptTurnResult{{ID: 21, SpaceID: 3, ExptID: 1, ExptRunID: 103, ItemID: 10, Status: int32(entity.TurnRunState_Fail)}}, nil)
	require.NoError(t, err)
	assert.True(t, applied)
	require.NoError(t, source.ExpectationsWereMet())
	require.NoError(t, replica.ExpectationsWereMet())
}

func TestApplyItemRunResults_RejectsForeignScopeBeforeTransaction(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*model.ExptTurnResult, *model.ExptTurnEvaluatorResultRef)
	}{
		{name: "turn space", change: func(tr *model.ExptTurnResult, _ *model.ExptTurnEvaluatorResultRef) { tr.SpaceID = 4 }},
		{name: "turn experiment", change: func(tr *model.ExptTurnResult, _ *model.ExptTurnEvaluatorResultRef) { tr.ExptID = 2 }},
		{name: "turn item", change: func(tr *model.ExptTurnResult, _ *model.ExptTurnEvaluatorResultRef) { tr.ItemID = 11 }},
		{name: "turn run", change: func(tr *model.ExptTurnResult, _ *model.ExptTurnEvaluatorResultRef) { tr.ExptRunID = 104 }},
		{name: "ref space", change: func(_ *model.ExptTurnResult, ref *model.ExptTurnEvaluatorResultRef) { ref.SpaceID = 4 }},
		{name: "ref experiment", change: func(_ *model.ExptTurnResult, ref *model.ExptTurnEvaluatorResultRef) { ref.ExptID = 2 }},
		{name: "ref unrelated turn", change: func(_ *model.ExptTurnResult, ref *model.ExptTurnEvaluatorResultRef) { ref.ExptTurnResultID = 22 }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			provider := dbmocks.NewMockProvider(ctrl)
			dao := &ExptTurnResultDAOImpl{provider: provider}
			tr := &model.ExptTurnResult{ID: 21, SpaceID: 3, ExptID: 1, ExptRunID: 103, ItemID: 10}
			ref := &model.ExptTurnEvaluatorResultRef{ID: 71, SpaceID: 3, ExptID: 1, ExptTurnResultID: 21, EvaluatorResultID: 301}
			tt.change(tr, ref)
			applied, err := dao.ApplyItemRunResults(context.Background(), 1, 103, 10, 3, []*model.ExptTurnResult{tr}, []*model.ExptTurnEvaluatorResultRef{ref})
			assert.Error(t, err)
			assert.False(t, applied)
		})
	}
}

func TestApplyItemRunResults_MissingCanonicalTurnRollsBack(t *testing.T) {
	ctrl := gomock.NewController(t)
	gormDB, source, replica := newRunLogResolverTestDB(t)
	provider := dbmocks.NewMockProvider(ctrl)
	provider.EXPECT().NewSession(gomock.Any()).Return(gormDB)
	dao := &ExptTurnResultDAOImpl{provider: provider}
	source.ExpectBegin()
	source.ExpectQuery("SELECT .*expt_item_result.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "expt_run_id"}).AddRow(11, 103))
	source.ExpectQuery("SELECT .*expt_item_result_run_log.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "result_state"}).AddRow(12, int32(entity.ExptItemResultStateLogged)))
	source.ExpectQuery("SELECT .*expt_turn_result.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	source.ExpectRollback()
	applied, err := dao.ApplyItemRunResults(context.Background(), 1, 103, 10, 3, []*model.ExptTurnResult{{ID: 21, SpaceID: 3, ExptID: 1, ExptRunID: 103, ItemID: 10}}, nil)
	require.ErrorContains(t, err, "item projection turn missing")
	require.False(t, applied)
	require.NoError(t, source.ExpectationsWereMet())
	require.NoError(t, replica.ExpectationsWereMet())
}

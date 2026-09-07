// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	userinfomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	evaluatorrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator"
	evaluatormysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

type projectionReadDBKey struct{}

type projectionReadDBProvider struct{ db.Provider }

// The evaluator DAO constructor is a singleton; bind each test connection to its context.
func (projectionReadDBProvider) NewSession(ctx context.Context, _ ...db.Option) *gorm.DB {
	return ctx.Value(projectionReadDBKey{}).(*gorm.DB).Session(&gorm.Session{})
}

type projectionReadTurnRepo struct {
	repo.IExptTurnResultRepo
	apply func([]*entity.ExptTurnResult, []*entity.ExptTurnEvaluatorResultRef)
}

func (r projectionReadTurnRepo) ApplyItemRunResults(_ context.Context, _, _, _, _ int64, turns []*entity.ExptTurnResult, refs []*entity.ExptTurnEvaluatorResultRef) (bool, error) {
	r.apply(turns, refs)
	return true, nil
}

func TestRecordItemRunLogs_ReadsPrimarySnapshotWithLaggingReplica(t *testing.T) {
	ctrl := gomock.NewController(t)
	sourceSQL, source, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sourceSQL.Close() })
	replicaSQL, replica, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = replicaSQL.Close() })
	sourceDialector := mysql.New(mysql.Config{Conn: sourceSQL, SkipInitializeWithVersion: true})
	replicaDialector := mysql.New(mysql.Config{Conn: replicaSQL, SkipInitializeWithVersion: true})
	gormDB, err := gorm.Open(sourceDialector, &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gormDB.Use(dbresolver.Register(dbresolver.Config{
		Sources: []gorm.Dialector{sourceDialector}, Replicas: []gorm.Dialector{replicaDialector},
	})))
	ctx := context.WithValue(context.Background(), projectionReadDBKey{}, gormDB)
	provider := projectionReadDBProvider{}
	itemRepo := experimentrepo.NewExptItemResultRepo(experimentmysql.NewExptItemResultDAO(provider))
	turnRepo := experimentrepo.NewExptTurnResultRepo(nil, experimentmysql.NewExptTurnResultDAO(provider), nil)
	recordRepo := evaluatorrepo.NewEvaluatorRecordRepo(nil, provider, evaluatormysql.NewEvaluatorRecordDAO(provider), nil)
	userInfo := userinfomocks.NewMockUserInfoService(ctrl)
	userInfo.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).Times(2)
	recordService := &EvaluatorRecordServiceImpl{evaluatorRecordRepo: recordRepo, userInfoService: userInfo}

	expectReads := func(mock sqlmock.Sqlmock, targetID, recordID int64, score float64) {
		mock.ExpectQuery("SELECT .* FROM `expt_item_result_run_log`").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "item_id", "status", "result_state"}).AddRow(11, 3, 1, 103, 10, int32(entity.ItemRunState_Fail), int32(entity.ExptItemResultStateLogged)))
		refs := json.Jsonify(&entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 401, Alias: "judge-a", RecordID: recordID}}})
		mock.ExpectQuery("SELECT .* FROM `expt_turn_result_run_log`").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "item_id", "turn_id", "status", "target_result_id", "evaluator_result_ids"}).AddRow(12, 3, 1, 103, 10, 20, int32(entity.TurnRunState_Fail), targetID, []byte(refs)))
		mock.ExpectQuery("SELECT .* FROM `expt_turn_result`").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "item_id", "turn_id", "target_result_id", "weighted_score"}).AddRow(21, 3, 1, 103, 10, 20, targetID, score))
		output := json.Jsonify(&entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(score)}})
		mock.ExpectQuery("SELECT .* FROM `evaluator_record`").WillReturnRows(sqlmock.NewRows([]string{"id", "evaluator_version_id", "alias", "status", "score", "output_data"}).AddRow(recordID, 401, "judge-a", int32(entity.EvaluatorRunStatusSuccess), score, []byte(output)))
	}

	// Unmarked reads see a Logged item but stale turn payload and score on the replica.
	expectReads(replica, 201, 301, 0.25)
	item, err := itemRepo.GetItemRunLog(ctx, 1, 103, 10, 3)
	require.NoError(t, err)
	require.Equal(t, int32(entity.ExptItemResultStateLogged), item.ResultState)
	logs, err := turnRepo.GetItemTurnRunLogs(ctx, 1, 103, 10, 3)
	require.NoError(t, err)
	require.Equal(t, int64(201), logs[0].TargetResultID)
	turns, err := itemRepo.GetItemTurnResults(ctx, 3, 1, 10)
	require.NoError(t, err)
	require.Equal(t, gptr.Of(0.25), turns[0].WeightedScore)
	records, err := recordService.BatchGetEvaluatorRecord(ctx, []int64{301}, false, false)
	require.NoError(t, err)
	require.Equal(t, gptr.Of(0.25), records[0].EvaluatorOutputData.EvaluatorResult.Score)
	require.NoError(t, replica.ExpectationsWereMet())

	expectReads(source, 202, 302, 0.8)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{701}, nil)
	applied := false
	projection := projectionReadTurnRepo{IExptTurnResultRepo: turnRepo, apply: func(turns []*entity.ExptTurnResult, refs []*entity.ExptTurnEvaluatorResultRef) {
		applied = true
		require.Len(t, turns, 1)
		require.Equal(t, int64(202), turns[0].TargetResultID)
		require.Equal(t, gptr.Of(0.8), turns[0].WeightedScore)
		require.Len(t, refs, 1)
		require.Equal(t, int64(302), refs[0].EvaluatorResultID)
		require.Equal(t, "judge-a", refs[0].Alias)
	}}
	expt := buildRetryFailureSingleSetExpt(3, 401)
	svc := &ExptResultServiceImpl{ExptItemResultRepo: itemRepo, ExptTurnResultRepo: projection, evaluatorRecordService: recordService, idgen: idgen, scoreCalculator: NewEvaluatorScoreCalculator(nil, nil)}
	_, err = svc.RecordItemRunLogs(ctx, 1, 103, 10, 3, expt)
	require.NoError(t, err)
	require.True(t, applied)
	require.NoError(t, source.ExpectationsWereMet())
	require.NoError(t, replica.ExpectationsWereMet())
}

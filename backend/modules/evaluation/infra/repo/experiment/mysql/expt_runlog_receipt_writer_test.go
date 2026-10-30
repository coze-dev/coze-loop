// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type appendedRunItems struct{}

func (appendedRunItems) Match(value driver.Value) bool {
	bytes, ok := value.([]byte)
	if !ok {
		return false
	}
	var items []entity.ExptRunLogItems
	if json.Unmarshal(bytes, &items) != nil || len(items) != 3 {
		return false
	}
	return len(items[0].ItemIDs) == 1 && items[0].ItemIDs[0] == 11 && items[1].ManualRetryReceipt != nil &&
		items[1].ManualRetryReceipt.RequestID == "rq1" && len(items[2].ItemIDs) == 1 && items[2].ItemIDs[0] == 12 &&
		json.Valid(bytes) && items[1].ManualRetryReceipt.PayloadHash == "hash"
}

func TestAppendRunItemIDsLocksCurrentJSON(t *testing.T) {
	provider, mock := newManualRetrySQLProvider(t)
	dao, ok := any(NewExptRunLogDAO(provider)).(interface {
		AppendRunItemIDs(context.Context, int64, int64, int64, []int64) error
	})
	require.True(t, ok, "existing run append must lock and read current JSON")
	raw := `[{"ItemIDs":[11],"CreateAt":1700000000},{"ItemIDs":null,"CreateAt":1700000001,"manual_retry_receipt":{"request_id":"rq1","payload_hash":"hash","accepted_count":1,"rejected_items":[],"snapshot_at_ms":1700000000123},"future":{"id":9223372036854775806}}]`
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `expt_run_log` WHERE \\(space_id = \\? AND expt_id = \\? AND expt_run_id = \\?\\).*FOR UPDATE").
		WithArgs(int64(9), int64(7), int64(8), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "item_ids"}).AddRow(8, 9, 7, 8, []byte(raw)))
	mock.ExpectExec("UPDATE `expt_run_log` SET `item_ids`=\\?,`updated_at`=\\? WHERE .*`space_id` = \\?.*`expt_id` = \\?.*`expt_run_id` = \\?").
		WithArgs(appendedRunItems{}, sqlmock.AnyArg(), int64(9), int64(7), int64(8)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, dao.AppendRunItemIDs(context.Background(), 7, 8, 9, []int64{12}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAppendRunItemIDsRejectsCorruptOrDuplicate(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		ids       []int64
	}{
		{"corrupt", `{`, []int64{12}}, {"duplicate", `[{"ItemIDs":[11]}]`, []int64{11}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider, mock := newManualRetrySQLProvider(t)
			dao, ok := any(NewExptRunLogDAO(provider)).(interface {
				AppendRunItemIDs(context.Context, int64, int64, int64, []int64) error
			})
			require.True(t, ok)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "item_ids"}).AddRow(8, 9, 7, 8, []byte(tc.raw)))
			mock.ExpectRollback()
			require.Error(t, dao.AppendRunItemIDs(context.Background(), 7, 8, 9, tc.ids))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRunLogSavePreservesCurrentReceipt(t *testing.T) {
	provider, mock := newManualRetrySQLProvider(t)
	mock.ExpectExec("UPDATE `expt_run_log` SET `credit_cost`=\\?,`fail_cnt`=\\?,`pending_cnt`=\\?,`processing_cnt`=\\?,`status`=\\?,`status_message`=\\?,`success_cnt`=\\?,`terminated_cnt`=\\?,`token_cost`=\\?,`updated_at`=\\? WHERE .*`space_id` = \\?.*`expt_id` = \\?.*`expt_run_id` = \\?").
		WithArgs(float64(1), int32(0), int32(0), int32(0), int64(13), nil, int32(1), int32(0), nil, sqlmock.AnyArg(), int64(9), int64(7), int64(8)).WillReturnError(errors.New("write failed"))
	err := NewExptRunLogDAO(provider).Save(context.Background(), &model.ExptRunLog{ID: 8, SpaceID: 9, ExptID: 7, ExptRunID: 8, CreditCost: 1, SuccessCnt: 1, Status: gptr.Of(int64(13)), ItemIds: gptr.Of([]byte(`[]`))})
	require.ErrorContains(t, err, "write failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRunLogSaveAndAppendInterleaveKeepsMetadata(t *testing.T) {
	ctx := context.Background()
	provider := db.NewTestDB(t, &model.ExptRunLog{})
	require.NotNil(t, provider)
	dao := NewExptRunLogDAO(provider)
	row := &model.ExptRunLog{ID: 8, SpaceID: 9, ExptID: 7, ExptRunID: 8, Status: gptr.Of(int64(3)), ItemIds: gptr.Of([]byte(`[{"ItemIDs":[11]}]`))}
	require.NoError(t, dao.Create(ctx, row))
	stale, err := dao.Get(ctx, 7, 8)
	require.NoError(t, err)
	raw := `[{"ItemIDs":[11],"CreateAt":null},{"ItemIDs":null,"CreateAt":1700000001,"manual_retry_receipt":{"request_id":"rq1","payload_hash":"hash","accepted_count":1,"rejected_items":[],"snapshot_at_ms":1700000000123},"future":{"id":9223372036854775806}}]`
	require.NoError(t, dao.Update(ctx, 7, 8, 9, map[string]any{"item_ids": []byte(raw)}))
	stale.Status = gptr.Of(int64(13))
	require.NoError(t, dao.Save(ctx, stale))
	got, err := dao.Get(ctx, 7, 8)
	require.NoError(t, err)
	require.JSONEq(t, raw, string(*got.ItemIds))
	appendDAO, ok := any(dao).(interface {
		AppendRunItemIDs(context.Context, int64, int64, int64, []int64) error
	})
	require.True(t, ok)
	require.NoError(t, appendDAO.AppendRunItemIDs(ctx, 7, 8, 9, []int64{12}))
	require.NoError(t, dao.Save(ctx, stale))
	got, err = dao.Get(ctx, 7, 8)
	require.NoError(t, err)
	var items []entity.ExptRunLogItems
	require.NoError(t, json.Unmarshal(*got.ItemIds, &items))
	require.Equal(t, []int64{11, 12}, (&entity.ExptRunLog{ItemIds: items}).GetItemIDs())
	require.Equal(t, "rq1", items[1].ManualRetryReceipt.RequestID)
	var fields []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(*got.ItemIds, &fields))
	require.JSONEq(t, `{"id":9223372036854775806}`, string(fields[1]["future"]))
}

func TestUpdateRunStatsLocksControlRowsAndRollsBackOnError(t *testing.T) {
	provider, mock := newManualRetrySQLProvider(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `experiment` WHERE .*`id` = \\?.*FOR UPDATE").WithArgs(int64(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "latest_run_id", "status"}).AddRow(7, 9, 8, int32(entity.ExptStatus_Processing)))
	mock.ExpectQuery("SELECT .* FROM `expt_run_log` WHERE \\(id = \\? AND expt_id = \\? AND expt_run_id = \\?\\).*FOR UPDATE").WithArgs(int64(8), int64(7), int64(8), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "status"}).AddRow(8, 9, 7, 8, int64(entity.ExptStatus_Processing)))
	mock.ExpectExec("UPDATE `expt_stats` SET .*`fail_cnt`=\\?.*`pending_cnt`=\\?.*`processing_cnt`=\\?.*`success_cnt`=\\?.*`terminated_cnt`=\\?.*WHERE .*`expt_id` = \\?.*`space_id` = \\?").
		WithArgs(int32(0), int32(0), int32(0), int32(1), int32(0), sqlmock.AnyArg(), int64(7), int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `expt_run_log` SET `status`=\\?,`success_cnt`=\\? WHERE .*`space_id` = \\?.*`expt_id` = \\?.*`expt_run_id` = \\?").
		WithArgs(int64(entity.ExptStatus_Processing), int32(1), int64(9), int64(7), int64(8)).WillReturnError(errors.New("run update failed"))
	mock.ExpectRollback()
	err := NewExptRunLogDAO(provider).UpdateRunStats(context.Background(), 7, 8, 9,
		map[string]any{"status": int64(entity.ExptStatus_Processing), "success_cnt": int32(1)}, &model.ExptStats{SuccessCnt: 1})
	require.ErrorContains(t, err, "run update failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

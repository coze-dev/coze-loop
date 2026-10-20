// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestResultPublishMySQLReuseRefsAndReceipt(t *testing.T) {
	f := newCurrentResultFixture(t, "rq1")
	ctx := context.Background()
	raw := []byte(`[{"ItemIDs":[9223372036854775806],"CreateAt":1700000000,"manual_retry_receipt":{"request_id":"rq1","payload_hash":"original","accepted_count":1,"rejected_items":null,"snapshot_at_ms":1}}]`)
	require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("item_ids", raw).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.ItemRunState_Fail), "result_state": int32(entity.ExptItemResultStateLogged)}).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Updates(map[string]any{"status": int32(entity.TurnRunState_Fail), "evaluator_result_ids": []byte(`null`)}).Error)
	refs, _ := json.Marshal(&entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: f.base + 51, Alias: "alias", RecordID: f.base + 61}}})
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND turn_id=0", f.space).Update("evaluator_result_ids", refs).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnEvaluatorResultRef{}).Where("space_id=?", f.space).Update("source_type", int32(entity.EvaluatorRecordSourceTypeBuiltin)).Error)
	port := NewExptResultPublishDAO(f.provider)
	source, err := port.GetItemResultPublishSource(ctx, f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	publication := &entity.ExptItemResultPublication{Source: source, Refs: source.Source.CurrentRefs}
	for _, old := range source.Source.CanonicalTurns {
		turn := *old
		log := source.Source.Turns[old.TurnID]
		turn.Status = int32(log.Status)
		turn.TargetResultID = log.TargetResultID
		turn.TraceID = log.TraceID
		turn.LogID = log.LogID
		turn.ErrMsg = log.ErrMsg
		turn.WeightedScore = nil
		publication.Turns = append(publication.Turns, &turn)
	}
	before := f.snapshot()
	ok, err := port.CommitItemResult(ctx, publication)
	require.NoError(t, err)
	require.True(t, ok)
	after := f.snapshot()
	require.Equal(t, before["expt_turn_evaluator_result_ref"], after["expt_turn_evaluator_result_ref"])
	require.Equal(t, before["expt_run_log"], after["expt_run_log"])
	ok, err = port.CommitItemResult(ctx, publication)
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, after, f.snapshot())
}

func TestResultPublishMySQLCanonicalLockAgainstAdmission(t *testing.T) {
	f := newCurrentResultFixture(t, "rq1")
	ctx := context.Background()
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.ItemRunState_Fail), "result_state": int32(entity.ExptItemResultStateLogged)}).Error)
	port := NewExptResultPublishDAO(f.provider)
	source, err := port.GetItemResultPublishSource(ctx, f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	publication := &entity.ExptItemResultPublication{Source: source}
	blocked := make(chan struct{})
	release := make(chan struct{})
	admissionDone := make(chan error, 1)
	go func() {
		admissionDone <- f.provider.Transaction(ctx, func(tx *gorm.DB) error {
			var row model.ExptItemResult
			if err := tx.Raw("SELECT * FROM expt_item_result WHERE space_id=? AND expt_id=? AND item_id=? FOR UPDATE", f.space, f.expt, f.item).Scan(&row).Error; err != nil {
				return err
			}
			require.IsType(t, &sql.Tx{}, tx.Statement.ConnPool)
			ext := map[string]string{}
			inner, _ := json.Marshal(&entity.ManualRetryContext{RunID: f.run, RequestID: "rq2", Mode: entity.EvaluationModeFailRetry, Operator: "test", AcceptedAtMS: 1})
			ext[entity.ManualRetryExtKey] = string(inner)
			raw, _ := json.Marshal(ext)
			if err := tx.Model(&row).Update("ext", raw).Error; err != nil {
				return err
			}
			close(blocked)
			<-release
			return nil
		})
	}()
	<-blocked
	result := make(chan error, 1)
	go func() {
		ok, err := port.CommitItemResult(ctx, publication)
		if ok {
			err = errors.New("old publication wrote after admission")
		}
		result <- err
	}()
	select {
	case err := <-result:
		t.Fatalf("publication ignored canonical row lock: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-admissionDone)
	require.NoError(t, <-result)
	var log model.ExptItemResultRunLog
	require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item).Take(&log).Error)
	require.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(log.ResultState))
}
